# aside-ctx

Content-addressed context store for AI conversations.

---

## The short version

A Go sidecar that watches AI session logs, finds large tool outputs (>1KB), stores them by SHA-256 hash, and replaces the inline text with a tiny handle reference. On the next turn, the AI sees a 187-byte handle instead of 45KB of terminal spew. Same content always produces the same handle — deduplication is free.

~65% reduction in prompt overhead. Content-addressed. Append-only. Zero dependencies.

---

## Why this exists

There is a scene in Michael Lewis's *The Big Short* where a hedge fund manager named Steve Eisman realizes that the entire subprime mortgage market is built on a lie. He doesn't discover this by reading the fine print on a single mortgage. He discovers it by looking at the data in aggregate — by noticing that the default rate on mortgages with no documentation is *identical* to the default rate on mortgages with full documentation. Which means the documentation is theater. The system is carrying dead weight it pretends is structural.

I had my own Steve Eisman moment, except instead of mortgage-backed securities, I was looking at AI conversations.

Every time I talk to an AI agent — Claude, GPT, whatever — the model has to re-read every tool output I've ever produced in that conversation. Every JSON blob. Every terminal dump. Every file I asked it to read three weeks ago and never looked at again. I measured it. 71.2% of every conversation was stale output. The AI was spending most of its attention budget reading things nobody would ever need again.

This is not a bug. It is a design choice baked into how chat-based agents work. The conversation transcript is the only memory. Everything stays, forever, or until context collapse forces a summary that loses the signal with the noise.

The fix is embarrassingly simple. You don't need to carry the full text. You carry a fingerprint — a SHA-256 hash — and only fetch the original when you actually need to read it. Same content always produces the same fingerprint, so if the same terminal output appears in three different conversations, it's stored once.

I built it in Go because this is a systems problem, not a data science problem. A sidecar. No dependencies. One HTTP API on :9876. An optimizer watches session files, finds blobs over 1KB, stores them, replaces the inline text with a handle. The AI knows how to resolve handles because I put the instructions in its system prompt.

The result: ~65% less prompt overhead. The AI spends less time re-reading yesterday's garbage and more time solving today's problems. A 60KB memory file compresses to 13KB. A 45KB terminal dump becomes a 187-byte reference.

Content-addressed storage is not new. Git uses it. IPFS uses it. What's new is applying it to the thing that costs the most in AI: context.

I called it aside-ctx because it lives alongside the AI, not inside it. A sidecar for context.

---

## How it works

```
┌──────────┐    session logs    ┌────────────┐   POST /store   ┌──────────┐
│ AI Agent ├───────────────────►│ Optimizer  ├────────────────►│ aside-ctx│
│ (Claude) │                    │ (watcher)  │                 │ (:9876)  │
└──────────┘                    └────────────┘                 └────┬─────┘
      ▲                                                            │
      │              ┌──────────────────────┐                      │
      └──────────────┤ [stored in aside-ctx] │◄────────────────────┘
                     │ handle=sha256:abc...  │   GET /resolve/{h}
                     │ digest="build OK"     │
                     │ size=45721            │
                     └──────────────────────┘
```

The optimizer polls session JSONL files every 10 seconds. When it finds a tool result larger than 1KB, it sends the content to aside-ctx's `/store` endpoint. The daemon computes SHA-256, checks if it already exists (dedup), stores the blob, and returns the handle. The optimizer rewrites the session file — the 45KB of terminal output becomes a single line:

```
[stored in aside-ctx] tool=bash handle=sha256:a1b2c3... digest="build succeeded" size=45721
```

On the next conversation turn, the AI sees the handle. If it needs the full content — say, to debug a build failure — it resolves it via `curl http://127.0.0.1:9876/resolve/sha256:a1b2c3...`. If it doesn't need it, it moves on. The context window stays lean.

---

## Project structure

```
aside-ctx/
├── main.go                 HTTP server on :9876
├── store.go                SHA-256 flat file store + JSONL index
├── handlers.go             POST /store, GET /resolve/{handle}, GET /stats, GET /health
├── digest.go               Digest generation
├── cmd/optimizer/main.go   Session log watcher (10s poll, >1KB threshold)
├── cmd/memory-indexer/main.go  MEMORY.md compression tool
├── *._test.go              Test suite (store, handlers, main)
├── INTEGRATION.md          Integration spec
└── ARCHITECTURE.html       Visual architecture diagram
```

---

## Quick start

```bash
# Start the daemon
./aside-ctx --port 9876 --store ~/.aside-ctx

# Store content
curl -s -X POST http://127.0.0.1:9876/store -d '{"content": "..."}'
# → {"handle":"sha256:abc...","digest":"...","size":12345,"exists":false}

# Resolve a handle
curl -s http://127.0.0.1:9876/resolve/sha256:abc...
# → {"handle":"sha256:abc...","content":"...","size":12345}

# Stats
curl -s http://127.0.0.1:9876/stats
# → {"entries":970,"total_bytes":4976558}
```

---

## Use it as a library

```go
import "github.com/gammaton588/aside-ctx"

store, _ := aside_ctx.NewStore("/path/to/store")
handle, _ := store.Put([]byte("your content"))
data, _ := store.Get(handle)
```

---

## Why Go

This is a systems problem: file I/O, HTTP, process management, tight loops. Go gives us goroutines for concurrent session watching, a standard library that handles HTTP without frameworks, and a single binary with no runtime dependencies. The optimizer, the daemon, the memory indexer — all one `go build` away.

---

## The numbers

| Metric | Before | After |
|--------|--------|-------|
| Prompt overhead | 100% (baseline) | ~35% |
| 45KB terminal output | 45,721 bytes inline | 187 bytes (handle) |
| 60KB memory file | 60,408 bytes | 13,409 bytes (index) |
| Dedup | None | Automatic (SHA-256) |
| Retrieval | Always loaded | On demand |

---

## License

MIT
