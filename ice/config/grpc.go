package config

import "time"

type GrpcServerConfig struct {
	// Addr is host:port passed to net/http directly, i.e. :8080 means listen to all requests to port 8080
	Addr string `yaml:"addr"`
	// Secure specifies if SSL should be used
	Secure bool `yaml:"secure"`
	// Cert is path of ssl cert generated by openssl
	Cert string `yaml:"cert"`
	// Key is path of ssl key generated by openssl
	Key string `yaml:"key"`
	// EnableTracing decides if tracing is enabled on grpc server
	EnableTracing bool `yaml:"enableTracing"`
	// ShutdownDuration for graceful shutdown grpc server
	ShutdownDuration time.Duration `yaml:"shutdownDuration"`
}
