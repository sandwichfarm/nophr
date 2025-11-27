package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/nbd-wtf/go-nostr"
	"github.com/sandwichfarm/nophr/internal/config"
)

func setupTestStorage(t *testing.T) (*Storage, func()) {
	t.Helper()

	// Create temp directory
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	cfg := &config.Storage{
		Driver:     "sqlite",
		SQLitePath: dbPath,
	}

	ctx := context.Background()
	storage, err := New(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	cleanup := func() {
		storage.Close()
		os.RemoveAll(tmpDir)
	}

	return storage, cleanup
}

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *config.Storage
		wantErr bool
	}{
		{
			name: "valid sqlite config",
			cfg: &config.Storage{
				Driver:     "sqlite",
				SQLitePath: filepath.Join(t.TempDir(), "test.db"),
			},
			wantErr: false,
		},
		{
			name: "unsupported driver",
			cfg: &config.Storage{
				Driver: "postgres",
			},
			wantErr: true,
		},
		{
			name: "lmdb not implemented",
			cfg: &config.Storage{
				Driver:   "lmdb",
				LMDBPath: "/tmp/test.lmdb",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			s, err := New(ctx, tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if s != nil {
				defer s.Close()
			}
		})
	}
}

func TestStoreAndQueryEvents(t *testing.T) {
	s, cleanup := setupTestStorage(t)
	defer cleanup()

	ctx := context.Background()

	// Create a test event
	event := &nostr.Event{
		ID:        "test-event-id",
		PubKey:    "test-pubkey",
		CreatedAt: nostr.Now(),
		Kind:      1,
		Tags:      nostr.Tags{},
		Content:   "Hello, Nostr!",
		Sig:       "test-signature",
	}

	// Store the event
	if err := s.StoreEvent(ctx, event); err != nil {
		t.Fatalf("Failed to store event: %v", err)
	}

	// Query the event
	filter := nostr.Filter{
		IDs: []string{event.ID},
	}

	events, err := s.QueryEvents(ctx, filter)
	if err != nil {
		t.Fatalf("Failed to query events: %v", err)
	}

	if len(events) != 1 {
		t.Errorf("Expected 1 event, got %d", len(events))
	}

	if events[0].ID != event.ID {
		t.Errorf("Expected event ID %s, got %s", event.ID, events[0].ID)
	}
}

func TestRelayHints(t *testing.T) {
	s, cleanup := setupTestStorage(t)
	defer cleanup()

	ctx := context.Background()

	hint := &RelayHint{
		Pubkey:          "test-pubkey",
		Relay:           "wss://relay.test",
		CanRead:         true,
		CanWrite:        true,
		Freshness:       12345,
		LastSeenEventID: "event-123",
	}

	// Save relay hint
	if err := s.SaveRelayHint(ctx, hint); err != nil {
		t.Fatalf("Failed to save relay hint: %v", err)
	}

	// Get relay hints
	hints, err := s.GetRelayHints(ctx, hint.Pubkey)
	if err != nil {
		t.Fatalf("Failed to get relay hints: %v", err)
	}

	if len(hints) != 1 {
		t.Errorf("Expected 1 hint, got %d", len(hints))
	}

	if hints[0].Relay != hint.Relay {
		t.Errorf("Expected relay %s, got %s", hint.Relay, hints[0].Relay)
	}

	// Get write relays
	writeRelays, err := s.GetWriteRelays(ctx, hint.Pubkey)
	if err != nil {
		t.Fatalf("Failed to get write relays: %v", err)
	}

	if len(writeRelays) != 1 {
		t.Errorf("Expected 1 write relay, got %d", len(writeRelays))
	}

	// Get read relays
	readRelays, err := s.GetReadRelays(ctx, hint.Pubkey)
	if err != nil {
		t.Fatalf("Failed to get read relays: %v", err)
	}

	if len(readRelays) != 1 {
		t.Errorf("Expected 1 read relay, got %d", len(readRelays))
	}

	// Delete relay hints
	if err := s.DeleteRelayHints(ctx, hint.Pubkey); err != nil {
		t.Fatalf("Failed to delete relay hints: %v", err)
	}

	hints, err = s.GetRelayHints(ctx, hint.Pubkey)
	if err != nil {
		t.Fatalf("Failed to get relay hints after delete: %v", err)
	}

	if len(hints) != 0 {
		t.Errorf("Expected 0 hints after delete, got %d", len(hints))
	}
}

func TestGraphNodes(t *testing.T) {
	s, cleanup := setupTestStorage(t)
	defer cleanup()

	ctx := context.Background()

	node := &GraphNode{
		RootPubkey: "root-pubkey",
		Pubkey:     "follower-pubkey",
		Depth:      1,
		Mutual:     true,
		LastSeen:   12345,
	}

	// Save graph node
	if err := s.SaveGraphNode(ctx, node); err != nil {
		t.Fatalf("Failed to save graph node: %v", err)
	}

	// Get graph nodes
	nodes, err := s.GetGraphNodes(ctx, node.RootPubkey, 2)
	if err != nil {
		t.Fatalf("Failed to get graph nodes: %v", err)
	}

	if len(nodes) != 1 {
		t.Errorf("Expected 1 node, got %d", len(nodes))
	}

	if nodes[0].Pubkey != node.Pubkey {
		t.Errorf("Expected pubkey %s, got %s", node.Pubkey, nodes[0].Pubkey)
	}

	// Get following pubkeys
	following, err := s.GetFollowingPubkeys(ctx, node.RootPubkey)
	if err != nil {
		t.Fatalf("Failed to get following pubkeys: %v", err)
	}

	if len(following) != 1 {
		t.Errorf("Expected 1 following, got %d", len(following))
	}

	// Get mutual pubkeys
	mutuals, err := s.GetMutualPubkeys(ctx, node.RootPubkey)
	if err != nil {
		t.Fatalf("Failed to get mutual pubkeys: %v", err)
	}

	if len(mutuals) != 1 {
		t.Errorf("Expected 1 mutual, got %d", len(mutuals))
	}

	// Delete graph nodes
	if err := s.DeleteGraphNodes(ctx, node.RootPubkey); err != nil {
		t.Fatalf("Failed to delete graph nodes: %v", err)
	}

	nodes, err = s.GetGraphNodes(ctx, node.RootPubkey, 2)
	if err != nil {
		t.Fatalf("Failed to get graph nodes after delete: %v", err)
	}

	if len(nodes) != 0 {
		t.Errorf("Expected 0 nodes after delete, got %d", len(nodes))
	}
}

func TestSyncState(t *testing.T) {
	s, cleanup := setupTestStorage(t)
	defer cleanup()

	ctx := context.Background()

	state := &SyncState{
		Relay:     "wss://relay.test",
		Kind:      1,
		Since:     12345,
		UpdatedAt: 67890,
	}

	// Save sync state
	if err := s.SaveSyncState(ctx, state); err != nil {
		t.Fatalf("Failed to save sync state: %v", err)
	}

	// Get sync state
	retrieved, err := s.GetSyncState(ctx, state.Relay, state.Kind)
	if err != nil {
		t.Fatalf("Failed to get sync state: %v", err)
	}

	if retrieved.Since != state.Since {
		t.Errorf("Expected since %d, got %d", state.Since, retrieved.Since)
	}

	// Update sync cursor
	newSince := int64(99999)
	if err := s.UpdateSyncCursor(ctx, state.Relay, state.Kind, newSince); err != nil {
		t.Fatalf("Failed to update sync cursor: %v", err)
	}

	retrieved, err = s.GetSyncState(ctx, state.Relay, state.Kind)
	if err != nil {
		t.Fatalf("Failed to get sync state after update: %v", err)
	}

	if retrieved.Since != newSince {
		t.Errorf("Expected since %d after update, got %d", newSince, retrieved.Since)
	}

	// Get all sync states
	states, err := s.GetAllSyncStates(ctx)
	if err != nil {
		t.Fatalf("Failed to get all sync states: %v", err)
	}

	if len(states) != 1 {
		t.Errorf("Expected 1 sync state, got %d", len(states))
	}

	// Delete sync state
	if err := s.DeleteSyncState(ctx, state.Relay, state.Kind); err != nil {
		t.Fatalf("Failed to delete sync state: %v", err)
	}

	states, err = s.GetAllSyncStates(ctx)
	if err != nil {
		t.Fatalf("Failed to get all sync states after delete: %v", err)
	}

	if len(states) != 0 {
		t.Errorf("Expected 0 sync states after delete, got %d", len(states))
	}
}

func TestAggregates(t *testing.T) {
	s, cleanup := setupTestStorage(t)
	defer cleanup()

	ctx := context.Background()

	agg := &Aggregate{
		EventID:           "event-123",
		ReplyCount:        5,
		ReactionTotal:     10,
		ReactionCounts:    map[string]int{"+": 8, "❤️": 2},
		ZapSatsTotal:      1000,
		LastInteractionAt: 12345,
	}

	// Save aggregate
	if err := s.SaveAggregate(ctx, agg); err != nil {
		t.Fatalf("Failed to save aggregate: %v", err)
	}

	// Get aggregate
	retrieved, err := s.GetAggregate(ctx, agg.EventID)
	if err != nil {
		t.Fatalf("Failed to get aggregate: %v", err)
	}

	if retrieved.ReplyCount != agg.ReplyCount {
		t.Errorf("Expected reply count %d, got %d", agg.ReplyCount, retrieved.ReplyCount)
	}

	if retrieved.ReactionCounts["+"] != 8 {
		t.Errorf("Expected + reaction count 8, got %d", retrieved.ReactionCounts["+"])
	}

	// Increment reply count
	if err := s.IncrementReplyCount(ctx, agg.EventID, 12346); err != nil {
		t.Fatalf("Failed to increment reply count: %v", err)
	}

	retrieved, err = s.GetAggregate(ctx, agg.EventID)
	if err != nil {
		t.Fatalf("Failed to get aggregate after increment: %v", err)
	}

	if retrieved.ReplyCount != 6 {
		t.Errorf("Expected reply count 6 after increment, got %d", retrieved.ReplyCount)
	}

	// Increment reaction
	if err := s.IncrementReaction(ctx, agg.EventID, "🔥", 12347); err != nil {
		t.Fatalf("Failed to increment reaction: %v", err)
	}

	retrieved, err = s.GetAggregate(ctx, agg.EventID)
	if err != nil {
		t.Fatalf("Failed to get aggregate after reaction: %v", err)
	}

	if retrieved.ReactionTotal != 11 {
		t.Errorf("Expected reaction total 11, got %d", retrieved.ReactionTotal)
	}

	// Add zap amount
	if err := s.AddZapAmount(ctx, agg.EventID, 500, 12348); err != nil {
		t.Fatalf("Failed to add zap amount: %v", err)
	}

	retrieved, err = s.GetAggregate(ctx, agg.EventID)
	if err != nil {
		t.Fatalf("Failed to get aggregate after zap: %v", err)
	}

	if retrieved.ZapSatsTotal != 1500 {
		t.Errorf("Expected zap sats total 1500, got %d", retrieved.ZapSatsTotal)
	}

	// Get aggregates (batch)
	aggregates, err := s.GetAggregates(ctx, []string{agg.EventID, "nonexistent"})
	if err != nil {
		t.Fatalf("Failed to get aggregates: %v", err)
	}

	if len(aggregates) != 1 {
		t.Errorf("Expected 1 aggregate, got %d", len(aggregates))
	}

	// Delete aggregate
	if err := s.DeleteAggregate(ctx, agg.EventID); err != nil {
		t.Fatalf("Failed to delete aggregate: %v", err)
	}

	_, err = s.GetAggregate(ctx, agg.EventID)
	if err == nil {
		t.Error("Expected error when getting deleted aggregate, got nil")
	}
}
