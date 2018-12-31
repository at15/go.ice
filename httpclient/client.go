package httpclient

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"github.com/dyweb/gommon/errors"
	"github.com/dyweb/gommon/util/httputil"
)

type Client struct {
	// configured by user
	base string
	// json means both request and response are talking in json
	json bool
	// headers are base headers send out in every request
	headers    map[string]string
	errHandler ErrorHandler

	// h is the underlying http.Client
	h *http.Client
}

// TODO: allow using config struct
func New(base string, opts ...Option) (*Client, error) {
	base = strings.TrimSpace(base)
	if base == "" {
		return nil, errors.New("base path is empty")
	}
	var c *Client
	switch {
	case strings.HasPrefix(base, "http"):
		u, err := url.Parse(base)
		if err != nil {
			return nil, errors.Wrap(err, "error parse http(s) url")
		}
		c = &Client{
			base: u.String(),
			h:    httputil.NewUnPooledClient(),
		}
	case strings.HasPrefix(base, "unix") || strings.HasPrefix(base, "/"):
		// TODO: might use url.Parse to handle unix:// better?
		sock := strings.TrimPrefix(base, "unix://")
		c = &Client{
			base: UnixBasePath,
			h:    httputil.NewClient(httputil.NewUnPooledUnixTransport(sock)),
		}
	default:
		return nil, errors.Errorf("unknown protocol, didn't find http or unix in %s", base)
	}
	c.errHandler = DefaultHandler()
	return c, applyOptions(c, opts...)
}

func (c *Client) Get(ctx *Context, path string, resBody interface{}) error {
	return c.FetchTo(ctx, httputil.Get, path, nil, resBody)
}

func (c *Client) GetRaw(ctx *Context, path string) (*http.Response, error) {
	return c.Do(ctx, httputil.Get, path, nil)
}

func (c *Client) GetIgnoreRes(ctx *Context, path string) error {
	return c.FetchToNull(ctx, httputil.Get, path, nil)
}

func (c *Client) Post(ctx *Context, path string, reqBody interface{}, resBody interface{}) error {
	return c.FetchTo(ctx, httputil.Post, path, reqBody, resBody)
}

func (c *Client) PostIgnoreRes(ctx *Context, path string, reqBody interface{}) error {
	return c.FetchToNull(ctx, httputil.Post, path, reqBody)
}

func (c *Client) FetchTo(ctx *Context, method httputil.Method, path string, reqBody interface{}, resBody interface{}) error {
	if !c.json {
		return errors.New("only json encoding is support for FetchTo")
	}
	if reqBody == resBody {
		return errors.New("request body and response body are same interface{}, typo?")
	}
	if resBody == nil {
		return errors.New("response body can't be nil")
	}

	res, err := c.Do(ctx, method, path, reqBody)
	if err != nil {
		return err
	}
	b, err := DrainResponseBody(res)
	if err != nil {
		return err
	}
	if err := json.NewDecoder(bytes.NewReader(b)).Decode(resBody); err != nil {
		// TODO: might add error wrapping to provide deeper stack
		return &ErrDecoding{
			Codec: "json",
			Err:   err,
			Body:  string(b),
		}
	}
	return nil
}

func (c *Client) FetchToNull(ctx *Context, method httputil.Method, path string, reqBody interface{}) error {
	res, err := c.Do(ctx, method, path, reqBody)
	if err != nil {
		return err
	}
	if _, err := io.Copy(ioutil.Discard, res.Body); err != nil {
		return errors.Wrap(err, "error copy response body to /dev/null")
	}
	if err := res.Body.Close(); err != nil {
		return errors.Wrap(err, "error close response body after drain to /dev/null")
	}
	return nil
}

// Do handles application level error, default is based on status code and drain entire body as string.
// User can give a handler when create client or have request specific handler using context
//
// It does not drain response body, use FetchTo if want to decode body as json
func (c *Client) Do(ctx *Context, method httputil.Method, path string, reqBody interface{}) (*http.Response, error) {
	req, err := c.NewRequest(ctx, method, path, reqBody)
	if err != nil {
		return nil, err
	}
	res, err := c.h.Do(req)
	// network error
	if err != nil {
		return nil, err
	}

	// handle application error
	errHandler := c.errHandler
	// request specific error handle override using context
	if ctx.errHandler != nil {
		errHandler = ctx.errHandler
	}
	if errHandler.IsError(res.StatusCode, res) {
		b, err := DrainResponseBody(res)
		if err != nil {
			return res, err
		}
		return res, errHandler.DecodeError(res.StatusCode, b, res)
	}
	return res, nil
}

// NewRequest create a http.Request by concat base path in client and path,
// it will encode the body if the body is not already encoded and it's a json client
//
// You should use high level wrapper like FetchTo most of the time
func (c *Client) NewRequest(ctx *Context, method httputil.Method, path string, reqBody interface{}) (*http.Request, error) {
	if c == nil || c.h == nil {
		return nil, errors.New("client is not initialized")
	}

	var (
		encodedBody io.Reader
		err         error
	)
	if reqBody != nil {
		if encodedBody, err = encodeBody(reqBody, c.json); err != nil {
			return nil, err
		}
	}
	u := JoinPath(c.base, path)
	req, err := http.NewRequest(string(method), u, encodedBody)
	if err != nil {
		return nil, errors.Wrap(err, "error create http request")
	}
	if len(c.headers) > 0 {
		for k, v := range c.headers {
			req.Header.Set(k, v)
		}
	}
	if len(ctx.headers) > 0 {
		for k, v := range ctx.headers {
			req.Header.Set(k, v)
		}
	}
	if len(ctx.params) > 0 {
		q := req.URL.Query()
		for k, v := range ctx.params {
			q.Set(k, v)
		}
		req.URL.RawQuery = q.Encode()
	}
	req = req.WithContext(ctx)
	return req, nil
}

func encodeBody(body interface{}, encodeToJson bool) (io.Reader, error) {
	if _, ok := body.(io.Reader); ok {
		return body.(io.Reader), nil
	}
	switch body.(type) {
	case io.Reader:
		return body.(io.Reader), nil
	case []byte:
		return bytes.NewReader(body.([]byte)), nil
	case string:
		return strings.NewReader(body.(string)), nil
	}
	if !encodeToJson {
		return nil, errors.New("request body must be io.Reader/bytes/string or set the client to auto encode request to json")
	}
	buf := bytes.Buffer{}
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		return nil, errors.Wrap(err, "error encode request body to json")
	}
	return &buf, nil
}

// TODO: add a client method to return underlying client
// Transport returns the transport of the http.Client being used for user to modify tls config etc.
// It returns (nil, false) if the transport is the default transport or the type didn't match.
// You should NOT use http.DefaultTransport in the first place, and modify it is even worse
func (c *Client) Transport() (tr *http.Transport, ok bool) {
	// avoid nil ptr
	if c == nil {
		return
	}
	// get the transport from http.Client
	tr, ok = c.h.Transport.(*http.Transport)
	if !ok {
		return
	}
	// it's default, you should not modify it
	// TODO: might add our own un exported transports
	if tr == http.DefaultTransport {
		return nil, false
	}
	return
}
