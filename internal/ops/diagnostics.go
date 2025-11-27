package ops

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/sandwichfarm/nophr/internal/storage"
	"github.com/sandwichfarm/nophr/internal/sync"
)

// SystemStats contains overall system statistics
type SystemStats struct {
	Version   string
	Commit    string
	Uptime    time.Duration
	StartTime time.Time

	// Runtime stats
	GoVersion      string
	NumGoroutines  int
	MemAllocMB     float64
	MemTotalAllocMB float64
	MemSysMB       float64
	NumGC          uint32
}

// StorageStats contains storage-related statistics
type StorageStats struct {
	Driver          string
	TotalEvents     int64
	EventsByKind    map[int]int64
	DatabaseSizeMB  float64
	OldestEventTime *time.Time
	NewestEventTime *time.Time
}

// SyncStats contains sync engine statistics
type SyncStats struct {
	Enabled         bool
	RelayCount      int
	ConnectedRelays int
	TotalSynced     int64
	LastSyncTime    *time.Time
	Cursors         []CursorInfo
}

// CursorInfo contains cursor information for a relay/kind pair
type CursorInfo struct {
	Relay    string
	Kind     int
	Position int64
	Updated  time.Time
}

// RelayHealth contains health information for a relay
type RelayHealth struct {
	URL         string
	Connected   bool
	LastConnect *time.Time
	LastError   *string
	EventsSynced int64
}

// AggregateStats contains aggregate computation statistics
type AggregateStats struct {
	TotalAggregates int64
	ByKind          map[int]int64
	LastReconcile   *time.Time
}

// Phase 20: RetentionDiagStats contains retention-related diagnostics
type RetentionDiagStats struct {
	Enabled             bool
	AdvancedEnabled     bool
	KeepDays            int
	TotalEvents         int64
	EstimatedPrunable   int64
	TotalProtected      int64
	TotalWithMetadata   int64
	OldestEvent         *time.Time
	NewestEvent         *time.Time
	Cutoff              *time.Time
}

// DiagnosticsCollector collects system diagnostics
type DiagnosticsCollector struct {
	version       string
	commit        string
	startTime     time.Time
	storage       *storage.Storage
	syncEngine    *sync.Engine
	retentionMgr  *RetentionManager // Phase 20
}

// NewDiagnosticsCollector creates a new diagnostics collector
func NewDiagnosticsCollector(version, commit string, st *storage.Storage, syncEng *sync.Engine) *DiagnosticsCollector {
	return &DiagnosticsCollector{
		version:    version,
		commit:     commit,
		startTime:  time.Now(),
		storage:    st,
		syncEngine: syncEng,
	}
}

// SetRetentionManager sets the retention manager for diagnostics (Phase 20)
func (d *DiagnosticsCollector) SetRetentionManager(rm *RetentionManager) {
	d.retentionMgr = rm
}

// CollectSystemStats collects system-level statistics
func (d *DiagnosticsCollector) CollectSystemStats() *SystemStats {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return &SystemStats{
		Version:   d.version,
		Commit:    d.commit,
		Uptime:    time.Since(d.startTime),
		StartTime: d.startTime,

		GoVersion:      runtime.Version(),
		NumGoroutines:  runtime.NumGoroutine(),
		MemAllocMB:     float64(m.Alloc) / 1024 / 1024,
		MemTotalAllocMB: float64(m.TotalAlloc) / 1024 / 1024,
		MemSysMB:       float64(m.Sys) / 1024 / 1024,
		NumGC:          m.NumGC,
	}
}

// CollectStorageStats collects storage-related statistics
func (d *DiagnosticsCollector) CollectStorageStats(ctx context.Context) (*StorageStats, error) {
	stats := &StorageStats{
		Driver:       d.storage.Driver(),
		EventsByKind: make(map[int]int64),
	}

	// Get total event count
	total, err := d.storage.CountEvents(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to count events: %w", err)
	}
	stats.TotalEvents = total

	// Get counts by kind
	kindCounts, err := d.storage.CountEventsByKind(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to count events by kind: %w", err)
	}
	stats.EventsByKind = kindCounts

	// Get database size
	sizeMB, err := d.storage.DatabaseSize(ctx)
	if err == nil {
		stats.DatabaseSizeMB = sizeMB
	}

	// Get time range
	oldest, newest, err := d.storage.EventTimeRange(ctx)
	if err == nil {
		stats.OldestEventTime = oldest
		stats.NewestEventTime = newest
	}

	return stats, nil
}

// CollectSyncStats collects sync engine statistics
func (d *DiagnosticsCollector) CollectSyncStats(ctx context.Context) (*SyncStats, error) {
	if d.syncEngine == nil {
		return &SyncStats{Enabled: false}, nil
	}

	stats := &SyncStats{
		Enabled: true,
	}

	// Get relay health information
	relays := d.syncEngine.GetRelays()
	stats.RelayCount = len(relays)

	for _, relay := range relays {
		if relay.IsConnected() {
			stats.ConnectedRelays++
		}
	}

	// Get total synced count
	total, err := d.syncEngine.TotalSynced(ctx)
	if err == nil {
		stats.TotalSynced = total
	}

	// Get last sync time
	lastSync, err := d.syncEngine.LastSyncTime(ctx)
	if err == nil && lastSync != nil {
		stats.LastSyncTime = lastSync
	}

	// Get cursor information
	cursors, err := d.storage.GetAllCursors(ctx)
	if err == nil {
		stats.Cursors = make([]CursorInfo, 0, len(cursors))
		for _, c := range cursors {
			stats.Cursors = append(stats.Cursors, CursorInfo{
				Relay:    c.Relay,
				Kind:     c.Kind,
				Position: c.Position,
				Updated:  c.Updated,
			})
		}
	}

	return stats, nil
}

// CollectRelayHealth collects relay health information
func (d *DiagnosticsCollector) CollectRelayHealth(ctx context.Context) ([]RelayHealth, error) {
	if d.syncEngine == nil {
		return nil, nil
	}

	relays := d.syncEngine.GetRelays()
	health := make([]RelayHealth, 0, len(relays))

	for _, relay := range relays {
		h := RelayHealth{
			URL:       relay.URL(),
			Connected: relay.IsConnected(),
		}

		if lastConnect := relay.LastConnectTime(); lastConnect != nil {
			h.LastConnect = lastConnect
		}

		if lastErr := relay.LastError(); lastErr != nil {
			errStr := lastErr.Error()
			h.LastError = &errStr
		}

		synced, err := d.storage.CountEventsByRelay(ctx, relay.URL())
		if err == nil {
			h.EventsSynced = synced
		}

		health = append(health, h)
	}

	return health, nil
}

// CollectAggregateStats collects aggregate statistics
func (d *DiagnosticsCollector) CollectAggregateStats(ctx context.Context) (*AggregateStats, error) {
	stats := &AggregateStats{
		ByKind: make(map[int]int64),
	}

	total, err := d.storage.CountAggregates(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to count aggregates: %w", err)
	}
	stats.TotalAggregates = total

	byKind, err := d.storage.CountAggregatesByKind(ctx)
	if err == nil {
		stats.ByKind = byKind
	}

	lastReconcile, err := d.storage.LastReconcileTime(ctx)
	if err == nil && lastReconcile != nil {
		stats.LastReconcile = lastReconcile
	}

	return stats, nil
}

// CollectRetentionStats collects retention statistics (Phase 20)
func (d *DiagnosticsCollector) CollectRetentionStats(ctx context.Context) (*RetentionDiagStats, error) {
	if d.retentionMgr == nil {
		return &RetentionDiagStats{Enabled: false}, nil
	}

	stats := &RetentionDiagStats{
		Enabled: true,
	}

	// Get retention stats from manager
	retStats, err := d.retentionMgr.GetRetentionStats(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get retention stats: %w", err)
	}

	stats.KeepDays = retStats.KeepDays
	stats.TotalEvents = retStats.TotalEvents
	stats.EstimatedPrunable = retStats.EstimatedPrunable
	if !retStats.OldestEvent.IsZero() {
		stats.OldestEvent = &retStats.OldestEvent
	}
	if !retStats.NewestEvent.IsZero() {
		stats.NewestEvent = &retStats.NewestEvent
	}
	if !retStats.Cutoff.IsZero() {
		stats.Cutoff = &retStats.Cutoff
	}

	// Check if advanced retention is enabled
	if d.retentionMgr.config.Advanced != nil && d.retentionMgr.config.Advanced.Enabled {
		stats.AdvancedEnabled = true

		// Get protected event count
		protectedCount, err := d.storage.CountRetentionProtected(ctx)
		if err == nil {
			stats.TotalProtected = protectedCount
		}

		// Get total events with retention metadata
		metadataCount, err := d.storage.CountRetentionMetadata(ctx)
		if err == nil {
			stats.TotalWithMetadata = metadataCount
		}
	}

	return stats, nil
}

// CollectAll collects all diagnostic information
func (d *DiagnosticsCollector) CollectAll(ctx context.Context) (*Diagnostics, error) {
	diag := &Diagnostics{
		CollectedAt: time.Now(),
	}

	// Collect system stats
	diag.System = d.CollectSystemStats()

	// Collect storage stats
	storageStats, err := d.CollectStorageStats(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to collect storage stats: %w", err)
	}
	diag.Storage = storageStats

	// Collect sync stats
	syncStats, err := d.CollectSyncStats(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to collect sync stats: %w", err)
	}
	diag.Sync = syncStats

	// Collect relay health
	relayHealth, err := d.CollectRelayHealth(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to collect relay health: %w", err)
	}
	diag.Relays = relayHealth

	// Collect aggregate stats
	aggStats, err := d.CollectAggregateStats(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to collect aggregate stats: %w", err)
	}
	diag.Aggregates = aggStats

	// Phase 20: Collect retention stats
	retStats, err := d.CollectRetentionStats(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to collect retention stats: %w", err)
	}
	diag.Retention = retStats

	return diag, nil
}

// Diagnostics contains all diagnostic information
type Diagnostics struct {
	CollectedAt time.Time
	System      *SystemStats
	Storage     *StorageStats
	Sync        *SyncStats
	Relays      []RelayHealth
	Aggregates  *AggregateStats
	Retention   *RetentionDiagStats // Phase 20
}

// FormatAsText formats diagnostics as plain text
func (d *Diagnostics) FormatAsText() string {
	var out string

	// System info
	out += fmt.Sprintf("=== nophr Diagnostics ===\n")
	out += fmt.Sprintf("Collected: %s\n\n", d.CollectedAt.Format(time.RFC3339))

	out += fmt.Sprintf("--- System ---\n")
	out += fmt.Sprintf("Version: %s (%s)\n", d.System.Version, d.System.Commit)
	out += fmt.Sprintf("Uptime: %s\n", d.System.Uptime.Round(time.Second))
	out += fmt.Sprintf("Go Version: %s\n", d.System.GoVersion)
	out += fmt.Sprintf("Goroutines: %d\n", d.System.NumGoroutines)
	out += fmt.Sprintf("Memory: %.2f MB allocated, %.2f MB system\n", d.System.MemAllocMB, d.System.MemSysMB)
	out += fmt.Sprintf("GC Runs: %d\n\n", d.System.NumGC)

	// Storage info
	out += fmt.Sprintf("--- Storage ---\n")
	out += fmt.Sprintf("Driver: %s\n", d.Storage.Driver)
	out += fmt.Sprintf("Total Events: %d\n", d.Storage.TotalEvents)
	out += fmt.Sprintf("Database Size: %.2f MB\n", d.Storage.DatabaseSizeMB)
	if d.Storage.OldestEventTime != nil {
		out += fmt.Sprintf("Oldest Event: %s\n", d.Storage.OldestEventTime.Format(time.RFC3339))
	}
	if d.Storage.NewestEventTime != nil {
		out += fmt.Sprintf("Newest Event: %s\n", d.Storage.NewestEventTime.Format(time.RFC3339))
	}
	out += fmt.Sprintf("\nEvents by Kind:\n")
	for kind, count := range d.Storage.EventsByKind {
		out += fmt.Sprintf("  Kind %d: %d events\n", kind, count)
	}
	out += "\n"

	// Sync info
	out += fmt.Sprintf("--- Sync ---\n")
	out += fmt.Sprintf("Enabled: %v\n", d.Sync.Enabled)
	if d.Sync.Enabled {
		out += fmt.Sprintf("Relays: %d total, %d connected\n", d.Sync.RelayCount, d.Sync.ConnectedRelays)
		out += fmt.Sprintf("Total Synced: %d events\n", d.Sync.TotalSynced)
		if d.Sync.LastSyncTime != nil {
			out += fmt.Sprintf("Last Sync: %s\n", d.Sync.LastSyncTime.Format(time.RFC3339))
		}
	}
	out += "\n"

	// Relay health
	if len(d.Relays) > 0 {
		out += fmt.Sprintf("--- Relay Health ---\n")
		for _, relay := range d.Relays {
			status := "disconnected"
			if relay.Connected {
				status = "connected"
			}
			out += fmt.Sprintf("%s: %s\n", relay.URL, status)
			if relay.LastConnect != nil {
				out += fmt.Sprintf("  Last Connect: %s\n", relay.LastConnect.Format(time.RFC3339))
			}
			if relay.LastError != nil {
				out += fmt.Sprintf("  Last Error: %s\n", *relay.LastError)
			}
			out += fmt.Sprintf("  Events Synced: %d\n", relay.EventsSynced)
		}
		out += "\n"
	}

	// Aggregates
	out += fmt.Sprintf("--- Aggregates ---\n")
	out += fmt.Sprintf("Total: %d\n", d.Aggregates.TotalAggregates)
	if d.Aggregates.LastReconcile != nil {
		out += fmt.Sprintf("Last Reconcile: %s\n", d.Aggregates.LastReconcile.Format(time.RFC3339))
	}
	out += "\n"

	// Phase 20: Retention
	out += fmt.Sprintf("--- Retention ---\n")
	if d.Retention != nil {
		out += fmt.Sprintf("Enabled: %v\n", d.Retention.Enabled)
		if d.Retention.Enabled {
			out += fmt.Sprintf("Keep Days: %d\n", d.Retention.KeepDays)
			if d.Retention.Cutoff != nil {
				out += fmt.Sprintf("Cutoff Date: %s\n", d.Retention.Cutoff.Format(time.RFC3339))
			}
			out += fmt.Sprintf("Total Events: %d\n", d.Retention.TotalEvents)
			out += fmt.Sprintf("Estimated Prunable: %d\n", d.Retention.EstimatedPrunable)
			if d.Retention.AdvancedEnabled {
				out += fmt.Sprintf("Advanced Retention: enabled\n")
				out += fmt.Sprintf("  Protected Events: %d\n", d.Retention.TotalProtected)
				out += fmt.Sprintf("  Events with Metadata: %d\n", d.Retention.TotalWithMetadata)
			}
		}
	} else {
		out += fmt.Sprintf("Not configured\n")
	}

	return out
}

// FormatAsGophermap formats diagnostics as a gophermap
func (d *Diagnostics) FormatAsGophermap(host string, port int) string {
	var out string

	out += fmt.Sprintf("inophr Diagnostics\t\t%s\t%d\r\n", host, port)
	out += fmt.Sprintf("i\t\t%s\t%d\r\n", host, port)
	out += fmt.Sprintf("iCollected: %s\t\t%s\t%d\r\n", d.CollectedAt.Format(time.RFC3339), host, port)
	out += fmt.Sprintf("i\t\t%s\t%d\r\n", host, port)

	out += fmt.Sprintf("i=== System ===\t\t%s\t%d\r\n", host, port)
	out += fmt.Sprintf("iVersion: %s\t\t%s\t%d\r\n", d.System.Version, host, port)
	out += fmt.Sprintf("iUptime: %s\t\t%s\t%d\r\n", d.System.Uptime.Round(time.Second), host, port)
	out += fmt.Sprintf("iMemory: %.2f MB\t\t%s\t%d\r\n", d.System.MemAllocMB, host, port)
	out += fmt.Sprintf("i\t\t%s\t%d\r\n", host, port)

	out += fmt.Sprintf("i=== Storage ===\t\t%s\t%d\r\n", host, port)
	out += fmt.Sprintf("iDriver: %s\t\t%s\t%d\r\n", d.Storage.Driver, host, port)
	out += fmt.Sprintf("iTotal Events: %d\t\t%s\t%d\r\n", d.Storage.TotalEvents, host, port)
	out += fmt.Sprintf("iDatabase: %.2f MB\t\t%s\t%d\r\n", d.Storage.DatabaseSizeMB, host, port)

	return out
}

// FormatAsGemtext formats diagnostics as gemtext
func (d *Diagnostics) FormatAsGemtext() string {
	var out string

	out += "# nophr Diagnostics\n\n"
	out += fmt.Sprintf("Collected: %s\n\n", d.CollectedAt.Format(time.RFC3339))

	out += "## System\n\n"
	out += fmt.Sprintf("* Version: %s (%s)\n", d.System.Version, d.System.Commit)
	out += fmt.Sprintf("* Uptime: %s\n", d.System.Uptime.Round(time.Second))
	out += fmt.Sprintf("* Go Version: %s\n", d.System.GoVersion)
	out += fmt.Sprintf("* Goroutines: %d\n", d.System.NumGoroutines)
	out += fmt.Sprintf("* Memory: %.2f MB allocated\n", d.System.MemAllocMB)
	out += "\n"

	out += "## Storage\n\n"
	out += fmt.Sprintf("* Driver: %s\n", d.Storage.Driver)
	out += fmt.Sprintf("* Total Events: %d\n", d.Storage.TotalEvents)
	out += fmt.Sprintf("* Database Size: %.2f MB\n", d.Storage.DatabaseSizeMB)
	out += "\n"

	out += "## Sync\n\n"
	out += fmt.Sprintf("* Enabled: %v\n", d.Sync.Enabled)
	if d.Sync.Enabled {
		out += fmt.Sprintf("* Relays: %d total, %d connected\n", d.Sync.RelayCount, d.Sync.ConnectedRelays)
		out += fmt.Sprintf("* Total Synced: %d events\n", d.Sync.TotalSynced)
	}
	out += "\n"

	// Phase 20: Retention
	out += "## Retention\n\n"
	if d.Retention != nil {
		out += fmt.Sprintf("* Enabled: %v\n", d.Retention.Enabled)
		if d.Retention.Enabled {
			out += fmt.Sprintf("* Keep Days: %d\n", d.Retention.KeepDays)
			out += fmt.Sprintf("* Estimated Prunable: %d events\n", d.Retention.EstimatedPrunable)
			if d.Retention.AdvancedEnabled {
				out += fmt.Sprintf("* Advanced Retention: enabled\n")
				out += fmt.Sprintf("* Protected Events: %d\n", d.Retention.TotalProtected)
			}
		}
	} else {
		out += "* Not configured\n"
	}

	return out
}
