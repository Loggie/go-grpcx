# go-grpcx

gRPC server and client helpers with OpenTelemetry.

```go
s := grpcx.NewServer(grpcx.WithUnaryInterceptors(u...), grpcx.WithStreamInterceptors(st...))
examplepb.RegisterExampleServiceServer(s, impl)
s.SetServing()
go s.Serve(lis)
s.GracefulStop()

conn, err := grpcx.Dial(ctx, "localhost:8080")
conn, err := grpcx.Dial(ctx, "https://api.example.com")
conn, err := grpcx.Dial(ctx, "https://svc-xyz.a.run.app", gcpauth.IDToken())
conn, err := grpcx.Dial(ctx, "https://svc.example.com", grpcx.WithBearer(secret))
conn, err := grpcx.Dial(ctx, target, grpcx.WithTokenSource(ccConfig.TokenSource(ctx)))
```

Provider-specific auth lives in subpackages (`gcpauth`) and plugs in through `grpcx.WithCredentials`.
