# Phase 10: Caching Layer - Completion Report

## Overview

Phase 10 implemented a comprehensive caching system to dramatically improve protocol server performance by caching rendered responses and reducing database queries.

**Status**: ✅ Complete

**Date Completed**: 2025-10-24

## Deliverables

### 1. Cache Interface and Base Types ✅

**File**: `internal/cache/cache.go`

**Features**:
- Unified `Cache` interface for all implementations
- Get, Set, Delete, Clear, Has operations
- Statistics tracking (hits, misses, evictions, timing)
- TTL-based expiration
- Configurable cache engines
- NullCache for disabled caching

**Interface**:
```go
type Cache interface {
    Get(ctx context.Context, key string) ([]byte, bool, error)
    Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
    Delete(ctx context.Context, key string) error
    Clear(ctx context.Context) error
    Has(ctx context.Context, key string) (bool, error)
    Stats(ctx context.Context) (*Stats, error)
    Close() error
}
```

**Statistics**:
```go
type Stats struct {
    Hits          int64
    Misses        int64
    Keys          int64
    SizeBytes     int64
    Evictions     int64
    HitRate       float64
    AvgGetTimeMs  float64
    AvgSetTimeMs  float64
}
```

### 2. In-Memory Cache Engine ✅

**File**: `internal/cache/memory.go`

**Features**:
- Thread-safe with RWMutex
- LRU eviction when size limit reached
- Automatic cleanup of expired entries
- Per-entry access tracking and hit counting
- Configurable cleanup interval
- Size-based capacity management

**Key Features**:
- **Expiration**: Automatic removal of expired entries
- **Eviction**: LRU-based eviction when max size exceeded
- **Cleanup Loop**: Background goroutine removes expired entries
- **Statistics**: Tracks hits, misses, evictions, and timing

**Configuration**:
```yaml
caching:
  enabled: true
  engine: "memory"
  max_size_mb: 100
  default_ttl_seconds: 300
  cleanup_interval_seconds: 60
```

### 3. Redis Cache Engine ✅

**File**: `internal/cache/redis.go`

**Features**:
- Redis client integration via `go-redis/v9`
- Connection pooling and automatic reconnection
- Native Redis TTL support
- Pattern-based key scanning
- INFO stats integration
- Production-ready for distributed caching

**Configuration**:
```yaml
caching:
  enabled: true
  engine: "redis"
  redis_url: "redis://localhost:6379/0"
  default_ttl_seconds: 300
```

**Advantages**:
- Shared cache across multiple nophr instances
- Persistent cache across restarts
- Better memory management
- Built-in clustering support

### 4. Cache Key Generation ✅

**File**: `internal/cache/keys.go`

**Features**:
- Hierarchical key structure with colons
- Protocol-specific key builders
- Hash-based key truncation for long keys
- Pattern matching support for bulk operations
- Automatic invalidation pattern generation

**Key Formats**:
```
gopher:/path/to/selector        -> gopher:/path/to/selector
gemini:/path?query=test         -> gemini:/path:q:query=test
finger:username                 -> finger:username
event:event123:gopher:text      -> event:event123:gopher:text
section:notes:gemini:p2         -> section:notes:gemini:p2
thread:root123:gopher           -> thread:root123:gopher
profile:pubkey123:gemini        -> profile:pubkey123:gemini
aggregate:event123              -> aggregate:event123
kind0:pubkey123                 -> kind0:pubkey123
kind3:pubkey123                 -> kind3:pubkey123
```

**Pattern Matching**:
```
gopher:*                        - All Gopher responses
gemini:*                        - All Gemini responses
event:event123:*                - All renderings of event123
profile:pubkey123:*             - All renderings of profile
```

### 5. Cache Invalidation Logic ✅

**File**: `internal/cache/invalidator.go`

**Features**:
- Event-based automatic invalidation
- Pattern-based bulk invalidation
- Protocol-specific invalidation
- Profile and section invalidation
- Cache warming utilities

**Invalidation Triggers**:
- **Kind 0 (Profile)**: Invalidates profile and kind0 caches
- **Kind 1 (Note)**: Invalidates notes section
- **Kind 3 (Contacts)**: Invalidates kind3 cache
- **Kind 7 (Reaction)**: Invalidates parent event aggregates
- **Kind 9735 (Zap)**: Invalidates parent event aggregates

**Usage**:
```go
invalidator := cache.NewInvalidator(cache)

// Automatic invalidation on event ingestion
invalidator.OnEventIngested(ctx, event)

// Manual invalidation
invalidator.InvalidateGopher(ctx)
invalidator.InvalidateProfile(ctx, pubkey)
invalidator.InvalidateSection(ctx, "notes")
```

**Cache Warming**:
```go
warmer := cache.NewWarmer(cache)

// Pre-populate frequently accessed pages
warmer.WarmGopherHome(ctx, content, 5*time.Minute)
warmer.WarmProfile(ctx, pubkey, "gemini", content, 10*time.Minute)
```

### 6. Tests ✅

**Files**:
- `internal/cache/cache_test.go` - Cache operations testing
- `internal/cache/keys_test.go` - Key generation testing

**Coverage**:
- Cache CRUD operations
- TTL expiration
- Eviction policies
- Statistics tracking
- Cleanup mechanisms
- Key builder functionality
- Pattern matching
- Hash consistency

**Test Results**:
```bash
$ go test ./internal/cache/...
ok      github.com/sandwichfarm/nophr/internal/cache    0.5s
```

## File Structure

```
internal/cache/
├── cache.go           # Cache interface and factory
├── memory.go          # In-memory cache implementation
├── redis.go           # Redis cache implementation
├── keys.go            # Key generation and patterns
├── invalidator.go     # Cache invalidation logic
├── cache_test.go      # Cache operation tests
└── keys_test.go       # Key generation tests
```

## Integration with Protocol Servers

The cache integrates with protocol servers as follows:

### Gopher Server Integration

```go
// In gopher server request handler
func (s *Server) handleRequest(selector string) ([]byte, error) {
    // Try cache first
    cacheKey := cache.GopherKey(selector)
    if cached, hit, err := s.cache.Get(ctx, cacheKey); hit && err == nil {
        return cached, nil
    }

    // Generate response
    response := s.generateResponse(selector)

    // Cache the response
    ttl := s.config.Caching.TTL.Render["gopher_menu"]
    s.cache.Set(ctx, cacheKey, response, time.Duration(ttl)*time.Second)

    return response, nil
}
```

### Gemini Server Integration

```go
// In gemini server request handler
func (s *Server) handleRequest(path, query string) ([]byte, error) {
    // Try cache first
    cacheKey := cache.GeminiKey(path, query)
    if cached, hit, err := s.cache.Get(ctx, cacheKey); hit && err == nil {
        return cached, nil
    }

    // Generate response
    response := s.generateResponse(path, query)

    // Cache the response
    ttl := s.config.Caching.TTL.Render["gemini_page"]
    s.cache.Set(ctx, cacheKey, response, time.Duration(ttl)*time.Second)

    return response, nil
}
```

### Finger Server Integration

```go
// In finger server request handler
func (s *Server) handleQuery(username string) ([]byte, error) {
    // Try cache first
    cacheKey := cache.FingerKey(username)
    if cached, hit, err := s.cache.Get(ctx, cacheKey); hit && err == nil {
        return cached, nil
    }

    // Generate response
    response := s.generateResponse(username)

    // Cache the response
    ttl := s.config.Caching.TTL.Render["finger_response"]
    s.cache.Set(ctx, cacheKey, response, time.Duration(ttl)*time.Second)

    return response, nil
}
```

### Event Ingestion Integration

```go
// In sync engine after storing event
func (e *Engine) onEventStored(event *nostr.Event) {
    // Automatically invalidate related cache entries
    if e.invalidator != nil {
        e.invalidator.OnEventIngested(ctx, event)
    }
}
```

## Configuration

### Complete Configuration Example

```yaml
caching:
  enabled: true
  engine: "memory"  # or "redis"
  redis_url: "redis://localhost:6379/0"  # if engine=redis
  max_size_mb: 100  # for memory cache
  default_ttl_seconds: 300

  ttl:
    sections:
      notes: 60        # seconds
      comments: 30
      articles: 300
      interactions: 10
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

## Usage Examples

### Basic Setup

```go
// Create cache
cacheConfig := &cache.Config{
    Enabled:         cfg.Caching.Enabled,
    Engine:          cfg.Caching.Engine,
    RedisURL:        cfg.Caching.RedisURL,
    MaxSize:         int64(cfg.Caching.MaxSizeMB) * 1024 * 1024,
    DefaultTTL:      5 * time.Minute,
    CleanupInterval: 1 * time.Minute,
}

cache, err := cache.New(cacheConfig)
if err != nil {
    log.Fatal(err)
}
defer cache.Close()

// Create invalidator
invalidator := cache.NewInvalidator(cache)

// Pass to protocol servers
gopherServer := gopher.NewServer(cfg, storage, cache)
geminiServer := gemini.NewServer(cfg, storage, cache)
```

### Cache Operations

```go
// Set
key := cache.GopherKey("/")
content := []byte("home page content")
cache.Set(ctx, key, content, 5*time.Minute)

// Get
if cached, hit, err := cache.Get(ctx, key); hit {
    // Use cached content
    return cached, nil
}

// Delete
cache.Delete(ctx, key)

// Clear all
cache.Clear(ctx)

// Check existence
exists, _ := cache.Has(ctx, key)

// Get statistics
stats, _ := cache.Stats(ctx)
fmt.Printf("Hit rate: %.2f%%\n", stats.HitRate * 100)
```

### Invalidation

```go
// Invalidate on event ingestion
invalidator.OnEventIngested(ctx, event)

// Manual invalidation
invalidator.InvalidateGopher(ctx)
invalidator.InvalidateProfile(ctx, pubkey)
invalidator.InvalidateSection(ctx, "notes")

// Clear everything
invalidator.InvalidateAll(ctx)
```

## Performance Impact

### Expected Improvements

With caching enabled:
- **Response Time**: 10-100x faster for cached responses
- **Database Load**: 80-95% reduction in queries
- **CPU Usage**: 50-70% reduction for rendering
- **Throughput**: 5-10x increase in requests/second

### Benchmark Results (Estimated)

```
Without Cache:
  Gopher home page:     50ms
  Gemini profile:       100ms
  Thread rendering:     200ms
  Database queries/req: 5-10

With Memory Cache:
  Gopher home page:     0.5ms   (100x faster)
  Gemini profile:       1ms     (100x faster)
  Thread rendering:     2ms     (100x faster)
  Database queries/req: 0 (cache hit)

Cache Stats After 1000 Requests:
  Hits: 950
  Misses: 50
  Hit Rate: 95%
  Avg Get Time: 0.3ms
```

## Cache Size Estimation

Typical cache entries:
- Gopher menu: 1-5 KB
- Gemini page: 2-10 KB
- Profile: 1-3 KB
- Thread: 5-50 KB

For 1000 cached entries:
- Average: 5 KB per entry
- Total: ~5 MB
- Recommended: 100 MB for safety

## Eviction and TTL Strategy

### TTL Guidelines

- **Static Content** (profiles, kind 0): 1 hour
- **Semi-Static** (menus, sections): 5 minutes
- **Dynamic** (threads, interactions): 1 minute
- **High-Traffic** (home pages): 5 minutes

### Eviction Strategy

Memory cache uses LRU (Least Recently Used):
1. Track access time for each entry
2. When size limit exceeded, remove oldest entries
3. Continue until enough space available

## Monitoring

### Cache Statistics

Monitor these metrics:
- **Hit Rate**: Should be > 80% for good caching
- **Evictions**: High evictions indicate cache too small
- **Average Times**: Should be < 1ms for memory cache
- **Size**: Should stay under configured max

### Logging

```go
stats, _ := cache.Stats(ctx)
logger.Info("cache stats",
    "hits", stats.Hits,
    "misses", stats.Misses,
    "hit_rate", stats.HitRate,
    "keys", stats.Keys,
    "size_mb", stats.SizeBytes / 1024 / 1024,
    "evictions", stats.Evictions)
```

## Future Enhancements

- [ ] Cache compression for large entries
- [ ] Distributed cache invalidation (pub/sub)
- [ ] Cache warming on startup
- [ ] Adaptive TTL based on access patterns
- [ ] Memcached support
- [ ] Cache tiering (L1 memory, L2 Redis)
- [ ] Per-user cache quotas
- [ ] Cache analytics and reporting

## Completion Criteria

All Phase 10 requirements have been met:

- [x] Cache interface defined
- [x] In-memory cache implementation
- [x] Redis cache implementation (optional)
- [x] Per-protocol TTL configuration
- [x] Cache key generation (hash-based)
- [x] Invalidation on new events
- [x] Cache warming strategies
- [x] Tests for cache behavior
- [x] Integration points documented

## Next Phase

**Phase 11: Sections and Layouts** - Implement configurable sections and page layouts for better content organization and navigation.

## References

- go-redis: https://github.com/redis/go-redis
- Redis Documentation: https://redis.io/docs/
- LRU Cache Algorithm: https://en.wikipedia.org/wiki/Cache_replacement_policies#Least_recently_used_(LRU)
