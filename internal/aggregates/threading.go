package aggregates

import (
	"fmt"

	"github.com/nbd-wtf/go-nostr"
)

// ThreadInfo contains thread relationship information extracted from an event
type ThreadInfo struct {
	RootEventID  string   // The root event of the thread
	ReplyToID    string   // The direct parent event being replied to
	MentionedIDs []string // Other events mentioned in the thread
}

// ParseThreadInfo extracts thread relationship info from a note event using NIP-10
func ParseThreadInfo(event *nostr.Event) (*ThreadInfo, error) {
	if !isThreadableKind(event.Kind) {
		return nil, fmt.Errorf("expected threadable kind (1 or 30023), got %d", event.Kind)
	}

	info := &ThreadInfo{
		MentionedIDs: make([]string, 0),
	}

	// Extract all e tags
	eTags := make([]nostr.Tag, 0)
	for _, tag := range event.Tags {
		if len(tag) >= 2 && tag[0] == "e" {
			eTags = append(eTags, tag)
		}
	}

	if len(eTags) == 0 {
		// Not a reply, it's a root post
		return info, nil
	}

	// Try preferred marked format first
	if hasMarkedTags(eTags) {
		return parseMarkedFormat(eTags), nil
	}

	// Fall back to deprecated positional format
	return parsePositionalFormat(eTags), nil
}

// hasMarkedTags checks if any e tag has a marker (root/reply/mention)
func hasMarkedTags(eTags []nostr.Tag) bool {
	for _, tag := range eTags {
		if len(tag) >= 4 && tag[3] != "" {
			return true
		}
	}
	return false
}

// parseMarkedFormat parses NIP-10 marked e tags (preferred format)
func parseMarkedFormat(eTags []nostr.Tag) *ThreadInfo {
	info := &ThreadInfo{
		MentionedIDs: make([]string, 0),
	}

	for _, tag := range eTags {
		eventID := tag[1]
		marker := ""
		if len(tag) >= 4 {
			marker = tag[3]
		}

		switch marker {
		case "root":
			info.RootEventID = eventID
		case "reply":
			info.ReplyToID = eventID
		case "mention":
			info.MentionedIDs = append(info.MentionedIDs, eventID)
		default:
			// No marker - treat as mention
			info.MentionedIDs = append(info.MentionedIDs, eventID)
		}
	}

	// If we have a reply but no root, the reply is also the root
	if info.ReplyToID != "" && info.RootEventID == "" {
		info.RootEventID = info.ReplyToID
	}

	return info
}

// parsePositionalFormat parses deprecated positional e tag format
func parsePositionalFormat(eTags []nostr.Tag) *ThreadInfo {
	info := &ThreadInfo{
		MentionedIDs: make([]string, 0),
	}

	switch len(eTags) {
	case 1:
		// Single e tag: reply to this event (which is also the root)
		info.RootEventID = eTags[0][1]
		info.ReplyToID = eTags[0][1]

	case 2:
		// Two e tags: [root, reply]
		info.RootEventID = eTags[0][1]
		info.ReplyToID = eTags[1][1]

	default:
		// Many e tags: [root, ...mentions, reply]
		info.RootEventID = eTags[0][1]
		info.ReplyToID = eTags[len(eTags)-1][1]

		// Middle tags are mentions
		for i := 1; i < len(eTags)-1; i++ {
			info.MentionedIDs = append(info.MentionedIDs, eTags[i][1])
		}
	}

	return info
}

// IsReply returns true if this event is a reply to another event
func (ti *ThreadInfo) IsReply() bool {
	return ti.ReplyToID != ""
}

// IsRoot returns true if this event starts a new thread
func (ti *ThreadInfo) IsRoot() bool {
	return ti.RootEventID == "" && ti.ReplyToID == ""
}

// GetRootOrSelf returns the root event ID, or the event itself if it's a root
func (ti *ThreadInfo) GetRootOrSelf(eventID string) string {
	if ti.RootEventID != "" {
		return ti.RootEventID
	}
	return eventID
}

// ExtractMentionedPubkeys extracts pubkeys from p tags
func ExtractMentionedPubkeys(event *nostr.Event) []string {
	pubkeys := make([]string, 0)
	for _, tag := range event.Tags {
		if len(tag) >= 2 && tag[0] == "p" {
			pubkeys = append(pubkeys, tag[1])
		}
	}
	return pubkeys
}

// IsMentioningPubkey checks if an event mentions a specific pubkey
func IsMentioningPubkey(event *nostr.Event, pubkey string) bool {
	for _, tag := range event.Tags {
		if len(tag) >= 2 && tag[0] == "p" && tag[1] == pubkey {
			return true
		}
	}
	return false
}

func isThreadableKind(kind int) bool {
	return kind == 1 || kind == 30023
}
