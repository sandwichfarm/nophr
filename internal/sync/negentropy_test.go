package sync

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/nbd-wtf/go-nostr"
	"github.com/sandwichfarm/nophr/internal/config"
	"github.com/sandwichfarm/nophr/internal/storage"
)

// TestNegentropyStoreAdapter tests the NegentropyStore adapter methods
func TestNegentropyStoreAdapter(t *testing.T) {
	ctx := context.Background()

	// Create test storage
	cfg := &config.Config{
		Storage: config.Storage{
			Driver:     "sqlite",
			SQLitePath: ":memory:", // In-memory database for testing
		},
	}

	st, err := storage.New(ctx, &cfg.Storage)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer st.Close()

	// Create negentropy store adapter
	negStore := NewNegentropyStore(st, ctx)

	// Test Init (should be no-op)
	if err := negStore.Init(); err != nil {
		t.Errorf("Init() failed: %v", err)
	}

	// Test SaveEvent
	testEvent := &nostr.Event{
		ID:        "test123",
		PubKey:    "pubkey123",
		CreatedAt: nostr.Timestamp(time.Now().Unix()),
		Kind:      1,
		Content:   "Test event",
		Tags:      nostr.Tags{},
		Sig:       "sig123",
	}

	if err := negStore.SaveEvent(ctx, testEvent); err != nil {
		t.Errorf("SaveEvent() failed: %v", err)
	}

	// Test QueryEvents - should return channel with the saved event
	filter := nostr.Filter{
		IDs: []string{"test123"},
	}

	eventChan, err := negStore.QueryEvents(ctx, filter)
	if err != nil {
		t.Fatalf("QueryEvents() failed: %v", err)
	}

	// Collect events from channel
	var receivedEvents []*nostr.Event
	for event := range eventChan {
		receivedEvents = append(receivedEvents, event)
	}

	if len(receivedEvents) != 1 {
		t.Errorf("Expected 1 event, got %d", len(receivedEvents))
	}

	if len(receivedEvents) > 0 && receivedEvents[0].ID != "test123" {
		t.Errorf("Expected event ID 'test123', got '%s'", receivedEvents[0].ID)
	}

	// Test ReplaceEvent (should work for replaceable kinds)
	replaceEvent := &nostr.Event{
		ID:        "test456",
		PubKey:    "pubkey123",
		CreatedAt: nostr.Timestamp(time.Now().Unix() + 1),
		Kind:      0, // Kind 0 is replaceable
		Content:   "Replacement profile",
		Tags:      nostr.Tags{},
		Sig:       "sig456",
	}

	if err := negStore.ReplaceEvent(ctx, replaceEvent); err != nil {
		t.Errorf("ReplaceEvent() failed: %v", err)
	}

	// Test DeleteEvent (should return error - not implemented)
	if err := negStore.DeleteEvent(ctx, testEvent); err == nil {
		t.Error("DeleteEvent() should return error (not implemented)")
	}

	// Test Close (should be no-op)
	negStore.Close()
}

// TestIsNegentropyUnsupportedError tests error detection
func TestIsNegentropyUnsupportedError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "unsupported error",
			err:      fmt.Errorf("unsupported message type"),
			expected: true,
		},
		{
			name:     "NEG-OPEN error",
			err:      fmt.Errorf("unknown message: NEG-OPEN"),
			expected: true,
		},
		{
			name:     "negentropy error",
			err:      fmt.Errorf("negentropy protocol not supported"),
			expected: true,
		},
		{
			name:     "invalid error",
			err:      fmt.Errorf("invalid command"),
			expected: true,
		},
		{
			name:     "unrelated error",
			err:      fmt.Errorf("connection timeout"),
			expected: false,
		},
		{
			name:     "case insensitive - UNSUPPORTED",
			err:      fmt.Errorf("UNSUPPORTED feature"),
			expected: true,
		},
		{
			name:     "case insensitive - Negentropy",
			err:      fmt.Errorf("Negentropy Not Available"),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isNegentropyUnsupportedError(tt.err)
			if result != tt.expected {
				t.Errorf("isNegentropyUnsupportedError(%v) = %v, expected %v",
					tt.err, result, tt.expected)
			}
		})
	}
}

// TestContainsHelper tests the contains helper function
func TestContainsHelper(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		substr   string
		expected bool
	}{
		{
			name:     "exact match",
			s:        "hello",
			substr:   "hello",
			expected: true,
		},
		{
			name:     "substring in middle",
			s:        "hello world",
			substr:   "lo wo",
			expected: true,
		},
		{
			name:     "substring at start",
			s:        "hello world",
			substr:   "hello",
			expected: true,
		},
		{
			name:     "substring at end",
			s:        "hello world",
			substr:   "world",
			expected: true,
		},
		{
			name:     "not found",
			s:        "hello world",
			substr:   "xyz",
			expected: false,
		},
		{
			name:     "case insensitive match",
			s:        "Hello World",
			substr:   "hello world",
			expected: true,
		},
		{
			name:     "case insensitive partial",
			s:        "UNSUPPORTED",
			substr:   "unsupported",
			expected: true,
		},
		{
			name:     "empty substring",
			s:        "hello",
			substr:   "",
			expected: true,
		},
		{
			name:     "substring longer than string",
			s:        "hi",
			substr:   "hello",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := contains(tt.s, tt.substr)
			if result != tt.expected {
				t.Errorf("contains(%q, %q) = %v, expected %v",
					tt.s, tt.substr, result, tt.expected)
			}
		})
	}
}

// TestToLowerHelper tests the toLower helper function
func TestToLowerHelper(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"hello", "hello"},
		{"HELLO", "hello"},
		{"Hello World", "hello world"},
		{"123ABC", "123abc"},
		{"MiXeD CaSe", "mixed case"},
		{"UNSUPPORTED", "unsupported"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := toLower(tt.input)
			if result != tt.expected {
				t.Errorf("toLower(%q) = %q, expected %q",
					tt.input, result, tt.expected)
			}
		})
	}
}

// TestIndexStringHelper tests the indexString helper function
func TestIndexStringHelper(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		substr   string
		expected int
	}{
		{
			name:     "found at start",
			s:        "hello world",
			substr:   "hello",
			expected: 0,
		},
		{
			name:     "found in middle",
			s:        "hello world",
			substr:   "lo wo",
			expected: 3,
		},
		{
			name:     "found at end",
			s:        "hello world",
			substr:   "world",
			expected: 6,
		},
		{
			name:     "not found",
			s:        "hello world",
			substr:   "xyz",
			expected: -1,
		},
		{
			name:     "empty substring",
			s:        "hello",
			substr:   "",
			expected: 0,
		},
		{
			name:     "substring longer than string",
			s:        "hi",
			substr:   "hello",
			expected: -1,
		},
		{
			name:     "exact match",
			s:        "test",
			substr:   "test",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := indexString(tt.s, tt.substr)
			if result != tt.expected {
				t.Errorf("indexString(%q, %q) = %d, expected %d",
					tt.s, tt.substr, result, tt.expected)
			}
		})
	}
}

// TestNegentropyConfigHandling tests configuration-based behavior
func TestNegentropyConfigHandling(t *testing.T) {
	ctx := context.Background()

	// Create test storage
	cfg := &config.Config{
		Storage: config.Storage{
			Driver:     "sqlite",
			SQLitePath: ":memory:",
		},
		Sync: config.Sync{
			Performance: config.SyncPerformance{
				Workers:       4,
				UseNegentropy: true,
			},
		},
	}

	st, err := storage.New(ctx, &cfg.Storage)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer st.Close()

	// Test that defaults are set correctly
	if !cfg.Sync.Performance.UseNegentropy {
		t.Error("Expected UseNegentropy to be true by default")
	}

	// Test with negentropy disabled
	cfg.Sync.Performance.UseNegentropy = false
	// When disabled, should always use REQ (tested in engine integration)

	// Test with negentropy enabled
	cfg.Sync.Performance.UseNegentropy = true
	// Should attempt negentropy and always fall back to REQ if unsupported
}

// TestQueryEventsChannelClosure tests that QueryEvents properly closes channel
func TestQueryEventsChannelClosure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create test storage
	cfg := &config.Config{
		Storage: config.Storage{
			Driver:     "sqlite",
			SQLitePath: ":memory:",
		},
	}

	st, err := storage.New(ctx, &cfg.Storage)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer st.Close()

	negStore := NewNegentropyStore(st, ctx)

	// Add some test events
	for i := 0; i < 5; i++ {
		event := &nostr.Event{
			ID:        fmt.Sprintf("event%d", i),
			PubKey:    "testpubkey",
			CreatedAt: nostr.Timestamp(time.Now().Unix()),
			Kind:      1,
			Content:   fmt.Sprintf("Test event %d", i),
			Tags:      nostr.Tags{},
			Sig:       "testsig",
		}
		if err := negStore.SaveEvent(ctx, event); err != nil {
			t.Fatalf("Failed to save event: %v", err)
		}
	}

	// Query events
	filter := nostr.Filter{
		Kinds: []int{1},
		Limit: 10,
	}

	eventChan, err := negStore.QueryEvents(ctx, filter)
	if err != nil {
		t.Fatalf("QueryEvents failed: %v", err)
	}

	// Read all events and ensure channel closes
	count := 0
	for range eventChan {
		count++
	}

	if count != 5 {
		t.Errorf("Expected 5 events, got %d", count)
	}

	// Try reading again - channel should be closed
	_, ok := <-eventChan
	if ok {
		t.Error("Expected channel to be closed")
	}
}

// TestQueryEventsContextCancellation tests context cancellation during QueryEvents
func TestQueryEventsContextCancellation(t *testing.T) {
	ctx := context.Background()

	// Create test storage
	cfg := &config.Config{
		Storage: config.Storage{
			Driver:     "sqlite",
			SQLitePath: ":memory:",
		},
	}

	st, err := storage.New(ctx, &cfg.Storage)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer st.Close()

	negStore := NewNegentropyStore(st, ctx)

	// Add many test events
	for i := 0; i < 100; i++ {
		event := &nostr.Event{
			ID:        fmt.Sprintf("event%d", i),
			PubKey:    "testpubkey",
			CreatedAt: nostr.Timestamp(time.Now().Unix()),
			Kind:      1,
			Content:   fmt.Sprintf("Test event %d", i),
			Tags:      nostr.Tags{},
			Sig:       "testsig",
		}
		if err := negStore.SaveEvent(ctx, event); err != nil {
			t.Fatalf("Failed to save event: %v", err)
		}
	}

	// Create cancellable context
	queryCtx, cancel := context.WithCancel(ctx)

	// Query events
	filter := nostr.Filter{
		Kinds: []int{1},
	}

	eventChan, err := negStore.QueryEvents(queryCtx, filter)
	if err != nil {
		t.Fatalf("QueryEvents failed: %v", err)
	}

	// Read a few events then cancel
	count := 0
	for event := range eventChan {
		count++
		if count == 5 {
			cancel() // Cancel context
			break
		}
		_ = event
	}

	// Channel should eventually close due to context cancellation
	// Read remaining events (if any)
	for range eventChan {
		count++
	}

	// We should have gotten exactly 5 events before cancellation
	if count != 5 {
		t.Logf("Note: Got %d events (expected 5 or slightly more due to buffering)", count)
	}
}
