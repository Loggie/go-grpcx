// Package gcpauth provides Google Cloud credentials for grpcx.Dial.
package gcpauth

import (
	"context"

	"google.golang.org/api/idtoken"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/oauth"

	"github.com/loggie/go-grpcx"
)

// IDToken attaches a Google ID token whose audience is the target URL.
func IDToken() grpcx.Option {
	return grpcx.WithCredentials(func(ctx context.Context, target string) (credentials.PerRPCCredentials, error) {
		ts, err := idtoken.NewTokenSource(ctx, target)
		if err != nil {
			return nil, err
		}
		return oauth.TokenSource{TokenSource: ts}, nil
	})
}
