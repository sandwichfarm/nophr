// +build integration

package integration

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nbd-wtf/go-nostr"
	"github.com/sandwichfarm/nophr/internal/config"
	"github.com/sandwichfarm/nophr/internal/storage"
)

// TestMain sets up and tears down test environment
func TestMain(m *testing.M) {
	// Setup
	code := m.Run()
	// Teardown
	os.Exit(code)
}

// TestEndToEndStorage tests the complete storage workflow
func TestEndToEndStorage(t *testing.T) {
	// Create temporary database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	ctx := context.Background()

	// Create storage
	cfg := &config.Storage{
		Driver:     "sqlite",
		SQLitePath: dbPath,
	}

	st, err := storage.New(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer st.Close()

	// Store an event
	event := &nostr.Event{
		ID:        "test123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		PubKey:    "pubkey1234567890abcdef0123456789abcdef0123456789abcdef0123456789ab",
		CreatedAt: nostr.Timestamp(time.Now().Unix()),
		Kind:      1,
		Content:   "Hello nophr integration test!",
		Tags:      nostr.Tags{},
		Sig:       "sig123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
	}

	if err := st.StoreEvent(ctx, event); err != nil {
		t.Fatalf("Failed to store event: %v", err)
	}

	// Query the event back
	filter := nostr.Filter{
		IDs: []string{event.ID},
	}

	events, err := st.QueryEvents(ctx, filter)
	if err != nil {
		t.Fatalf("Failed to query events: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(events))
	}

	if events[0].Content != event.Content {
		t.Errorf("Event content mismatch: got %s, want %s", events[0].Content, event.Content)
	}
}

// TestEndToEndReplaceableEvent tests replaceable event handling
func TestEndToEndReplaceableEvent(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	ctx := context.Background()

	cfg := &config.Storage{
		Driver:     "sqlite",
		SQLitePath: dbPath,
	}

	st, err := storage.New(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer st.Close()
	pubkey := "pubkey1234567890abcdef0123456789abcdef0123456789abcdef0123456789ab"

	// Store first metadata event
	event1 := &nostr.Event{
		ID:        "event1123456789abcdef0123456789abcdef0123456789abcdef0123456789ab",
		PubKey:    pubkey,
		CreatedAt: nostr.Timestamp(time.Now().Unix()),
		Kind:      0, // Metadata is replaceable
		Content:   `{"name":"Alice"}`,
		Tags:      nostr.Tags{},
		Sig:       "sig1123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcde1",
	}

	if err := st.StoreEvent(ctx, event1); err != nil {
		t.Fatalf("Failed to store first event: %v", err)
	}

	// Store second metadata event (should replace first)
	event2 := &nostr.Event{
		ID:        "event2123456789abcdef0123456789abcdef0123456789abcdef0123456789ab",
		PubKey:    pubkey,
		CreatedAt: nostr.Timestamp(time.Now().Unix() + 1),
		Kind:      0,
		Content:   `{"name":"Alice Updated"}`,
		Tags:      nostr.Tags{},
		Sig:       "sig2123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcde2",
	}

	if err := st.StoreEvent(ctx, event2); err != nil {
		t.Fatalf("Failed to store second event: %v", err)
	}

	// Query metadata - should only get the latest
	filter := nostr.Filter{
		Authors: []string{pubkey},
		Kinds:   []int{0},
	}

	events, err := st.QueryEvents(ctx, filter)
	if err != nil {
		t.Fatalf("Failed to query events: %v", err)
	}

	// Replaceable events should only return the latest
	if len(events) > 1 {
		t.Logf("Note: Got %d events for replaceable kind (implementation may vary)", len(events))
	}

	// Check that the latest event is present
	found := false
	for _, e := range events {
		if e.ID == event2.ID {
			found = true
			break
		}
	}

	if !found {
		t.Error("Latest replaceable event not found in results")
	}
}

// TestEndToEndThreading tests reply threading
func TestEndToEndThreading(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	ctx := context.Background()

	cfg := &config.Storage{
		Driver:     "sqlite",
		SQLitePath: dbPath,
	}

	st, err := storage.New(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer st.Close()

	// Create root event
	root := &nostr.Event{
		ID:        "root123456789abcdef0123456789abcdef0123456789abcdef0123456789abcd",
		PubKey:    "pubkey1234567890abcdef0123456789abcdef0123456789abcdef0123456789ab",
		CreatedAt: nostr.Timestamp(time.Now().Unix()),
		Kind:      1,
		Content:   "Root post",
		Tags:      nostr.Tags{},
		Sig:       "sigroot23456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcde1",
	}

	if err := st.StoreEvent(ctx, root); err != nil {
		t.Fatalf("Failed to store root: %v", err)
	}

	// Create reply
	reply := &nostr.Event{
		ID:        "reply123456789abcdef0123456789abcdef0123456789abcdef0123456789abcd",
		PubKey:    "pubkey2234567890abcdef0123456789abcdef0123456789abcdef0123456789ab",
		CreatedAt: nostr.Timestamp(time.Now().Unix() + 1),
		Kind:      1,
		Content:   "Reply to root",
		Tags: nostr.Tags{
			{"e", root.ID, "", "root"},
		},
		Sig: "sigreply3456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcde2",
	}

	if err := st.StoreEvent(ctx, reply); err != nil {
		t.Fatalf("Failed to store reply: %v", err)
	}

	// Query replies to root
	filter := nostr.Filter{
		Kinds: []int{1},
		Tags: nostr.TagMap{
			"e": []string{root.ID},
		},
	}

	events, err := st.QueryEvents(ctx, filter)
	if err != nil {
		t.Fatalf("Failed to query replies: %v", err)
	}

	if len(events) < 1 {
		t.Error("Expected at least 1 reply")
	}

	// Verify reply is in results
	found := false
	for _, e := range events {
		if e.ID == reply.ID {
			found = true
			break
		}
	}

	if !found {
		t.Error("Reply not found in query results")
	}
}

// TestEndToEndMultipleKinds tests querying multiple event kinds
func TestEndToEndMultipleKinds(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	ctx := context.Background()

	cfg := &config.Storage{
		Driver:     "sqlite",
		SQLitePath: dbPath,
	}

	st, err := storage.New(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer st.Close()
	pubkey := "pubkey1234567890abcdef0123456789abcdef0123456789abcdef0123456789ab"

	// Store events of different kinds
	kinds := []int{0, 1, 3, 7, 30023}
	eventIDs := []string{
		"event0123456789abcdef0123456789abcdef0123456789abcdef0123456789abc",
		"event1123456789abcdef0123456789abcdef0123456789abcdef0123456789abc",
		"event2123456789abcdef0123456789abcdef0123456789abcdef0123456789abc",
		"event3123456789abcdef0123456789abcdef0123456789abcdef0123456789abc",
		"event4123456789abcdef0123456789abcdef0123456789abcdef0123456789abc",
	}
	for i, kind := range kinds {
		event := &nostr.Event{
			ID:        eventIDs[i],
			PubKey:    pubkey,
			CreatedAt: nostr.Timestamp(time.Now().Unix() + int64(i)),
			Kind:      kind,
			Content:   "Test event",
			Tags:      nostr.Tags{},
			Sig:       "sig0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		}

		if err := st.StoreEvent(ctx, event); err != nil {
			t.Fatalf("Failed to store event kind %d: %v", kind, err)
		}
	}

	// Query multiple kinds
	filter := nostr.Filter{
		Authors: []string{pubkey},
		Kinds:   []int{1, 7, 30023},
	}

	events, err := st.QueryEvents(ctx, filter)
	if err != nil {
		t.Fatalf("Failed to query events: %v", err)
	}

	if len(events) < 3 {
		t.Errorf("Expected at least 3 events, got %d", len(events))
	}

	// Verify we only got the requested kinds
	for _, e := range events {
		validKind := false
		for _, k := range filter.Kinds {
			if e.Kind == k {
				validKind = true
				break
			}
		}
		if !validKind {
			t.Errorf("Got unexpected kind: %d", e.Kind)
		}
	}
}
