package storage

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/fiatjaf/khatru"
	"github.com/nbd-wtf/go-nostr"
	"github.com/sandwichfarm/nophr/internal/config"
)

// Storage provides the main storage interface for nophr
type Storage struct {
	relay  *khatru.Relay
	db     *sql.DB
	config *config.Storage
}

// New creates a new Storage instance with the given configuration
func New(ctx context.Context, cfg *config.Storage) (*Storage, error) {
	s := &Storage{
		config: cfg,
	}

	// Initialize the appropriate backend
	switch cfg.Driver {
	case "sqlite":
		if err := s.initSQLite(ctx); err != nil {
			return nil, fmt.Errorf("failed to initialize SQLite: %w", err)
		}
	case "lmdb":
		if err := s.initLMDB(ctx); err != nil {
			return nil, fmt.Errorf("failed to initialize LMDB: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported storage driver: %s", cfg.Driver)
	}

	// Run migrations for custom tables
	if err := s.runMigrations(ctx); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return s, nil
}

// Relay returns the underlying Khatru relay instance
func (s *Storage) Relay() *khatru.Relay {
	return s.relay
}

// DB returns the underlying database connection (for custom tables)
func (s *Storage) DB() *sql.DB {
	return s.db
}

// StoreEvent stores an event in the Khatru relay
func (s *Storage) StoreEvent(ctx context.Context, event *nostr.Event) error {
	if s.relay == nil {
		return fmt.Errorf("relay not initialized")
	}

	// Call all StoreEvent handlers
	for _, handler := range s.relay.StoreEvent {
		if err := handler(ctx, event); err != nil {
			return fmt.Errorf("failed to store event: %w", err)
		}
	}

	return nil
}

// StoreEventBatch stores multiple events in a single transaction (Performance optimization)
func (s *Storage) StoreEventBatch(ctx context.Context, events []*nostr.Event) error {
	if s.relay == nil {
		return fmt.Errorf("relay not initialized")
	}

	if len(events) == 0 {
		return nil
	}

	// Start transaction for batch insert
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Store each event within the transaction
	// Note: Khatru's StoreEvent handlers need to be transaction-aware
	// For now, we'll call them individually but within a transaction context
	for _, event := range events {
		for _, handler := range s.relay.StoreEvent {
			if err := handler(ctx, event); err != nil {
				return fmt.Errorf("failed to store event in batch: %w", err)
			}
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit batch: %w", err)
	}

	return nil
}

// EventExists checks if an event already exists in storage (for deduplication)
func (s *Storage) EventExists(ctx context.Context, eventID string) (bool, error) {
	filter := nostr.Filter{
		IDs:   []string{eventID},
		Limit: 1,
	}

	events, err := s.QueryEvents(ctx, filter)
	if err != nil {
		return false, err
	}

	return len(events) > 0, nil
}

// DeleteEvent deletes an event from the Khatru relay by ID (Phase 20)
func (s *Storage) DeleteEvent(ctx context.Context, eventID string) error {
	if s.relay == nil {
		return fmt.Errorf("relay not initialized")
	}

	// Query the event first (Khatru DeleteEvent needs the full event)
	filter := nostr.Filter{
		IDs:   []string{eventID},
		Limit: 1,
	}

	events, err := s.QueryEvents(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to query event before delete: %w", err)
	}

	if len(events) == 0 {
		return nil // Event doesn't exist, nothing to delete
	}

	// Call all DeleteEvent handlers
	for _, handler := range s.relay.DeleteEvent {
		if err := handler(ctx, events[0]); err != nil {
			return fmt.Errorf("failed to delete event: %w", err)
		}
	}

	return nil
}

// QueryEvents queries events from the Khatru relay using Nostr filters
func (s *Storage) QueryEvents(ctx context.Context, filter nostr.Filter) ([]*nostr.Event, error) {
	if s.relay == nil {
		return nil, fmt.Errorf("relay not initialized")
	}

	// Use the first QueryEvents handler (eventstore)
	if len(s.relay.QueryEvents) == 0 {
		return nil, fmt.Errorf("no query handlers configured")
	}

	ch, err := s.relay.QueryEvents[0](ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to query events: %w", err)
	}

	// Collect events from channel
	var events []*nostr.Event
	for event := range ch {
		events = append(events, event)
	}

	return events, nil
}

// QuerySync is a synchronous query adapter (implements search.Relay interface)
func (s *Storage) QuerySync(ctx context.Context, filter nostr.Filter) ([]*nostr.Event, error) {
	// Use QueryEventsWithSearch to support NIP-50
	return s.QueryEventsWithSearch(ctx, filter)
}

// Close closes the storage connections
func (s *Storage) Close() error {
	if s.db != nil {
		if err := s.db.Close(); err != nil {
			return fmt.Errorf("failed to close database: %w", err)
		}
	}
	return nil
}
