package aggregates

import (
	"context"
	"fmt"

	"github.com/nbd-wtf/go-nostr"
	"github.com/sandwichfarm/nophr/internal/config"
	"github.com/sandwichfarm/nophr/internal/storage"
)

// ReactionProcessor handles reaction (kind 7) event processing
type ReactionProcessor struct {
	storage *storage.Storage
	config  *config.Inbox
}

// NewReactionProcessor creates a new reaction processor
func NewReactionProcessor(st *storage.Storage, cfg *config.Inbox) *ReactionProcessor {
	return &ReactionProcessor{
		storage: st,
		config:  cfg,
	}
}

// ProcessReaction processes a kind 7 reaction event and updates aggregates
func (rp *ReactionProcessor) ProcessReaction(ctx context.Context, event *nostr.Event) error {
	if event.Kind != 7 {
		return fmt.Errorf("expected kind 7, got %d", event.Kind)
	}

	// Find the target event ID
	targetEventID := ""
	for _, tag := range event.Tags {
		if len(tag) >= 2 && tag[0] == "e" {
			targetEventID = tag[1]
			break
		}
	}

	if targetEventID == "" {
		return fmt.Errorf("reaction has no target event")
	}

	// Get reaction content (emoji or +)
	reaction := event.Content
	if reaction == "" {
		reaction = "+" // Default like
	}

	// Apply noise filter if configured
	if !rp.isAllowedReaction(reaction) {
		return nil // Silently ignore filtered reactions
	}

	// Update aggregate
	return rp.storage.IncrementReaction(ctx, targetEventID, reaction, int64(event.CreatedAt))
}

// isAllowedReaction checks if a reaction passes noise filters
func (rp *ReactionProcessor) isAllowedReaction(reaction string) bool {
	if rp.config == nil || len(rp.config.NoiseFilters.AllowedReactionChars) == 0 {
		return true // No filter configured, allow all
	}

	// Check if reaction is in allowed list
	for _, allowed := range rp.config.NoiseFilters.AllowedReactionChars {
		if reaction == allowed {
			return true
		}
	}

	return false
}

// GetReactionStats returns reaction statistics for an event
func (rp *ReactionProcessor) GetReactionStats(ctx context.Context, eventID string) (map[string]int, int, error) {
	agg, err := rp.storage.GetAggregate(ctx, eventID)
	if err != nil {
		return nil, 0, err
	}

	return agg.ReactionCounts, agg.ReactionTotal, nil
}

// GetTopReactions returns the most popular reactions for an event
func (rp *ReactionProcessor) GetTopReactions(ctx context.Context, eventID string, limit int) ([]ReactionStat, error) {
	agg, err := rp.storage.GetAggregate(ctx, eventID)
	if err != nil {
		return nil, err
	}

	// Convert map to sorted slice
	stats := make([]ReactionStat, 0, len(agg.ReactionCounts))
	for emoji, count := range agg.ReactionCounts {
		stats = append(stats, ReactionStat{
			Emoji: emoji,
			Count: count,
		})
	}

	// Sort by count descending
	for i := 0; i < len(stats); i++ {
		for j := i + 1; j < len(stats); j++ {
			if stats[j].Count > stats[i].Count {
				stats[i], stats[j] = stats[j], stats[i]
			}
		}
	}

	// Apply limit
	if limit > 0 && len(stats) > limit {
		stats = stats[:limit]
	}

	return stats, nil
}

// ReactionStat represents a reaction and its count
type ReactionStat struct {
	Emoji string
	Count int
}
