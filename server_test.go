package grpcx_test

import (
	"context"
	"net"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/test/bufconn"

	"github.com/loggie/go-grpcx"
)

// It swaps the global tracer provider, so it doesn't run in parallel with other tests.
func TestNewServerInterceptors(t *testing.T) { //nolint:paralleltest // sets the global tracer provider
	rec := tracetest.NewSpanRecorder()
	otel.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec)))
	t.Cleanup(func() { otel.SetTracerProvider(noop.NewTracerProvider()) })

	// The extra stream interceptor runs after SpanStreamInterceptor, so rpc.method is already on the span.
	var sawMethod bool
	extra := func(srv any, ss grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		sawMethod = hasAttribute(trace.SpanFromContext(ss.Context()), "rpc.method")
		return handler(srv, ss)
	}
	var unaryCalls int
	unary := func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		unaryCalls++
		return handler(ctx, req)
	}

	lis := bufconn.Listen(1 << 20)
	srv := grpcx.NewServer(grpcx.WithUnaryInterceptors(unary), grpcx.WithStreamInterceptors(extra))
	srv.SetServing()
	go func() { _ = srv.Serve(lis) }()
	defer srv.Stop()

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	client := healthpb.NewHealthClient(conn)

	if _, err = client.Check(context.Background(), &healthpb.HealthCheckRequest{}); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	watch, err := client.Watch(ctx, &healthpb.HealthCheckRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = watch.Recv(); err != nil {
		t.Fatal(err)
	}
	cancel()

	spans := waitForSpans(rec, 2)
	if unaryCalls != 1 || !sawMethod {
		t.Errorf("unary interceptor calls = %d, stream interceptor saw rpc.method = %v", unaryCalls, sawMethod)
	}
	want := map[string]string{
		"grpc.health.v1.Health/Check": "OK",
		"grpc.health.v1.Health/Watch": "CANCELLED",
	}
	for _, s := range spans {
		attrs := map[attribute.Key]attribute.Value{}
		for _, kv := range s.Attributes() {
			attrs[kv.Key] = kv.Value
		}
		method := attrs["rpc.method"].AsString()
		code, ok := want[method]
		if !ok {
			t.Errorf("unexpected span %q with rpc.method %q", s.Name(), method)
			continue
		}
		if attrs["rpc.system.name"].AsString() != "grpc" || attrs["rpc.response.status_code"].AsString() != code {
			t.Errorf("%s: attributes %v", method, s.Attributes())
		}
		delete(want, method)
	}
	if len(want) > 0 {
		t.Errorf("missing spans for %v", want)
	}
}

func hasAttribute(span trace.Span, key attribute.Key) bool {
	ro, ok := span.(sdktrace.ReadOnlySpan)
	if !ok {
		return false
	}
	for _, kv := range ro.Attributes() {
		if kv.Key == key {
			return true
		}
	}
	return false
}

// waitForSpans waits until n spans have ended; servers end spans after the response is sent.
func waitForSpans(rec *tracetest.SpanRecorder, n int) []sdktrace.ReadOnlySpan {
	for range 200 {
		if len(rec.Ended()) >= n {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	return rec.Ended()
}
