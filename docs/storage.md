# Storage Guide

SQLite is supported. LMDB is not supported in this build.

Complete guide to nophr's storage layer, database backends, and data management.

## Overview

nophr uses [Khatru](https://github.com/fiatjaf/khatru) as an embedded Nostr relay for event storage. Khatru provides battle-tested event storage, querying, deduplication, and NIP compliance.

**Key Concepts:**
- **Khatru**: Embedded Nostr relay (library, not separate service)
- **eventstore**: Pluggable database backend (this build uses SQLite; LMDB and other backends are conceptual only)
- **Custom tables**: nophr-specific data (relay hints, social graph, sync state, aggregates)

**Architecture:**
```
┌─────────────────────┐
│  Sync Engine        │  ← Pulls events from remote relays
└──────┬──────────────┘
       │ StoreEvent()
       ↓
┌─────────────────────┐
│  Khatru (embedded)  │  ← Event storage, validation, querying
└──────┬──────────────┘
       │ eventstore interface
       ↓
┌─────────────────────┐
│  SQLite             │  ← Database file (LMDB planned, not available in this build)
└─────────────────────┘
```

---

## Database Backends

nophr uses Khatru's [eventstore](https://github.com/fiatjaf/eventstore) plugin system. This build supports SQLite. LMDB and other backends discussed below describe planned or hypothetical configurations and cannot be used with current binaries.

### SQLite (Default)

**Characteristics:**
- Single `.db` file
- Zero configuration
- Excellent for <100K events
- Simple backups (copy file)
- Limited concurrent writes

**Configuration:**
```yaml
storage:
  driver: "sqlite"
  sqlite_path: "./data/nophr.db"
```

**File location:**
```bash
# Default location
./data/nophr.db

# Custom location
/var/lib/nophr/nophr.db
```

**Best for:**
- Personal use
- Single-tenant deployments
- <100K events
- Simple setup

### LMDB (Alternative, not available in this build)

LMDB is not supported in this build. The configuration options exist for future compatibility, but selecting `driver: "lmdb"` currently fails with a clear error and storage does not initialize.

**Characteristics:**
- Directory with data files
- Zero configuration
- Excellent for millions of events
- High concurrent write performance
- Requires max size setting

**Configuration:**
```yaml
storage:
  driver: "lmdb"
  lmdb_path: "./data/nophr.lmdb"
  lmdb_max_size_mb: 10240  # 10GB max
```

**Directory structure:**
```bash
./data/nophr.lmdb/
├── data.mdb      # Main data file
└── lock.mdb      # Lock file
```

**Best for (when implemented):**
- High-volume event syncing
- >100K events
- Need for high write throughput
- Streaming use cases

**LMDB Max Size:**
- Set `lmdb_max_size_mb` to expected database size
- Default: 10GB (10240 MB)
- Cannot grow beyond max size (plan accordingly)
- Suggested: 2-3x expected event count * ~1KB per event

**Example sizing:**
- 100K events: 2GB → set `lmdb_max_size_mb: 2048`
- 1M events: 5GB → set `lmdb_max_size_mb: 5120`
- 10M events: 20GB → set `lmdb_max_size_mb: 20480`

---

## Comparison: SQLite vs LMDB (design)

| Feature | SQLite | LMDB |
|---------|--------|------|
| **File format** | Single .db file | Directory |
| **Setup complexity** | Zero config | Zero config (set max size) |
| **Performance (<100K)** | Excellent | Excellent |
| **Performance (>1M)** | Good | Excellent |
| **Concurrent writes** | Limited | Excellent |
| **Disk space** | Grows automatically | Fixed maximum |
| **Backups** | Copy .db file | Copy directory |
| **Corruption risk** | Low | Very low |
| **Portability** | High (single file) | Medium (directory) |
| **Best use case** | Personal, <100K events | High-volume, streaming |

**Recommendation (for this build):**
- **Use SQLite** – it is the only supported backend.

LMDB details in this guide describe the intended behavior for future builds; they are not active in current releases.

---

## What Khatru Handles

Khatru provides the core Nostr relay functionality:

1. **Event Storage**
   - Stores events with efficient indexing
   - Deduplication (rejects duplicate event IDs)
   - Signature verification

2. **Replaceable Events**
   - Handles replaceable events (kinds 0, 3, 10002)
   - Handles parameterized replaceable events (kind 30023)
   - Keeps only latest per pubkey/kind/d-tag

3. **Querying**
   - Standard Nostr filter queries (authors, kinds, #e, #p tags)
   - Since/until time ranges
   - Limit/offset pagination

4. **NIP Compliance**
   - NIP-01: Basic protocol
   - NIP-10: Threading tags
   - NIP-33: Parameterized replaceable events
   - And more...

**What you DON'T need to implement:**
- Event validation logic
- Replaceable event handling
- Index management
- Signature verification

**Implementation:** `internal/storage/storage.go`, `internal/storage/sqlite.go`, `internal/storage/lmdb.go` (LMDB is stubbed and returns a not-implemented error in this build)

---

## Custom Tables

nophr adds custom tables for features beyond basic event storage:

### 1. relay_hints

Tracks relay hints from NIP-65 (kind 10002).

```sql
CREATE TABLE relay_hints (
  pubkey TEXT NOT NULL,
  relay TEXT NOT NULL,
  can_read INTEGER NOT NULL,    -- 0 or 1
  can_write INTEGER NOT NULL,   -- 0 or 1
  freshness INTEGER NOT NULL,   -- created_at of hint event
  last_seen_event_id TEXT NOT NULL,
  PRIMARY KEY (pubkey, relay)
);
CREATE INDEX idx_relay_hints_pubkey ON relay_hints(pubkey, freshness DESC);
```

**Purpose:**
- Determine which relays to query for each author
- Built from kind 10002 events
- Used by relay discovery

**Example:**
```
pubkey: npub1abc...
relay: wss://relay.damus.io
can_read: 1
can_write: 0
freshness: 1698765432
```

### 2. graph_nodes

Owner-centric social graph cache.

```sql
CREATE TABLE graph_nodes (
  root_pubkey TEXT NOT NULL,    -- the owner
  pubkey TEXT NOT NULL,
  depth INTEGER NOT NULL,       -- FOAF distance from owner
  mutual INTEGER NOT NULL,      -- 0 or 1 (bidirectional follow)
  last_seen INTEGER NOT NULL,
  PRIMARY KEY (root_pubkey, pubkey)
);
CREATE INDEX idx_graph_nodes ON graph_nodes(root_pubkey, depth, mutual);
```

**Purpose:**
- Efficiently determine which authors are in sync scope
- Supports following/mutual/FOAF modes
- Computed from kind 3 (contacts) events

**Example:**
```
root_pubkey: npub1owner...
pubkey: npub1friend...
depth: 1        -- direct follow
mutual: 1       -- they follow back
```

### 3. sync_state

Cursor tracking per relay/kind.

```sql
CREATE TABLE sync_state (
  relay TEXT NOT NULL,
  kind INTEGER NOT NULL,
  since INTEGER NOT NULL,       -- since cursor for subscriptions
  updated_at INTEGER NOT NULL,
  PRIMARY KEY (relay, kind)
);
```

**Purpose:**
- Avoid re-syncing old events
- Track progress per relay
- Resume sync after restart

**Example:**
```
relay: wss://relay.damus.io
kind: 1
since: 1698765432      -- only fetch events after this timestamp
updated_at: 1698765500
```

### 4. aggregates

Interaction rollup cache.

```sql
CREATE TABLE aggregates (
  event_id TEXT PRIMARY KEY,
  reply_count INTEGER,
  reaction_total INTEGER,
  reaction_counts_json TEXT,    -- JSON map: char -> count
  zap_sats_total INTEGER,
  last_interaction_at INTEGER
);
```

**Purpose:**
- Cache interaction counts for display
- Avoid re-computing on every page load
- Updated on event ingestion and periodically reconciled

**Example:**
```
event_id: abc123...
reply_count: 5
reaction_total: 12
reaction_counts_json: {"+": 10, "❤️": 2}
zap_sats_total: 21000   -- 21 sats
last_interaction_at: 1698765500
```

**Implementation:** `internal/storage/relay_hints.go`, `internal/storage/graph_nodes.go`, `internal/storage/sync_state.go`, `internal/storage/aggregates.go`

---

## Database Initialization

nophr automatically initializes the database on first run.

**What happens:**
1. Check if database file/directory exists
2. If not, create it
3. Run migrations (create custom tables)
4. Initialize Khatru eventstore
5. Ready for event storage

**Manual initialization (if needed):**
```bash
# Ensure data directory exists
mkdir -p ./data

# Run nophr (will initialize on startup)
./dist/nophr --config nophr.yaml
```

**Output:**
```
Initializing storage...
  Storage: sqlite initialized
```

**Migration files:** `internal/storage/migrations.go`

---

## Backup and Restore

### SQLite Backups

**Hot backup (while running):**
```bash
sqlite3 ./data/nophr.db ".backup ./backups/nophr-$(date +%Y%m%d).db"
```

**Cold backup (nophr stopped):**
```bash
cp ./data/nophr.db ./backups/nophr-$(date +%Y%m%d).db
```

**Restore:**
```bash
cp ./backups/nophr-20251024.db ./data/nophr.db
```

### LMDB Backups (future)

LMDB is not supported in this build. When LMDB support is added, backups will look similar to:

**Cold backup (nophr stopped):**
```bash
cp -r ./data/nophr.lmdb ./backups/nophr-$(date +%Y%m%d).lmdb
```

**Hot backup:**
LMDB doesn't support hot backups easily. Stop nophr first.

**Restore:**
```bash
rm -rf ./data/nophr.lmdb
cp -r ./backups/nophr-20251024.lmdb ./data/nophr.lmdb
```

**Automated backups:**
```bash
# Cron job (daily at 2am)
0 2 * * * /usr/local/bin/nophr-backup.sh
```

**Example backup script:**
```bash
#!/bin/bash
# nophr-backup.sh
BACKUP_DIR="/var/backups/nophr"
DATE=$(date +%Y%m%d)
cp ./data/nophr.db "$BACKUP_DIR/nophr-$DATE.db"
# Keep last 7 days
find "$BACKUP_DIR" -name "nophr-*.db" -mtime +7 -delete
```

---

## Database Maintenance

### Retention and Pruning

Events older than `sync.retention.keep_days` are automatically pruned.

**Configuration:**
```yaml
sync:
  retention:
    keep_days: 365
    prune_on_start: true
```

**What gets pruned:**
- Events older than `keep_days`
- Except: kind 0 (profiles), kind 3 (follows) - never pruned
- Replaceable events: only latest kept anyway

Manual pruning via CLI is not available in this build.

### Vacuum (SQLite Only)

Reclaim disk space after deleting events:

```bash
sqlite3 ./data/nophr.db "VACUUM;"
```

**Automated vacuum:**
```bash
# Weekly vacuum (Sunday 3am)
0 3 * * 0 sqlite3 /path/to/nophr.db "VACUUM;"
```

### Database Size Monitoring

**SQLite:**
```bash
du -h ./data/nophr.db
```

**Check event count:**
```bash
sqlite3 ./data/nophr.db "SELECT COUNT(*) FROM events;"
```

---

## Switching Backends

Only SQLite is supported in this build. Selecting any other `storage.driver` will cause startup to fail.

---

## Troubleshooting

### "failed to initialize storage: unable to open database file"

**Cause:** Data directory doesn't exist.

**Fix:**
```bash
mkdir -p ./data
```

### "LMDB: database full"

LMDB is not supported in this build. If you configure `driver: "lmdb"`, nophr will fail to start before this error could occur. The following settings are reserved for future LMDB support:

**Planned fix:** Increase max size in config:
```yaml
storage:
  lmdb_max_size_mb: 20480  # Increase to 20GB
```

Restart nophr.

### "database is locked" (SQLite)

**Cause:** Another process has database open, or unclean shutdown.

**Fix:**
1. Ensure only one nophr instance running
2. Check for stale lock files
3. Restart nophr

### Corrupted database

**SQLite integrity check:**
```bash
sqlite3 ./data/nophr.db "PRAGMA integrity_check;"
```

**If corrupted:**
1. Restore from backup
2. Or delete database and re-sync from relays

---

## Performance Tuning

### SQLite Optimizations

**Pragmas (already set by eventstore):**
```sql
PRAGMA journal_mode = WAL;      -- Write-Ahead Logging
PRAGMA synchronous = NORMAL;    -- Faster commits
PRAGMA cache_size = -64000;     -- 64MB cache
```

**Custom indexes (if needed):**
```sql
-- Speed up specific queries
CREATE INDEX idx_events_kind_created ON events(kind, created_at DESC);
```

### LMDB Optimizations (future)

LMDB-related tuning (max size, sync modes, etc.) applies only once LMDB support is implemented. For this build, these notes are informational only.

---

## Monitoring

**Database size growth:**
```bash
watch -n 60 du -h ./data/nophr.db
```

**Event count growth:**
```bash
watch -n 60 'sqlite3 ./data/nophr.db "SELECT COUNT(*) FROM events;"'
```

**Custom table sizes:**
```bash
sqlite3 ./data/nophr.db <<EOF
SELECT 'relay_hints', COUNT(*) FROM relay_hints;
SELECT 'graph_nodes', COUNT(*) FROM graph_nodes;
SELECT 'sync_state', COUNT(*) FROM sync_state;
SELECT 'aggregates', COUNT(*) FROM aggregates;
EOF
```

---

## Advanced Topics

### Direct SQL Queries (Read-Only)

**Inspect stored events:**
```bash
sqlite3 ./data/nophr.db "SELECT id, kind, created_at, pubkey FROM events LIMIT 10;"
```

**Check relay hints:**
```bash
sqlite3 ./data/nophr.db "SELECT * FROM relay_hints WHERE pubkey = 'hex_pubkey';"
```

**Warning:** DO NOT modify data directly via SQL. Use nophr's APIs.

### Custom eventstore Implementations

Khatru supports custom eventstore backends. See [eventstore documentation](https://github.com/fiatjaf/eventstore).

**Potential backends:**
- Badger
- LevelDB
- PostgreSQL (custom implementation)

**Currently supported in this build:** SQLite (LMDB and other backends are not yet supported)

---

## References

- **Khatru:** https://github.com/fiatjaf/khatru
- **eventstore:** https://github.com/fiatjaf/eventstore
- **SQLite:** https://www.sqlite.org/
- **LMDB:** https://www.symas.com/lmdb
 

---

**Next:** [Configuration Reference](configuration.md) | [Protocols Guide](protocols.md) | [Architecture Overview](architecture.md)
