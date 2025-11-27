package aggregates

import (
	"testing"

	"github.com/nbd-wtf/go-nostr"
	"github.com/sandwichfarm/nophr/internal/config"
)

func TestPassesContentFilter(t *testing.T) {
	tests := []struct {
		name       string
		cfg        config.ContentFiltering
		event      *EnrichedEvent
		shouldPass bool
	}{
		{
			name: "filtering disabled - should pass",
			cfg: config.ContentFiltering{
				Enabled: false,
			},
			event: &EnrichedEvent{
				Event: &nostr.Event{ID: "test1"},
				Aggregates: &EventAggregates{
					ReactionTotal: 0,
					ZapSatsTotal:  0,
				},
			},
			shouldPass: true,
		},
		{
			name: "meets min reactions threshold",
			cfg: config.ContentFiltering{
				Enabled:      true,
				MinReactions: 5,
			},
			event: &EnrichedEvent{
				Event: &nostr.Event{ID: "test2"},
				Aggregates: &EventAggregates{
					ReactionTotal: 10,
				},
			},
			shouldPass: true,
		},
		{
			name: "fails min reactions threshold",
			cfg: config.ContentFiltering{
				Enabled:      true,
				MinReactions: 10,
			},
			event: &EnrichedEvent{
				Event: &nostr.Event{ID: "test3"},
				Aggregates: &EventAggregates{
					ReactionTotal: 5,
				},
			},
			shouldPass: false,
		},
		{
			name: "meets min zap sats threshold",
			cfg: config.ContentFiltering{
				Enabled:    true,
				MinZapSats: 1000,
			},
			event: &EnrichedEvent{
				Event: &nostr.Event{ID: "test4"},
				Aggregates: &EventAggregates{
					ZapSatsTotal: 5000,
				},
			},
			shouldPass: true,
		},
		{
			name: "fails min zap sats threshold",
			cfg: config.ContentFiltering{
				Enabled:    true,
				MinZapSats: 10000,
			},
			event: &EnrichedEvent{
				Event: &nostr.Event{ID: "test5"},
				Aggregates: &EventAggregates{
					ZapSatsTotal: 1000,
				},
			},
			shouldPass: false,
		},
		{
			name: "meets min engagement threshold",
			cfg: config.ContentFiltering{
				Enabled:       true,
				MinEngagement: 20,
			},
			event: &EnrichedEvent{
				Event: &nostr.Event{ID: "test6"},
				Aggregates: &EventAggregates{
					ReplyCount:    5,
					ReactionTotal: 10,
					ZapSatsTotal:  5000, // 5 points from sats (5000/1000)
				},
			},
			shouldPass: true,
		},
		{
			name: "fails min engagement threshold",
			cfg: config.ContentFiltering{
				Enabled:       true,
				MinEngagement: 100,
			},
			event: &EnrichedEvent{
				Event: &nostr.Event{ID: "test7"},
				Aggregates: &EventAggregates{
					ReplyCount:    2,
					ReactionTotal: 3,
					ZapSatsTotal:  100,
				},
			},
			shouldPass: false,
		},
		{
			name: "hide no interactions - has interactions",
			cfg: config.ContentFiltering{
				Enabled:            true,
				HideNoInteractions: true,
			},
			event: &EnrichedEvent{
				Event: &nostr.Event{ID: "test8"},
				Aggregates: &EventAggregates{
					ReplyCount: 1,
				},
			},
			shouldPass: true,
		},
		{
			name: "hide no interactions - no interactions",
			cfg: config.ContentFiltering{
				Enabled:            true,
				HideNoInteractions: true,
			},
			event: &EnrichedEvent{
				Event: &nostr.Event{ID: "test9"},
				Aggregates: &EventAggregates{
					ReplyCount:    0,
					ReactionTotal: 0,
					ZapSatsTotal:  0,
				},
			},
			shouldPass: false,
		},
		{
			name: "multiple criteria - all pass",
			cfg: config.ContentFiltering{
				Enabled:      true,
				MinReactions: 5,
				MinZapSats:   1000,
			},
			event: &EnrichedEvent{
				Event: &nostr.Event{ID: "test10"},
				Aggregates: &EventAggregates{
					ReactionTotal: 10,
					ZapSatsTotal:  2000,
				},
			},
			shouldPass: true,
		},
		{
			name: "multiple criteria - one fails",
			cfg: config.ContentFiltering{
				Enabled:      true,
				MinReactions: 10,
				MinZapSats:   1000,
			},
			event: &EnrichedEvent{
				Event: &nostr.Event{ID: "test11"},
				Aggregates: &EventAggregates{
					ReactionTotal: 5,  // fails
					ZapSatsTotal:  2000, // passes
				},
			},
			shouldPass: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			qh := &QueryHelper{
				config: &config.Config{
					Behavior: config.Behavior{
						ContentFiltering: tt.cfg,
					},
				},
			}

			result := qh.passesContentFilter(tt.event)
			if result != tt.shouldPass {
				t.Errorf("Expected %v, got %v", tt.shouldPass, result)
			}
		})
	}
}

func TestFilterAndSortEvents(t *testing.T) {
	// Create test events with different interaction scores
	events := []*EnrichedEvent{
		{
			Event: &nostr.Event{ID: "event1", CreatedAt: 1000},
			Aggregates: &EventAggregates{
				ReactionTotal: 5,
				ZapSatsTotal:  1000,
				ReplyCount:    2,
			},
		},
		{
			Event: &nostr.Event{ID: "event2", CreatedAt: 2000},
			Aggregates: &EventAggregates{
				ReactionTotal: 10,
				ZapSatsTotal:  5000,
				ReplyCount:    5,
			},
		},
		{
			Event: &nostr.Event{ID: "event3", CreatedAt: 1500},
			Aggregates: &EventAggregates{
				ReactionTotal: 3,
				ZapSatsTotal:  500,
				ReplyCount:    1,
			},
		},
		{
			Event: &nostr.Event{ID: "event4", CreatedAt: 2500},
			Aggregates: &EventAggregates{
				ReactionTotal: 0,
				ZapSatsTotal:  0,
				ReplyCount:    0,
			},
		},
	}

	tests := []struct {
		name           string
		sortMode       string
		filterEnabled  bool
		minReactions   int
		expectedCount  int
		expectedFirst  string // ID of expected first event
	}{
		{
			name:          "chronological sort - no filter",
			sortMode:      "chronological",
			filterEnabled: false,
			expectedCount: 4,
			expectedFirst: "event1", // Oldest first
		},
		{
			name:          "engagement sort - no filter",
			sortMode:      "engagement",
			filterEnabled: false,
			expectedCount: 4,
			expectedFirst: "event2", // Highest engagement
		},
		{
			name:          "zaps sort - no filter",
			sortMode:      "zaps",
			filterEnabled: false,
			expectedCount: 4,
			expectedFirst: "event2", // Most zaps
		},
		{
			name:          "reactions sort - no filter",
			sortMode:      "reactions",
			filterEnabled: false,
			expectedCount: 4,
			expectedFirst: "event2", // Most reactions
		},
		{
			name:          "engagement sort with filter",
			sortMode:      "engagement",
			filterEnabled: true,
			minReactions:  5,
			expectedCount: 2, // Only event1 and event2 have >= 5 reactions
			expectedFirst: "event2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			qh := &QueryHelper{
				config: &config.Config{
					Behavior: config.Behavior{
						ContentFiltering: config.ContentFiltering{
							Enabled:      tt.filterEnabled,
							MinReactions: tt.minReactions,
						},
					},
				},
			}

			result := qh.filterAndSortEvents(events, tt.sortMode)

			if len(result) != tt.expectedCount {
				t.Errorf("Expected %d events, got %d", tt.expectedCount, len(result))
			}

			if len(result) > 0 && result[0].Event.ID != tt.expectedFirst {
				t.Errorf("Expected first event to be %s, got %s", tt.expectedFirst, result[0].Event.ID)
			}
		})
	}
}

func TestInteractionScore(t *testing.T) {
	tests := []struct {
		name     string
		agg      *EventAggregates
		expected int64
	}{
		{
			name: "all zeros",
			agg: &EventAggregates{
				ReplyCount:    0,
				ReactionTotal: 0,
				ZapSatsTotal:  0,
			},
			expected: 0,
		},
		{
			name: "only replies",
			agg: &EventAggregates{
				ReplyCount:    5,
				ReactionTotal: 0,
				ZapSatsTotal:  0,
			},
			expected: 5,
		},
		{
			name: "only reactions",
			agg: &EventAggregates{
				ReplyCount:    0,
				ReactionTotal: 10,
				ZapSatsTotal:  0,
			},
			expected: 10,
		},
		{
			name: "only zaps",
			agg: &EventAggregates{
				ReplyCount:    0,
				ReactionTotal: 0,
				ZapSatsTotal:  3000, // 3 points (3000/1000)
			},
			expected: 3,
		},
		{
			name: "combined interactions",
			agg: &EventAggregates{
				ReplyCount:    5,
				ReactionTotal: 10,
				ZapSatsTotal:  3000, // 3 points (3000/1000)
			},
			expected: 18, // 5 + 10 + 3
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := tt.agg.InteractionScore()
			if score != tt.expected {
				t.Errorf("Expected score %d, got %d", tt.expected, score)
			}
		})
	}
}

func TestHasInteractions(t *testing.T) {
	tests := []struct {
		name     string
		agg      *EventAggregates
		expected bool
	}{
		{
			name: "no interactions",
			agg: &EventAggregates{
				ReplyCount:    0,
				ReactionTotal: 0,
				ZapSatsTotal:  0,
			},
			expected: false,
		},
		{
			name: "has replies",
			agg: &EventAggregates{
				ReplyCount:    1,
				ReactionTotal: 0,
				ZapSatsTotal:  0,
			},
			expected: true,
		},
		{
			name: "has reactions",
			agg: &EventAggregates{
				ReplyCount:    0,
				ReactionTotal: 1,
				ZapSatsTotal:  0,
			},
			expected: true,
		},
		{
			name: "has zaps",
			agg: &EventAggregates{
				ReplyCount:    0,
				ReactionTotal: 0,
				ZapSatsTotal:  100,
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.agg.HasInteractions()
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}
