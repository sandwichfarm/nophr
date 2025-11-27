package aggregates

import (
	"context"
	"fmt"

	"github.com/nbd-wtf/go-nostr"
	"github.com/sandwichfarm/nophr/internal/config"
	"github.com/sandwichfarm/nophr/internal/storage"
)

// Manager coordinates all aggregate processing
type Manager struct {
	storage   *storage.Storage
	config    *config.Config
	reactions *ReactionProcessor
	zaps      *ZapProcessor
}

// NewManager creates a new aggregates manager
func NewManager(st *storage.Storage, cfg *config.Config) *Manager {
	return &Manager{
		storage:   st,
		config:    cfg,
		reactions: NewReactionProcessor(st, &cfg.Inbox),
		zaps:      NewZapProcessor(st, &cfg.Inbox),
	}
}

// ProcessEvent processes an event and updates relevant aggregates
func (m *Manager) ProcessEvent(ctx context.Context, event *nostr.Event) error {
	switch event.Kind {
	case 1:
		// Note - check if it's a reply and update aggregate
		return m.processReply(ctx, event)

	case 7:
		// Reaction
		return m.reactions.ProcessReaction(ctx, event)

	case 9735:
		// Zap
		return m.zaps.ProcessZap(ctx, event)

	default:
		// Other kinds don't affect aggregates
		return nil
	}
}

// processReply processes a note that might be a reply
func (m *Manager) processReply(ctx context.Context, event *nostr.Event) error {
	threadInfo, err := ParseThreadInfo(event)
	if err != nil {
		return err
	}

	// If it's a reply, update the parent's reply count
	if threadInfo.IsReply() {
		return m.storage.IncrementReplyCount(ctx, threadInfo.ReplyToID, int64(event.CreatedAt))
	}

	return nil
}

// GetEventAggregates returns all aggregates for an event
func (m *Manager) GetEventAggregates(ctx context.Context, eventID string) (*EventAggregates, error) {
	agg, err := m.storage.GetAggregate(ctx, eventID)
	if err != nil {
		return nil, err
	}

	return &EventAggregates{
		EventID:          agg.EventID,
		ReplyCount:       agg.ReplyCount,
		ReactionTotal:    agg.ReactionTotal,
		ReactionCounts:   agg.ReactionCounts,
		ZapSatsTotal:     agg.ZapSatsTotal,
		LastInteraction:  agg.LastInteractionAt,
	}, nil
}

// GetMultipleAggregates returns aggregates for multiple events
func (m *Manager) GetMultipleAggregates(ctx context.Context, eventIDs []string) (map[string]*EventAggregates, error) {
	aggs, err := m.storage.GetAggregates(ctx, eventIDs)
	if err != nil {
		return nil, err
	}

	result := make(map[string]*EventAggregates)
	for _, agg := range aggs {
		result[agg.EventID] = &EventAggregates{
			EventID:         agg.EventID,
			ReplyCount:      agg.ReplyCount,
			ReactionTotal:   agg.ReactionTotal,
			ReactionCounts:  agg.ReactionCounts,
			ZapSatsTotal:    agg.ZapSatsTotal,
			LastInteraction: agg.LastInteractionAt,
		}
	}

	return result, nil
}

// EventAggregates contains all aggregate data for an event
type EventAggregates struct {
	EventID         string
	ReplyCount      int
	ReactionTotal   int
	ReactionCounts  map[string]int
	ZapSatsTotal    int64
	LastInteraction int64
}

// HasInteractions returns true if the event has any interactions
func (ea *EventAggregates) HasInteractions() bool {
	return ea.ReplyCount > 0 || ea.ReactionTotal > 0 || ea.ZapSatsTotal > 0
}

// InteractionScore returns a simple score for sorting by interaction
func (ea *EventAggregates) InteractionScore() int64 {
	// Weight: 1 point per reply, 1 per reaction, 0.001 per sat
	score := int64(ea.ReplyCount + ea.ReactionTotal)
	score += ea.ZapSatsTotal / 1000
	return score
}

// GetThreadRoot returns the root event ID for a thread
func (m *Manager) GetThreadRoot(ctx context.Context, event *nostr.Event) (string, error) {
	if event.Kind != 1 {
		return "", fmt.Errorf("only notes (kind 1) have threads")
	}

	threadInfo, err := ParseThreadInfo(event)
	if err != nil {
		return "", err
	}

	return threadInfo.GetRootOrSelf(event.ID), nil
}

// IsReplyTo checks if an event is a reply to a specific event
func (m *Manager) IsReplyTo(ctx context.Context, event *nostr.Event, targetEventID string) bool {
	if event.Kind != 1 {
		return false
	}

	threadInfo, err := ParseThreadInfo(event)
	if err != nil {
		return false
	}

	return threadInfo.ReplyToID == targetEventID
}

// IsMentioning checks if an event mentions a specific pubkey
func (m *Manager) IsMentioning(ctx context.Context, event *nostr.Event, targetPubkey string) bool {
	return IsMentioningPubkey(event, targetPubkey)
}
