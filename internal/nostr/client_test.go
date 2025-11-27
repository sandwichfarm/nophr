package nostr

import (
	"context"
	"testing"
	"time"

	"github.com/sandwichfarm/nophr/internal/config"
)

func TestNew(t *testing.T) {
	ctx := context.Background()
	cfg := &config.Relays{
		Seeds: []string{"wss://relay.test"},
		Policy: config.RelayPolicy{
			ConnectTimeoutMs: 30000,
		},
	}

	client := New(ctx, cfg)
	if client == nil {
		t.Fatal("Expected client, got nil")
	}

	if client.Pool() == nil {
		t.Error("Expected pool to be initialized")
	}

	defer client.Close()
}

func TestGetSeedRelays(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *config.Relays
		expected []string
	}{
		{
			name: "with seed relays",
			cfg: &config.Relays{
				Seeds: []string{"wss://relay1.test", "wss://relay2.test"},
			},
			expected: []string{"wss://relay1.test", "wss://relay2.test"},
		},
		{
			name:     "nil config",
			cfg:      nil,
			expected: []string{},
		},
		{
			name:     "empty seed relays",
			cfg:      &config.Relays{Seeds: []string{}},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			client := New(ctx, tt.cfg)
			defer client.Close()

			relays := client.GetSeedRelays()
			if len(relays) != len(tt.expected) {
				t.Errorf("Expected %d relays, got %d", len(tt.expected), len(relays))
			}
		})
	}
}

func TestGetDefaultTimeout(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *config.Relays
		expected time.Duration
	}{
		{
			name: "with timeout",
			cfg: &config.Relays{
				Policy: config.RelayPolicy{ConnectTimeoutMs: 60000},
			},
			expected: 60 * time.Second,
		},
		{
			name:     "nil config",
			cfg:      nil,
			expected: 30 * time.Second,
		},
		{
			name: "zero timeout",
			cfg: &config.Relays{
				Policy: config.RelayPolicy{ConnectTimeoutMs: 0},
			},
			expected: 30 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			client := New(ctx, tt.cfg)
			defer client.Close()

			timeout := client.GetDefaultTimeout()
			if timeout != tt.expected {
				t.Errorf("Expected timeout %v, got %v", tt.expected, timeout)
			}
		})
	}
}
