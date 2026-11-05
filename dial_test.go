package grpcx

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc/credentials"
)

func TestDialCredentialsOnlyOverTLS(t *testing.T) {
	var gotTarget string
	calls := 0
	opt := WithCredentials(func(_ context.Context, target string) (credentials.PerRPCCredentials, error) {
		calls++
		gotTarget = target
		return nil, errors.New("boom")
	})

	conn, err := Dial(context.Background(), "localhost:8080", opt)
	if err != nil {
		t.Fatalf("plaintext dial: %v", err)
	}
	_ = conn.Close()
	if calls != 0 {
		t.Fatalf("credentials built for plaintext target")
	}

	if _, err := Dial(context.Background(), "https://svc.example.com", opt); err == nil {
		t.Fatal("want credentials error for https target")
	}
	if gotTarget != "https://svc.example.com" {
		t.Fatalf("target = %q", gotTarget)
	}
}

func TestDialHTTPSNoCredentials(t *testing.T) {
	for _, target := range []string{"https://svc.example.com", "https://svc.example.com:8443"} {
		conn, err := Dial(context.Background(), target)
		if err != nil {
			t.Fatalf("%s: %v", target, err)
		}
		_ = conn.Close()
	}
}
