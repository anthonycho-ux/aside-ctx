# aside-ctx

Content-addressed blob store for tool results, stdlib-only Go module (`go 1.25`, package main).

## Build / test

- `go build -o aside-ctx .`
- `go test -race ./...`
- `go vet ./...`
- `gofmt -l .` should print nothing (`gofmt` lives at `$(go env GOROOT)/bin/gofmt` — not on PATH here).

## Store root semantics

- `NewStore(basePath)` treats `basePath` as the store root: blobs land at `<basePath>/store/<hex>`, the index at `<basePath>/index.jsonl`.
- Server default root is `~/.aside-ctx` (`--store` flag overrides; `~`/`~/` expanded by the binary).
- Handles are `sha256:<64 lowercase hex>`; `Resolve` rejects anything else (traversal-safe).
- Server binds 127.0.0.1 only, `--port` default 9876. Routes: POST /store, GET /resolve/{handle}, GET /stats, GET /health.
