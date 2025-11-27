# Configuration Reference

 

Complete reference for nophr's YAML configuration file.

## Overview

nophr uses YAML for configuration with environment variable overrides for secrets. Configuration is validated on startup.

**Generate example configuration:**
```bash
nophr init > nophr.yaml
```

**Load configuration:**
```bash
nophr --config nophr.yaml
```

## Configuration Sections

- [site](#site) - Site metadata
- [identity](#identity) - Your Nostr identity
- [protocols](#protocols) - Protocol server settings
- [relays](#relays) - Seed relays and policies
- [discovery](#discovery) - Relay discovery (NIP-65)
- [sync](#sync) - Event synchronization scope
- [inbox](#inbox) - Interaction aggregation
- [storage](#storage) - Database backend
- [rendering](#rendering) - Protocol-specific rendering
- [caching](#caching) - Response caching
- [logging](#logging) - Logging configuration
- [sections](#sections) - Custom filtered views
- [layout](#layout) - (DEPRECATED - use sections instead)
- [security](#security) - Security features (deny lists, rate limiting, validation)
- [display](#display) - Display control (feed/detail views, limits)
- [presentation](#presentation) - Visual presentation (headers, footers, separators)
- [behavior](#behavior) - Behavior control (filtering, sorting, pagination)

---

## site

Site metadata displayed in protocol responses.

```yaml
site:
  title: "My Notes"
  description: "Personal Nostr gopherhole"
  operator: "Alice"
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `title` | string | Yes | Site name shown in menus/headers |
| `description` | string | Yes | Brief site description |
| `operator` | string | Yes | Your name or handle |

**Example:**
```yaml
site:
  title: "Alice's Nostr Archive"
  description: "Notes, articles, and interactions from Nostr"
  operator: "Alice (@alice)"
```

---

## identity

Your Nostr identity (public and private keys).

```yaml
identity:
  npub: "npub1..." # Your Nostr public key (REQUIRED)
  # nsec is NEVER in config - use NOPHR_NSEC env var
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `npub` | string | **Yes** | Your Nostr public key (npub1...) |
| `nsec` | string | No | **NEVER IN FILE** - Set via `NOPHR_NSEC` env var |

**Security:**
- ✅ `npub` goes in config file (public key, safe to share)
- ❌ `nsec` NEVER in config file (private key, keep secret!)
- ✅ Set `nsec` via environment: `export NOPHR_NSEC="nsec1..."`

**Get your npub:**
- From any Nostr client (profile settings)
- Must start with `npub1`

**Example:**
```yaml
identity:
  npub: "npub1a2b3c4d5e6f7g8h9i0j..."
```

 

---

## protocols

Enable/disable protocol servers and configure ports.

```yaml
protocols:
  gopher:
    enabled: true
    host: "gopher.example.com"
    port: 70
    bind: "0.0.0.0"
  gemini:
    enabled: true
    host: "gemini.example.com"
    port: 1965
    bind: "0.0.0.0"
    tls:
      cert_path: "./certs/cert.pem"
      key_path: "./certs/key.pem"
      auto_generate: true
  finger:
    enabled: true
    port: 79
    bind: "0.0.0.0"
    max_users: 100
```

### protocols.gopher

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `true` | Enable Gopher server |
| `host` | string | `localhost` | Hostname for gopher:// URLs |
| `port` | int | `70` | TCP port (RFC 1436 standard) |
| `bind` | string | `0.0.0.0` | Interface to bind to |

**Notes:**
- Port 70 requires root/sudo on most systems
- Use `127.0.0.1` to bind only to localhost
- `host` used in gophermap links

### protocols.gemini

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `true` | Enable Gemini server |
| `host` | string | `localhost` | Hostname for gemini:// URLs |
| `port` | int | `1965` | TLS port (Gemini standard) |
| `bind` | string | `0.0.0.0` | Interface to bind to |
| `tls.cert_path` | string | `./certs/cert.pem` | Path to TLS certificate |
| `tls.key_path` | string | `./certs/key.pem` | Path to TLS private key |
| `tls.auto_generate` | bool | `true` | Generate self-signed cert if missing |

**TLS Certificates:**
- If `auto_generate: true` and cert files missing, creates self-signed cert
- For production, use proper TLS cert (Let's Encrypt, etc.)
- Self-signed certs require TOFU (Trust On First Use) in Gemini clients

**Generate cert manually:**
```bash
mkdir -p certs
openssl req -x509 -newkey rsa:4096 -keyout certs/key.pem \
  -out certs/cert.pem -days 365 -nodes -subj "/CN=gemini.example.com"
```

### protocols.finger

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `true` | Enable Finger server |
| `port` | int | `79` | TCP port (RFC 742 standard) |
| `bind` | string | `0.0.0.0` | Interface to bind to |
| `max_users` | int | `100` | Max users queryable (owner + followed) |

**Notes:**
- Port 79 requires root/sudo
- `max_users` limits which followed users are fingerable

---

## relays

Seed relays and connection policies.

```yaml
relays:
  seeds:
    - "wss://relay.damus.io"
    - "wss://relay.nostr.band"
    - "wss://nos.lol"
  policy:
    connect_timeout_ms: 5000
    max_concurrent_subs: 8
    backoff_ms: [500, 1500, 5000]
```

### relays.seeds

**Type:** Array of strings (WebSocket URLs)

Seed relays used for initial relay discovery. After startup, nophr discovers additional relays via NIP-65 (kind 10002).

**Requirements:**
- Must start with `wss://` (TLS) or `ws://` (unencrypted)
- At least one seed required
- Choose reliable, well-connected relays

**Popular seed relays:**
```yaml
seeds:
  - "wss://relay.damus.io"        # Large, reliable
  - "wss://relay.nostr.band"      # Aggregator, good coverage
  - "wss://nos.lol"               # Fast, popular
  - "wss://relay.snort.social"    # Well-maintained
  - "wss://nostr.wine"            # Paid, quality
```

### relays.policy

Connection behavior and limits.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `connect_timeout_ms` | int | `5000` | Connection timeout (milliseconds) |
| `max_concurrent_subs` | int | `8` | Max concurrent subscriptions per relay |
| `backoff_ms` | int[] | `[500, 1500, 5000]` | Retry backoff schedule (ms) |

**Backoff behavior:**
- First retry: 500ms delay
- Second retry: 1500ms delay
- Third+ retry: 5000ms delay
- Prevents hammering unavailable relays

---

## discovery

Relay discovery settings using NIP-65 (kind 10002).

```yaml
discovery:
  refresh_seconds: 900
  use_owner_hints: true
  use_author_hints: true
  fallback_to_seeds: true
  max_relays_per_author: 8
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `refresh_seconds` | int | `900` | How often to refresh kind 10002 (15 min) |
| `use_owner_hints` | bool | `true` | Use owner's relay hints for owner data |
| `use_author_hints` | bool | `true` | Use authors' relay hints for their data |
| `fallback_to_seeds` | bool | `true` | Use seeds if hints missing/stale |
| `max_relays_per_author` | int | `8` | Safety cap per author |

**How it works:**
1. Fetch kind 10002 from seed relays (owner + followed users)
2. Parse relay hints (read/write tags)
3. Connect to discovered relays for targeted queries
4. Refresh periodically to catch relay changes

 

---

## sync

Event synchronization scope and retention.

```yaml
sync:
  enabled: true  # Enable/disable sync engine
  kinds:
    profiles: true      # kind 0 - user profiles/metadata
    notes: true         # kind 1 - short text notes
    contact_list: true  # kind 3 - following lists
    reposts: true       # kind 6 - reposts/boosts
    reactions: true     # kind 7 - reactions (likes, emoji)
    zaps: true          # kind 9735 - lightning zaps
    articles: true      # kind 30023 - long-form articles
    relay_list: true    # kind 10002 - relay preferences (NIP-65)
    allowlist: []       # Additional custom kinds to sync
  scope:
    mode: "foaf"
    depth: 2
    include_direct_mentions: true
    include_threads_of_mine: true
    max_authors: 5000
    allowlist_pubkeys: []
    denylist_pubkeys: []
  retention:
    keep_days: 365
    prune_on_start: true
```

### sync.enabled

**Type:** Boolean
**Default:** `true`

Enable or disable the sync engine.

```yaml
sync:
  enabled: true   # Sync engine runs, pulls events from relays
  # enabled: false  # Sync engine disabled, no new events synced
```

**When disabled:**
- No events are synced from remote relays
- Only serves existing events from database
- Useful for read-only deployments or maintenance

**When enabled:**
- Sync engine starts and connects to relays
- Events are pulled based on scope configuration
- Relay discovery runs periodically

 

### sync.kinds

**Type:** Object with boolean flags and allowlist array

Granular control over which Nostr event kinds to synchronize.

| Field | Type | Default | Kind | Description |
|-------|------|---------|------|-------------|
| `profiles` | bool | `true` | 0 | User profiles/metadata (name, avatar, bio) |
| `notes` | bool | `true` | 1 | Short text notes and posts |
| `contact_list` | bool | `true` | 3 | Following lists (social graph) |
| `reposts` | bool | `true` | 6 | Reposts/boosts of other notes |
| `reactions` | bool | `true` | 7 | Reactions (likes, emoji responses) |
| `zaps` | bool | `true` | 9735 | Lightning zap receipts (tips) |
| `articles` | bool | `true` | 30023 | Long-form articles (blog posts) |
| `relay_list` | bool | `true` | 10002 | Relay preferences (NIP-65) |
| `allowlist` | []int | `[]` | - | Additional custom kinds to sync |

**Selective sync examples:**

```yaml
# Sync only notes and profiles (minimal)
kinds:
  profiles: true
  notes: true
  contact_list: false
  reposts: false
  reactions: false
  zaps: false
  articles: false
  relay_list: false
```

```yaml
# Sync everything except reactions and zaps
kinds:
  profiles: true
  notes: true
  contact_list: true
  reposts: true
  reactions: false  # Don't sync reactions
  zaps: false       # Don't sync zaps
  articles: true
  relay_list: true
```

```yaml
# Add custom kinds with allowlist
kinds:
  profiles: true
  notes: true
  contact_list: true
  reposts: true
  reactions: true
  zaps: true
  articles: true
  relay_list: true
  allowlist: [30311, 34235]  # Add NIP-89 app handlers, NIP-94 file metadata
```

**Benefits of granular control:**
- Reduce storage requirements by disabling unused kinds
- Improve sync speed by syncing fewer event types
- Fine-tune content for your use case
- Easily add custom NIPs with allowlist

### sync.scope

Controls whose events to sync.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `mode` | string | `foaf` | Sync mode (see below) |
| `depth` | int | `2` | FOAF depth (when mode=foaf) |
| `include_direct_mentions` | bool | `true` | Include events mentioning you |
| `include_threads_of_mine` | bool | `true` | Include threads you participated in |
| `max_authors` | int | `5000` | Safety cap on total authors |
| `allowlist_pubkeys` | string[] | `[]` | Always include these pubkeys |
| `denylist_pubkeys` | string[] | `[]` | Never include these pubkeys |

**Sync modes:**

| Mode | Description | Authors Synced |
|------|-------------|----------------|
| `self` | Only your events | 1 (you) |
| `following` | You + who you follow | 1 + kind 3 count |
| `mutual` | You + mutual follows | 1 + bidirectional follows |
| `foaf` | Friend-of-a-friend | Grows exponentially by depth |

**FOAF depth examples:**
- `depth: 1` = You + following (same as `following` mode)
- `depth: 2` = You + following + their follows (2nd degree)
- `depth: 3` = 3rd degree connections (can be huge!)

**Recommendations:**
- Start with `mode: following` or `mode: mutual`
- Use `mode: foaf` with `depth: 2` cautiously (may sync thousands)
- Set `max_authors` to prevent runaway sync
- Use `denylist_pubkeys` for spam accounts

### sync.retention

Data retention and pruning.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `keep_days` | int | `365` | Keep events newer than N days |
| `prune_on_start` | bool | `true` | Prune old events at startup |

**Pruning behavior:**
- Events older than `keep_days` are deleted
- Kind 0 (profiles) and kind 3 (follows) never pruned
- Replaceable events (kind 10002, 30023) keep only latest

 

### sync.retention.advanced

**Advanced configurable retention system** - sophisticated, multi-dimensional retention rules.

 

```yaml
sync:
  retention:
    keep_days: 365
    prune_on_start: true

    advanced:
      enabled: false              # Must explicitly enable
      mode: "rules"               # rules|caps

      evaluation:
        on_ingest: true           # Evaluate new events immediately
        re_eval_interval_hours: 168  # Re-evaluate weekly
        batch_size: 1000

      global_caps:
        max_total_events: 1000000
        max_storage_mb: 5000
        max_events_per_kind:
          1: 100000               # Max 100k notes
          30023: 10000            # Max 10k articles

      rules:
        - name: "protect_owner"
          priority: 1000
          conditions:
            author_is_owner: true
          action:
            retain: true          # Never delete

        - name: "close_network"
          priority: 800
          conditions:
            social_distance_max: 1
            kinds: [1, 30023]
          action:
            retain_days: 365

        - name: "default"
          priority: 100
          conditions:
            all: true
          action:
            retain_days: 90
```

**Key features:**

| Feature | Description |
|---------|-------------|
| **Rule-based** | Define retention rules with conditions and priorities |
| **Multi-dimensional** | Filter by kind, author, social distance, interaction count, etc. |
| **Cap enforcement** | Hard limits on total events, storage, per-kind counts |
| **Score-based pruning** | When caps exceeded, delete lowest-scored events first |
| **Protected events** | Mark events that should never be deleted |
| **Incremental evaluation** | Evaluate on ingestion + periodic re-evaluation |

**Condition types (gates):**
- `author_is_owner` - Event is from owner
- `social_distance_max` - FOAF distance ≤ N
- `kinds` - Event kind matches list
- `min_interactions` - Has at least N replies/reactions/zaps
- `age_days_max` - Event age ≤ N days
- `content_length_min` - Content ≥ N chars
- `is_thread_root` - Is root of thread
- `has_replies` - Has at least one reply

**Action types:**
- `retain: true` - Never delete (protected)
- `retain_days: N` - Keep for N days
- `retain: false` - Eligible for deletion

**Priority:**
- Higher priority rules match first
- If multiple rules match, highest priority wins
- Default rule (lowest priority) catches all

**Backward compatibility:**
- If `advanced.enabled: false`, uses simple `keep_days` only
- Invalid advanced config falls back to simple mode with warning
- Simple mode remains fully functional

 

 

---

## inbox

Inbox aggregation of replies, reactions, and zaps.

```yaml
inbox:
  include_replies: true
  include_reactions: true
  include_zaps: true
  group_by_thread: true
  collapse_reposts: true
  noise_filters:
    min_zap_sats: 1
    allowed_reaction_chars: ["+"]
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `include_replies` | bool | `true` | Show replies to your notes |
| `include_reactions` | bool | `true` | Show kind 7 reactions |
| `include_zaps` | bool | `true` | Show kind 9735 zaps |
| `group_by_thread` | bool | `true` | Group inbox by thread root |
| `collapse_reposts` | bool | `true` | Collapse multiple reposts |
| `noise_filters.min_zap_sats` | int | `1` | Minimum zap amount to show |
| `noise_filters.allowed_reaction_chars` | string[] | `["+"]` | Filter reactions (e.g., only "+") |

**Noise filtering:**
- Filter out tiny zaps: `min_zap_sats: 100` (0.1 sat minimum)
- Allow only specific reactions: `allowed_reaction_chars: ["+", "❤️", "🔥"]`
- Prevent spam/unwanted reactions

 

---

 

## storage

Database backend configuration.

```yaml
storage:
  driver: "sqlite"
  sqlite_path: "./data/nophr.db"
  lmdb_path: "./data/nophr.lmdb"
  lmdb_max_size_mb: 10240
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `driver` | string | `sqlite` | Database backend (`sqlite` or `lmdb`) |
| `sqlite_path` | string | `./data/nophr.db` | SQLite database file path |
| `lmdb_path` | string | `./data/nophr.lmdb` | LMDB database directory |
| `lmdb_max_size_mb` | int | `10240` | LMDB max size (MB) - 10GB default |

**Choosing a backend:**

| Feature | SQLite | LMDB |
|---------|--------|------|
| File format | Single .db file | Directory with data files |
| Setup | Zero config | Zero config |
| Performance | Good for <100K events | Excellent for millions |
| Concurrency | Limited writes | Excellent |
| Backups | Copy .db file | Copy directory |
| Best for | Personal use | High-volume streaming |

**Recommendations:**
- **Start with SQLite** - simpler, sufficient for most users
- LMDB is not supported in this build
- Both use Khatru eventstore interface (see [storage.md](storage.md))

 

---

## rendering

Protocol-specific rendering options.

```yaml
rendering:
  gopher:
    max_line_length: 70
    show_timestamps: true
    date_format: "2006-01-02 15:04 MST"
    thread_indent: "  "
  gemini:
    max_line_length: 80
    show_timestamps: true
    emoji: true
  finger:
    plan_source: "kind_0"
    recent_notes_count: 5
```

### rendering.gopher

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `max_line_length` | int | `70` | Wrap text at N characters |
| `show_timestamps` | bool | `true` | Show event timestamps |
| `date_format` | string | `2006-01-02 15:04 MST` | Go time format string |
| `thread_indent` | string | `"  "` | Indent string for replies |

**Gopher conventions:**
- 70 chars is traditional (old terminal width)
- Plain ASCII, no ANSI colors
- Minimal formatting

### rendering.gemini

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `max_line_length` | int | `80` | Wrap text at N characters |
| `show_timestamps` | bool | `true` | Show event timestamps |
| `emoji` | bool | `true` | Allow emoji in gemtext |

**Gemini conventions:**
- 80 chars common but not required
- UTF-8 supported (emoji OK)
- Gemtext format (headings, links, quotes)

### rendering.finger

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `plan_source` | string | `kind_0` | Source for .plan field (`kind_0` or `kind_1`) |
| `recent_notes_count` | int | `5` | Number of recent notes to show |

**Plan source:**
- `kind_0`: Use profile "about" field as .plan
- `kind_1`: Use most recent note as .plan

 

---

## caching

Response caching configuration for dramatic performance improvements.

```yaml
caching:
  enabled: true
  engine: "memory"  # or "redis"
  redis_url: ""  # Set via NOPHR_REDIS_URL env var

  ttl:
    sections:
      notes: 60        # 1 minute
      comments: 30     # 30 seconds
      articles: 300    # 5 minutes
      interactions: 10 # 10 seconds
    render:
      gopher_menu: 300      # 5 minutes
      gemini_page: 300      # 5 minutes
      finger_response: 60   # 1 minute
      kind_1: 86400         # 24 hours
      kind_30023: 604800    # 7 days
      kind_0: 3600          # 1 hour
      kind_3: 600           # 10 minutes

  aggregates:
    enabled: true
    update_on_ingest: true
    reconciler_interval_seconds: 900  # 15 minutes
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `true` | Master switch for caching |
| `engine` | string | `memory` | Cache backend (`memory` or `redis`) |
| `redis_url` | string | `""` | Redis URL (via `NOPHR_REDIS_URL` env) |
| `ttl.sections.*` | int | varies | Section cache TTLs (seconds) |
| `ttl.render.*` | int | varies | Render cache TTLs (seconds) |
| `aggregates.enabled` | bool | `true` | Cache aggregate computations |
| `aggregates.update_on_ingest` | bool | `true` | Update on new events |
| `aggregates.reconciler_interval_seconds` | int | `900` | Reconcile drift (15 min) |

### caching.enabled

**Type:** Boolean
**Default:** `true`

Enable or disable the caching layer.

```yaml
caching:
  enabled: true   # Caching active, responses cached
  # enabled: false  # No caching, always regenerate responses
```

**Performance Impact:**
- **Enabled**: 10-100x faster responses, 80-95% reduction in database queries
- **Disabled**: Always regenerates responses, higher latency and CPU usage

### caching.engine

**Type:** String (`memory` or `redis`)
**Default:** `memory`

Cache backend engine.

**Memory Cache:**
```yaml
caching:
  engine: "memory"
  max_size_mb: 100
```

- Thread-safe in-memory cache
- LRU eviction when size limit reached
- Automatic cleanup of expired entries
- Best for single-instance deployments
- No external dependencies

**Redis Cache:**
```yaml
caching:
  engine: "redis"
  redis_url: "redis://localhost:6379/0"
```

- Distributed cache across multiple instances
- Persistent across restarts
- Better memory management
- Built-in clustering support
- Requires external Redis server

**When to use Redis:**
- Running multiple nophr instances
- Need persistent cache across restarts
- Limited memory on host
- Want shared cache for load balancing

### Cache Invalidation

Cache entries are automatically invalidated when relevant events are ingested:

| Event Kind | Invalidates |
|------------|-------------|
| Kind 0 (Profile) | Profile cache, kind0 cache |
| Kind 1 (Note) | Notes section cache |
| Kind 3 (Contacts) | Kind3 cache |
| Kind 7 (Reaction) | Parent event aggregates |
| Kind 9735 (Zap) | Parent event aggregates |

**Manual Invalidation:**
Cache is cleared when:
- Configuration changes
- Sync scope changes
- Manual server restart

### Cache Keys

Cache uses hierarchical keys:

```
gopher:/path/to/selector        - Gopher response
gemini:/path?query=test         - Gemini response
finger:username                 - Finger response
event:event123:gopher:text      - Event rendering
section:notes:gemini:p2         - Section page
thread:root123:gopher           - Thread rendering
profile:pubkey123:gemini        - Profile page
aggregate:event123              - Interaction counts
kind0:pubkey123                 - Profile metadata
kind3:pubkey123                 - Contact list
```

**Pattern Matching** (for bulk operations):
```
gopher:*                  - All Gopher responses
event:event123:*          - All renderings of event
profile:pubkey123:*       - All profile renderings
```

### TTL Strategy

**Short TTL (10-60s):** Live/changing content
- Interactions, inbox, recent notes

**Medium TTL (300-600s):** Semi-static content
- Sections, menus, navigation

**Long TTL (hours/days):** Immutable content
- Old events, profiles, articles

### Cache Statistics

Monitor cache performance:

```
Cache Statistics:
  Hits: 950
  Misses: 50
  Hit Rate: 95%
  Keys: 150
  Size: 12.3 MB / 100 MB
  Evictions: 5
  Avg Get Time: 0.3ms
  Avg Set Time: 0.5ms
```

**Target Metrics:**
- Hit Rate: > 80%
- Avg Get Time: < 1ms (memory), < 5ms (Redis)
- Evictions: Low (increase max_size_mb if high)

### Redis Configuration

**Environment Variable:**
```bash
export NOPHR_REDIS_URL="redis://localhost:6379/0"
```

**Redis URL Format:**
```
redis://[user:password@]host:port[/database]
```

**Examples:**
```bash
# Local Redis, no auth
export NOPHR_REDIS_URL="redis://localhost:6379/0"

# Remote Redis with password
export NOPHR_REDIS_URL="redis://:mypassword@redis.example.com:6379/0"

# Redis with username and password
export NOPHR_REDIS_URL="redis://user:pass@redis.example.com:6379/0"
```

**See also:** [deployment.md](deployment.md#redis-setup) for Redis installation and configuration.

 

---

## logging

Logging configuration.

```yaml
logging:
  level: "info"  # debug|info|warn|error
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `level` | string | `info` | Log level |

**Log levels:**
- `debug`: Verbose (all events, queries, connections)
- `info`: Normal (startup, errors, important events)
- `warn`: Warnings only
- `error`: Errors only

**Example:**
```bash
# Debug mode for troubleshooting
NOPHR_LOG_LEVEL=debug nophr --config nophr.yaml
```

 

---

## sections

Custom filtered views for organizing and presenting content at specific URL paths.

**Note:** Sections are OPTIONAL. Built-in endpoints (`/notes`, `/articles`, `/replies`, `/mentions`) work without any section configuration.

```yaml
sections:
  - name: diy-preview
    path: /
    title: "Latest DIY Projects"
    description: "Recent DIY posts"
    order: 0
    limit: 5
    show_dates: true
    show_authors: true
    sort_by: created_at
    sort_order: desc
    filters:
      kinds: [1, 30023]
      tags:
        t: ["diy"]
    more_link:
      text: "View all DIY posts"
      section_ref: diy-full

  - name: diy-full
    path: /diy
    title: "DIY Projects"
    description: "All DIY projects and tutorials"
    limit: 20
    filters:
      kinds: [1, 30023]
      tags:
        t: ["diy"]
    sort_by: created_at
    sort_order: desc
```

### sections Configuration

Sections allow you to create custom filtered views of your content at specific URL paths.

**Section fields:**

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `name` | string | Yes | - | Unique identifier for the section |
| `path` | string | Yes | - | URL path (e.g., `/diy`, `/art`, `/` for homepage) |
| `title` | string | Yes | - | Display title for the section |
| `description` | string | No | - | Section description |
| `order` | int | No | `0` | Display order when multiple sections share a path |
| `limit` | int | No | `20` | Events per page |
| `show_dates` | bool | No | `true` | Display event timestamps |
| `show_authors` | bool | No | `true` | Display author names |
| `sort_by` | string | No | `created_at` | Sort field: `created_at`, `reactions`, `zaps`, `replies` |
| `sort_order` | string | No | `desc` | Sort order: `asc` or `desc` |
| `group_by` | string | No | - | Grouping: `day`, `week`, `month`, `year`, `author`, `kind` |
| `filters` | object | No | - | Filter criteria (see below) |
| `more_link` | object | No | - | Optional link to full paginated view (see below) |

**Filter options:**

```yaml
filters:
  kinds: [1, 30023]                    # Event kinds to include
  authors: ["pubkey1", "pubkey2"]      # Filter by pubkeys (hex or npub)
  tags:
    t: ["diy", "art"]                  # Filter by hashtags
    p: ["pubkey"]                      # Filter by p-tags
    e: ["eventid"]                     # Filter by e-tags
  since: "2024-01-01T00:00:00Z"        # RFC3339 timestamp or "-24h" duration
  until: "2025-01-01T00:00:00Z"        # RFC3339 timestamp or "-1d" duration
  search: "keyword"                    # NIP-50 search term
  scope: "following"                   # self, following, mutual, foaf, all
  is_reply: true                       # true = only replies, false = only roots
```

**MoreLink structure:**

| Field | Type | Description |
|-------|------|-------------|
| `text` | string | Link text (e.g., "More DIY posts", "View all articles") |
| `section_ref` | string | Name of the section to link to (must be registered) |

**Built-in endpoints vs. Custom sections:**

nophr provides built-in router endpoints:
- `/notes` - Short-form notes (kind 1, non-replies)
- `/articles` - Long-form articles (kind 30023)
- `/replies` - Replies to your content
- `/mentions` - Posts mentioning you
- `/search` - Search interface

**Custom sections** are for filtered views at custom paths:
- `/diy` - Posts tagged with #diy
- `/art` - Posts tagged with #art
- `/following` - Posts from people you follow
- `/` - Homepage with multiple section previews

**Note:** The concepts of "inbox" and "outbox" as section names are DEPRECATED. Use the built-in router endpoints instead (`/notes`, `/replies`, `/mentions`, `/articles`). Sections are for custom filtered content only.

### sections Examples

**Example 1: Homepage with multiple section previews**

```yaml
sections:
  - name: diy-preview
    path: /
    title: "Latest DIY Projects"
    description: "Recent DIY posts"
    order: 0                        # First section on homepage
    limit: 5
    sort_by: created_at
    sort_order: desc
    filters:
      kinds: [1, 30023]
      tags:
        t: ["diy"]
    more_link:
      text: "View all DIY posts"
      section_ref: diy-full

  - name: art-preview
    path: /
    title: "Recent Art"
    description: "Latest art posts"
    order: 1                        # Second section on homepage
    limit: 5
    sort_by: created_at
    sort_order: desc
    filters:
      kinds: [1, 30023]
      tags:
        t: ["art"]
    more_link:
      text: "View all art posts"
      section_ref: art-full

  - name: diy-full
    path: /diy
    title: "DIY Projects"
    description: "All DIY projects and tutorials"
    limit: 20
    sort_by: created_at
    sort_order: desc
    filters:
      kinds: [1, 30023]
      tags:
        t: ["diy"]

  - name: art-full
    path: /art
    title: "Art Gallery"
    description: "All art and creative posts"
    limit: 20
    sort_by: reactions
    sort_order: desc
    filters:
      kinds: [1, 30023]
      tags:
        t: ["art"]
```

**Example 2: Following timeline**

```yaml
sections:
  - name: following-timeline
    path: /following
    title: "Timeline"
    description: "Recent posts from people you follow"
    limit: 50
    sort_by: created_at
    sort_order: desc
    filters:
      kinds: [1]
      scope: following
```

**Example 3: Time-based filtering**

```yaml
sections:
  - name: recent-week
    path: /recent
    title: "This Week"
    description: "Posts from the last 7 days"
    limit: 30
    filters:
      kinds: [1, 30023]
      since: "-7d"                   # 7 days ago
```

**Example 4: Specific authors**

```yaml
sections:
  - name: priority-authors
    path: /priority
    title: "Priority Authors"
    description: "Posts from my favorite people"
    limit: 20
    filters:
      kinds: [1, 30023]
      authors:
        - "npub1abc..."
        - "npub1xyz..."
```

 

---

## layout

**DEPRECATED:** The `layout.sections` configuration format is no longer used. Use top-level `sections:` array instead (see above).

If you have old config with `layout.sections`, migrate to the new format:

**Old format (deprecated):**
```yaml
layout:
  sections:
    mySection:
      title: "My Section"
```

**New format (correct):**
```yaml
sections:
  - name: mySection
    path: /my-section
    title: "My Section"
    filters:
      kinds: [1]
```

---

## security

Security features including deny lists, rate limiting, input validation, and content filtering.

```yaml
security:
  # Deny list configuration
  denylist:
    enabled: true
    pubkeys:
      - "deadbeef1234567890abcdef1234567890abcdef1234567890abcdef12345678"
      - "cafebabe1234567890abcdef1234567890abcdef1234567890abcdef12345678"

  # Content filtering
  content_filter:
    enabled: true
    banned_words:
      - "spam"
      - "scam"
    case_sensitive: false

  # Rate limiting
  ratelimit:
    enabled: true
    global:
      requests_per_minute: 60
      burst_size: 10
    per_protocol:
      gopher:
        requests_per_minute: 30
        burst_size: 5
      gemini:
        requests_per_minute: 60
        burst_size: 10
      finger:
        requests_per_minute: 20
        burst_size: 3

  # Input validation
  validation:
    enabled: true
    max_selector_length: 1024
    max_query_length: 2048
    max_path_length: 2048
    strict_mode: true

  # Security policy
  policy:
    allow_anonymous: true
    require_nip05: false
    block_tor: false
    block_vpn: false
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `denylist.enabled` | bool | `true` | Enable deny list filtering |
| `denylist.pubkeys` | []string | `[]` | Blocked pubkeys (hex format) |
| `content_filter.enabled` | bool | `true` | Enable content filtering |
| `content_filter.banned_words` | []string | `[]` | List of banned words |
| `content_filter.case_sensitive` | bool | `false` | Case-sensitive matching |
| `ratelimit.enabled` | bool | `true` | Enable rate limiting |
| `ratelimit.global.requests_per_minute` | int | `60` | Global rate limit |
| `ratelimit.global.burst_size` | int | `10` | Burst allowance |
| `ratelimit.per_protocol.*` | object | - | Per-protocol rate limits |
| `validation.enabled` | bool | `true` | Enable input validation |
| `validation.max_selector_length` | int | `1024` | Max Gopher selector length |
| `validation.max_query_length` | int | `2048` | Max Gemini query length |
| `validation.max_path_length` | int | `2048` | Max path length |
| `validation.strict_mode` | bool | `true` | Strict validation mode |
| `policy.allow_anonymous` | bool | `true` | Allow anonymous access |
| `policy.require_nip05` | bool | `false` | Require NIP-05 verification |

### security.denylist

Block specific Nostr pubkeys from appearing in your gateway.

**Pubkey format:** Full 64-character hex pubkey (not npub)

**Usage:**
```yaml
denylist:
  enabled: true
  pubkeys:
    - "deadbeef1234567890abcdef1234567890abcdef1234567890abcdef12345678"
```

**Dynamic management:**
- Pubkeys can be added/removed at runtime
- Thread-safe concurrent access
- Applies to all protocol servers

**What gets blocked:**
- Events authored by blocked pubkeys
- Profile information
- All interactions (replies, reactions, zaps) from blocked pubkeys

### security.content_filter

Filter events based on content patterns and banned words.

**Configuration:**
```yaml
content_filter:
  enabled: true
  banned_words:
    - "spam"
    - "phishing"
    - "scam"
  case_sensitive: false
```

**Behavior:**
- Checks event content for banned words
- Can be case-sensitive or insensitive
- Combines with deny list for comprehensive filtering
- Does not modify content, only filters visibility

### security.ratelimit

Prevent abuse with token bucket rate limiting.

**Algorithm:**
- Each client gets a bucket with N tokens
- Each request consumes 1 token
- Tokens refill over time (requests_per_minute / 60 per second)
- When bucket empty, requests are denied until refill

**Global rate limit:**
```yaml
ratelimit:
  enabled: true
  global:
    requests_per_minute: 60  # 1 request per second average
    burst_size: 10           # Allow bursts up to 10 requests
```

**Per-protocol limits:**
```yaml
ratelimit:
  per_protocol:
    gopher:
      requests_per_minute: 30  # Slower for Gopher
    gemini:
      requests_per_minute: 60  # Normal for Gemini
    finger:
      requests_per_minute: 20  # Slowest for Finger
```

**Client identification:**
- By IP address
- Shared across protocols unless per-protocol limits set
- Old client buckets automatically cleaned up

**Response when limited:**
- Gopher: Returns error message
- Gemini: Returns 44 status (slow down)
- Finger: Closes connection

### security.validation

Validate and sanitize all user input to prevent injection attacks.

**Protections against:**
- **CRLF injection**: `\r\n` characters removed
- **Null byte injection**: `\x00` characters removed
- **Directory traversal**: `..` sequences blocked
- **XSS attacks**: Script tags and dangerous HTML removed
- **Length limits**: Enforces maximum input lengths

**Strict mode:**
```yaml
validation:
  enabled: true
  strict_mode: true  # Reject invalid input
  # strict_mode: false  # Sanitize invalid input
```

**Strict mode true:** Rejects requests with invalid characters
**Strict mode false:** Attempts to sanitize and continues

**What gets validated:**
- Gopher selectors
- Gemini paths and queries
- Finger usernames
- Pubkeys and event IDs
- URLs and references

### security.policy

Security policy settings.

```yaml
policy:
  allow_anonymous: true      # Allow access without authentication
  require_nip05: false       # Require NIP-05 verification
  block_tor: false           # Block Tor exit nodes
  block_vpn: false           # Block known VPN IPs
```

 

### Security Best Practices

1. **Enable all security features** in production
2. **Use strict validation mode** to catch attacks early
3. **Set appropriate rate limits** based on your capacity
4. **Regularly review deny list** for new abusive pubkeys
5. **Monitor logs** for suspicious activity
6. **Keep banned words list updated** for your community standards
7. **Never commit secrets** to configuration files (use environment variables)

### Security Monitoring

Monitor these metrics:
- Rate limit hits per client
- Validation failures
- Deny list blocks
- Content filter matches

**See also:** [security.md](security.md) for comprehensive security guide

 

---

## display

Control what information is shown in feed/list views versus detail/individual event views.

```yaml
display:
  feed:
    show_interactions: true  # Show aggregate stats in list views
    show_reactions: true     # Include reaction counts
    show_zaps: true          # Include zap amounts
    show_replies: true       # Include reply counts

  detail:
    show_interactions: true  # Show aggregate stats on event pages
    show_reactions: true     # Include reaction breakdown
    show_zaps: true          # Include zap total
    show_replies: true       # Include reply count
    show_thread: true        # Show thread context/replies

  limits:
    summary_length: 100         # Characters to show in list previews
    max_content_length: 5000    # Maximum content length before truncation
    max_thread_depth: 10        # Maximum depth for thread display
    max_replies_in_feed: 3      # Max replies to show in feed items
    truncate_indicator: "..."   # String to append when content is truncated
```

### display.feed

Controls what appears in feed/list views (e.g., `/notes`, `/articles`).

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `show_interactions` | bool | `true` | Show interaction stats for each item |
| `show_reactions` | bool | `true` | Include reaction counts in stats |
| `show_zaps` | bool | `true` | Include zap amounts in stats |
| `show_replies` | bool | `true` | Include reply counts in stats |

**Example - minimal feed view:**
```yaml
feed:
  show_interactions: false  # Hide all interaction stats in lists
```

**Example - show only replies:**
```yaml
feed:
  show_interactions: true
  show_reactions: false
  show_zaps: false
  show_replies: true  # Only show reply counts
```

### display.detail

Controls what appears on individual event pages (e.g., `/event/abc123`).

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `show_interactions` | bool | `true` | Show interaction stats |
| `show_reactions` | bool | `true` | Show reaction breakdown |
| `show_zaps` | bool | `true` | Show total zap amount |
| `show_replies` | bool | `true` | Show reply count |
| `show_thread` | bool | `true` | Show full thread context |

**Example - hide all interactions on detail pages:**
```yaml
detail:
  show_interactions: false
  show_thread: false
```

### display.limits

Truncation and display limits.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `summary_length` | int | `100` | Max characters in list previews |
| `max_content_length` | int | `5000` | Max content length before truncation |
| `max_thread_depth` | int | `10` | Max depth for thread display |
| `max_replies_in_feed` | int | `3` | Max replies shown per feed item |
| `truncate_indicator` | string | `"..."` | Append when content truncated |

**Example - longer previews:**
```yaml
limits:
  summary_length: 200
  max_content_length: 10000
  truncate_indicator: " [continued...]"
```

 

---

## presentation

Visual presentation and layout customization including headers, footers, and separators.

```yaml
presentation:
  headers:
    global:
      enabled: false
      content: ""              # Inline header text
      file_path: ""            # Or load from file
    per_page: {}               # Page-specific headers

  footers:
    global:
      enabled: false
      content: ""              # Inline footer text
      file_path: ""            # Or load from file
    per_page: {}               # Page-specific footers

  separators:
    item:
      gopher: ""               # Between list items
      gemini: ""
      finger: ""
    section:
      gopher: "---"            # Between major sections
      gemini: "---"
      finger: "---"
```

### presentation.headers

Add custom headers to pages.

**Global header** (appears on all pages):
```yaml
headers:
  global:
    enabled: true
    content: |
      Welcome to My Nostr Gateway
      Updated: {{date}}
```

**Load from file:**
```yaml
headers:
  global:
    enabled: true
    file_path: "./headers/global.txt"
```

**Per-page headers:**
```yaml
headers:
  per_page:
    notes:
      enabled: true
      content: "My Personal Notes Collection"
    articles:
      enabled: true
      content: "Long-form Articles and Essays"
```

### presentation.footers

Add custom footers to pages (same structure as headers).

```yaml
footers:
  global:
    enabled: true
    content: |
      ---
      Powered by nophr
      {{site.operator}} - {{year}}
```

### presentation.separators

Customize separators between items and sections.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `item.gopher` | string | `""` | Between items in Gopher lists |
| `item.gemini` | string | `""` | Between items in Gemini lists |
| `item.finger` | string | `""` | Between items in Finger responses |
| `section.gopher` | string | `"---"` | Between major sections (Gopher) |
| `section.gemini` | string | `"---"` | Between major sections (Gemini) |
| `section.finger` | string | `"---"` | Between major sections (Finger) |

**Example - custom separators:**
```yaml
separators:
  item:
    gopher: "- - -"
    gemini: "━━━"
  section:
    gopher: "========================================"
    gemini: "════════════════════════════════════════"
```

### Template Variables

Headers and footers support template variables:

| Variable | Description | Example |
|----------|-------------|---------|
| `{{site.title}}` | Site title | "My Nostr Site" |
| `{{site.description}}` | Site description | "Personal gateway" |
| `{{site.operator}}` | Operator name | "Alice" |
| `{{date}}` | Current date | "2025-10-24" |
| `{{datetime}}` | Current date/time | "2025-10-24 15:30:00" |
| `{{year}}` | Current year | "2025" |

**Example with templates:**
```yaml
footers:
  global:
    enabled: true
    content: |
      This gateway is operated by {{site.operator}}
      Last updated: {{datetime}}
      © {{year}} - All rights reserved
```

 

---

## behavior

Query behavior, content filtering, and sorting preferences.

```yaml
behavior:
  content_filtering:
    enabled: false             # Master switch for content filtering
    min_reactions: 0           # Minimum reactions to display note
    min_zap_sats: 0            # Minimum sats zapped to display note
    min_engagement: 0          # Minimum combined engagement score
    hide_no_interactions: false # Hide notes with no interactions

  sort_preferences:
    notes: "chronological"     # chronological|engagement|zaps|reactions
    articles: "chronological"
    replies: "chronological"
    mentions: "chronological"

  pagination:
    enabled: false             # Enable pagination (future)
    items_per_page: 50
    max_pages: 10
```

### behavior.content_filtering

Filter content based on engagement thresholds.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `false` | Enable content filtering |
| `min_reactions` | int | `0` | Minimum reactions required |
| `min_zap_sats` | int | `0` | Minimum sats zapped required |
| `min_engagement` | int | `0` | Minimum combined engagement score |
| `hide_no_interactions` | bool | `false` | Hide notes with zero interactions |

**Example - show only popular content:**
```yaml
content_filtering:
  enabled: true
  min_reactions: 5        # At least 5 reactions
  min_zap_sats: 1000      # At least 1000 sats zapped
  min_engagement: 10      # Engagement score >= 10
```

**Engagement score calculation:**
- 1 point per reaction
- 1 point per 100 sats zapped
- 2 points per reply

**Example - hide unpopular content:**
```yaml
content_filtering:
  enabled: true
  hide_no_interactions: true  # Only show notes with some interaction
```

### behavior.sort_preferences

Control how content is sorted in each section.

| Field | Type | Default | Options |
|-------|------|---------|---------|
| `notes` | string | `chronological` | `chronological`, `engagement`, `zaps`, `reactions` |
| `articles` | string | `chronological` | `chronological`, `engagement`, `zaps`, `reactions` |
| `replies` | string | `chronological` | `chronological`, `engagement`, `zaps`, `reactions` |
| `mentions` | string | `chronological` | `chronological`, `engagement`, `zaps`, `reactions` |

**Sort modes:**
- `chronological`: Newest first (by created_at timestamp)
- `engagement`: Most engaged first (by total engagement score)
- `zaps`: Most zapped first (by total sats)
- `reactions`: Most reacted first (by reaction count)

**Example - engagement-based sorting:**
```yaml
sort_preferences:
  notes: "engagement"    # Show most engaged notes first
  articles: "zaps"       # Show most zapped articles first
  replies: "chronological" # Keep replies in chronological order
```

### behavior.pagination

 

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `false` | Enable pagination |
| `items_per_page` | int | `50` | Items per page |
| `max_pages` | int | `10` | Maximum pages to generate |

 

 

---

## Environment Variable Overrides

Any configuration value can be overridden with `NOPHR_*` environment variables.

**Important overrides:**

| Variable | Overrides | Example |
|----------|-----------|---------|
| `NOPHR_NSEC` | `identity.nsec` | `nsec1abc...` |
| `NOPHR_REDIS_URL` | `caching.redis_url` | `redis://localhost:6379` |

**Example:**
```bash
export NOPHR_NSEC="nsec1..."
export NOPHR_REDIS_URL="redis://localhost:6379"
nophr --config nophr.yaml
```

---

## Validation

Configuration is validated on startup. Common errors:

| Error | Fix |
|-------|-----|
| `identity.npub is required` | Set `identity.npub` in config |
| `identity.npub must start with 'npub1'` | Use valid npub (check format) |
| `at least one protocol must be enabled` | Enable at least one of gopher/gemini/finger |
| `port must be between 1 and 65535` | Fix port number |
| `relay seed must start with ws:// or wss://` | Fix relay URL format |
| `invalid sync mode` | Use: `self`, `following`, `mutual`, or `foaf` |
| `invalid storage driver` | Use: `sqlite` or `lmdb` |
| `invalid log level` | Use: `debug`, `info`, `warn`, or `error` |

**Validate config without starting servers:**
```bash
 
```

For now, config is validated on startup. Check output for validation errors.

---

## Complete Example

See [configs/nophr.example.yaml](../configs/nophr.example.yaml) for a complete, commented example configuration.

---

**Next:** [Storage Guide](storage.md) | [Protocol Servers](protocols.md) | [Getting Started](getting-started.md)
