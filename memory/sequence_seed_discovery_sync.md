Seed → Discovery → Sync (Draft Sequence)

Narrative
1) Startup
- App loads config (npub, seed relays, scope, kinds, policies). Compute config hash for caches.

2) Bootstrap from Seeds
- Subscribe on seed relays for owner's latest kinds 0, 3, 10002.
- Ingest profile (0) and contacts (3); store and index.
- Parse 10002 (NIP-65) to build active read/write relay hints for owner.

3) Build Author Set
- Use scope policy (self/following/mutual/FOAF depth) from contacts (3).
- Apply allow/deny lists and caps; always include direct mentions and threads of owner's posts if configured.

4) Discover Relays for Authors
- For each author in scope, try to fetch 10002 from known read relays for that author.
- If unknown, fall back to seeds for that author’s 10002.
- Persist relay_hints(pubkey, relay, read/write, freshness).

5) Subscribe and Sync
- Subscribe to discovered read relays with filters:
  - Kinds: configured set (e.g., 0,1,3,6,7,9735,30023) plus 10002 as needed.
  - Authors: owner + in-scope authors (with caps), split into batches per relay.
  - Since cursors per relay/kind to avoid duplicates.
- Ingest events; update events/refs tables and aggregates; cache renders and section results.

6) Serve and Refresh
- Serve pages using cached sections and renders; SSE streams interactions live.
- Periodically refresh replaceable kinds (0/3/10002) for owner and authors.
- On 10002 changes, adjust active connections (add/drop relays) without restart.
- If hints stale or missing, temporarily read from seeds and schedule rediscovery.

ASCII Sequence (simplified)

Operator       App                     Seed Relays                Active Read Relays          Authors
   |           |                            |                              |                     |
   |  config   |                            |                              |                     |
   |---------> |                            |                              |                     |
   |           | SUB owner 0,3,10002        |                              |                     |
   |           |--------------------------->|                              |                     |
   |           |    EVENT 0/3/10002         |                              |                     |
   |           |<---------------------------|                              |                     |
   |           | Parse 10002; store hints   |                              |                     |
   |           | Build author set (scope)   |                              |                     |
   |           | For each author:           |                              |                     |
   |           |  try 10002 on known relays |--------------->(if known)---->|                     |
   |           |  else 10002 from seeds     |------------------------------>|                     |
   |           |           EVENT 10002      |<------------------------------|                     |
   |           | Persist hints per author   |                              |                     |
   |           | SUB kinds for authors      |                              |<--------------------|
   |           | with since cursors         |                              |    EVENTS…          |
   |           | Ingest -> DB, aggregates   |                              |                     |
   |           | Serve pages; open SSE      |                              |                     |
   |   view    |<---------------------------|                              |                     |
   |           | Periodic refresh (0/3/10002) and adjust connections on change                  |

Mermaid (alternative)

```mermaid
sequenceDiagram
  autonumber
  participant Operator
  participant App as nophr App
  participant Seeds as Seed Relays
  participant Relays as Active Read Relays

  Operator->>App: Load config (npub, seeds, scope)
  App->>Seeds: SUB owner kinds 0,3,10002
  Seeds-->>App: EVENT 0/3/10002
  App->>App: Parse 10002; persist relay_hints(owner)
  App->>Seeds: SUB owner kind 3 (contacts)
  Seeds-->>App: EVENT 3 (follows)
  App->>App: Build author set (scope policy)
  loop Each author
    App->>Relays: SUB 10002 via known read relays
    alt No hints yet
      App->>Seeds: SUB 10002 via seeds
    end
    Relays-->>App: EVENT 10002 (hints)
    App->>App: Persist relay_hints(author)
  end
  App->>Relays: SUB replaceables [0,3,10002,30023] (no since)
  App->>Relays: SUB non-replaceables [1,6,7,9735] with since cursors
  Relays-->>App: EVENTS…
  App->>App: Ingest -> DB; update refs/aggregates; cache
  App-->>Operator: Serve pages; SSE streams interactions
  App->>Seeds: Periodic refresh of replaceables (0/3/10002)
  App->>Relays: Adjust connections on 10002 changes
```

Notes
- Discovery is ongoing: hints may change; connections update without restart.
- Seeds are only for bootstrap and fallback; active reads come from authors' hints where possible.
- Replaceable kinds (0/3/10002/30023) are refreshed regardless of since cursors.

Error Paths & Backoff (Draft)

- Seeds unreachable
  - Try next seed; exponential backoff per relay (policy.backoff_ms); continue serving from existing data.
  - Surface status on diagnostics page; suggest operator to update seeds.
- Owner 10002 missing or stale
  - Read owner data from seeds; schedule rediscovery at discovery.refresh_seconds; log warning when freshness > 30d.
- Author 10002 missing
  - Fallback to seeds for that author; mark author as pending; stagger retries to avoid thundering herd.
- Read relay refuses SUB / rate-limits
  - Mark relay unhealthy; backoff and rotate to next hint; respect max_relays_per_author.
- Event storm / overload
  - Apply backpressure: reduce authors per filter, increase since window granularity, temporarily pause interactions ingestion if CPU-bound.
- Clock skew detected
  - Since > now + skew_threshold; clamp since to now-guard; warn operator to sync system time.
- Replaceable churn (flapping hints)
  - Debounce 10002 changes: require stability window before reconnect; keep previous connections warm until switch.
- SSE blocked by proxy
  - Detect via missing heartbeat; advise enabling streaming (disable buffering) or fall back to polling.

Mermaid error branches (simplified)

```mermaid
sequenceDiagram
  participant App as nophr App
  participant Seeds as Seed Relays
  participant Relays as Active Read Relays

  App->>Seeds: SUB owner 0/3/10002
  alt Seeds unreachable
    App->>App: Backoff; try next seed; serve from cache/DB
  else Seeds ok
    Seeds-->>App: EVENT 0/3/10002
    App->>App: Parse 10002
  end

  App->>Relays: SUB authors with hints
  alt Relay refuses or throttles
    App->>App: Mark unhealthy; rotate/backoff; use alternative relays or seeds
  else Relay ok
    Relays-->>App: EVENTS…
  end

  App->>App: Periodic refresh 0/3/10002
  alt Hints changed rapidly
    App->>App: Debounce switch-over; keep old connections until stable
  end
```
