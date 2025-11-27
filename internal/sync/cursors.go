package sync

import (
	"context"
	"fmt"
	"time"

	"github.com/nbd-wtf/go-nostr"
	"github.com/sandwichfarm/nophr/internal/storage"
)

// CursorManager handles sync cursor tracking to prevent re-syncing old events
type CursorManager struct {
	storage *storage.Storage
}

// NewCursorManager creates a new cursor manager
func NewCursorManager(st *storage.Storage) *CursorManager {
	return &CursorManager{
		storage: st,
	}
}

// GetSinceCursor returns the since timestamp for a given relay and kind
// Returns 0 if no cursor exists (first sync)
func (cm *CursorManager) GetSinceCursor(ctx context.Context, relay string, kind int) (int64, error) {
	state, err := cm.storage.GetSyncState(ctx, relay, kind)
	if err != nil {
		// If no state exists, start from 0 (will sync all events)
		return 0, nil
	}

	return state.Since, nil
}

// UpdateCursor updates the sync cursor for a relay and kind
func (cm *CursorManager) UpdateCursor(ctx context.Context, relay string, kind int, since int64) error {
	return cm.storage.UpdateSyncCursor(ctx, relay, kind, since)
}

// GetSinceCursorForRelay returns the oldest since timestamp across all kinds for a relay
// This is useful for creating a single subscription that covers all kinds
func (cm *CursorManager) GetSinceCursorForRelay(ctx context.Context, relay string, kinds []int) (int64, error) {
	if len(kinds) == 0 {
		return 0, nil
	}

	// Get the minimum (oldest) since across all kinds
	var minSince int64 = 0

	for _, kind := range kinds {
		since, err := cm.GetSinceCursor(ctx, relay, kind)
		if err != nil {
			continue
		}

		if minSince == 0 || since < minSince {
			minSince = since
		}
	}

	return minSince, nil
}

// UpdateCursorsFromEvents updates cursors based on the latest event timestamps
func (cm *CursorManager) UpdateCursorsFromEvents(ctx context.Context, relay string, events []*nostr.Event) error {
	if len(events) == 0 {
		return nil
	}

	// Group events by kind
	kindTimestamps := make(map[int]int64)

	for _, event := range events {
		timestamp := int64(event.CreatedAt)
		if existing, ok := kindTimestamps[event.Kind]; !ok || timestamp > existing {
			kindTimestamps[event.Kind] = timestamp
		}
	}

	// Update cursor for each kind
	for kind, timestamp := range kindTimestamps {
		// Get current cursor
		currentSince, err := cm.GetSinceCursor(ctx, relay, kind)
		if err != nil {
			return fmt.Errorf("failed to get current cursor: %w", err)
		}

		// Only update if new timestamp is newer
		if timestamp > currentSince {
			if err := cm.UpdateCursor(ctx, relay, kind, timestamp); err != nil {
				return fmt.Errorf("failed to update cursor: %w", err)
			}
		}
	}

	return nil
}

// InitializeCursor creates an initial cursor for a relay and kind
func (cm *CursorManager) InitializeCursor(ctx context.Context, relay string, kind int, since int64) error {
	state := &storage.SyncState{
		Relay:     relay,
		Kind:      kind,
		Since:     since,
		UpdatedAt: time.Now().Unix(),
	}

	return cm.storage.SaveSyncState(ctx, state)
}

// GetAllCursors returns all sync cursors for a relay
func (cm *CursorManager) GetAllCursors(ctx context.Context) (map[string]map[int]int64, error) {
	states, err := cm.storage.GetAllSyncStates(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get sync states: %w", err)
	}

	// Organize by relay -> kind -> timestamp
	cursors := make(map[string]map[int]int64)
	for _, state := range states {
		if _, ok := cursors[state.Relay]; !ok {
			cursors[state.Relay] = make(map[int]int64)
		}
		cursors[state.Relay][state.Kind] = state.Since
	}

	return cursors, nil
}

// IsReplaceableKind returns true if the kind should be synced without cursors
// Replaceable kinds (0, 3, 10002) and parameterized replaceable kinds (30023) are always fetched fresh
func (cm *CursorManager) IsReplaceableKind(kind int) bool {
	replaceableKinds := map[int]bool{
		0:     true, // Profile (NIP-01 replaceable)
		3:     true, // Contacts (NIP-01 replaceable)
		10002: true, // Relay hints (NIP-01 replaceable)
		30023: true, // Long-form articles (NIP-33 parameterized replaceable)
	}
	return replaceableKinds[kind]
}

// ShouldRefreshReplaceable checks if enough time has passed to refresh replaceable events
func (cm *CursorManager) ShouldRefreshReplaceable(ctx context.Context, relay string, kind int, refreshInterval time.Duration) (bool, error) {
	if !cm.IsReplaceableKind(kind) {
		return false, nil
	}

	state, err := cm.storage.GetSyncState(ctx, relay, kind)
	if err != nil {
		// No state = never synced = should refresh
		return true, nil
	}

	timeSinceUpdate := time.Now().Unix() - state.UpdatedAt
	return timeSinceUpdate >= int64(refreshInterval.Seconds()), nil
}
