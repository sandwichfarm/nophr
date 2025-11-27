package nostr

import (
	"context"
	"fmt"
	"time"

	"github.com/nbd-wtf/go-nostr"
	"github.com/sandwichfarm/nophr/internal/config"
)

// Client provides a high-level interface for interacting with Nostr relays
type Client struct {
	pool        *nostr.SimplePool
	relayConfig *config.Relays
	ctx         context.Context
}

// New creates a new Nostr client with the given configuration
func New(ctx context.Context, relayConfig *config.Relays) *Client {
	pool := nostr.NewSimplePool(ctx)
	return &Client{
		pool:        pool,
		relayConfig: relayConfig,
		ctx:         ctx,
	}
}

// Pool returns the underlying SimplePool for advanced operations
func (c *Client) Pool() *nostr.SimplePool {
	return c.pool
}

// FetchEvents fetches events from the given relays matching the filter
func (c *Client) FetchEvents(ctx context.Context, relays []string, filter nostr.Filter) ([]*nostr.Event, error) {
	events := make([]*nostr.Event, 0)

	// Use SubManyEose to get events and wait for EOSE
	for relayEvent := range c.pool.SubManyEose(ctx, relays, nostr.Filters{filter}) {
		if relayEvent.Event != nil {
			events = append(events, relayEvent.Event)
		}
	}

	return events, nil
}

// FetchEvent fetches a single event by ID from the given relays
func (c *Client) FetchEvent(ctx context.Context, relays []string, eventID string) (*nostr.Event, error) {
	filter := nostr.Filter{
		IDs: []string{eventID},
	}

	result := c.pool.QuerySingle(ctx, relays, filter)
	if result == nil || result.Event == nil {
		return nil, fmt.Errorf("event not found: %s", eventID)
	}

	return result.Event, nil
}

// PublishEvent publishes an event to the given relays
func (c *Client) PublishEvent(ctx context.Context, relays []string, event *nostr.Event) error {
	results := c.pool.PublishMany(ctx, relays, *event)

	var lastErr error
	successCount := 0

	for result := range results {
		if result.Error != nil {
			lastErr = result.Error
		} else {
			successCount++
		}
	}

	if successCount == 0 && lastErr != nil {
		return fmt.Errorf("failed to publish to any relay: %w", lastErr)
	}

	return nil
}

// SubscribeEvents subscribes to events matching the filter on the given relays
// Returns a channel of events that will be closed when the context is cancelled
func (c *Client) SubscribeEvents(ctx context.Context, relays []string, filters nostr.Filters) <-chan *nostr.Event {
	eventChan := make(chan *nostr.Event, 100)

	go func() {
		defer close(eventChan)

		fmt.Printf("[NOSTR CLIENT] Starting SubMany for %d relays with %d filters\n", len(relays), len(filters))
		for i, relay := range relays {
			fmt.Printf("[NOSTR CLIENT]   Relay %d: %s\n", i+1, relay)
		}

		eventCount := 0
		for relayEvent := range c.pool.SubMany(ctx, relays, filters) {
			if relayEvent.Event != nil {
				eventCount++
				if eventCount == 1 {
					fmt.Printf("[NOSTR CLIENT] ✓ First event received from %s\n", relayEvent.Relay.URL)
				}
				if eventCount%10 == 0 {
					fmt.Printf("[NOSTR CLIENT] Received %d events so far...\n", eventCount)
				}

				select {
				case eventChan <- relayEvent.Event:
				case <-ctx.Done():
					fmt.Printf("[NOSTR CLIENT] Context cancelled after receiving %d events\n", eventCount)
					return
				}
			} else if relayEvent.Relay != nil {
				// Log relay connection events
				fmt.Printf("[NOSTR CLIENT] Relay event from %s (no event data)\n", relayEvent.Relay.URL)
			}
		}

		fmt.Printf("[NOSTR CLIENT] SubMany channel closed. Total events received: %d\n", eventCount)
	}()

	return eventChan
}

// Close closes all relay connections
func (c *Client) Close() {
	c.pool.Close("client shutting down")
}

// GetSeedRelays returns the configured seed relays
func (c *Client) GetSeedRelays() []string {
	if c.relayConfig == nil {
		return []string{}
	}
	return c.relayConfig.Seeds
}

// GetDefaultTimeout returns the configured timeout duration
func (c *Client) GetDefaultTimeout() time.Duration {
	if c.relayConfig == nil || c.relayConfig.Policy.ConnectTimeoutMs == 0 {
		return 30 * time.Second
	}
	return time.Duration(c.relayConfig.Policy.ConnectTimeoutMs) * time.Millisecond
}
