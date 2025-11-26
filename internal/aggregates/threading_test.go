package aggregates

import (
	"testing"

	"github.com/nbd-wtf/go-nostr"
)

func TestParseThreadInfo_MarkedFormat(t *testing.T) {
	event := &nostr.Event{
		Kind: 1,
		Tags: nostr.Tags{
			{"e", "root-event-id", "", "root"},
			{"e", "parent-event-id", "", "reply"},
			{"e", "mention-event-id", "", "mention"},
		},
	}

	info, err := ParseThreadInfo(event)
	if err != nil {
		t.Fatalf("ParseThreadInfo() error = %v", err)
	}

	if info.RootEventID != "root-event-id" {
		t.Errorf("Expected root 'root-event-id', got %s", info.RootEventID)
	}

	if info.ReplyToID != "parent-event-id" {
		t.Errorf("Expected reply 'parent-event-id', got %s", info.ReplyToID)
	}

	if len(info.MentionedIDs) != 1 || info.MentionedIDs[0] != "mention-event-id" {
		t.Errorf("Expected mention 'mention-event-id', got %v", info.MentionedIDs)
	}
}

func TestParseThreadInfo_PositionalFormat_OneTag(t *testing.T) {
	event := &nostr.Event{
		Kind: 1,
		Tags: nostr.Tags{
			{"e", "parent-id"},
		},
	}

	info, err := ParseThreadInfo(event)
	if err != nil {
		t.Fatalf("ParseThreadInfo() error = %v", err)
	}

	if info.RootEventID != "parent-id" {
		t.Errorf("Expected root 'parent-id', got %s", info.RootEventID)
	}

	if info.ReplyToID != "parent-id" {
		t.Errorf("Expected reply 'parent-id', got %s", info.ReplyToID)
	}
}

func TestParseThreadInfo_PositionalFormat_TwoTags(t *testing.T) {
	event := &nostr.Event{
		Kind: 1,
		Tags: nostr.Tags{
			{"e", "root-id"},
			{"e", "parent-id"},
		},
	}

	info, err := ParseThreadInfo(event)
	if err != nil {
		t.Fatalf("ParseThreadInfo() error = %v", err)
	}

	if info.RootEventID != "root-id" {
		t.Errorf("Expected root 'root-id', got %s", info.RootEventID)
	}

	if info.ReplyToID != "parent-id" {
		t.Errorf("Expected reply 'parent-id', got %s", info.ReplyToID)
	}
}

func TestParseThreadInfo_PositionalFormat_ManyTags(t *testing.T) {
	event := &nostr.Event{
		Kind: 1,
		Tags: nostr.Tags{
			{"e", "root-id"},
			{"e", "mention1"},
			{"e", "mention2"},
			{"e", "parent-id"},
		},
	}

	info, err := ParseThreadInfo(event)
	if err != nil {
		t.Fatalf("ParseThreadInfo() error = %v", err)
	}

	if info.RootEventID != "root-id" {
		t.Errorf("Expected root 'root-id', got %s", info.RootEventID)
	}

	if info.ReplyToID != "parent-id" {
		t.Errorf("Expected reply 'parent-id', got %s", info.ReplyToID)
	}

	if len(info.MentionedIDs) != 2 {
		t.Errorf("Expected 2 mentions, got %d", len(info.MentionedIDs))
	}
}

func TestParseThreadInfo_NoTags(t *testing.T) {
	event := &nostr.Event{
		Kind: 1,
		Tags: nostr.Tags{},
	}

	info, err := ParseThreadInfo(event)
	if err != nil {
		t.Fatalf("ParseThreadInfo() error = %v", err)
	}

	if !info.IsRoot() {
		t.Error("Expected event to be a root")
	}

	if info.IsReply() {
		t.Error("Expected event not to be a reply")
	}
}

func TestParseThreadInfo_InvalidKind(t *testing.T) {
	event := &nostr.Event{
		Kind: 3, // Not a threadable kind
	}

	_, err := ParseThreadInfo(event)
	if err == nil {
		t.Error("Expected error for non-note kind")
	}
}

func TestParseThreadInfo_LongFormAllowed(t *testing.T) {
	event := &nostr.Event{
		Kind: 30023,
		Tags: nostr.Tags{},
	}

	if _, err := ParseThreadInfo(event); err != nil {
		t.Fatalf("expected long-form to be threadable, got error %v", err)
	}
}

func TestThreadInfo_IsReply(t *testing.T) {
	tests := []struct {
		name     string
		info     *ThreadInfo
		expected bool
	}{
		{
			name:     "with reply",
			info:     &ThreadInfo{ReplyToID: "parent"},
			expected: true,
		},
		{
			name:     "without reply",
			info:     &ThreadInfo{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.info.IsReply()
			if result != tt.expected {
				t.Errorf("IsReply() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestThreadInfo_IsRoot(t *testing.T) {
	tests := []struct {
		name     string
		info     *ThreadInfo
		expected bool
	}{
		{
			name:     "root event",
			info:     &ThreadInfo{},
			expected: true,
		},
		{
			name:     "reply event",
			info:     &ThreadInfo{ReplyToID: "parent"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.info.IsRoot()
			if result != tt.expected {
				t.Errorf("IsRoot() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestExtractMentionedPubkeys(t *testing.T) {
	event := &nostr.Event{
		Tags: nostr.Tags{
			{"p", "pubkey1"},
			{"p", "pubkey2"},
			{"e", "event-id"},
			{"p", "pubkey3"},
		},
	}

	pubkeys := ExtractMentionedPubkeys(event)

	if len(pubkeys) != 3 {
		t.Errorf("Expected 3 pubkeys, got %d", len(pubkeys))
	}

	expected := []string{"pubkey1", "pubkey2", "pubkey3"}
	for i, pk := range expected {
		if pubkeys[i] != pk {
			t.Errorf("Expected pubkey[%d] = %s, got %s", i, pk, pubkeys[i])
		}
	}
}

func TestIsMentioningPubkey(t *testing.T) {
	event := &nostr.Event{
		Tags: nostr.Tags{
			{"p", "pubkey1"},
			{"p", "pubkey2"},
		},
	}

	if !IsMentioningPubkey(event, "pubkey1") {
		t.Error("Expected to find pubkey1")
	}

	if !IsMentioningPubkey(event, "pubkey2") {
		t.Error("Expected to find pubkey2")
	}

	if IsMentioningPubkey(event, "pubkey3") {
		t.Error("Did not expect to find pubkey3")
	}
}
