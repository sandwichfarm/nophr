package sync

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/nbd-wtf/go-nostr"
	"github.com/sandwichfarm/nophr/internal/config"
	"github.com/sandwichfarm/nophr/internal/storage"
)

func setupTestCursorManager(t *testing.T) (*CursorManager, *storage.Storage, func()) {
	t.Helper()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	cfg := &config.Storage{
		Driver:     "sqlite",
		SQLitePath: dbPath,
	}

	ctx := context.Background()
	st, err := storage.New(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	cm := NewCursorManager(st)

	cleanup := func() {
		st.Close()
	}

	return cm, st, cleanup
}

func TestNewCursorManager(t *testing.T) {
	cm, _, cleanup := setupTestCursorManager(t)
	defer cleanup()

	if cm == nil {
		t.Fatal("Expected cursor manager, got nil")
	}
}

func TestGetSinceCursor(t *testing.T) {
	cm, _, cleanup := setupTestCursorManager(t)
	defer cleanup()

	ctx := context.Background()

	// No cursor exists initially
	since, err := cm.GetSinceCursor(ctx, "wss://relay.test", 1)
	if err != nil {
		t.Fatalf("GetSinceCursor() error = %v", err)
	}
	if since != 0 {
		t.Errorf("Expected 0 for new cursor, got %d", since)
	}
}

func TestUpdateCursor(t *testing.T) {
	cm, _, cleanup := setupTestCursorManager(t)
	defer cleanup()

	ctx := context.Background()
	relay := "wss://relay.test"
	kind := 1
	since := int64(12345)

	// Initialize cursor
	if err := cm.InitializeCursor(ctx, relay, kind, since); err != nil {
		t.Fatalf("InitializeCursor() error = %v", err)
	}

	// Get cursor
	retrieved, err := cm.GetSinceCursor(ctx, relay, kind)
	if err != nil {
		t.Fatalf("GetSinceCursor() error = %v", err)
	}

	if retrieved != since {
		t.Errorf("Expected since %d, got %d", since, retrieved)
	}

	// Update cursor
	newSince := int64(67890)
	if err := cm.UpdateCursor(ctx, relay, kind, newSince); err != nil {
		t.Fatalf("UpdateCursor() error = %v", err)
	}

	// Verify update
	retrieved, err = cm.GetSinceCursor(ctx, relay, kind)
	if err != nil {
		t.Fatalf("GetSinceCursor() error = %v", err)
	}

	if retrieved != newSince {
		t.Errorf("Expected updated since %d, got %d", newSince, retrieved)
	}
}

func TestGetSinceCursorForRelay(t *testing.T) {
	cm, _, cleanup := setupTestCursorManager(t)
	defer cleanup()

	ctx := context.Background()
	relay := "wss://relay.test"

	// Initialize multiple kinds with different timestamps
	cm.InitializeCursor(ctx, relay, 1, 1000)
	cm.InitializeCursor(ctx, relay, 3, 2000)
	cm.InitializeCursor(ctx, relay, 7, 500)

	// Get oldest cursor
	since, err := cm.GetSinceCursorForRelay(ctx, relay, []int{1, 3, 7})
	if err != nil {
		t.Fatalf("GetSinceCursorForRelay() error = %v", err)
	}

	// Should return the oldest (500)
	if since != 500 {
		t.Errorf("Expected oldest since 500, got %d", since)
	}
}

func TestUpdateCursorsFromEvents(t *testing.T) {
	cm, _, cleanup := setupTestCursorManager(t)
	defer cleanup()

	ctx := context.Background()
	relay := "wss://relay.test"

	events := []*nostr.Event{
		{
			Kind:      1,
			CreatedAt: 1000,
		},
		{
			Kind:      1,
			CreatedAt: 2000,
		},
		{
			Kind:      3,
			CreatedAt: 1500,
		},
	}

	// Update cursors from events
	if err := cm.UpdateCursorsFromEvents(ctx, relay, events); err != nil {
		t.Fatalf("UpdateCursorsFromEvents() error = %v", err)
	}

	// Check kind 1 cursor (should be 2000, the latest)
	since1, err := cm.GetSinceCursor(ctx, relay, 1)
	if err != nil {
		t.Fatalf("GetSinceCursor() error = %v", err)
	}
	if since1 != 2000 {
		t.Errorf("Expected kind 1 cursor 2000, got %d", since1)
	}

	// Check kind 3 cursor
	since3, err := cm.GetSinceCursor(ctx, relay, 3)
	if err != nil {
		t.Fatalf("GetSinceCursor() error = %v", err)
	}
	if since3 != 1500 {
		t.Errorf("Expected kind 3 cursor 1500, got %d", since3)
	}
}

func TestIsReplaceableKind(t *testing.T) {
	cm, _, cleanup := setupTestCursorManager(t)
	defer cleanup()

	tests := []struct {
		kind     int
		expected bool
	}{
		{0, true},
		{3, true},
		{10002, true},
		{1, false},
		{7, false},
	}

	for _, tt := range tests {
		result := cm.IsReplaceableKind(tt.kind)
		if result != tt.expected {
			t.Errorf("IsReplaceableKind(%d) = %v, expected %v", tt.kind, result, tt.expected)
		}
	}
}

func TestGetAllCursors(t *testing.T) {
	cm, _, cleanup := setupTestCursorManager(t)
	defer cleanup()

	ctx := context.Background()

	// Initialize some cursors
	cm.InitializeCursor(ctx, "wss://relay1.test", 1, 1000)
	cm.InitializeCursor(ctx, "wss://relay1.test", 3, 2000)
	cm.InitializeCursor(ctx, "wss://relay2.test", 1, 1500)

	// Get all cursors
	cursors, err := cm.GetAllCursors(ctx)
	if err != nil {
		t.Fatalf("GetAllCursors() error = %v", err)
	}

	if len(cursors) != 2 {
		t.Errorf("Expected 2 relays, got %d", len(cursors))
	}

	if len(cursors["wss://relay1.test"]) != 2 {
		t.Errorf("Expected 2 kinds for relay1, got %d", len(cursors["wss://relay1.test"]))
	}

	if cursors["wss://relay1.test"][1] != 1000 {
		t.Errorf("Expected cursor 1000 for relay1 kind 1, got %d", cursors["wss://relay1.test"][1])
	}
}
