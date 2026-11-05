package grpcx

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/url"

	"go.opentelemetry.io/otel"
	"golang.org/x/oauth2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/credentials/oauth"
	oteltracing "google.golang.org/grpc/experimental/opentelemetry"
	"google.golang.org/grpc/stats/opentelemetry"
)

// CredentialsFunc builds per-RPC credentials for a target URL.
type CredentialsFunc func(ctx context.Context, target string) (credentials.PerRPCCredentials, error)

// Option configures Dial.
type Option func(*dialConfig)

type dialConfig struct {
	creds    CredentialsFunc
	dialOpts []grpc.DialOption
}

// WithCredentials attaches per-RPC credentials built from the target URL.
func WithCredentials(fn CredentialsFunc) Option {
	return func(c *dialConfig) { c.creds = fn }
}

// WithPerRPC attaches fixed per-RPC credentials.
func WithPerRPC(creds credentials.PerRPCCredentials) Option {
	return WithCredentials(func(context.Context, string) (credentials.PerRPCCredentials, error) { return creds, nil })
}

// WithTokenSource sends "authorization: Bearer <token>" from ts, e.g. an OAuth2 client-credentials source.
func WithTokenSource(ts oauth2.TokenSource) Option {
	return WithPerRPC(oauth.TokenSource{TokenSource: ts})
}

// WithBearer sends a static "authorization: Bearer <token>", e.g. a shared secret from config.
func WithBearer(token string) Option {
	return WithTokenSource(oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token}))
}

// WithDialOptions appends raw gRPC dial options.
func WithDialOptions(opts ...grpc.DialOption) Option {
	return func(c *dialConfig) { c.dialOpts = append(c.dialOpts, opts...) }
}

// Dial creates a client connection to another service, instrumented with OpenTelemetry.
//
// A "https://host[:port]" target connects over TLS (port 443 by default) and attaches the configured credentials.
// A "[http://]host:port" target connects without TLS and never sends credentials.
func Dial(ctx context.Context, target string, opts ...Option) (*grpc.ClientConn, error) {
	var cfg dialConfig
	for _, o := range opts {
		o(&cfg)
	}

	dialOpts := append([]grpc.DialOption{
		opentelemetry.DialOption(opentelemetry.Options{
			MetricsOptions: opentelemetry.MetricsOptions{
				MeterProvider: otel.GetMeterProvider(),
			},
			TraceOptions: oteltracing.TraceOptions{
				TracerProvider:    otel.GetTracerProvider(),
				TextMapPropagator: otel.GetTextMapPropagator(),
			},
		}),
	}, cfg.dialOpts...)

	u, err := url.Parse(target)
	if err != nil || u.Scheme != "https" {
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))
		return grpc.NewClient(target, dialOpts...)
	}

	dialOpts = append(dialOpts,
		grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12})))
	if cfg.creds != nil {
		creds, err := cfg.creds(ctx, target)
		if err != nil {
			return nil, fmt.Errorf("credentials for %s: %w", target, err)
		}
		dialOpts = append(dialOpts, grpc.WithPerRPCCredentials(creds))
	}

	port := u.Port()
	if port == "" {
		port = "443"
	}
	return grpc.NewClient(u.Hostname()+":"+port, dialOpts...)
}
