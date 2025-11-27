package nostr

import (
	"fmt"
	"strings"

	"github.com/nbd-wtf/go-nostr"
	"github.com/sandwichfarm/nophr/internal/storage"
)

// ParseRelayHints extracts relay hints from a NIP-65 kind 10002 event
func ParseRelayHints(event *nostr.Event) ([]*storage.RelayHint, error) {
	if event.Kind != 10002 {
		return nil, fmt.Errorf("expected kind 10002, got %d", event.Kind)
	}

	hints := make([]*storage.RelayHint, 0, len(event.Tags))

	for _, tag := range event.Tags {
		if len(tag) < 2 || tag[0] != "r" {
			continue
		}

		relay := tag[1]

		// Normalize relay URL
		relay = strings.TrimSpace(relay)
		if relay == "" {
			continue
		}

		hint := &storage.RelayHint{
			Pubkey:          event.PubKey,
			Relay:           relay,
			CanRead:         true,
			CanWrite:        true,
			Freshness:       int64(event.CreatedAt),
			LastSeenEventID: event.ID,
		}

		// Check for read/write markers
		if len(tag) >= 3 {
			marker := strings.ToLower(tag[2])
			switch marker {
			case "read":
				hint.CanWrite = false
			case "write":
				hint.CanRead = false
			}
		}

		hints = append(hints, hint)
	}

	return hints, nil
}

// BuildRelayListEvent creates a NIP-65 kind 10002 event
// Used for publishing your own relay list
func BuildRelayListEvent(hints []*storage.RelayHint) *nostr.Event {
	event := &nostr.Event{
		Kind:      10002,
		CreatedAt: nostr.Now(),
		Tags:      make(nostr.Tags, 0, len(hints)),
	}

	for _, hint := range hints {
		tag := make(nostr.Tag, 0, 3)
		tag = append(tag, "r", hint.Relay)

		// Add read/write marker
		if hint.CanRead && !hint.CanWrite {
			tag = append(tag, "read")
		} else if hint.CanWrite && !hint.CanRead {
			tag = append(tag, "write")
		}

		event.Tags = append(event.Tags, tag)
	}

	return event
}

// ValidateRelayURL performs basic validation on a relay URL
func ValidateRelayURL(url string) bool {
	return nostr.IsValidRelayURL(url)
}
