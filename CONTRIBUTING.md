# Contributing

Ferro Operator Console is a standalone Go module. Keep the runtime boundary
with AI Gateway HTTP-only; do not import packages from the gateway module.

Before opening a change, run:

```bash
./scripts/check-module-boundary.sh
go vet ./...
go test -race ./...
golangci-lint run ./...
```

For contract changes, run `FERRO_GATEWAY_SOURCE=/path/to/ai-gateway make itest`.
