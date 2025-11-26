package sync

import (
	"testing"

	"github.com/sandwich/nophr/internal/config"
)

func TestNewFilterBuilder(t *testing.T) {
	cfg := &config.Sync{
		Kinds: config.SyncKinds{
			Notes:       true,
			ContactList: true,
		},
	}

	fb := NewFilterBuilder(cfg)
	if fb == nil {
		t.Fatal("Expected filter builder, got nil")
	}

	if fb.config != cfg {
		t.Error("Config not set correctly")
	}
}

func TestBuildFilters(t *testing.T) {
	tests := []struct {
		name            string
		cfg             *config.Sync
		authors         []string
		since           int64
		expectedFilters int
		expectedKinds   int
	}{
		{
			name: "with configured kinds",
			cfg: &config.Sync{
				Kinds: config.SyncKinds{
					Notes:       true,
					ContactList: true,
					Articles:    true,
				},
			},
			authors:         []string{"pubkey1", "pubkey2"},
			since:           12345,
			expectedFilters: 2, // replaceable (kinds 3) + regular (kinds 1)
			expectedKinds:   3,
		},
		{
			name:            "with default kinds",
			cfg:             &config.Sync{Kinds: config.SyncKinds{}},
			authors:         []string{"pubkey1"},
			since:           0,
			expectedFilters: 2, // replaceable + regular split
			expectedKinds:   8, // Empty SyncKinds falls back to 8 default kinds
		},
		{
			name:            "empty authors",
			cfg:             &config.Sync{},
			authors:         []string{},
			since:           0,
			expectedFilters: 0,
			expectedKinds:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fb := NewFilterBuilder(tt.cfg)
			filters := fb.BuildFilters(tt.authors, tt.since)

			if len(tt.authors) == 0 {
				if len(filters) != 0 {
					t.Errorf("Expected 0 filters for empty authors, got %d", len(filters))
				}
				return
			}

			if len(filters) != tt.expectedFilters {
				t.Errorf("Expected %d filters, got %d", tt.expectedFilters, len(filters))
			}

			totalKinds := 0
			for _, f := range filters {
				totalKinds += len(f.Kinds)
			}
			if totalKinds != tt.expectedKinds {
				t.Errorf("Expected %d total kinds, got %d", tt.expectedKinds, totalKinds)
			}
		})
	}
}

func TestBuildMentionFilter(t *testing.T) {
	cfg := &config.Sync{
		Kinds: config.SyncKinds{
			Notes: true,
		},
	}

	fb := NewFilterBuilder(cfg)
	filter := fb.BuildMentionFilter("owner-pubkey", 12345)

	if len(filter.Tags) == 0 {
		t.Error("Expected p tag in mention filter")
	}

	if filter.Tags["p"][0] != "owner-pubkey" {
		t.Error("Expected owner pubkey in p tag")
	}
}

func TestBuildThreadFilter(t *testing.T) {
	cfg := &config.Sync{}
	fb := NewFilterBuilder(cfg)

	eventIDs := []string{"event1", "event2"}
	filter := fb.BuildThreadFilter(eventIDs, 12345)

	if len(filter.Kinds) != 1 || filter.Kinds[0] != 1 {
		t.Error("Expected kind 1 for thread filter")
	}

	if len(filter.Tags) == 0 {
		t.Error("Expected e tag in thread filter")
	}
}

func TestBuildReplaceableFilter(t *testing.T) {
	cfg := &config.Sync{}
	fb := NewFilterBuilder(cfg)

	authors := []string{"pubkey1", "pubkey2"}
	filter := fb.BuildReplaceableFilter(authors)

	expectedKinds := []int{0, 3, 10002, 30023} // Added 30023 (long-form articles)
	if len(filter.Kinds) != len(expectedKinds) {
		t.Errorf("Expected %d kinds, got %d", len(expectedKinds), len(filter.Kinds))
	}

	for i, kind := range expectedKinds {
		if filter.Kinds[i] != kind {
			t.Errorf("Expected kind %d at index %d, got %d", kind, i, filter.Kinds[i])
		}
	}
}

func TestShouldIncludeAuthor(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *config.Sync
		pubkey   string
		expected bool
	}{
		{
			name: "not in denylist",
			cfg: &config.Sync{
				Scope: config.SyncScope{
					DenylistPubkeys: []string{"denied"},
				},
			},
			pubkey:   "allowed",
			expected: true,
		},
		{
			name: "in denylist",
			cfg: &config.Sync{
				Scope: config.SyncScope{
					DenylistPubkeys: []string{"denied"},
				},
			},
			pubkey:   "denied",
			expected: false,
		},
		{
			name: "in allowlist",
			cfg: &config.Sync{
				Scope: config.SyncScope{
					AllowlistPubkeys: []string{"allowed"},
				},
			},
			pubkey:   "allowed",
			expected: true,
		},
		{
			name: "not in allowlist",
			cfg: &config.Sync{
				Scope: config.SyncScope{
					AllowlistPubkeys: []string{"allowed"},
				},
			},
			pubkey:   "other",
			expected: false,
		},
		{
			name:     "no lists",
			cfg:      &config.Sync{},
			pubkey:   "anyone",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fb := NewFilterBuilder(tt.cfg)
			result := fb.ShouldIncludeAuthor(tt.pubkey)

			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestGetConfiguredKinds(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *config.Sync
		expected int
	}{
		{
			name: "custom kinds",
			cfg: &config.Sync{
				Kinds: config.SyncKinds{
					Notes:       true,
					ContactList: true,
					Reactions:   true,
				},
			},
			expected: 3,
		},
		{
			name:     "empty kinds (defaults)",
			cfg:      &config.Sync{Kinds: config.SyncKinds{}},
			expected: 8, // Falls back to 8 default kinds
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fb := NewFilterBuilder(tt.cfg)
			kinds := fb.GetConfiguredKinds()

			if len(kinds) != tt.expected {
				t.Errorf("Expected %d kinds, got %d", tt.expected, len(kinds))
			}
		})
	}
}
