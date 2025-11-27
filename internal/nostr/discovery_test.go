package nostr

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/sandwichfarm/nophr/internal/config"
	"github.com/sandwichfarm/nophr/internal/storage"
)

func setupTestDiscovery(t *testing.T) (*Discovery, *storage.Storage, func()) {
	t.Helper()

	// Create temp storage
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	storageCfg := &config.Storage{
		Driver:     "sqlite",
		SQLitePath: dbPath,
	}

	ctx := context.Background()
	st, err := storage.New(ctx, storageCfg)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	// Create client
	relaysCfg := &config.Relays{
		Seeds: []string{"wss://relay.test"},
		Policy: config.RelayPolicy{
			ConnectTimeoutMs: 30000,
		},
	}
	client := New(ctx, relaysCfg)

	// Create discovery
	discovery := NewDiscovery(client, st)

	cleanup := func() {
		client.Close()
		st.Close()
	}

	return discovery, st, cleanup
}

func TestNewDiscovery(t *testing.T) {
	discovery, _, cleanup := setupTestDiscovery(t)
	defer cleanup()

	if discovery == nil {
		t.Fatal("Expected discovery, got nil")
	}

	if discovery.client == nil {
		t.Error("Expected client to be initialized")
	}

	if discovery.storage == nil {
		t.Error("Expected storage to be initialized")
	}
}

func TestGetRelaysForPubkey(t *testing.T) {
	discovery, st, cleanup := setupTestDiscovery(t)
	defer cleanup()

	ctx := context.Background()
	pubkey := "test-pubkey"

	// Initially no relays
	relays, err := discovery.GetRelaysForPubkey(ctx, pubkey)
	if err != nil {
		t.Fatalf("GetRelaysForPubkey() error = %v", err)
	}
	if len(relays) != 0 {
		t.Errorf("Expected 0 relays, got %d", len(relays))
	}

	// Add some relay hints
	hint := &storage.RelayHint{
		Pubkey:          pubkey,
		Relay:           "wss://relay.test",
		CanRead:         true,
		CanWrite:        true,
		Freshness:       12345,
		LastSeenEventID: "event-123",
	}

	if err := st.SaveRelayHint(ctx, hint); err != nil {
		t.Fatalf("SaveRelayHint() error = %v", err)
	}

	// Now should have relays
	relays, err = discovery.GetRelaysForPubkey(ctx, pubkey)
	if err != nil {
		t.Fatalf("GetRelaysForPubkey() error = %v", err)
	}
	if len(relays) != 1 {
		t.Errorf("Expected 1 relay, got %d", len(relays))
	}
	if relays[0] != "wss://relay.test" {
		t.Errorf("Expected relay wss://relay.test, got %s", relays[0])
	}
}

func TestGetRelaysForPubkeyFallback(t *testing.T) {
	discovery, st, cleanup := setupTestDiscovery(t)
	defer cleanup()

	ctx := context.Background()
	pubkey := "test-pubkey"

	// Add write-only relay
	hint := &storage.RelayHint{
		Pubkey:          pubkey,
		Relay:           "wss://write-relay.test",
		CanRead:         false,
		CanWrite:        true,
		Freshness:       12345,
		LastSeenEventID: "event-123",
	}

	if err := st.SaveRelayHint(ctx, hint); err != nil {
		t.Fatalf("SaveRelayHint() error = %v", err)
	}

	// Should fall back to write relays since no read relays exist
	relays, err := discovery.GetRelaysForPubkey(ctx, pubkey)
	if err != nil {
		t.Fatalf("GetRelaysForPubkey() error = %v", err)
	}
	if len(relays) != 1 {
		t.Errorf("Expected 1 relay (fallback to write), got %d", len(relays))
	}
	if relays[0] != "wss://write-relay.test" {
		t.Errorf("Expected relay wss://write-relay.test, got %s", relays[0])
	}
}

func TestDiscoverRelayHintsForPubkeys_Empty(t *testing.T) {
	discovery, _, cleanup := setupTestDiscovery(t)
	defer cleanup()

	ctx := context.Background()

	// Empty pubkeys should not error
	err := discovery.DiscoverRelayHintsForPubkeys(ctx, []string{}, []string{"wss://relay.test"})
	if err != nil {
		t.Errorf("DiscoverRelayHintsForPubkeys() with empty pubkeys should not error, got: %v", err)
	}

	// No relays should error
	err = discovery.DiscoverRelayHintsForPubkeys(ctx, []string{"pubkey"}, []string{})
	if err == nil {
		t.Error("DiscoverRelayHintsForPubkeys() with no relays should error")
	}
}

func TestBootstrapFromSeeds_NoSeeds(t *testing.T) {
	// Create discovery with no seed relays
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	storageCfg := &config.Storage{
		Driver:     "sqlite",
		SQLitePath: dbPath,
	}

	ctx := context.Background()
	st, err := storage.New(ctx, storageCfg)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer st.Close()

	// Create client with no seed relays
	relaysCfg := &config.Relays{
		Seeds: []string{},
	}
	client := New(ctx, relaysCfg)
	defer client.Close()

	discovery := NewDiscovery(client, st)

	// Should error with no seed relays
	err = discovery.BootstrapFromSeeds(ctx, "test-pubkey")
	if err == nil {
		t.Error("BootstrapFromSeeds() with no seed relays should error")
	}
}
