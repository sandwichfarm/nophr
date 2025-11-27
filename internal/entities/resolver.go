package entities

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip19"
	"github.com/sandwichfarm/nophr/internal/storage"
)

// Entity represents a resolved NIP-19 entity
type Entity struct {
	Type         string // "npub", "nprofile", "note", "nevent", "naddr"
	DisplayName  string // Human-readable name
	Link         string // Internal link path
	OriginalText string // Original nostr: string
}

// Resolver handles NIP-19 entity resolution
type Resolver struct {
	storage *storage.Storage
}

// NewResolver creates a new entity resolver
func NewResolver(st *storage.Storage) *Resolver {
	return &Resolver{
		storage: st,
	}
}

// Regular expression to match nostr: URIs
var nostrEntityRegex = regexp.MustCompile(`nostr:(npub1[a-z0-9]+|nprofile1[a-z0-9]+|note1[a-z0-9]+|nevent1[a-z0-9]+|naddr1[a-z0-9]+)`)

// FindEntities finds all NIP-19 entities in text
func (r *Resolver) FindEntities(text string) []string {
	matches := nostrEntityRegex.FindAllString(text, -1)
	// Remove "nostr:" prefix
	entities := make([]string, len(matches))
	for i, match := range matches {
		entities[i] = strings.TrimPrefix(match, "nostr:")
	}
	return entities
}

// ResolveEntity resolves a single NIP-19 entity
func (r *Resolver) ResolveEntity(ctx context.Context, nip19Entity string) (*Entity, error) {
	prefix, decoded, err := nip19.Decode(nip19Entity)
	if err != nil {
		return nil, fmt.Errorf("failed to decode NIP-19: %w", err)
	}

	entity := &Entity{
		Type:         prefix,
		OriginalText: "nostr:" + nip19Entity,
	}

	switch prefix {
	case "npub":
		pubkey := decoded.(string)
		entity.Link = "/profile/" + pubkey
		entity.DisplayName = r.resolvePubkeyName(ctx, pubkey)

	case "nprofile":
		profileData := decoded.(nostr.ProfilePointer)
		entity.Link = "/profile/" + profileData.PublicKey
		entity.DisplayName = r.resolvePubkeyName(ctx, profileData.PublicKey)

	case "note":
		eventID := decoded.(string)
		entity.Link = "/note/" + eventID
		entity.DisplayName = r.resolveNoteTitle(ctx, eventID)

	case "nevent":
		eventPointer := decoded.(nostr.EventPointer)
		entity.Link = "/note/" + eventPointer.ID
		entity.DisplayName = r.resolveNoteTitle(ctx, eventPointer.ID)

	case "naddr":
		addrPointer := decoded.(nostr.EntityPointer)
		entity.Link = fmt.Sprintf("/addr/%d/%s/%s", addrPointer.Kind, addrPointer.PublicKey, addrPointer.Identifier)
		entity.DisplayName = r.resolveAddrTitle(ctx, &addrPointer)

	default:
		return nil, fmt.Errorf("unsupported NIP-19 type: %s", prefix)
	}

	return entity, nil
}

// resolvePubkeyName fetches the display name for a pubkey
func (r *Resolver) resolvePubkeyName(ctx context.Context, pubkey string) string {
	// Try to get profile from storage
	filter := nostr.Filter{
		Authors: []string{pubkey},
		Kinds:   []int{0}, // Profile metadata
		Limit:   1,
	}

	events, err := r.storage.QueryEvents(ctx, filter)
	if err != nil || len(events) == 0 {
		// Fallback to truncated pubkey
		return truncatePubkey(pubkey)
	}

	// Parse profile metadata
	var metadata struct {
		Name        string `json:"name"`
		DisplayName string `json:"display_name"`
		Nip05       string `json:"nip05"`
	}

	if err := json.Unmarshal([]byte(events[0].Content), &metadata); err != nil {
		return truncatePubkey(pubkey)
	}

	// Priority: display_name > name > nip05 > truncated pubkey
	if metadata.DisplayName != "" {
		return metadata.DisplayName
	}
	if metadata.Name != "" {
		return metadata.Name
	}
	if metadata.Nip05 != "" {
		return metadata.Nip05
	}

	return truncatePubkey(pubkey)
}

// resolveNoteTitle fetches the title/preview for a note
func (r *Resolver) resolveNoteTitle(ctx context.Context, eventID string) string {
	filter := nostr.Filter{
		IDs:   []string{eventID},
		Limit: 1,
	}

	events, err := r.storage.QueryEvents(ctx, filter)
	if err != nil || len(events) == 0 {
		return fmt.Sprintf("Note %s...", truncate(eventID, 8))
	}

	event := events[0]

	// For kind 1 (notes), use first line
	if event.Kind == 1 {
		lines := strings.Split(event.Content, "\n")
		if len(lines) > 0 && lines[0] != "" {
			return truncate(lines[0], 40)
		}
	}

	// For kind 30023 (articles), check for title tag
	if event.Kind == 30023 {
		for _, tag := range event.Tags {
			if len(tag) >= 2 && tag[0] == "title" {
				return tag[1]
			}
		}
	}

	return fmt.Sprintf("Event %s...", truncate(eventID, 8))
}

// resolveAddrTitle fetches the title for a parameterized replaceable event
func (r *Resolver) resolveAddrTitle(ctx context.Context, addr *nostr.EntityPointer) string {
	filter := nostr.Filter{
		Authors: []string{addr.PublicKey},
		Kinds:   []int{addr.Kind},
		Tags: nostr.TagMap{
			"d": []string{addr.Identifier},
		},
		Limit: 1,
	}

	events, err := r.storage.QueryEvents(ctx, filter)
	if err != nil || len(events) == 0 {
		return fmt.Sprintf("%s by %s", addr.Identifier, truncatePubkey(addr.PublicKey))
	}

	event := events[0]

	// Check for title tag (common in articles)
	for _, tag := range event.Tags {
		if len(tag) >= 2 && tag[0] == "title" {
			return tag[1]
		}
	}

	// Fallback to identifier
	if addr.Identifier != "" {
		return addr.Identifier
	}

	return fmt.Sprintf("Article by %s", truncatePubkey(addr.PublicKey))
}

// ReplaceEntities replaces all NIP-19 entities in text with their resolved forms.
// Returns the modified text.
func (r *Resolver) ReplaceEntities(ctx context.Context, text string, formatter func(*Entity) string) string {
	result, _ := r.ReplaceEntitiesWithMetadata(ctx, text, formatter)
	return result
}

func dedupeEntities(entities []*Entity) []*Entity {
	seen := make(map[string]struct{})
	unique := make([]*Entity, 0, len(entities))
	for _, e := range entities {
		if _, ok := seen[e.OriginalText]; ok {
			continue
		}
		seen[e.OriginalText] = struct{}{}
		unique = append(unique, e)
	}
	return unique
}

// ReplaceEntitiesWithMetadata replaces all NIP-19 entities and also returns the resolved entities.
// This allows callers to render portal links or other contextual output based on the matches found.
func (r *Resolver) ReplaceEntitiesWithMetadata(ctx context.Context, text string, formatter func(*Entity) string) (string, []*Entity) {
	resolved := make([]*Entity, 0)

	replaced := nostrEntityRegex.ReplaceAllStringFunc(text, func(match string) string {
		entityStr := strings.TrimPrefix(match, "nostr:")
		entity, err := r.ResolveEntity(ctx, entityStr)
		if err != nil {
			// Keep original if resolution fails
			return match
		}

		resolved = append(resolved, entity)
		return formatter(entity)
	})

	return replaced, resolved
}

// DedupeEntities removes duplicate entities by OriginalText, preserving order
func DedupeEntities(entities []*Entity) []*Entity {
	seen := make(map[string]struct{})
	unique := make([]*Entity, 0, len(entities))
	for _, e := range entities {
		if _, ok := seen[e.OriginalText]; ok {
			continue
		}
		seen[e.OriginalText] = struct{}{}
		unique = append(unique, e)
	}
	return unique
}

// Helper functions

func truncatePubkey(pubkey string) string {
	if len(pubkey) <= 16 {
		return pubkey
	}
	return pubkey[:8] + "..." + pubkey[len(pubkey)-8:]
}

func truncate(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	return text[:maxLen-3] + "..."
}
