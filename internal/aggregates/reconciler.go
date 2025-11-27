package aggregates

import (
	"context"
	"fmt"
	"time"

	"github.com/nbd-wtf/go-nostr"
	"github.com/sandwichfarm/nophr/internal/storage"
)

// Reconciler performs periodic recalculation of aggregates to fix drift
type Reconciler struct {
	storage *storage.Storage
	manager *Manager
}

// NewReconciler creates a new reconciler
func NewReconciler(st *storage.Storage, mgr *Manager) *Reconciler {
	return &Reconciler{
		storage: st,
		manager: mgr,
	}
}

// ReconcileEvent recalculates aggregates for a single event by querying all interactions
func (r *Reconciler) ReconcileEvent(ctx context.Context, eventID string) error {
	// Query all reactions (kind 7) for this event
	reactionFilter := nostr.Filter{
		Kinds: []int{7},
		Tags: nostr.TagMap{
			"e": []string{eventID},
		},
	}

	reactions, err := r.storage.QueryEvents(ctx, reactionFilter)
	if err != nil {
		return fmt.Errorf("failed to query reactions: %w", err)
	}

	// Count reactions by emoji
	reactionCounts := make(map[string]int)
	reactionTotal := 0
	latestReaction := int64(0)

	for _, reaction := range reactions {
		emoji := reaction.Content
		if emoji == "" {
			emoji = "+"
		}
		reactionCounts[emoji]++
		reactionTotal++

		if int64(reaction.CreatedAt) > latestReaction {
			latestReaction = int64(reaction.CreatedAt)
		}
	}

	// Query all replies (kind 1 with e tag pointing to this event)
	replyFilter := nostr.Filter{
		Kinds: []int{1},
		Tags: nostr.TagMap{
			"e": []string{eventID},
		},
	}

	replies, err := r.storage.QueryEvents(ctx, replyFilter)
	if err != nil {
		return fmt.Errorf("failed to query replies: %w", err)
	}

	replyCount := 0
	latestReply := int64(0)

	for _, reply := range replies {
		// Verify it's actually a reply to this specific event
		threadInfo, err := ParseThreadInfo(reply)
		if err != nil {
			continue
		}

		if threadInfo.ReplyToID == eventID {
			replyCount++
			if int64(reply.CreatedAt) > latestReply {
				latestReply = int64(reply.CreatedAt)
			}
		}
	}

	// Query all zaps (kind 9735) for this event
	zapFilter := nostr.Filter{
		Kinds: []int{9735},
		Tags: nostr.TagMap{
			"e": []string{eventID},
		},
	}

	zaps, err := r.storage.QueryEvents(ctx, zapFilter)
	if err != nil {
		return fmt.Errorf("failed to query zaps: %w", err)
	}

	zapTotal := int64(0)
	latestZap := int64(0)

	zapProc := NewZapProcessor(r.storage, nil)
	for _, zap := range zaps {
		zapInfo, err := zapProc.parseZapEvent(zap)
		if err != nil {
			continue
		}

		zapTotal += zapInfo.Amount
		if int64(zap.CreatedAt) > latestZap {
			latestZap = int64(zap.CreatedAt)
		}
	}

	// Determine latest interaction
	lastInteraction := latestReaction
	if latestReply > lastInteraction {
		lastInteraction = latestReply
	}
	if latestZap > lastInteraction {
		lastInteraction = latestZap
	}

	// Save the reconciled aggregate
	agg := &storage.Aggregate{
		EventID:           eventID,
		ReplyCount:        replyCount,
		ReactionTotal:     reactionTotal,
		ReactionCounts:    reactionCounts,
		ZapSatsTotal:      zapTotal,
		LastInteractionAt: lastInteraction,
	}

	return r.storage.SaveAggregate(ctx, agg)
}

// ReconcileAll recalculates aggregates for all events that have interactions
func (r *Reconciler) ReconcileAll(ctx context.Context) error {
	// Get all events that have interactions (kind 1 notes with e tags pointing to them)
	// This is a simplified approach - a production system might track this differently

	// Query all reactions to find unique event IDs
	reactionFilter := nostr.Filter{
		Kinds: []int{7},
		Limit: 10000, // Reasonable limit
	}

	reactions, err := r.storage.QueryEvents(ctx, reactionFilter)
	if err != nil {
		return fmt.Errorf("failed to query reactions: %w", err)
	}

	eventIDs := make(map[string]bool)
	for _, reaction := range reactions {
		for _, tag := range reaction.Tags {
			if len(tag) >= 2 && tag[0] == "e" {
				eventIDs[tag[1]] = true
			}
		}
	}

	// Query all replies
	replyFilter := nostr.Filter{
		Kinds: []int{1},
		Limit: 10000,
	}

	replies, err := r.storage.QueryEvents(ctx, replyFilter)
	if err != nil {
		return fmt.Errorf("failed to query replies: %w", err)
	}

	for _, reply := range replies {
		threadInfo, err := ParseThreadInfo(reply)
		if err != nil {
			continue
		}

		if threadInfo.ReplyToID != "" {
			eventIDs[threadInfo.ReplyToID] = true
		}
	}

	// Query all zaps
	zapFilter := nostr.Filter{
		Kinds: []int{9735},
		Limit: 10000,
	}

	zaps, err := r.storage.QueryEvents(ctx, zapFilter)
	if err != nil {
		return fmt.Errorf("failed to query zaps: %w", err)
	}

	for _, zap := range zaps {
		for _, tag := range zap.Tags {
			if len(tag) >= 2 && tag[0] == "e" {
				eventIDs[tag[1]] = true
			}
		}
	}

	// Reconcile each event
	for eventID := range eventIDs {
		if err := r.ReconcileEvent(ctx, eventID); err != nil {
			// Log error but continue
			fmt.Printf("Failed to reconcile event %s: %v\n", eventID, err)
		}
	}

	return nil
}

// ReconcileRecent recalculates aggregates for events with recent interactions
func (r *Reconciler) ReconcileRecent(ctx context.Context, since time.Duration) error {
	sinceTs := nostr.Timestamp(time.Now().Add(-since).Unix())

	// Get recent interactions
	filters := []nostr.Filter{
		{
			Kinds: []int{7}, // Reactions
			Since: &sinceTs,
		},
		{
			Kinds: []int{1}, // Replies
			Since: &sinceTs,
		},
		{
			Kinds: []int{9735}, // Zaps
			Since: &sinceTs,
		},
	}

	eventIDs := make(map[string]bool)

	for _, filter := range filters {
		events, err := r.storage.QueryEvents(ctx, filter)
		if err != nil {
			continue
		}

		for _, event := range events {
			for _, tag := range event.Tags {
				if len(tag) >= 2 && tag[0] == "e" {
					eventIDs[tag[1]] = true
				}
			}
		}
	}

	// Reconcile each event
	for eventID := range eventIDs {
		if err := r.ReconcileEvent(ctx, eventID); err != nil {
			fmt.Printf("Failed to reconcile event %s: %v\n", eventID, err)
		}
	}

	return nil
}
