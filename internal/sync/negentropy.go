package sync

import (
	"context"
	"fmt"
	"time"

	"github.com/fiatjaf/eventstore"
	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip77"
	"github.com/sandwichfarm/nophr/internal/storage"
)

// NegentropyStore adapts nophr's storage to eventstore.Store interface
// This allows us to use nip77.NegentropySync with our existing storage
type NegentropyStore struct {
	storage *storage.Storage
	ctx     context.Context
}

// NewNegentropyStore creates a new adapter wrapping nophr storage
func NewNegentropyStore(storage *storage.Storage, ctx context.Context) *NegentropyStore {
	return &NegentropyStore{
		storage: storage,
		ctx:     ctx,
	}
}

// Init implements eventstore.Store interface (no-op for us)
func (s *NegentropyStore) Init() error {
	// Storage is already initialized
	return nil
}

// Close implements eventstore.Store interface (no-op for us)
func (s *NegentropyStore) Close() {
	// We don't close storage here, it's managed by the main app
}

// QueryEvents implements eventstore.Store interface
// Returns a channel of events matching the filter
func (s *NegentropyStore) QueryEvents(ctx context.Context, filter nostr.Filter) (chan *nostr.Event, error) {
	// Query all matching events from storage
	events, err := s.storage.QueryEvents(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to query events: %w", err)
	}

	// Create channel and send events
	ch := make(chan *nostr.Event, len(events))
	go func() {
		defer close(ch)
		for _, event := range events {
			select {
			case ch <- event:
			case <-ctx.Done():
				return
			}
		}
	}()

	return ch, nil
}

// SaveEvent implements eventstore.Store interface
func (s *NegentropyStore) SaveEvent(ctx context.Context, event *nostr.Event) error {
	return s.storage.StoreEvent(ctx, event)
}

// DeleteEvent implements eventstore.Store interface
func (s *NegentropyStore) DeleteEvent(ctx context.Context, event *nostr.Event) error {
	// nophr doesn't currently support deleting events from khatru
	// This would need to be implemented if we want to support NIP-09
	return fmt.Errorf("delete not implemented")
}

// ReplaceEvent implements eventstore.Store interface
func (s *NegentropyStore) ReplaceEvent(ctx context.Context, event *nostr.Event) error {
	// For replaceable events (kinds 0, 3, 10002, etc.), khatru handles this automatically
	// We just call SaveEvent and khatru will replace the old event
	return s.SaveEvent(ctx, event)
}

// NegentropySync attempts to sync with a relay using NIP-77 negentropy
// Returns (success, error) where success=false triggers REQ fallback
func (e *Engine) NegentropySync(ctx context.Context, relayURL string, filter nostr.Filter) (bool, error) {
	// Check if relay supports negentropy
	caps, err := e.nostrClient.GetRelayCapabilities(ctx, relayURL, e.storage)
	if err != nil {
		// Capability check failed, fall back to REQ
		fmt.Printf("[SYNC] Failed to check capabilities for %s: %v (falling back to REQ)\n", relayURL, err)
		return false, nil
	}

	if !caps.SupportsNegentropy {
		// Relay doesn't support negentropy, fall back to REQ
		return false, nil
	}

	// Relay supports negentropy, attempt sync
	fmt.Printf("[SYNC] Using negentropy for %s\n", relayURL)

	// Create negentropy store adapter
	store := NewNegentropyStore(e.storage, ctx)
	relayWrapper := &eventstore.RelayWrapper{Store: store}

	// Attempt negentropy sync (DOWN direction = fetch missing events from relay)
	syncCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	err = nip77.NegentropySync(syncCtx, relayWrapper, relayURL, filter, nip77.Down)
	if err != nil {
		// Check if error indicates unsupported
		if isNegentropyUnsupportedError(err) {
			fmt.Printf("[SYNC] %s doesn't support negentropy (marking in cache): %v\n", relayURL, err)
			// Update cache to mark as not supported
			e.markRelayDoesNotSupportNegentropy(ctx, relayURL)
			return false, nil // Fall back to REQ
		}

		// Hard error (connection issue, etc.)
		return false, fmt.Errorf("negentropy sync failed: %w", err)
	}

	fmt.Printf("[SYNC] ✓ Negentropy sync complete for %s\n", relayURL)
	return true, nil
}

// isNegentropyUnsupportedError checks if an error indicates NIP-77 is not supported
func isNegentropyUnsupportedError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()
	// Common error patterns from relays that don't support negentropy
	unsupportedPatterns := []string{
		"unsupported",
		"unknown message",
		"neg-open",
		"neg-err",
		"negentropy",
		"invalid",
	}

	for _, pattern := range unsupportedPatterns {
		if contains(errStr, pattern) {
			return true
		}
	}

	return false
}

// markRelayDoesNotSupportNegentropy updates cache to mark relay as not supporting NIP-77
func (e *Engine) markRelayDoesNotSupportNegentropy(ctx context.Context, url string) {
	caps, err := e.storage.GetRelayCapabilities(ctx, url)
	if err != nil {
		fmt.Printf("[SYNC] ⚠ Failed to get relay capabilities for cache update: %v\n", err)
		return
	}

	if caps == nil {
		caps = &storage.RelayCapabilities{URL: url}
	}

	caps.SupportsNegentropy = false
	caps.LastChecked = time.Now()
	caps.CheckExpiry = caps.LastChecked.Add(7 * 24 * time.Hour)

	if err := e.storage.SaveRelayCapabilities(ctx, caps); err != nil {
		fmt.Printf("[SYNC] ⚠ Failed to update relay capabilities cache: %v\n", err)
	}
}

// contains checks if s contains substr (case-insensitive)
func contains(s, substr string) bool {
	s = toLower(s)
	substr = toLower(substr)
	return len(s) >= len(substr) && (s == substr || indexString(s, substr) >= 0)
}

// toLower converts string to lowercase
func toLower(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if 'A' <= c && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}

// indexString finds the index of substr in s
func indexString(s, substr string) int {
	n := len(substr)
	for i := 0; i+n <= len(s); i++ {
		if s[i:i+n] == substr {
			return i
		}
	}
	return -1
}
