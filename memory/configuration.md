Configuration

Principles
- Config-first: everything is configurable; sensible defaults are provided.
- Env overrides: any config key may be overridden by env vars (prefix NOPHR_...).
- Secrets: never store secrets in files. Keep nsec and DB credentials in env only.

Top-level keys (schema outline)
- site: title, description, operator info
- identity: npub (file), nsec (env only)
- protocols: gopher, gemini, finger server settings
- relays: seed list used only for kinds 0/3/10002 and discovery policy
- discovery: refresh cadence, fallbacks, connection policies
- sync: kinds, scope, inclusions, caps, retention, pruning
- inbox/outbox: inclusion flags and rollup settings
- storage: driver, sqlite path or postgres url
- rendering: protocol-specific rendering options
- logging: level
- layout: sections and pages for protocol views

Example (YAML)

site:
  title: "My Notes"
  description: "Personal Nostr gopherhole"
  operator: "Alice"

identity:
  npub: "npub1..."           # Required
  # NOPHR_NSEC in env only

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
      cert_path: "./certs/cert.pem"  # or auto-generate self-signed
      key_path: "./certs/key.pem"
      auto_generate: true
  finger:
    enabled: true
    port: 79
    bind: "0.0.0.0"
    max_users: 100               # limit finger queries to owner + top N followed

export:
  gopher:
    enabled: false               # generate static gopher holes when true
    output_dir: "./export/gopher"
    host: "gopher.example.com"   # host/port used in generated gophermaps
    port: 70
    max_items: 200               # cap items per section to keep maps small

relays:
  seeds:
    - "wss://relay.example1"
    - "wss://relay.example2"
  policy:
    connect_timeout_ms: 5000
    max_concurrent_subs: 8
    backoff_ms: [500, 1500, 5000]

discovery:
  refresh_seconds: 900        # How often to refresh kind 10002 (NIP-65)
  use_owner_hints: true       # Use owner's 10002 for owner data
  use_author_hints: true      # Use authors' 10002 for their data
  fallback_to_seeds: true     # When hints are missing/stale
  max_relays_per_author: 8    # Safety cap

sync:
  kinds: [0,1,3,6,7,9735,30023,10002]
  scope:
    mode: "foaf"             # self|following|mutual|foaf
    depth: 2                  # used when mode=foaf
    include_direct_mentions: true
    include_threads_of_mine: true
    max_authors: 5000
    allowlist_pubkeys: []
    denylist_pubkeys: []
  retention:
    keep_days: 365
    prune_on_start: true

    # Advanced retention (optional, see memory/retention_advanced.md)
    advanced:
      enabled: false         # Must explicitly enable
      mode: "rules"          # rules|caps

      evaluation:
        on_ingest: true
        re_eval_interval_hours: 168
        batch_size: 1000

      global_caps:
        max_total_events: 1000000
        max_storage_mb: 5000
        max_events_per_kind:
          1: 100000
          30023: 10000

      rules:
        - name: "protect_owner"
          description: "Never delete owner's content"
          priority: 1000
          conditions:
            author_is_owner: true
          action:
            retain: true

        - name: "close_network"
          description: "Keep direct follows for 1 year"
          priority: 800
          conditions:
            social_distance_max: 1
            kinds: [1, 30023]
          action:
            retain_days: 365

        - name: "default"
          description: "Default retention for other events"
          priority: 100
          conditions:
            all: true
          action:
            retain_days: 90

inbox:
  include_replies: true
  include_reactions: true     # kind 7
  include_zaps: true          # kind 9735
  group_by_thread: true
  collapse_reposts: true
  noise_filters:
    min_zap_sats: 1
    allowed_reaction_chars: ["+"]

outbox:
  publish:
    notes: true
    reactions: false
    zaps: false
  draft_dir: "./content"
  auto_sign: false

storage:
  driver: "sqlite"           # sqlite|lmdb (via Khatru eventstore)
  sqlite_path: "./data/nophr.db"
  lmdb_path: "./data/nophr.lmdb"  # if driver=lmdb
  lmdb_max_size_mb: 10240    # max DB size for LMDB (10GB default)

rendering:
  gopher:
    max_line_length: 70        # wrap text for gopher clients
    show_timestamps: true
    date_format: "2006-01-02 15:04 MST"
    thread_indent: "  "
  gemini:
    max_line_length: 80
    show_timestamps: true
    emoji: true                # allow emoji in gemtext
  finger:
    plan_source: "kind_0"      # use kind 0 (profile) about field as .plan
    recent_notes_count: 5      # show last N notes in finger response

logging:
  level: "info"

layout:
  # See layouts_sections.md for full spec
  sections: {}
  pages: {}

Notes
- protocols: configure which servers to run (Gopher/Gemini/Finger); can run all three simultaneously.
- Gemini TLS auto-generate writes to tls.cert_path/key_path; if those paths are not writable, Gemini falls back to an in-memory self-signed cert and logs a warning—set writable paths to persist.
- export.gopher: when enabled, new owner root notes (kind 1 without e tags) and articles (kind 30023) trigger static gopher hole regeneration into output_dir; host/port fields control the generated selectors.
- storage: uses Khatru (Go relay framework) with eventstore backend (SQLite or LMDB); no PostgreSQL.
- relays.seeds are bootstrap-only endpoints to find kinds 0/3/10002 from Nostr network.
- discovery builds the active relay set per pubkey from NIP-65 (kind 10002).
- sync.scope limits which authors and threads are synchronized and stored in local Khatru instance.
- rendering: protocol-specific formatting options for Gopher menus, Gemini gemtext, and Finger responses.
- layout controls how content is presented via query-driven sections across protocols.

caching:
  enabled: true                 # master switch
  engine: "memory"             # memory|redis
  redis_url: ""               # via env if engine=redis
  ttl:
    sections:
      notes: 60                 # seconds
      comments: 30
      articles: 300
      interactions: 10
    render:
      gopher_menu: 300          # cache gophermap generation
      gemini_page: 300          # cache gemtext rendering
      finger_response: 60       # cache finger queries
      kind_1: 86400             # 24h
      kind_30023: 604800        # 7d
      kind_0: 3600              # 1h
      kind_3: 600               # 10m
  aggregates:
    enabled: true
    update_on_ingest: true
    reconciler_interval_seconds: 900
  overrides:
    # per-section overrides by id (optional)
    # notes: { ttl: 120 }
