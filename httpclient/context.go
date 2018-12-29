package httpclient

import (
	"context"
	"time"
)

var _ context.Context = (*Context)(nil)
// initTime is just a dummy time so we can return a garbage value when time.Time is required
var initTime = time.Now()

// Context
//
// It is lazy initialized, only call `make` when they are actually write to,
// so all the maps are EMPTY even when using factory func.
// User (including this package itself) should use setter when set value.
type Context struct {
	// headers is request specific headers, headers configured in client will be override
	headers map[string]string
	// params is the query parameters attached to url, i.e. query?name=foo&type=bar
	params  map[string]string

	// values improve performance by set value in place
	// TODO: do I really need this map?
	values map[string]interface{}
	stdCtx context.Context
}

// Bkg returns a Context that does not embed context.Context,
// it behaves like context.Background(), however we can't use a singleton like context package
// because Context in httpclient can be modified in place to store req/response body etc.
// So we always return pointer to a fresh new instance.
//
// We return pointer because it is meant to be modified along the way, it is not immutable like context.Context
// Also the context.Context is implemented using pointer receiver
func Bkg() *Context {
	return &Context{}
}

// NewContext returns a context that embed a context.Context
func NewContext(ctx context.Context) *Context {
	return &Context{
		stdCtx: ctx,
	}
}

// Deadline returns Deadline() from underlying context.Context if set
func (c *Context) Deadline() (deadline time.Time, ok bool) {
	if c != nil && c.stdCtx != nil {
		return c.stdCtx.Deadline()
	}
	// NOTE: we are using named return, so empty value will be returned
	// learned this from context.Context's emptyCtx implementation
	return
}

// Done returns Done() from underlying context.Context if set
func (c *Context) Done() <-chan struct{} {
	if c != nil && c.stdCtx != nil {
		return c.stdCtx.Done()
	}
	// Done may return nil if this context can never be canceled
	return nil
}

// Err returns Err() from underlying context.Context if set
func (c *Context) Err() error {
	if c != nil && c.stdCtx != nil {
		return c.stdCtx.Err()
	}
	return nil
}

// Value first checks the map[string]interface{},
// if not found, it use the underlying context.Context if is set
// if not set, it returns nil
func (c *Context) Value(key interface{}) interface{} {
	if c != nil && c.values != nil {
		k, ok := key.(string)
		if ok {
			v, ok := c.values[k]
			if ok {
				return v
			}
		}
	}
	if c != nil && c.stdCtx != nil {
		return c.stdCtx.Value(key)
	}
	return nil
}