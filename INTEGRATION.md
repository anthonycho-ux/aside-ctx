Goal: Integrate aside-ctx into aside's tool pipeline — tool results stored content-addressed,
prompt carries handles+digests instead of raw blobs, deref tool resolves handles on demand.

Done when:
1. aside-ctx daemon runs as a persistent launchd service on 127.0.0.1:9876 ✓ (started)
2. Tool result post-processing stores blobs in aside-ctx, returns handles
3. System prompt carries handles+digests instead of raw tool output
4. A deref tool exists in aside's manifest (POST /resolve POST /store api)
5. Verified with a real aside session: 71.2% tool output share measurably reduced to digests

Architecture:
- aside is a precompiled Go binary (Mach-O 64-bit at ~/.aside/cli/Aside CLI.app/Contents/MacOS/aside)
- aside writes tool results (tools/out/) after each turn
- The integration works at the agent wrapper / tool-pipeline level:
  a) A wrapper intercepts tool results after execution
  b) POST content to aside-ctx (/store) → get sha256 handle + digest
  c) Replace raw content in the system prompt with handle + short digest
  d) A deref tool in aside's manifest allows: deref(handle) → GET /resolve/{handle} → full content
  e) Stale observation supersession: tool results for the same resource key replace the prior handle

Implementation approach:
1. Start and verify aside-ctx daemon (done — running on :9876)
2. Build the wrapper agent (Go or shell) that:
   - Watches for tool execution events
   - Stores large tool results in aside-ctx
   - Modifies the observation format to {op, resource, handle, digest}
3. Register the deref tool in aside's tool manifest
4. Test with a real aside session, measure delta

Key files:
- aside-ctx: ~/Projects/magpie-autoresearch/research/aside-ctx/
- aside CLI: ~/.aside/cli/Aside CLI.app/
- tool output dir: ~/.aside/u/0/tools/out/
- state DB: ~/.aside/u/0/state.db
