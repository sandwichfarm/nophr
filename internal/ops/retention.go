package ops

import (
	"context"
	"fmt"
	"time"

	"github.com/nbd-wtf/go-nostr"
	"github.com/sandwichfarm/nophr/internal/config"
	"github.com/sandwichfarm/nophr/internal/retention"
	"github.com/sandwichfarm/nophr/internal/storage"
)

// RetentionManager handles data retention and pruning
type RetentionManager struct {
	storage         *storage.Storage
	config          *config.Retention
	logger          *Logger
	retentionEngine *retention.Engine // Phase 20: Advanced retention
	ownerPubkey     string

	// Background worker control
	stopChan chan struct{}
	doneChan chan struct{}
}

// NewRetentionManager creates a new retention manager
func NewRetentionManager(st *storage.Storage, cfg *config.Retention, logger *Logger, ownerPubkey string) *RetentionManager {
	rm := &RetentionManager{
		storage:     st,
		config:      cfg,
		logger:      logger.WithComponent("retention"),
		ownerPubkey: ownerPubkey,
		stopChan:    make(chan struct{}),
		doneChan:    make(chan struct{}),
	}

	// Initialize advanced retention engine if enabled
	if cfg.Advanced != nil && cfg.Advanced.Enabled {
		// Create adapters for storage and social graph interfaces
		storageAdapter := &storageAdapter{storage: st}
		graphAdapter := &graphAdapter{storage: st}

		rm.retentionEngine = retention.NewEngine(
			cfg.Advanced,
			storageAdapter,
			graphAdapter,
			ownerPubkey,
		)

		logger.Info("advanced retention enabled",
			"mode", cfg.Advanced.Mode,
			"rules", len(cfg.Advanced.Rules))
	}

	return rm
}

// PruneOldEvents deletes events based on retention rules
// Routes to advanced or simple pruning based on configuration
func (r *RetentionManager) PruneOldEvents(ctx context.Context) (int64, error) {
	// Check if advanced retention is enabled
	if r.config.Advanced != nil && r.config.Advanced.Enabled && r.retentionEngine != nil {
		return r.PruneAdvanced(ctx)
	}

	// Fallback to simple time-based pruning
	return r.pruneSimple(ctx)
}

// pruneSimple performs simple time-based pruning (original implementation)
func (r *RetentionManager) pruneSimple(ctx context.Context) (int64, error) {
	start := time.Now()

	// Calculate cutoff time
	cutoff := time.Now().AddDate(0, 0, -r.config.KeepDays)

	r.logger.Info("starting simple retention pruning",
		"cutoff", cutoff.Format(time.RFC3339),
		"keep_days", r.config.KeepDays)

	// Delete events before cutoff
	deleted, err := r.storage.DeleteEventsBefore(ctx, cutoff)
	if err != nil {
		r.logger.LogRetentionPrune(int(deleted), time.Since(start), err)
		return 0, fmt.Errorf("failed to prune old events: %w", err)
	}

	r.logger.LogRetentionPrune(int(deleted), time.Since(start), nil)
	return deleted, nil
}

// PruneByKind deletes all events of a specific kind
func (r *RetentionManager) PruneByKind(ctx context.Context, kind int) (int64, error) {
	start := time.Now()

	r.logger.Info("pruning events by kind", "kind", kind)

	deleted, err := r.storage.DeleteEventsByKind(ctx, kind)
	if err != nil {
		r.logger.LogRetentionPrune(int(deleted), time.Since(start), err)
		return 0, fmt.Errorf("failed to prune events by kind: %w", err)
	}

	r.logger.Info("pruned events by kind",
		"kind", kind,
		"deleted", deleted,
		"duration_ms", time.Since(start).Milliseconds())

	return deleted, nil
}

// ShouldPruneOnStart returns true if pruning should run on startup
func (r *RetentionManager) ShouldPruneOnStart() bool {
	return r.config.PruneOnStart
}

// GetRetentionStats returns statistics about retention
func (r *RetentionManager) GetRetentionStats(ctx context.Context) (*RetentionStats, error) {
	stats := &RetentionStats{
		KeepDays:     r.config.KeepDays,
		PruneOnStart: r.config.PruneOnStart,
	}

	// Get total events
	total, err := r.storage.CountEvents(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to count events: %w", err)
	}
	stats.TotalEvents = total

	// Get time range
	oldest, newest, err := r.storage.EventTimeRange(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get event time range: %w", err)
	}

	if oldest != nil {
		stats.OldestEvent = *oldest
	}
	if newest != nil {
		stats.NewestEvent = *newest
	}

	// Calculate events eligible for pruning
	cutoff := time.Now().AddDate(0, 0, -r.config.KeepDays)
	stats.Cutoff = cutoff

	// Estimate prunable events (this is approximate)
	if oldest != nil && oldest.Before(cutoff) {
		// Some events are old enough to prune
		stats.EstimatedPrunable = int64(float64(total) * 0.1) // Very rough estimate
	}

	return stats, nil
}

// RetentionStats contains retention statistics
type RetentionStats struct {
	KeepDays          int
	PruneOnStart      bool
	TotalEvents       int64
	OldestEvent       time.Time
	NewestEvent       time.Time
	Cutoff            time.Time
	EstimatedPrunable int64
}

// PeriodicPruner runs periodic pruning
type PeriodicPruner struct {
	manager  *RetentionManager
	interval time.Duration
	logger   *Logger
	stopChan chan struct{}
}

// NewPeriodicPruner creates a new periodic pruner
func NewPeriodicPruner(manager *RetentionManager, interval time.Duration, logger *Logger) *PeriodicPruner {
	return &PeriodicPruner{
		manager:  manager,
		interval: interval,
		logger:   logger.WithComponent("periodic-pruner"),
		stopChan: make(chan struct{}),
	}
}

// Start begins periodic pruning
func (p *PeriodicPruner) Start(ctx context.Context) {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	p.logger.Info("periodic pruner started", "interval", p.interval)

	for {
		select {
		case <-ctx.Done():
			p.logger.Info("periodic pruner stopped")
			return
		case <-p.stopChan:
			p.logger.Info("periodic pruner stopped")
			return
		case <-ticker.C:
			p.logger.Debug("running periodic prune")
			deleted, err := p.manager.PruneOldEvents(ctx)
			if err != nil {
				p.logger.Error("periodic prune failed", "error", err)
			} else {
				p.logger.Info("periodic prune completed", "deleted", deleted)
			}
		}
	}
}

// Stop stops the periodic pruner
func (p *PeriodicPruner) Stop() {
	close(p.stopChan)
}

// ============================================================================
// Phase 20: Advanced Retention Methods
// ============================================================================

// PruneAdvanced performs advanced retention pruning using rules and caps
func (r *RetentionManager) PruneAdvanced(ctx context.Context) (int64, error) {
	if r.retentionEngine == nil {
		return 0, fmt.Errorf("advanced retention engine not initialized")
	}

	start := time.Now()
	r.logger.Info("starting advanced retention pruning")

	totalDeleted := int64(0)

	// Step 1: Prune expired events (based on retention_metadata)
	expired, err := r.pruneExpiredEvents(ctx)
	if err != nil {
		r.logger.Error("failed to prune expired events", "error", err)
	} else {
		totalDeleted += expired
		r.logger.Info("pruned expired events", "count", expired)
	}

	// Step 2: Enforce global caps
	if r.config.Advanced.GlobalCaps.MaxTotalEvents > 0 || r.config.Advanced.GlobalCaps.MaxStorageMB > 0 {
		capped, err := r.enforceGlobalCaps(ctx)
		if err != nil {
			r.logger.Error("failed to enforce global caps", "error", err)
		} else {
			totalDeleted += capped
			r.logger.Info("enforced global caps", "deleted", capped)
		}
	}

	r.logger.Info("advanced retention pruning completed",
		"total_deleted", totalDeleted,
		"duration_ms", time.Since(start).Milliseconds())

	return totalDeleted, nil
}

// pruneExpiredEvents deletes events that have passed their retain_until date
func (r *RetentionManager) pruneExpiredEvents(ctx context.Context) (int64, error) {
	// Get expired event IDs from retention_metadata
	expiredIDs, err := r.storage.GetExpiredEvents(ctx, 1000) // Process in batches
	if err != nil {
		return 0, fmt.Errorf("failed to get expired events: %w", err)
	}

	if len(expiredIDs) == 0 {
		return 0, nil
	}

	// Delete events
	deleted := int64(0)
	for _, eventID := range expiredIDs {
		// Delete event from storage
		if err := r.storage.DeleteEvent(ctx, eventID); err != nil {
			r.logger.Error("failed to delete expired event", "event_id", eventID, "error", err)
			continue
		}
		deleted++
	}

	return deleted, nil
}

// enforceGlobalCaps enforces storage caps by deleting lowest-priority events
func (r *RetentionManager) enforceGlobalCaps(ctx context.Context) (int64, error) {
	caps := r.config.Advanced.GlobalCaps

	// Check if we're over the total events cap
	totalEvents, err := r.storage.CountEvents(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to count events: %w", err)
	}

	eventsToDelete := 0
	if caps.MaxTotalEvents > 0 && int(totalEvents) > caps.MaxTotalEvents {
		eventsToDelete = int(totalEvents) - caps.MaxTotalEvents
		r.logger.Info("total events cap exceeded",
			"current", totalEvents,
			"max", caps.MaxTotalEvents,
			"to_delete", eventsToDelete)
	}

	if eventsToDelete == 0 {
		return 0, nil
	}

	// Get lowest-priority events by score
	candidates, err := r.storage.GetEventsByScore(ctx, eventsToDelete)
	if err != nil {
		return 0, fmt.Errorf("failed to get events by score: %w", err)
	}

	// Delete events
	deleted := int64(0)
	for _, meta := range candidates {
		if err := r.storage.DeleteEvent(ctx, meta.EventID); err != nil {
			r.logger.Error("failed to delete low-priority event", "event_id", meta.EventID, "error", err)
			continue
		}
		deleted++
	}

	return deleted, nil
}

// EvaluateEvent evaluates retention for a single event
func (r *RetentionManager) EvaluateEvent(ctx context.Context, event *nostr.Event) error {
	if r.retentionEngine == nil {
		return nil // Advanced retention not enabled, skip
	}

	decision, err := r.retentionEngine.EvaluateEvent(ctx, event)
	if err != nil {
		return fmt.Errorf("failed to evaluate event: %w", err)
	}

	// Store retention metadata
	meta := &storage.RetentionMetadata{
		EventID:         decision.EventID,
		RuleName:        decision.RuleName,
		RulePriority:    decision.RulePriority,
		RetainUntil:     decision.RetainUntil,
		LastEvaluatedAt: time.Now(),
		Score:           decision.Score,
		Protected:       decision.Protected,
	}

	if err := r.storage.StoreRetentionMetadata(ctx, meta); err != nil {
		return fmt.Errorf("failed to store retention metadata: %w", err)
	}

	return nil
}

// ============================================================================
// Adapters for retention engine interfaces
// ============================================================================

// storageAdapter adapts storage.Storage to retention.StorageReader
type storageAdapter struct {
	storage *storage.Storage
}

func (a *storageAdapter) GetAggregateByID(eventID string) (*retention.AggregateData, error) {
	ctx := context.Background()
	agg, err := a.storage.GetAggregate(ctx, eventID)
	if err != nil {
		return nil, err
	}
	if agg == nil {
		return &retention.AggregateData{}, nil
	}

	return &retention.AggregateData{
		ReplyCount:    agg.ReplyCount,
		ReactionTotal: agg.ReactionTotal,
		ZapSatsTotal:  agg.ZapSatsTotal,
	}, nil
}

func (a *storageAdapter) CountEventsByAuthor(pubkey string) (int, error) {
	// This would need a new storage method, for now return 0
	return 0, nil
}

func (a *storageAdapter) CountEventsByKind(kind int) (int, error) {
	// This would need a new storage method, for now return 0
	return 0, nil
}

// graphAdapter adapts storage.Storage to retention.SocialGraphReader
type graphAdapter struct {
	storage *storage.Storage
}

func (a *graphAdapter) GetDistance(ownerPubkey, targetPubkey string) int {
	ctx := context.Background()
	// Query graph_nodes table
	node, err := a.storage.GetGraphNode(ctx, ownerPubkey, targetPubkey)
	if err != nil || node == nil {
		return -1 // Not in graph
	}
	return node.Depth
}

func (a *graphAdapter) IsFollowing(ownerPubkey, targetPubkey string) bool {
	return a.GetDistance(ownerPubkey, targetPubkey) == 1
}

func (a *graphAdapter) IsMutual(ownerPubkey, targetPubkey string) bool {
	ctx := context.Background()
	node, err := a.storage.GetGraphNode(ctx, ownerPubkey, targetPubkey)
	if err != nil || node == nil {
		return false
	}
	return node.Mutual
}

// ============================================================================
// Background Re-evaluation Worker
// ============================================================================

// StartReEvaluationWorker starts the background re-evaluation worker
func (r *RetentionManager) StartReEvaluationWorker(ctx context.Context) {
	// Only start if advanced retention is enabled with re-evaluation configured
	if r.config.Advanced == nil || !r.config.Advanced.Enabled {
		return
	}

	if r.config.Advanced.Evaluation.ReEvalIntervalHrs <= 0 {
		r.logger.Info("re-evaluation worker not started (interval not configured)")
		return
	}

	interval := time.Duration(r.config.Advanced.Evaluation.ReEvalIntervalHrs) * time.Hour
	r.logger.Info("starting re-evaluation worker",
		"interval_hours", r.config.Advanced.Evaluation.ReEvalIntervalHrs)

	go r.reEvaluationLoop(ctx, interval)
}

// reEvaluationLoop runs the periodic re-evaluation
func (r *RetentionManager) reEvaluationLoop(ctx context.Context, interval time.Duration) {
	defer close(r.doneChan)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			r.logger.Info("re-evaluation worker stopped (context done)")
			return
		case <-r.stopChan:
			r.logger.Info("re-evaluation worker stopped (shutdown)")
			return
		case <-ticker.C:
			r.logger.Info("starting periodic re-evaluation")
			if err := r.reEvaluateEvents(ctx); err != nil {
				r.logger.Error("re-evaluation failed", "error", err)
			}
		}
	}
}

// reEvaluateEvents re-evaluates events that need updating
func (r *RetentionManager) reEvaluateEvents(ctx context.Context) error {
	start := time.Now()

	if r.retentionEngine == nil {
		return fmt.Errorf("retention engine not initialized")
	}

	// Get events that need re-evaluation
	cutoff := time.Now().Add(-time.Duration(r.config.Advanced.Evaluation.ReEvalIntervalHrs) * time.Hour)
	batchSize := r.config.Advanced.Evaluation.BatchSize
	if batchSize == 0 {
		batchSize = 1000
	}

	eventIDs, err := r.storage.GetEventsForReEvaluation(ctx, cutoff, batchSize)
	if err != nil {
		return fmt.Errorf("failed to get events for re-evaluation: %w", err)
	}

	if len(eventIDs) == 0 {
		r.logger.Info("no events need re-evaluation")
		return nil
	}

	r.logger.Info("re-evaluating events", "count", len(eventIDs))

	evaluated := 0
	errors := 0

	// Re-evaluate each event
	for _, eventID := range eventIDs {
		// Get the full event
		filter := nostr.Filter{
			IDs:   []string{eventID},
			Limit: 1,
		}

		events, err := r.storage.QueryEvents(ctx, filter)
		if err != nil || len(events) == 0 {
			errors++
			continue
		}

		// Re-evaluate retention
		if err := r.EvaluateEvent(ctx, events[0]); err != nil {
			r.logger.Error("failed to re-evaluate event", "event_id", eventID, "error", err)
			errors++
			continue
		}

		evaluated++
	}

	r.logger.Info("re-evaluation complete",
		"evaluated", evaluated,
		"errors", errors,
		"duration_ms", time.Since(start).Milliseconds())

	return nil
}

// Stop stops the re-evaluation worker gracefully
func (r *RetentionManager) Stop() {
	close(r.stopChan)
	<-r.doneChan
}

// ============================================================================
// Background Pruning Scheduler
// ============================================================================

// StartPruningScheduler starts the background pruning scheduler
// This runs periodic pruning independent of re-evaluation
func (r *RetentionManager) StartPruningScheduler(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		r.logger.Info("pruning scheduler not started (interval not configured)")
		return
	}

	r.logger.Info("starting pruning scheduler", "interval", interval)

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				r.logger.Info("pruning scheduler stopped (context done)")
				return
			case <-r.stopChan:
				r.logger.Info("pruning scheduler stopped (shutdown)")
				return
			case <-ticker.C:
				r.logger.Info("starting scheduled pruning")
				deleted, err := r.PruneOldEvents(ctx)
				if err != nil {
					r.logger.Error("scheduled pruning failed", "error", err)
				} else {
					r.logger.Info("scheduled pruning complete", "deleted", deleted)
				}
			}
		}
	}()
}
