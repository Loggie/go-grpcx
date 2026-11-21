// Package grpcx provides a gRPC server and client preconfigured with OpenTelemetry.
package grpcx

import (
	"context"
	"strconv"
	"strings"

	"go.opentelemetry.io/otel"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	oteltracing "google.golang.org/grpc/experimental/opentelemetry"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/stats/opentelemetry"
	"google.golang.org/grpc/status"
)

// Server is a grpc.Server with OpenTelemetry.
type Server struct {
	*grpc.Server

	health *health.Server
}

// ServerOption configures NewServer.
type ServerOption func(*serverConfig)

type serverConfig struct {
	unary  []grpc.UnaryServerInterceptor
	stream []grpc.StreamServerInterceptor
}

// WithUnaryInterceptors appends unary interceptors.
func WithUnaryInterceptors(interceptors ...grpc.UnaryServerInterceptor) ServerOption {
	return func(c *serverConfig) { c.unary = append(c.unary, interceptors...) }
}

// WithStreamInterceptors appends stream interceptors.
func WithStreamInterceptors(interceptors ...grpc.StreamServerInterceptor) ServerOption {
	return func(c *serverConfig) { c.stream = append(c.stream, interceptors...) }
}

// NewServer builds a Server.
func NewServer(opts ...ServerOption) *Server {
	var cfg serverConfig
	for _, o := range opts {
		o(&cfg)
	}

	server := grpc.NewServer(
		opentelemetry.ServerOption(opentelemetry.Options{
			MetricsOptions: opentelemetry.MetricsOptions{
				MeterProvider: otel.GetMeterProvider(),
			},
			TraceOptions: oteltracing.TraceOptions{
				TracerProvider:    otel.GetTracerProvider(),
				TextMapPropagator: otel.GetTextMapPropagator(),
			},
		}),
		grpc.ChainUnaryInterceptor(append([]grpc.UnaryServerInterceptor{SpanUnaryInterceptor()}, cfg.unary...)...),
		grpc.ChainStreamInterceptor(append([]grpc.StreamServerInterceptor{SpanStreamInterceptor()}, cfg.stream...)...),
	)

	healthServer := health.NewServer()
	healthpb.RegisterHealthServer(server, healthServer)
	reflection.Register(server)

	return &Server{Server: server, health: healthServer}
}

// SetServing marks the server healthy; call it after all services are registered.
func (s *Server) SetServing() {
	s.health.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
}

// GracefulStop reports NOT_SERVING, then waits for in-flight RPCs to finish.
func (s *Server) GracefulStop() {
	s.health.SetServingStatus("", healthpb.HealthCheckResponse_NOT_SERVING)
	s.Server.GracefulStop()
}

// SpanUnaryInterceptor adds RPC semantic-convention attributes (system, method, and
// status code) to the server's request span.
func SpanUnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		span := trace.SpanFromContext(ctx)
		span.SetAttributes(
			semconv.RPCSystemNameGRPC,
			semconv.RPCMethod(strings.TrimPrefix(info.FullMethod, "/")),
		)

		resp, err := handler(ctx, req)

		span.SetAttributes(
			semconv.RPCResponseStatusCodeKey.String(
				canonicalName(status.Code(err))))

		return resp, err
	}
}

// SpanStreamInterceptor adds RPC semantic-convention attributes (system, method, and
// status code) to the server's span for a streaming RPC.
func SpanStreamInterceptor() grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		span := trace.SpanFromContext(ss.Context())
		span.SetAttributes(
			semconv.RPCSystemNameGRPC,
			semconv.RPCMethod(strings.TrimPrefix(info.FullMethod, "/")),
		)

		err := handler(srv, ss)

		span.SetAttributes(
			semconv.RPCResponseStatusCodeKey.String(
				canonicalName(status.Code(err))))

		return err
	}
}

// canonicalName returns the canonical gRPC name of c, e.g. DEADLINE_EXCEEDED.
func canonicalName(c codes.Code) string {
	switch c {
	case codes.OK:
		return "OK"
	case codes.Canceled:
		return "CANCELLED"
	case codes.Unknown:
		return "UNKNOWN"
	case codes.InvalidArgument:
		return "INVALID_ARGUMENT"
	case codes.DeadlineExceeded:
		return "DEADLINE_EXCEEDED"
	case codes.NotFound:
		return "NOT_FOUND"
	case codes.AlreadyExists:
		return "ALREADY_EXISTS"
	case codes.PermissionDenied:
		return "PERMISSION_DENIED"
	case codes.ResourceExhausted:
		return "RESOURCE_EXHAUSTED"
	case codes.FailedPrecondition:
		return "FAILED_PRECONDITION"
	case codes.Aborted:
		return "ABORTED"
	case codes.OutOfRange:
		return "OUT_OF_RANGE"
	case codes.Unimplemented:
		return "UNIMPLEMENTED"
	case codes.Internal:
		return "INTERNAL"
	case codes.Unavailable:
		return "UNAVAILABLE"
	case codes.DataLoss:
		return "DATA_LOSS"
	case codes.Unauthenticated:
		return "UNAUTHENTICATED"
	default:
		return strconv.FormatUint(uint64(c), 10)
	}
}
