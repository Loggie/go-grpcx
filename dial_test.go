package grpcx_test

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc/credentials"

	"github.com/loggie/go-grpcx"
)

func TestDialCredentialsOnlyOverTLS(t *testing.T) {
	t.Parallel()
	var gotTarget string
	calls := 0
	opt := grpcx.WithCredentials(func(_ context.Context, target string) (credentials.PerRPCCredentials, error) {
		calls++
		gotTarget = target
		return nil, errors.New("boom")
	})

	conn, err := grpcx.Dial(context.Background(), "localhost:8080", opt)
	if err != nil {
		t.Fatalf("plaintext dial: %v", err)
	}
	_ = conn.Close()
	if calls != 0 {
		t.Fatalf("credentials built for plaintext target")
	}

	if _, err = grpcx.Dial(context.Background(), "https://svc.example.com", opt); err == nil {
		t.Fatal("want credentials error for https target")
	}
	if gotTarget != "https://svc.example.com" {
		t.Fatalf("target = %q", gotTarget)
	}
}

func TestDialHTTPSNoCredentials(t *testing.T) {
	t.Parallel()
	for _, target := range []string{"https://svc.example.com", "https://svc.example.com:8443"} {
		conn, err := grpcx.Dial(context.Background(), target)
		if err != nil {
			t.Fatalf("%s: %v", target, err)
		}
		_ = conn.Close()
	}
}
