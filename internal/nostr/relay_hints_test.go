package nostr

import (
	"testing"

	"github.com/nbd-wtf/go-nostr"
	"github.com/sandwichfarm/nophr/internal/storage"
)

func TestParseRelayHints(t *testing.T) {
	tests := []struct {
		name      string
		event     *nostr.Event
		wantCount int
		wantErr   bool
	}{
		{
			name: "valid relay hints with read/write markers",
			event: &nostr.Event{
				ID:        "test-event",
				PubKey:    "test-pubkey",
				CreatedAt: 12345,
				Kind:      10002,
				Tags: nostr.Tags{
					{"r", "wss://relay1.test", "read"},
					{"r", "wss://relay2.test", "write"},
					{"r", "wss://relay3.test"},
				},
			},
			wantCount: 3,
			wantErr:   false,
		},
		{
			name: "invalid kind",
			event: &nostr.Event{
				Kind: 1,
				Tags: nostr.Tags{
					{"r", "wss://relay.test"},
				},
			},
			wantCount: 0,
			wantErr:   true,
		},
		{
			name: "empty tags",
			event: &nostr.Event{
				Kind: 10002,
				Tags: nostr.Tags{},
			},
			wantCount: 0,
			wantErr:   false,
		},
		{
			name: "mixed tags",
			event: &nostr.Event{
				Kind: 10002,
				Tags: nostr.Tags{
					{"r", "wss://relay1.test"},
					{"e", "event-id"},
					{"r", "wss://relay2.test"},
					{"p", "pubkey"},
				},
			},
			wantCount: 2,
			wantErr:   false,
		},
		{
			name: "empty relay URL",
			event: &nostr.Event{
				Kind: 10002,
				Tags: nostr.Tags{
					{"r", ""},
					{"r", "wss://relay.test"},
				},
			},
			wantCount: 1,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hints, err := ParseRelayHints(tt.event)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseRelayHints() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if len(hints) != tt.wantCount {
				t.Errorf("Expected %d hints, got %d", tt.wantCount, len(hints))
			}
		})
	}
}

func TestParseRelayHintsMarkers(t *testing.T) {
	event := &nostr.Event{
		ID:        "test-event",
		PubKey:    "test-pubkey",
		CreatedAt: 12345,
		Kind:      10002,
		Tags: nostr.Tags{
			{"r", "wss://read-only.test", "read"},
			{"r", "wss://write-only.test", "write"},
			{"r", "wss://both.test"},
		},
	}

	hints, err := ParseRelayHints(event)
	if err != nil {
		t.Fatalf("ParseRelayHints() error = %v", err)
	}

	if len(hints) != 3 {
		t.Fatalf("Expected 3 hints, got %d", len(hints))
	}

	// Check read-only relay
	if hints[0].CanRead != true || hints[0].CanWrite != false {
		t.Error("Expected read-only relay, got different permissions")
	}

	// Check write-only relay
	if hints[1].CanRead != false || hints[1].CanWrite != true {
		t.Error("Expected write-only relay, got different permissions")
	}

	// Check read-write relay
	if hints[2].CanRead != true || hints[2].CanWrite != true {
		t.Error("Expected read-write relay, got different permissions")
	}
}

func TestBuildRelayListEvent(t *testing.T) {
	hints := []*storage.RelayHint{
		{
			Relay:    "wss://relay1.test",
			CanRead:  true,
			CanWrite: false,
		},
		{
			Relay:    "wss://relay2.test",
			CanRead:  false,
			CanWrite: true,
		},
		{
			Relay:    "wss://relay3.test",
			CanRead:  true,
			CanWrite: true,
		},
	}

	event := BuildRelayListEvent(hints)
	if event == nil {
		t.Fatal("Expected event, got nil")
	}

	if event.Kind != 10002 {
		t.Errorf("Expected kind 10002, got %d", event.Kind)
	}

	if len(event.Tags) != 3 {
		t.Errorf("Expected 3 tags, got %d", len(event.Tags))
	}

	// Check read-only tag
	if event.Tags[0][0] != "r" || event.Tags[0][1] != "wss://relay1.test" {
		t.Error("Unexpected read-only tag format")
	}
	if len(event.Tags[0]) != 3 || event.Tags[0][2] != "read" {
		t.Error("Expected read marker for read-only relay")
	}

	// Check write-only tag
	if event.Tags[1][0] != "r" || event.Tags[1][1] != "wss://relay2.test" {
		t.Error("Unexpected write-only tag format")
	}
	if len(event.Tags[1]) != 3 || event.Tags[1][2] != "write" {
		t.Error("Expected write marker for write-only relay")
	}

	// Check read-write tag (no marker)
	if event.Tags[2][0] != "r" || event.Tags[2][1] != "wss://relay3.test" {
		t.Error("Unexpected read-write tag format")
	}
	if len(event.Tags[2]) != 2 {
		t.Error("Expected no marker for read-write relay")
	}
}

func TestValidateRelayURL(t *testing.T) {
	tests := []struct {
		name  string
		url   string
		valid bool
	}{
		{
			name:  "valid wss URL",
			url:   "wss://relay.test",
			valid: true,
		},
		{
			name:  "valid ws URL",
			url:   "ws://relay.test",
			valid: true,
		},
		{
			name:  "invalid http URL",
			url:   "http://relay.test",
			valid: false,
		},
		{
			name:  "invalid https URL",
			url:   "https://relay.test",
			valid: false,
		},
		{
			name:  "empty URL",
			url:   "",
			valid: false,
		},
		{
			name:  "invalid format",
			url:   "not-a-url",
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid := ValidateRelayURL(tt.url)
			if valid != tt.valid {
				t.Errorf("ValidateRelayURL(%s) = %v, want %v", tt.url, valid, tt.valid)
			}
		})
	}
}
