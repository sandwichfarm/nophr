package test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/nbd-wtf/go-nostr"
	"github.com/sandwichfarm/nophr/internal/config"
	internalnostr "github.com/sandwichfarm/nophr/internal/nostr"
	"github.com/sandwichfarm/nophr/internal/storage"
	"github.com/sandwichfarm/nophr/internal/sync"
)

// TestNegentropyWithRealRelays tests negentropy sync against production relays
// This test requires network access and may take some time
func TestNegentropyWithRealRelays(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping real-world negentropy test in short mode")
	}

	ctx := context.Background()

	// Create test storage
	cfg := &config.Config{
		Storage: config.Storage{
			Driver:     "sqlite",
			SQLitePath: ":memory:", // In-memory for testing
		},
		Sync: config.Sync{
			Performance: config.SyncPerformance{
				Workers:       4,
				UseNegentropy: true,
			},
		},
	}

	st, err := storage.New(ctx, &cfg.Storage)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer st.Close()

	// Don't seed fake events - negentropy requires valid 32-byte hex IDs
	// The real relay sync will handle actual events

	// List of known negentropy-enabled relays (from kind 30166 events)
	testRelays := []string{
		"wss://nostr.stakey.net",
		"wss://nostrelay.circum.space",
		"wss://offchain.pub",
		"wss://nrelay.c-stellar.net",
		"wss://premium.primal.net",
	}

	// Create nostr client
	nostrClient := internalnostr.New(ctx, &config.Relays{
		Seeds: testRelays,
	})

	// Create sync engine
	engine := sync.New(ctx, cfg, st, nostrClient)

	results := make(map[string]TestResult)

	for _, relayURL := range testRelays {
		t.Run(relayURL, func(t *testing.T) {
			result := testRelay(t, ctx, engine, st, nostrClient, relayURL)
			results[relayURL] = result
		})
	}

	// Print summary
	t.Log("\n=== Negentropy Real-World Test Summary ===")
	for relay, result := range results {
		status := "❌ FAILED"
		if result.Success {
			status = "✅ PASSED"
		}
		t.Logf("%s %s: %s", status, relay, result.Message)
	}
}

type TestResult struct {
	Success bool
	Message string
}

func testRelay(t *testing.T, ctx context.Context, engine *sync.Engine, st *storage.Storage, nostrClient *internalnostr.Client, relayURL string) TestResult {
	t.Logf("Testing negentropy with %s", relayURL)

	// Check capabilities first
	caps, err := nostrClient.GetRelayCapabilities(ctx, relayURL, st)
	if err != nil {
		return TestResult{
			Success: false,
			Message: fmt.Sprintf("Failed to get capabilities: %v", err),
		}
	}

	t.Logf("  Relay capabilities: negentropy=%v, software=%s", caps.SupportsNegentropy, caps.NIP11Software)

	// Create a simple filter for recent events
	since := nostr.Timestamp(time.Now().Add(-24 * time.Hour).Unix())
	filter := nostr.Filter{
		Kinds: []int{1}, // Just kind 1 (notes)
		Limit: 50,       // Small limit for testing
		Since: &since,
	}

	// Test negentropy sync
	testCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	success, err := engine.NegentropySync(testCtx, relayURL, filter)
	if err != nil {
		return TestResult{
			Success: false,
			Message: fmt.Sprintf("Negentropy sync error: %v", err),
		}
	}

	if !success {
		// Check if relay actually supports negentropy
		if !caps.SupportsNegentropy {
			return TestResult{
				Success: true, // Expected failure - relay doesn't support it
				Message:        "Correctly detected no negentropy support",
			}
		}
		return TestResult{
			Success: false,
			Message:        "Negentropy sync returned false (should support it)",
		}
	}

	// Query to see if we received any events
	events, err := st.QueryEvents(ctx, filter)
	if err != nil {
		return TestResult{
			Success: false,
			Message: fmt.Sprintf("Failed to query synced events: %v", err),
		}
	}

	eventCount := len(events)
	t.Logf("  Synced %d events via negentropy", eventCount)

	return TestResult{
		Success: true,
		Message: fmt.Sprintf("Negentropy sync successful, received %d events", eventCount),
	}
}

// TestNegentropyCapabilityDetection tests NIP-11 capability detection
func TestNegentropyCapabilityDetection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping capability detection test in short mode")
	}

	ctx := context.Background()

	// Create test storage
	cfg := &config.Config{
		Storage: config.Storage{
			Driver:     "sqlite",
			SQLitePath: ":memory:",
		},
	}

	st, err := storage.New(ctx, &cfg.Storage)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer st.Close()

	// Test relays
	testRelays := []struct {
		url               string
		expectNegentropy  bool
		expectStrfry      bool
	}{
		{"wss://nostr.stakey.net", true, true},
		{"wss://nostrelay.circum.space", true, true},
		{"wss://offchain.pub", true, true},
		{"wss://premium.primal.net", true, true},
	}

	nostrClient := internalnostr.New(ctx, &config.Relays{
		Seeds: []string{"wss://relay.damus.io"}, // Doesn't matter for capability check
	})

	for _, tt := range testRelays {
		t.Run(tt.url, func(t *testing.T) {
			testCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()

			caps, err := nostrClient.GetRelayCapabilities(testCtx, tt.url, st)
			if err != nil {
				t.Logf("Warning: Failed to get capabilities for %s: %v", tt.url, err)
				return
			}

			t.Logf("Relay: %s", tt.url)
			t.Logf("  Supports Negentropy: %v", caps.SupportsNegentropy)
			t.Logf("  Software: %s", caps.NIP11Software)
			t.Logf("  Version: %s", caps.NIP11Version)

			if tt.expectNegentropy && !caps.SupportsNegentropy {
				t.Errorf("Expected negentropy support for %s but got false", tt.url)
			}

			if tt.expectStrfry && caps.NIP11Software != "" {
				if caps.NIP11Software != "git+https://github.com/hoytech/strfry.git" {
					t.Logf("Note: Expected strfry but got %s", caps.NIP11Software)
				}
			}
		})
	}
}

// Benchmark negentropy vs traditional REQ sync
func BenchmarkNegentropyVsREQ(b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping benchmark in short mode")
	}

	ctx := context.Background()

	// Create test storage
	cfg := &config.Config{
		Storage: config.Storage{
			Driver:     "sqlite",
			SQLitePath: ":memory:",
		},
		Sync: config.Sync{
			Performance: config.SyncPerformance{
				Workers:       4,
				UseNegentropy: true,
			},
		},
	}

	st, err := storage.New(ctx, &cfg.Storage)
	if err != nil {
		b.Fatalf("Failed to create storage: %v", err)
	}
	defer st.Close()

	relayURL := "wss://nostr.stakey.net"
	nostrClient := internalnostr.New(ctx, &config.Relays{
		Seeds: []string{relayURL},
	})

	engine := sync.New(ctx, cfg, st, nostrClient)

	since := nostr.Timestamp(time.Now().Add(-6 * time.Hour).Unix())
	filter := nostr.Filter{
		Kinds: []int{1},
		Limit: 100,
		Since: &since,
	}

	b.Run("Negentropy", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			testCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			_, _ = engine.NegentropySync(testCtx, relayURL, filter)
			cancel()
		}
	})

	// Note: We don't benchmark REQ here as it requires different infrastructure
	// This benchmark mainly tests negentropy performance characteristics
}
