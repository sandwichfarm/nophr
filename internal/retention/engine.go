package retention

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/nbd-wtf/go-nostr"
	"github.com/sandwichfarm/nophr/internal/config"
)

// Engine evaluates events against retention rules
type Engine struct {
	config      *config.AdvancedRetention
	storage     StorageReader
	socialGraph SocialGraphReader
	ownerPubkey string
	sortedRules []config.RetentionRule // Cached sorted rules (performance optimization)
}

// NewEngine creates a new retention engine
func NewEngine(cfg *config.AdvancedRetention, storage StorageReader, graph SocialGraphReader, ownerPubkey string) *Engine {
	e := &Engine{
		config:      cfg,
		storage:     storage,
		socialGraph: graph,
		ownerPubkey: ownerPubkey,
	}

	// Pre-sort rules once at initialization for performance
	e.sortedRules = make([]config.RetentionRule, len(cfg.Rules))
	copy(e.sortedRules, cfg.Rules)
	sort.Slice(e.sortedRules, func(i, j int) bool {
		return e.sortedRules[i].Priority > e.sortedRules[j].Priority
	})

	return e
}

// EvaluateEvent evaluates a single event against retention rules
func (e *Engine) EvaluateEvent(ctx context.Context, event *nostr.Event) (*RetentionDecision, error) {
	if e.config == nil || !e.config.Enabled {
		return nil, fmt.Errorf("advanced retention not enabled")
	}

	// Create evaluation context
	evalCtx := &EvalContext{
		Event:       event,
		Storage:     e.storage,
		SocialGraph: e.socialGraph,
		OwnerPubkey: e.ownerPubkey,
	}

	// Evaluate rules in order until one matches (use pre-sorted rules)
	for _, rule := range e.sortedRules {
		matches, err := e.evaluateConditions(evalCtx, rule.Conditions)
		if err != nil {
			// Log error but continue to next rule
			continue
		}

		if matches {
			// Apply action from matching rule
			decision, err := e.applyAction(event, rule)
			if err != nil {
				return nil, fmt.Errorf("failed to apply action for rule %s: %w", rule.Name, err)
			}
			return decision, nil
		}
	}

	// No rule matched - delete by default (safe default)
	return &RetentionDecision{
		EventID:      event.ID,
		RuleName:     "default_delete",
		RulePriority: 0,
		RetainUntil:  timePtr(time.Now()), // Immediate deletion
		Protected:    false,
		Score:        0,
	}, nil
}

// EvaluateBatch evaluates multiple events in a batch (optimized version)
func (e *Engine) EvaluateBatch(ctx context.Context, events []*nostr.Event) ([]*RetentionDecision, error) {
	decisions := make([]*RetentionDecision, 0, len(events))

	// TODO: Potential optimization - prefetch aggregates and graph data in batch
	// This would reduce database roundtrips significantly
	// For now, evaluate individually but could add:
	// - e.storage.GetAggregatesBatch(eventIDs)
	// - e.socialGraph.GetDistancesBatch(pubkeys)

	for _, event := range events {
		decision, err := e.EvaluateEvent(ctx, event)
		if err != nil {
			// Log error but continue with other events
			continue
		}
		decisions = append(decisions, decision)
	}

	return decisions, nil
}

// evaluateConditions evaluates all conditions in a rule
func (e *Engine) evaluateConditions(ctx *EvalContext, conditions config.RuleConditions) (bool, error) {
	// Check if this is a catch-all condition
	if conditions.All {
		return true, nil
	}

	// Evaluate logical operators
	if len(conditions.And) > 0 {
		for _, subCond := range conditions.And {
			match, err := e.evaluateConditions(ctx, subCond)
			if err != nil {
				return false, err
			}
			if !match {
				return false, nil // AND requires all to match
			}
		}
		return true, nil
	}

	if len(conditions.Or) > 0 {
		for _, subCond := range conditions.Or {
			match, err := e.evaluateConditions(ctx, subCond)
			if err != nil {
				return false, err
			}
			if match {
				return true, nil // OR requires at least one to match
			}
		}
		return false, nil
	}

	if len(conditions.Not) > 0 {
		for _, subCond := range conditions.Not {
			match, err := e.evaluateConditions(ctx, subCond)
			if err != nil {
				return false, err
			}
			if match {
				return false, nil // NOT inverts the result
			}
		}
		return true, nil
	}

	// Evaluate individual conditions
	// All specified conditions must match (implicit AND)

	// Kind-based conditions
	if len(conditions.Kinds) > 0 {
		if !intInSlice(ctx.Event.Kind, conditions.Kinds) {
			return false, nil
		}
	}

	if len(conditions.KindsExclude) > 0 {
		if intInSlice(ctx.Event.Kind, conditions.KindsExclude) {
			return false, nil
		}
	}

	// Author-based conditions
	if conditions.AuthorIsOwner {
		if ctx.Event.PubKey != ctx.OwnerPubkey {
			return false, nil
		}
	}

	if len(conditions.AuthorInList) > 0 {
		if !stringInSlice(ctx.Event.PubKey, conditions.AuthorInList) {
			return false, nil
		}
	}

	if len(conditions.AuthorNotInList) > 0 {
		if stringInSlice(ctx.Event.PubKey, conditions.AuthorNotInList) {
			return false, nil
		}
	}

	// Social distance conditions
	if conditions.SocialDistanceMax > 0 || conditions.AuthorIsFollowing || conditions.AuthorIsMutual {
		distance := ctx.SocialGraph.GetDistance(ctx.OwnerPubkey, ctx.Event.PubKey)

		if conditions.SocialDistanceMax > 0 && distance > conditions.SocialDistanceMax {
			return false, nil
		}

		if conditions.SocialDistanceMin > 0 && distance < conditions.SocialDistanceMin {
			return false, nil
		}

		if conditions.AuthorIsFollowing && distance != 1 {
			return false, nil
		}

		if conditions.AuthorIsMutual && !ctx.SocialGraph.IsMutual(ctx.OwnerPubkey, ctx.Event.PubKey) {
			return false, nil
		}
	}

	// Time-based conditions
	eventTime := time.Unix(int64(ctx.Event.CreatedAt), 0)
	now := time.Now()

	if conditions.AgeDaysMax > 0 {
		ageLimit := now.Add(-time.Duration(conditions.AgeDaysMax) * 24 * time.Hour)
		if eventTime.Before(ageLimit) {
			return false, nil
		}
	}

	if conditions.AgeDaysMin > 0 {
		ageLimit := now.Add(-time.Duration(conditions.AgeDaysMin) * 24 * time.Hour)
		if eventTime.After(ageLimit) {
			return false, nil
		}
	}

	// Size-based conditions
	if conditions.ContentSizeMax > 0 && len(ctx.Event.Content) > conditions.ContentSizeMax {
		return false, nil
	}

	if conditions.ContentSizeMin > 0 && len(ctx.Event.Content) < conditions.ContentSizeMin {
		return false, nil
	}

	if conditions.TagsCountMax > 0 && len(ctx.Event.Tags) > conditions.TagsCountMax {
		return false, nil
	}

	// Reference-based conditions (requires aggregates)
	if conditions.ReplyCountMin > 0 || conditions.ReactionCountMin > 0 || conditions.ZapSatsMin > 0 {
		agg, err := ctx.Storage.GetAggregateByID(ctx.Event.ID)
		if err != nil {
			// No aggregates found - treat as zero
			agg = &AggregateData{}
		}

		if conditions.ReplyCountMin > 0 && agg.ReplyCount < conditions.ReplyCountMin {
			return false, nil
		}

		if conditions.ReactionCountMin > 0 && agg.ReactionTotal < conditions.ReactionCountMin {
			return false, nil
		}

		if conditions.ZapSatsMin > 0 && agg.ZapSatsTotal < conditions.ZapSatsMin {
			return false, nil
		}
	}

	// If we get here, all conditions passed
	return true, nil
}

// applyAction converts a rule action to a retention decision
func (e *Engine) applyAction(event *nostr.Event, rule config.RetentionRule) (*RetentionDecision, error) {
	action := rule.Action

	decision := &RetentionDecision{
		EventID:      event.ID,
		RuleName:     rule.Name,
		RulePriority: rule.Priority,
		Protected:    false,
		Score:        0,
	}

	// Determine retention period
	if action.Retain {
		// Retain forever
		decision.RetainUntil = nil
		decision.Protected = true
	} else if action.RetainDays > 0 {
		// Retain for N days from created_at
		eventTime := time.Unix(int64(event.CreatedAt), 0)
		retainUntil := eventTime.Add(time.Duration(action.RetainDays) * 24 * time.Hour)
		decision.RetainUntil = &retainUntil
	} else if action.RetainUntil != "" {
		// Retain until specific date
		retainUntil, err := time.Parse(time.RFC3339, action.RetainUntil)
		if err != nil {
			return nil, fmt.Errorf("invalid retain_until date: %w", err)
		}
		decision.RetainUntil = &retainUntil
	} else if action.Delete {
		// Delete on next prune
		if action.DeleteAfterDays > 0 {
			deleteAt := time.Now().Add(time.Duration(action.DeleteAfterDays) * 24 * time.Hour)
			decision.RetainUntil = &deleteAt
		} else {
			// Immediate deletion
			now := time.Now()
			decision.RetainUntil = &now
		}
	}

	// Calculate score for cap enforcement
	decision.Score = e.calculateScore(event, rule.Priority)

	return decision, nil
}

// calculateScore calculates an event's priority score for cap enforcement
func (e *Engine) calculateScore(event *nostr.Event, rulePriority int) int {
	score := rulePriority * 100

	// Bonus for owner content
	if event.PubKey == e.ownerPubkey {
		score += 1000
	}

	// Bonus for close social distance
	distance := e.socialGraph.GetDistance(e.ownerPubkey, event.PubKey)
	if distance >= 0 {
		socialWeight := max(0, 10-distance)
		score += socialWeight * 100
	}

	// Age weight (newer is better)
	eventTime := time.Unix(int64(event.CreatedAt), 0)
	ageMonths := int(time.Since(eventTime).Hours() / 24 / 30)
	ageWeight := max(0, 10-ageMonths)
	score += ageWeight * 10

	// Interaction weight (from aggregates)
	if agg, err := e.storage.GetAggregateByID(event.ID); err == nil {
		interactionWeight := min(10, agg.ReplyCount+agg.ReactionTotal/10+int(agg.ZapSatsTotal/1000))
		score += interactionWeight * 5
	}

	return score
}

// Helper functions

func timePtr(t time.Time) *time.Time {
	return &t
}

func intInSlice(val int, slice []int) bool {
	for _, v := range slice {
		if v == val {
			return true
		}
	}
	return false
}

func stringInSlice(val string, slice []string) bool {
	for _, v := range slice {
		if v == val {
			return true
		}
	}
	return false
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
