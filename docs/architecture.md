# Architecture Overview

Technical deep-dive for developers and contributors

Complete architectural overview of nophr's design, components, and implementation.

## Executive Summary

nophr is a **personal Nostr gateway** that serves content via legacy internet protocols (Gopher, Gemini, Finger). It acts as a bridge between the modern Nostr protocol and classic protocols from the 1980s-90s.

**Key design principles:**
1. **Config-first** - Everything customizable via YAML
2. **Single-tenant** - Optimized for one operator
3. **Embedded storage** - Uses Khatru as library, not separate service
4. **Protocol agnostic** - Serve same content via multiple protocols
5. **Pull model** - Sync from remote relays, serve locally

---

## High-Level Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Nostr Network                            │
│  (Remote relays: wss://relay.damus.io, etc.)               │
└─────────────────────────┬───────────────────────────────────┘
                          │
                          │ WebSocket subscriptions
                          │ (filtered by scope/kinds)
                          ↓
┌─────────────────────────────────────────────────────────────┐
│                   Sync Engine                               │
│  - Relay discovery (NIP-65)                                 │
│  - Social graph (follows/FOAF)                              │
│  - Cursor management                                        │
│  - Event ingestion                                          │
└─────────────────────────┬───────────────────────────────────┘
                          │
                          │ StoreEvent()
                          ↓
┌─────────────────────────────────────────────────────────────┐
│              Khatru (Embedded Nostr Relay)                  │
│  - Event storage & indexing                                 │
│  - Signature verification                                   │
│  - Replaceable event handling                               │
│  - NIP compliance (01, 10, 33, etc.)                        │
└─────────────────────────┬───────────────────────────────────┘
                          │
                          │ eventstore interface
                          ↓
┌─────────────────────────────────────────────────────────────┐
│           Database Backend (SQLite)                         │
│  - Events (managed by Khatru)                               │
│  - Custom tables:                                           │
│    • relay_hints                                            │
│    • graph_nodes                                            │
│    • sync_state                                             │
│    • aggregates                                             │
└─────────────────────────┬───────────────────────────────────┘
                          │
                          │ QueryEvents()
                          ↓
┌─────────────────────────────────────────────────────────────┐
│              Content Query & Aggregation                    │
│  - Section filters                                          │
│  - Thread resolution                                        │
│  - Interaction rollups                                      │
│  - Caching layer                                            │
└───────────┬─────────────┬──────────────┬────────────────────┘
            │             │              │
            ↓             ↓              ↓
┌─────────────┐  ┌──────────────┐  ┌─────────────┐
│   Gopher    │  │    Gemini    │  │   Finger    │
│  Renderer   │  │   Renderer   │  │  Renderer   │
│             │  │              │  │             │
│ Markdown→   │  │ Markdown→    │  │ Markdown→   │
│ Plain Text  │  │ Gemtext      │  │ Compact     │
└──────┬──────┘  └──────┬───────┘  └──────┬──────┘
       │                │                 │
       ↓                ↓                 ↓
┌──────────────────────────────────────────────────────────────┐
│                   Protocol Servers                           │
│  - Gopher: port 70 (RFC 1436)                                │
│  - Gemini: port 1965 (TLS/TOFU)                              │
│  - Finger: port 79 (RFC 742)                                 │
└──────────────────────────────────────────────────────────────┘
```

Note: This build supports SQLite for the database backend. LMDB is not supported in this build.

---

## Component Breakdown

### 1. Configuration System

**Location:** `internal/config/`

**Purpose:** Load, validate, and apply configuration.

**Key files:**
- `config.go` - Struct definitions, loader
- `config_test.go` - Validation tests
- `example.yaml` - Embedded example config

**Features:**
- YAML parsing with validation
- Environment variable overrides (`NOPHR_*`)
- Defaults for all options
- Secrets via env only (never in files)

**Configuration flow:**
```
config.yaml
    ↓
Load() → Unmarshal YAML
    ↓
applyEnvOverrides() → Apply NOPHR_* env vars
    ↓
Validate() → Check required fields, formats
    ↓
*Config → Pass to components
```

 

---

### 2. Storage Layer

**Location:** `internal/storage/`

**Purpose:** Persist events and nophr-specific data.

**Architecture:**
```
┌─────────────────────────┐
│  Storage Interface      │
│  (storage.go)           │
└───────┬─────────────────┘
        │
        ↓
┌──────┐
│SQLite│
└──────┘
    ↓
┌──────────────┐
│   Khatru     │
│ (eventstore) │
└──────────────┘
```

**Key files:**
- `storage.go` - Interface, factory
- `sqlite.go` - SQLite implementation
- `lmdb.go` - LMDB implementation stub (returns not-implemented error in this build)
- `migrations.go` - Schema creation
- `relay_hints.go`, `graph_nodes.go`, `sync_state.go`, `aggregates.go` - Custom tables

**What Khatru handles:**
- Event storage (events table)
- Querying (Nostr filters)
- Signature verification
- Replaceable events logic

**What nophr adds:**
- relay_hints (NIP-65 data)
- graph_nodes (social graph cache)
- sync_state (cursors per relay/kind)
- aggregates (interaction rollups)

 

---

### 3. Nostr Client

**Location:** `internal/nostr/`

**Purpose:** Connect to remote Nostr relays.

**Key files:**
- `client.go` - WebSocket client pool
- `relay.go` - Per-relay connection management
- `discovery.go` - NIP-65 relay discovery
- `relay_hints.go` - Relay hint parsing

**Connection pool:**
```
┌───────────────────────────┐
│   Relay Connection Pool   │
│                           │
│  ┌─────┐  ┌─────┐        │
│  │Relay│  │Relay│  ...   │
│  │  1  │  │  2  │        │
│  └─────┘  └─────┘        │
└───────────────────────────┘
```

**Per-relay features:**
- WebSocket connection
- Subscription management
- Backoff/retry logic
- Health tracking

 

---

### 4. Sync Engine

**Location:** `internal/sync/`

**Purpose:** Pull events from remote relays, store locally.

**Key files:**
- `engine.go` - Main sync orchestration
- `filters.go` - Build Nostr filters from scope
- `graph.go` - Social graph computation
- `cursors.go` - Cursor tracking
- `scope.go` - Scope enforcement (self/following/mutual/foaf)

**Sync flow:**
```
1. Discovery
   └→ Fetch kind 10002 from seeds
   └→ Parse relay hints
   └→ Build relay pool

2. Graph Computation
   └→ Fetch kind 3 (contacts)
   └→ Compute depth, mutual
   └→ Populate graph_nodes

3. Filter Building
   └→ Per-kind filters
   └→ Per-author filters (from graph)
   └→ Apply scope modifiers

4. Subscription
   └→ Subscribe to each relay with filters
   └→ Use cursors (since timestamps)

5. Ingestion
   └→ Receive events via WebSocket
   └→ Validate & store (Khatru)
   └→ Update cursors
   └→ Trigger aggregates
```

 

---

### 5. Aggregates

**Location:** `internal/aggregates/`

**Purpose:** Count interactions (replies, reactions, zaps).

**Key files:**
- `aggregates.go` - Main aggregates manager
- `threading.go` - NIP-10 thread resolution
- `reactions.go` - Reaction counting
- `zaps.go` - Zap parsing and sum
- `reconciler.go` - Periodic recount
- `queries.go` - Helper queries

**Aggregate computation:**
```
Event (kind 1)
    ↓
Find referencing events:
  - kind 1 with #e → replies
  - kind 7 with #e → reactions
  - kind 9735 with #e → zaps
    ↓
Count and sum
    ↓
Store in aggregates table
```

**Update strategies:**
1. **On ingestion** - Update immediately when new interaction arrives
2. **Reconciler** - Periodic full recount (detect drift)

 

---

### 6. Markdown Conversion

**Location:** `internal/markdown/`

**Purpose:** Convert Nostr content (often markdown) to protocol-specific formats.

**Key files:**
- `parser.go` - Markdown AST parsing
- `gopher.go` - Markdown → plain text
- `gemini.go` - Markdown → gemtext
- `finger.go` - Markdown → stripped compact

**Conversion pipeline:**
```
Markdown content
    ↓
Parse to AST
    ↓
┌───────┬────────┬─────────┐
│       │        │         │
↓       ↓        ↓         ↓
Gopher  Gemini  Finger  (other)
(plain) (gemtext)(strip)
```

**Gopher transformations:**
- Headings → UPPERCASE or underline
- Bold → UPPERCASE or **keep**
- Links → `text <url>`
- Code → indent or separators
- Wrap at 70 chars

**Gemini transformations:**
- Headings → `# ## ###`
- Links → `=> url text` (separate lines)
- Lists → `* item`
- Code → `` ``` ... ``` ``
- Wrap at 80 chars (optional)

**Finger transformations:**
- Strip all markdown syntax
- Preserve bare URLs optionally
- Truncate to ~500 chars

 

---

### 7. Protocol Servers

**Location:** `internal/gopher/`, `internal/gemini/`, `internal/finger/`

#### Gopher Server

**Files:**
- `server.go` - TCP listener, connection handler
- `router.go` - Selector routing
- `gophermap.go` - Menu generation
- `renderer.go` - Event rendering

**Request flow:**
```
Client connects to port 70
    ↓
Send selector (e.g., "/notes")
    ↓
Router matches selector
    ↓
Query events from storage
    ↓
Render gophermap or text
    ↓
Send response
    ↓
Close connection
```

 

#### Gemini Server

**Files:**
- `server.go` - TLS listener, connection handler
- `router.go` - URL routing
- `renderer.go` - Gemtext rendering
- `protocol.go` - Gemini protocol helpers
- `tls.go` - TLS cert management

**Request flow:**
```
Client connects to port 1965 (TLS)
    ↓
Send URL (e.g., "gemini://host/notes")
    ↓
Router matches path
    ↓
Query events from storage
    ↓
Render gemtext
    ↓
Send response with status code
    ↓
Close connection
```

 

#### Finger Server

**Files:**
- `server.go` - TCP listener, connection handler
- `handler.go` - Query parsing, user lookup
- `renderer.go` - Finger response formatting

**Request flow:**
```
Client connects to port 79
    ↓
Send query (e.g., "npub1abc@host")
    ↓
Parse username
    ↓
Query profile (kind 0) + recent notes
    ↓
Format finger response
    ↓
Send response
    ↓
Close connection
```

 

---

### 8. Caching Layer

**Location:** `internal/cache/`

**Purpose:** Cache rendered responses, dramatically improve performance, reduce database load.

**Architecture:**
```
┌─────────────────────────┐
│   Cache Interface       │
│   (cache.go)            │
└───────┬─────────────────┘
        │
    ┌───┴───┐
    ↓       ↓
┌──────┐ ┌──────┐
│Memory│ │Redis │
└──────┘ └──────┘
```

**Key files:**
- `cache.go` - Cache interface, factory, NullCache
- `memory.go` - In-memory cache with LRU eviction
- `redis.go` - Redis cache implementation
- `keys.go` - Hierarchical key generation, pattern matching
- `invalidator.go` - Event-based cache invalidation, warming utilities
- `cache_test.go`, `keys_test.go` - Test coverage

**Features:**
- **Memory Cache**: Thread-safe, LRU eviction, automatic cleanup of expired entries
- **Redis Cache**: Distributed caching, persistent across restarts, clustering support
- **Statistics**: Hits, misses, evictions, hit rate, timing metrics
- **Automatic Invalidation**: Based on event kind (profile, notes, reactions, zaps)
- **Pattern Matching**: Bulk operations with wildcard patterns (`gopher:*`, `event:123:*`)
- **Cache Warming**: Pre-populate frequently accessed pages

**Cache keys:**
```
gopher:/path/to/selector        - Gopher response
gemini:/path?query=test         - Gemini response
finger:username                 - Finger response
event:event123:gopher:text      - Event rendering
section:notes:gemini:p2         - Section page
thread:root123:gopher           - Thread rendering
profile:pubkey123:gemini        - Profile page
aggregate:event123              - Interaction counts
```

**TTL strategy:**
- Short (10-60s): Live content (inbox, interactions)
- Medium (300-600s): Sections, menus
- Long (hours/days): Immutable (old events, profiles)

**Invalidation triggers:**
- Kind 0 (Profile): Invalidates profile cache, kind0 cache
- Kind 1 (Note): Invalidates notes section
- Kind 3 (Contacts): Invalidates kind3 cache
- Kind 7 (Reaction): Invalidates parent event aggregates
- Kind 9735 (Zap): Invalidates parent event aggregates
- Manual: Configuration changes, server restart

**Performance impact:**
- Response time: 10-100x faster for cached responses
- Database load: 80-95% reduction in queries
- CPU usage: 50-70% reduction for rendering
- Throughput: 5-10x increase in requests/second

 

---

### 9. Sections and Layouts

**Location:** `internal/sections/`

**Purpose:** Organize and present events through configurable sections with filtering, pagination, and archive generation.

**Architecture:**
```
┌─────────────────────────┐
│   Section Manager       │
│   (sections.go)         │
└───────┬─────────────────┘
        │
    ┌───┴───────────┐
    ↓               ↓
┌────────────┐  ┌────────────┐
│  Filters   │  │  Archives  │
│ (filters.go)│  │(archives.go)│
└────────────┘  └────────────┘
```

**Key files:**
- `sections.go` - Section definitions, manager, page composition
- `filters.go` - Filter query builder, time range helpers
- `archives.go` - Archive generation (day/month/year)
- `sections_test.go`, `filters_test.go` - Test coverage

**Features:**
- **Section Definition**: Title, description, filters, sorting, pagination
- **URL Path Mapping**: Sections can be mapped to specific URL paths (e.g., `/diy`, `/philosophy`)
- **Multiple Sections per Path**: Multiple sections can share the same path (ordered display)
- **Filtering**: By kinds, authors, tags, time ranges, scope (self/following/mutual/foaf)
- **Sorting**: By created_at, reactions, zaps, replies (asc/desc)
- **Pagination**: Automatic pagination with page numbers, totals, navigation
- **Grouping**: By day, week, month, year, author, kind
- **Archives**: Time-based archives (monthly/yearly) with event counts
- **Calendar Views**: Monthly calendar showing days with events
- **"More" Links**: Sections can include links to full paginated views

**Section structure:**
```go
type Section struct {
    Name        string     // Internal identifier
    Path        string     // URL path (e.g., "/diy", "/")
    Title       string     // Display title
    Description string     // Description
    Filters     FilterSet  // Event filters
    Order       int        // Display order (when multiple sections share path)
    MoreLink    *MoreLink  // Optional link to full paginated view
    // ... sorting, pagination, grouping options
}
```

**Custom sections examples:**
```yaml
sections:
  - name: diy-preview
    path: /               # Homepage
    title: "Latest DIY"
    filters:
      tags:
        t: ["diy"]
    limit: 5
    order: 0
    more_link:
      text: "More DIY posts"
      section_ref: "diy-full"

  - name: diy-full
    path: /diy            # Dedicated page
    title: "DIY Projects"
    filters:
      tags:
        t: ["diy"]
    limit: 20
```

**Note:** The `inbox` and `outbox` concepts are DEPRECATED. The router provides `/notes`, `/replies`, `/mentions`, `/articles` as built-in endpoints. Sections are for custom filtered views (e.g., `/art`, `/dev`, `/following`).

**Filter builder:**
```go
filter := sections.NewFilterBuilder().
    Kinds(1, 30023).
    Authors("pubkey1", "pubkey2").
    Since(sections.LastNDays(7).Start).
    Limit(20).
    Build()
```

**Time range helpers:**
- `Today()`, `Yesterday()`, `ThisWeek()`, `ThisMonth()`, `ThisYear()`
- `LastNDays(n)`, `LastNHours(n)`

**Archive structure:**
```
/archive/notes              - List of monthly archives
/archive/notes/2025/10      - October 2025 notes
/archive/notes/2025/10/24   - October 24, 2025 notes
```

**Page composition:**
```go
page, _ := manager.GetPage(ctx, "notes", 1)
// page.Events       - Events on this page
// page.PageNumber   - Current page
// page.TotalPages   - Total pages
// page.HasNext      - Has next page
// page.HasPrev      - Has previous page
```

**Integration:**
- Protocol servers use sections for navigation
- Each section can be cached independently
- Archives can be pre-generated and cached
- Supports custom sections via configuration

 

---

### 10. Security

**Location:** `internal/security/`

**Purpose:** Defense-in-depth security with deny lists, rate limiting, input validation, content filtering, and secret management.

**Architecture:**
```
┌─────────────────────────┐
│   Security Manager      │
└───────┬─────────────────┘
        │
    ┌───┴───────────────────┐
    ↓           ↓           ↓
┌──────────┐ ┌──────────┐ ┌──────────┐
│DenyList  │ │RateLimit │ │Validator │
└──────────┘ └──────────┘ └──────────┘
    ↓           ↓           ↓
┌──────────────────────────┐
│   Content Filter         │
└──────────────────────────┘
    ↓
┌──────────────────────────┐
│   Secret Manager         │
└──────────────────────────┘
```

**Key files:**
- `denylist.go` - Pubkey and content blocking
- `ratelimit.go` - Token bucket rate limiting
- `validation.go` - Input validation and sanitization
- `secrets.go` - Secure secret handling
- `security_test.go` - Comprehensive tests

**Features:**
- **Deny List**: Block specific pubkeys, thread-safe, dynamic add/remove
- **Content Filter**: Banned words filtering, case-sensitive/insensitive
- **Rate Limiting**: Token bucket algorithm, per-client, per-protocol limits
- **Input Validation**: CRLF/null byte/traversal protection, XSS prevention
- **Secret Management**: Environment-only loading, memory-only storage, automatic redaction
- **Combined Filtering**: Deny list + content filter integration

**Defense layers:**
```
Request → Rate Limiter → Input Validator → Deny List → Content Filter → Process
```

**Security protections:**
- **CRLF injection**: `\r\n` removed from all inputs
- **Null byte injection**: `\x00` removed
- **Directory traversal**: `..` sequences blocked
- **XSS attacks**: Script tags sanitized
- **DoS attacks**: Rate limiting per client
- **Abuse prevention**: Deny list for bad actors
- **Content moderation**: Banned words filtering

**Rate limiting algorithm:**
```
Token Bucket:
- Each client gets N tokens (burst_size)
- Each request consumes 1 token
- Tokens refill at rate R (requests_per_minute / 60 per second)
- When bucket empty, requests denied until refill
- Old buckets auto-cleaned up after inactivity
```

**Secret management:**
```go
// Secrets never touch disk
sm := security.NewSecretManager()
nsec, _ := sm.LoadNsecFromEnv()  // NOPHR_NSEC only

// Automatic redaction
ss := security.NewSecureString("secret123")
fmt.Println(ss.String())  // "secr...e123" (redacted)
actual := ss.Get()         // "secret123" (actual value)

// Safe logging
logger := security.NewSafeLogger()
safe := logger.SanitizeMessage(msg)  // Redacts any secrets
```

**Integration:**
- All protocol servers use validator for input
- Rate limiter middleware applied per-protocol
- Deny list filters events before rendering
- Content filter applied after deny list
- Secret manager handles nsec loading

**Performance:**
- Thread-safe with minimal lock contention
- RWMutex for concurrent reads (deny list)
- Per-client buckets for rate limiting
- Automatic cleanup of stale data
- Cache-friendly validation

 

---

### 11. Search

**Location:** `internal/search/`

**Purpose:** NIP-50 compliant search functionality with content matching, relevance ranking, and profile metadata parsing.

**Architecture:**
```
┌─────────────────────────┐
│   NIP-50 Engine         │
│   (nip50.go)            │
└───────┬─────────────────┘
        │
        ↓
┌─────────────────────────┐
│   Storage Search        │
│ (storage/search.go)     │
└───────┬─────────────────┘
        │
    ┌───┴───────────┐
    ↓               ↓
┌──────────┐  ┌──────────┐
│  Match   │  │   Rank   │
│ Events   │  │   By     │
│          │  │Relevance │
└──────────┘  └──────────┘
```

**Key files:**
- `nip50.go` - NIP-50 search engine, search options, query parsing
- `storage/search.go` - QueryEventsWithSearch implementation, relevance scoring
- `nostr/profile.go` - Profile metadata parsing (kind 0), display name/lightning helpers

**Features:**
- **NIP-50 Compliance**: Standard Nostr search protocol implementation
- **Content Matching**: Search in event content field (case-insensitive)
- **Relevance Ranking**: Score-based sorting with multiple signals
- **Profile Search**: Enhanced kind 0 (profile) event searching
- **Search Options**: Configurable kinds, authors, limits, time ranges
- **Query Parsing**: Advanced query syntax (`kind:1 bitcoin`)

**Search flow:**
```
1. User Query
   └→ Parse search text and options
   └→ Build Nostr filter with Search field

2. NIP-50 Engine
   └→ Apply search options (kinds, authors, limits)
   └→ Call relay QuerySync with filter

3. Storage Layer
   └→ Query all matching events
   └→ Filter by search term (content matching)
   └→ Calculate relevance scores

4. Relevance Ranking
   └→ Exact match: +100 points
   └→ Contains phrase: +50 points
   └→ Word matches: +10 per word
   └→ Shorter content: +5 (focused)
   └→ Profile (kind 0): +10 bonus

5. Results
   └→ Sort by score (descending)
   └→ Apply limit after sorting
   └→ Return ranked results
```

**Relevance scoring:**
```go
// Example scores
"bitcoin" searching for "bitcoin"           → 100 (exact match)
"Understanding bitcoin mining" → "bitcoin"   → 60 (phrase + word + short)
"BTC and bitcoin explained" → "bitcoin"     → 60 (phrase + word)
Profile with "bitcoin" in about             → 70 (phrase + word + profile)
```

**Search options:**
```go
results, err := engine.Search(ctx, "nostr protocol",
    search.WithKinds(1, 30023),      // Notes and articles only
    search.WithAuthors(pubkey1),      // Specific authors
    search.WithLimit(20),             // Max 20 results
    search.WithSince(timestamp),      // Recent events
)
```

**Query parsing:**
```go
// Advanced syntax
"kind:1 bitcoin"           → Search notes for "bitcoin"
"kind:30023 nostr"         → Search articles for "nostr"
"protocol"                 → Search all kinds for "protocol"
```

**Profile metadata:**
```go
// Kind 0 profile parsing
profile := nostr.ParseProfile(event)
name := profile.GetDisplayName()         // display_name or name
lightning := profile.GetLightningAddress() // lud16 or lud06
```

**Integration:**
- **Gopher**: `/search/<query>` with URL encoding (+ for spaces)
- **Gemini**: `/search` with input prompt (status 10) for query entry
- **Protocol Servers**: Use QueryEventsWithSearch for search endpoints
- **Caching**: Search results cacheable with TTL

**Performance:**
- Content matching: O(n) with early filtering
- Relevance scoring: O(n log n) for sorting (bubble sort for small n)
- Profile parsing: O(1) JSON unmarshaling
- Typical search: <100ms for 10K events

 

---

### 12. Entity Resolution (NIP-19)

**Location:** `internal/entities/`

**Purpose:** Parse, resolve, and format NIP-19 entities (npub, nprofile, note, nevent, naddr) found in content.

**Architecture:**
```
┌─────────────────────────┐
│   Entity Resolver       │
│   (resolver.go)         │
└───────┬─────────────────┘
        │
    ┌───┴─────────┐
    ↓             ↓
┌──────────┐  ┌──────────┐
│  Parse   │  │  Format  │
│ NIP-19   │  │ (protocol│
│ Entities │  │ specific)│
└──────────┘  └──────────┘
        │
        ↓
┌─────────────────────────┐
│   Storage Lookup        │
│ (profiles, events)      │
└─────────────────────────┘
```

**Key files:**
- `resolver.go` - NIP-19 entity detection, decoding, resolution
- `formatters.go` - Protocol-specific entity formatters (Gopher, Gemini, plain text)

**Features:**
- **Entity Detection**: Regex-based detection of `nostr:` URIs in content
- **NIP-19 Decoding**: Supports npub, nprofile, note, nevent, naddr
- **Name Resolution**: Fetches display names from kind 0 profiles
- **Title Resolution**: Extracts titles from notes and articles
- **Protocol Formatters**: Different output formats per protocol
- **Inline Replacement**: Replace entities in text with resolved forms

**Supported entity types:**
```
npub1...     → Profile (hex pubkey)
nprofile1... → Profile with relay hints
note1...     → Event (hex event ID)
nevent1...   → Event with relay hints and context
naddr1...    → Parameterized replaceable event (kind:pubkey:d-tag)
```

**Resolution flow:**
```
1. Text Scanning
   └→ Regex finds "nostr:npub1..."
   └→ Extract entity strings

2. NIP-19 Decoding
   └→ Decode prefix and payload
   └→ Extract pubkey/event ID/coordinates

3. Storage Lookup
   └→ Query kind 0 for profiles (display name)
   └→ Query event for notes (title/preview)
   └→ Query with d-tag for naddr (title)

4. Entity Object
   └→ Type, DisplayName, Link, OriginalText
   └→ Ready for formatting

5. Protocol Formatting
   └→ Gopher: "@DisplayName"
   └→ Gemini: "[DisplayName](link)"
   └→ Plain: "DisplayName"
```

**Example resolution:**
```go
// Input text
"Check out nostr:npub1abc... and nostr:note1xyz..."

// After resolution
"Check out @alice and Short note preview..."
```

**Display name priority:**
```
Kind 0 profile:
  1. display_name (highest priority)
  2. name
  3. nip05
  4. truncated pubkey (fallback)
```

**Link generation:**
```
npub/nprofile  → /profile/{hex_pubkey}
note/nevent    → /note/{hex_event_id}
naddr          → /addr/{kind}/{pubkey}/{d-tag}
```

**Protocol-specific formatters:**
- **GopherFormatter**: `@{DisplayName}` (no inline links)
- **GeminiFormatter**: `[{DisplayName}](gemini://HOST{Link})`
- **PlainTextFormatter**: `{DisplayName}` (just the name)
- **MarkdownFormatter**: `[{DisplayName}]({Link})`
- **HTMLFormatter**: `<a href="{Link}">{DisplayName}</a>`

**Integration:**
- Gopher renderer uses `resolver.ReplaceEntities()` before markdown conversion
- Gemini renderer uses `resolver.ReplaceEntities()` before rendering
- Content with `nostr:` URIs automatically shows human-readable references
- Storage lookups can be cached

**Performance:**
- Regex matching: O(n) with compiled pattern
- Entity resolution: O(1) per entity (storage lookup)
- Batch resolution: Parallelizable for multiple entities
- Typical overhead: <10ms for 10 entities

**Graceful degradation:**
- Missing profiles: Falls back to truncated pubkey
- Missing events: Shows "Note abc123..." or "Event abc123..."
- Invalid NIP-19: Keeps original `nostr:` URI unchanged
- Decode errors: Preserves original text

 

---

## Code Organization

```
nophr/
├── cmd/nophr/              # Main application entry point
│   └── main.go              # CLI, server startup
│
├── internal/                # Private application code
│   ├── config/              # Configuration
│   │   ├── config.go
│   │   ├── config_test.go
│   │   └── example.yaml
│   │
│   ├── storage/             # Storage layer
│   │   ├── storage.go       # Interface
│   │   ├── sqlite.go        # SQLite backend
│   │   ├── lmdb.go          # LMDB backend
│   │   ├── migrations.go    # Schema
│   │   ├── relay_hints.go   # Custom table
│   │   ├── graph_nodes.go   # Custom table
│   │   ├── sync_state.go    # Custom table
│   │   ├── aggregates.go    # Custom table
│   │   └── search.go        # NIP-50 search queries
│   │
│   ├── nostr/               # Nostr client
│   │   ├── client.go        # WebSocket pool
│   │   ├── relay.go         # Per-relay connection
│   │   ├── discovery.go     # NIP-65
│   │   ├── relay_hints.go   # Hint parsing
│   │   └── profile.go       # Profile metadata parsing
│   │
│   ├── sync/                # Sync engine
│   │   ├── engine.go        # Main orchestration
│   │   ├── filters.go       # Filter builder
│   │   ├── graph.go         # Social graph
│   │   ├── cursors.go       # Cursor tracking
│   │   └── scope.go         # Scope enforcement
│   │
│   ├── aggregates/          # Aggregates
│   │   ├── aggregates.go    # Manager
│   │   ├── threading.go     # NIP-10
│   │   ├── reactions.go     # Reactions
│   │   ├── zaps.go          # Zaps
│   │   ├── reconciler.go    # Periodic recount
│   │   └── queries.go       # Helpers
│   │
│   ├── cache/               # Caching layer
│   │   ├── cache.go         # Interface, factory
│   │   ├── memory.go        # In-memory cache
│   │   ├── redis.go         # Redis cache
│   │   ├── keys.go          # Key generation
│   │   ├── invalidator.go   # Cache invalidation
│   │   ├── cache_test.go    # Tests
│   │   └── keys_test.go     # Key tests
│   │
│   ├── sections/            # Sections and layouts
│   │   ├── sections.go      # Section manager
│   │   ├── filters.go       # Filter builder
│   │   ├── archives.go      # Archive generation
│   │   ├── sections_test.go # Tests
│   │   └── filters_test.go  # Filter tests
│   │
│   ├── security/            # Security features
│   │   ├── denylist.go      # Pubkey blocking
│   │   ├── ratelimit.go     # Rate limiting
│   │   ├── validation.go    # Input validation
│   │   ├── secrets.go       # Secret management
│   │   └── security_test.go # Security tests
│   │
│   ├── search/              # Search functionality
│   │   └── nip50.go         # NIP-50 search engine
│   │
│   ├── entities/            # NIP-19 entity resolution
│   │   ├── resolver.go      # Entity parsing and resolution
│   │   └── formatters.go    # Protocol-specific formatters
│   │
│   ├── markdown/            # Markdown conversion
│   │   ├── parser.go        # AST parsing
│   │   ├── gopher.go        # → plain text
│   │   ├── gemini.go        # → gemtext
│   │   └── finger.go        # → stripped
│   │
│   ├── gopher/              # Gopher protocol
│   │   ├── server.go        # TCP server
│   │   ├── router.go        # Selector routing
│   │   ├── gophermap.go     # Menu generation
│   │   └── renderer.go      # Event rendering
│   │
│   ├── gemini/              # Gemini protocol
│   │   ├── server.go        # TLS server
│   │   ├── router.go        # URL routing
│   │   ├── renderer.go      # Gemtext rendering
│   │   ├── protocol.go      # Protocol helpers
│   │   └── tls.go           # TLS management
│   │
│   └── finger/              # Finger protocol
│       ├── server.go        # TCP server
│       ├── handler.go       # Query parsing
│       └── renderer.go      # Response formatting
│
├── configs/                 # Example configurations
│   └── nophr.example.yaml
│
├── memory/                  # Design documentation
│   ├── README.md
│   ├── architecture.md
│   └── ...
│
├── docs/                    # User documentation
│   ├── getting-started.md
│   ├── configuration.md
│   ├── storage.md
│   ├── protocols.md
│   ├── nostr-integration.md
│   ├── architecture.md      # (this file)
│   ├── deployment.md
│   └── troubleshooting.md
│
├── scripts/                 # Build and CI scripts
│   ├── build.sh
│   ├── test.sh
│   └── lint.sh
│
├── Makefile                 # Build automation
├── go.mod                   # Go dependencies
├── README.md                # Project overview
├── CONTRIBUTING.md          # Contribution guidelines
└── AGENTS.md                # Agent/contributor instructions
```

---

## Technology Stack

### Language

**Go 1.25+**

**Why Go:**
- Excellent concurrency (goroutines for multiple protocols)
- Strong networking libraries
- Cross-platform compilation
- Great performance
- Mature ecosystem

### Core Dependencies

**Khatru** - Nostr relay framework
- https://github.com/fiatjaf/khatru
- Provides event storage, validation, querying
- Pluggable eventstore backends

**eventstore** - Database adapters
- https://github.com/fiatjaf/eventstore
- SQLite and LMDB implementations (nophr uses SQLite in this build)
- Used by Khatru

**go-nostr** - Nostr protocol
- https://github.com/nbd-wtf/go-nostr
- WebSocket client, event signing/verification
- NIP implementations

**gopkg.in/yaml.v3** - YAML parsing
- Configuration file parsing

### Why Khatru?

**Benefits:**
1. Battle-tested event storage (production Nostr relays)
2. NIP compliance out-of-box
3. Automatic signature verification
4. Pluggable backends (SQLite/LMDB)
5. Simpler codebase (no event plumbing)
6. Future-proof (protocol evolution)

**What Khatru handles:**
- Event storage, indexing, deduplication
- Replaceable event logic
- Event validation
- Querying via Nostr filters
- WebSocket relay interface (optional)

**What nophr adds:**
- Sync engine (pull from remote → push to Khatru)
- Relay discovery and social graph
- Aggregates (interaction counts)
- Protocol servers (Gopher/Gemini/Finger)
- Markdown conversion and rendering

---

## Data Flow Examples

### Example 1: Syncing a New Note

```
1. User publishes note (kind 1) on remote relay
   │
2. nophr sync engine subscribes to relay
   │
3. Relay sends EVENT message
   │
4. Sync engine receives event
   │
5. Sync engine calls khatru.StoreEvent(event)
   │
6. Khatru validates signature
   │
7. Khatru checks for duplicates
   │
8. Khatru stores event in SQLite/LMDB
   │
9. Sync engine updates sync_state cursor
   │
10. If event is reply/reaction/zap:
    └→ Aggregates manager updates aggregates table
   │
11. Event now queryable via protocols
```

### Example 2: Serving a Gopher Menu

```
1. Client connects to port 70
   │
2. Client sends selector: "/notes"
   │
3. Gopher server receives request
   │
4. Router matches "/notes" → notes section
   │
5. Query Khatru: filter={kinds:[1], authors:[owner]}
   │
6. Khatru returns events from eventstore
   │
7. Query aggregates for interaction counts
   │
8. Render gophermap:
   │  - Line per event
   │  - Include metadata (date, reactions, zaps)
   │
9. Send gophermap to client
   │
10. Close connection
```

### Example 3: Rendering a Gemini Article

```
1. Client connects to port 1965 (TLS)
   │
2. Client sends: gemini://host/article/xyz
   │
3. Gemini server receives request
   │
4. Router matches "/article/xyz" → article view
   │
5. Query Khatru: filter={kinds:[30023], ids:[xyz]}
   │
6. Khatru returns article event
   │
7. Parse markdown content
   │
8. Convert markdown → gemtext:
   │  - Headings → # ##
   │  - Links → => url text
   │  - Code → ``` ... ```
   │
9. Query aggregates for replies/reactions
   │
10. Append interaction summary
   │
11. Send gemtext response with status 20
   │
12. Close connection
```

---

## Concurrency Model

nophr uses Go's goroutines for concurrency:

**Protocol servers:**
- Each protocol server runs in own goroutine
- Each client connection handled in separate goroutine
- Concurrent connections: 1000+ per protocol (lightweight)

**Sync engine:**
- One goroutine per relay subscription
- Separate goroutines for cursor updates, graph computation
- Background reconciler goroutine (aggregate recount)

**Example:**
```go
// Main goroutine
func main() {
    // Start protocol servers (3 goroutines)
    go gopherServer.Start()
    go geminiServer.Start()
    go fingerServer.Start()

    // Start sync engine (N goroutines for N relays)
    go syncEngine.Start()

    // Wait for shutdown signal
    <-sigChan
}

// Per-protocol server
func (s *Server) Start() {
    for {
        conn := listener.Accept()
        go s.handleConnection(conn)  // New goroutine per connection
    }
}
```

---

## Testing Strategy

**Unit tests:**
- Per-package unit tests
- Mock interfaces (storage, relay clients)
- Test coverage target: >80%

**Integration tests:**
- Full flow tests (sync → storage → render)
- Test with real Khatru instance
- Mock remote relays

**Protocol compliance tests:**
- RFC 1436 (Gopher) compliance
- Gemini spec compliance
- RFC 742/1288 (Finger) compliance

**Test files:**
- `*_test.go` - Unit tests alongside source
- `test/integration/` - Integration tests
- `test/compliance/` - Protocol tests

 

---

## Security Considerations

### Secrets Management

**nsec (private key):**
- NEVER in config files
- Only via `NOPHR_NSEC` environment variable
- Never logged, never serialized

**Redis URL:**
- Via `NOPHR_REDIS_URL` environment variable
- Keep out of config files if contains password

### Port Binding

**Ports <1024 require root:**
- Gopher: 70
- Finger: 79

**Mitigation:**
- Systemd socket activation (recommended)
- Port forwarding (iptables)
- Run on higher ports (testing only)

### TLS (Gemini)

**Certificate validation:**
- Gemini uses TOFU (Trust On First Use)
- Client stores certificate fingerprint on first connect
- Subsequent connects verify against stored fingerprint

**Self-signed vs. proper certs:**
- Self-signed: OK for personal use
- Let's Encrypt: Recommended for production

### Input Validation

**Selectors/URLs:**
- Validate format, length
- Prevent directory traversal (../)
- Sanitize user input

**Event content:**
- Khatru validates signatures
- Markdown parsing should be safe (no execution)

### Rate Limiting

Use built-in rate limiting (see security settings), and optionally firewall rules (iptables, fail2ban).

---

## Performance Characteristics

### Resource Usage

**Memory:**
- Base: ~50MB
- Per protocol server: ~10MB
- Per relay connection: ~1MB
- Per cached response: varies (KB-MB)
- Total typical: 100-200MB

**Disk:**
- SQLite database: ~1KB per event
- 100K events: ~100MB

**CPU:**
- Idle: <1%
- Sync active: 5-10% (depends on relay count)
- Serving requests: <1% per connection

### Scalability

**Single-tenant design:**
- Optimized for one owner
- Supports thousands of followed users (with caps)
- Designed to scale to millions of events with appropriate backend tuning

**Concurrent connections:**
- Gopher: 1000+ (lightweight, <10KB per connection)
- Gemini: 1000+ (TLS overhead, ~50KB per connection)
- Finger: 1000+ (very lightweight, ~5KB per connection)

**Bottlenecks:**
- Database queries (mitigated by caching)
- Markdown rendering (CPU-bound)
- Aggregate computation (mitigated by caching + reconciler)

---

## Enhancements

- Operations and diagnostics (logging, health, statistics)
- Publisher (signing and sending events)
- Testing (unit/integration/compliance)
- Distribution (builds, Docker, service files)
- Advanced retention (rule-based, multi-dimensional)

---

## Design Decisions

### Why Khatru?

**Alternatives considered:**
- Custom event storage (too much work)
- Direct database access (no NIP compliance)
- Separate relay (unnecessary complexity)

**Khatru wins:**
- Battle-tested, production-ready
- NIP compliance out-of-box
- Pluggable backends
- Active development
- Go ecosystem

### Why SQLite?

**Alternatives considered:**
- PostgreSQL (too heavy for single-tenant)
- LMDB (better for high-volume, but more complex; not yet supported in this build)
- Badger/LevelDB (less mature in Nostr ecosystem)

**SQLite wins for default:**
- Zero configuration
- Single file (easy backups)
- Sufficient for most users
- Mature, stable

### Why Embedded?

**Alternatives considered:**
- Separate Nostr relay (Khatru as service)
- Client-server architecture

**Embedded wins:**
- Simpler deployment (one binary)
- No network overhead
- Direct API access (faster)
- Easier to reason about

### Why Three Protocols?

**Why not just Gopher?**
- Different audiences (Gopher purists, Gemini fans, Finger users)
- Showcase Nostr content in multiple contexts
- Educational (protocol comparison)

**Why not HTTP?**
- Nostr already has HTTP gateways (njump, etc.)
- Focus on underserved protocols
- Minimalist philosophy

---

## Contributing

See [CONTRIBUTING.md](../CONTRIBUTING.md) for contributor guidelines.

For AI agents working on this project, see [AGENTS.md](../AGENTS.md).

**Code style:**
- Go standard formatting (`gofmt`)
- Linting with `golangci-lint`
- Keep files <500 lines (see AGENTS.md for guidelines)
- DRY (Don't Repeat Yourself)
- Clear package boundaries

**Pull requests:**
- Write tests for new code
- Update docs if behavior changes
- Follow existing patterns
- Keep PRs focused (one feature/fix per PR)

---

## References

**Design documentation:**
 

**External:**
- Khatru: https://github.com/fiatjaf/khatru
- eventstore: https://github.com/fiatjaf/eventstore
- go-nostr: https://github.com/nbd-wtf/go-nostr
- Nostr NIPs: https://github.com/nostr-protocol/nips

---

**Next:** [Deployment Guide](deployment.md) | [Getting Started](getting-started.md) | [Configuration Reference](configuration.md)
