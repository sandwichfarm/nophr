package sync

import (
	"github.com/nbd-wtf/go-nostr"
	"github.com/sandwichfarm/nophr/internal/config"
)

// FilterBuilder creates Nostr filters based on sync configuration
type FilterBuilder struct {
	config *config.Sync
}

// NewFilterBuilder creates a new filter builder
func NewFilterBuilder(cfg *config.Sync) *FilterBuilder {
	return &FilterBuilder{
		config: cfg,
	}
}

// BuildFilters creates filters for syncing events based on scope and configuration
func (fb *FilterBuilder) BuildFilters(authors []string, since int64) []nostr.Filter {
	if len(authors) == 0 {
		return nil
	}

	kinds := fb.config.Kinds.ToIntSlice()
	if len(kinds) == 0 {
		// Default kinds per sync_scope.md
		kinds = []int{0, 1, 3, 6, 7, 9735, 30023, 10002}
	}

	// Apply max authors limit if configured
	if fb.config.Scope.MaxAuthors > 0 && len(authors) > fb.config.Scope.MaxAuthors {
		authors = authors[:fb.config.Scope.MaxAuthors]
	}

	var replaceableKinds []int
	var regularKinds []int
	for _, kind := range kinds {
		if kind == 0 || kind == 3 || kind == 10002 || kind == 30023 {
			replaceableKinds = append(replaceableKinds, kind)
			continue
		}
		regularKinds = append(regularKinds, kind)
	}

	filters := make([]nostr.Filter, 0)

	// Replaceable kinds (0,3,10002,30023) must be fetched without since cursors
	if len(replaceableKinds) > 0 {
		filters = append(filters, nostr.Filter{
			Authors: authors,
			Kinds:   replaceableKinds,
		})
	}

	// Non-replaceable kinds honor since cursor
	if len(regularKinds) > 0 {
		filter := nostr.Filter{
			Authors: authors,
			Kinds:   regularKinds,
		}

		if since > 0 {
			sinceTs := nostr.Timestamp(since)
			filter.Since = &sinceTs
		}

		filters = append(filters, filter)
	}

	return filters
}

// BuildMentionFilter creates a filter for events that mention the owner
func (fb *FilterBuilder) BuildMentionFilter(ownerPubkey string, since int64) nostr.Filter {
	kinds := fb.config.Kinds.ToIntSlice()
	if len(kinds) == 0 {
		kinds = []int{0, 1, 3, 6, 7, 9735, 30023, 10002}
	}

	filter := nostr.Filter{
		Kinds: kinds,
		Tags: nostr.TagMap{
			"p": []string{ownerPubkey},
		},
	}

	if since > 0 {
		sinceTs := nostr.Timestamp(since)
		filter.Since = &sinceTs
	}

	return filter
}

// BuildThreadFilter creates a filter for replies to the owner's events
func (fb *FilterBuilder) BuildThreadFilter(ownerEventIDs []string, since int64) nostr.Filter {
	filter := nostr.Filter{
		Kinds: []int{1}, // Notes/replies
		Tags: nostr.TagMap{
			"e": ownerEventIDs,
		},
	}

	if since > 0 {
		sinceTs := nostr.Timestamp(since)
		filter.Since = &sinceTs
	}

	return filter
}

// BuildReplaceableFilter creates a filter for replaceable events (kinds 0, 3, 10002, 30023)
// These are fetched without since cursors to ensure we have the latest versions
func (fb *FilterBuilder) BuildReplaceableFilter(authors []string) nostr.Filter {
	replaceableKinds := []int{0, 3, 10002, 30023}

	filter := nostr.Filter{
		Authors: authors,
		Kinds:   replaceableKinds,
	}

	// Apply max authors limit if configured
	if fb.config.Scope.MaxAuthors > 0 && len(authors) > fb.config.Scope.MaxAuthors {
		filter.Authors = authors[:fb.config.Scope.MaxAuthors]
	}

	return filter
}

// ShouldIncludeAuthor checks if an author should be included based on allowlist/denylist
func (fb *FilterBuilder) ShouldIncludeAuthor(pubkey string) bool {
	// Denylist takes precedence
	for _, denied := range fb.config.Scope.DenylistPubkeys {
		if denied == pubkey {
			return false
		}
	}

	// If allowlist is configured, only allow those pubkeys
	if len(fb.config.Scope.AllowlistPubkeys) > 0 {
		for _, allowed := range fb.config.Scope.AllowlistPubkeys {
			if allowed == pubkey {
				return true
			}
		}
		return false
	}

	return true
}

// GetConfiguredKinds returns the configured event kinds to sync
func (fb *FilterBuilder) GetConfiguredKinds() []int {
	kinds := fb.config.Kinds.ToIntSlice()
	if len(kinds) > 0 {
		return kinds
	}
	// Default kinds
	return []int{0, 1, 3, 6, 7, 9735, 30023, 10002}
}

// BuildNegentropyFilter creates an optimized filter for negentropy sync
// Negentropy excels at reconciling complete datasets, so we:
// - Don't use 'since' cursors (negentropy figures out what's missing)
// - Combine all kinds into a single filter (efficient for large datasets)
// - Let negentropy handle the reconciliation
func (fb *FilterBuilder) BuildNegentropyFilter(authors []string) nostr.Filter {
	if len(authors) == 0 {
		return nostr.Filter{}
	}

	kinds := fb.config.Kinds.ToIntSlice()
	if len(kinds) == 0 {
		kinds = []int{0, 1, 3, 6, 7, 9735, 30023, 10002}
	}

	filter := nostr.Filter{
		Authors: authors,
		Kinds:   kinds,
		// No 'since' - negentropy reconciles complete sets efficiently
	}

	// Apply max authors limit if configured
	if fb.config.Scope.MaxAuthors > 0 && len(authors) > fb.config.Scope.MaxAuthors {
		filter.Authors = authors[:fb.config.Scope.MaxAuthors]
	}

	return filter
}

// BuildInboxFilter creates a filter for interactions directed at the owner
// This queries the owner's INBOX (read relays) for:
// - Mentions (#p tag with owner pubkey)
// - Replies (kind 1 with #e or #p tags)
// - Reactions (kind 7)
// - Reposts (kind 6)
// - Zaps (kind 9735)
func (fb *FilterBuilder) BuildInboxFilter(ownerPubkey string, since int64) nostr.Filter {
	// Interaction kinds that can mention/tag the owner
	kinds := []int{1, 6, 7, 9735} // notes, reposts, reactions, zaps

	// Only include kinds that are enabled in config
	configKinds := fb.config.Kinds.ToIntSlice()
	if len(configKinds) > 0 {
		// Filter to only enabled interaction kinds
		enabledMap := make(map[int]bool)
		for _, k := range configKinds {
			enabledMap[k] = true
		}

		filteredKinds := make([]int, 0, len(kinds))
		for _, k := range kinds {
			if enabledMap[k] {
				filteredKinds = append(filteredKinds, k)
			}
		}
		kinds = filteredKinds
	}

	if len(kinds) == 0 {
		// No interaction kinds enabled, return empty filter
		return nostr.Filter{}
	}

	filter := nostr.Filter{
		Kinds: kinds,
		Tags: nostr.TagMap{
			"p": []string{ownerPubkey}, // Mentions/interactions to us
		},
	}

	if since > 0 {
		sinceTs := nostr.Timestamp(since)
		filter.Since = &sinceTs
	}

	return filter
}
