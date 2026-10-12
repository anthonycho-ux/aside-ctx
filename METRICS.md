# aside-ctx Metrics

## Compression Performance

| Metric | Before | After | Savings |
|--------|--------|-------|---------|
| Prompt overhead | 100% (baseline) | ~35% | ~65% |
| 45KB terminal output | 45,721 bytes inline | 187 bytes (handle) | 99.6% |
| 60KB memory file | 60,408 bytes | 13,409 bytes (index) | 77.8% |
| Dedup | None | Automatic (SHA-256) | Free |
| Retrieval | Always loaded | On demand | Variable |

## Pipeline

```
AI Agent session logs
       |
       v
Optimizer (poll every 10s, threshold >1KB)
       |
       v
POST /store endpoint
       |
       v
SHA-256 hash computation
       |
       v
Dedup check (has this hash been stored before?)
       |
       +-- Already exists -> return existing handle (no storage cost)
       |
       +-- New content -> store to flat file + JSONL index
       |
       v
Return handle: sha256:a1b2c3...
              
Session file rewritten:
  [stored in aside-ctx] tool=bash handle=sha256:a1b2c3... digest="build succeeded" size=45721

On next turn, AI either:
  - Moves on (handle is enough context)
  - Resolves via GET /resolve/{handle} if full content needed
```

## Storage

- **Engine:** SHA-256 flat file store + JSONL index
- **API endpoints:** POST /store, GET /resolve/{handle}, GET /stats, GET /health
- **Port:** :9876
- **Dependencies:** None (single Go binary)

## Stats Sample

```json
{"entries":970,"total_bytes":4976558}
```
