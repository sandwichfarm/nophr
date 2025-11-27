package nostr

import (
	"context"
	"fmt"
	"time"

	"github.com/nbd-wtf/go-nostr"
	"github.com/sandwichfarm/nophr/internal/storage"
)

// Discovery handles relay discovery using seed relays and NIP-65
type Discovery struct {
	client  *Client
	storage *storage.Storage
}

// NewDiscovery creates a new relay discovery instance
func NewDiscovery(client *Client, storage *storage.Storage) *Discovery {
	return &Discovery{
		client:  client,
		storage: storage,
	}
}

// BootstrapFromSeeds fetches relay hints for the operator's pubkey from seed relays
// This is the initial bootstrap step that discovers the operator's relay list
func (d *Discovery) BootstrapFromSeeds(ctx context.Context, operatorPubkey string) error {
	seedRelays := d.client.GetSeedRelays()
	if len(seedRelays) == 0 {
		return fmt.Errorf("no seed relays configured")
	}

	// Fetch the operator's NIP-65 relay list (kind 10002)
	filter := nostr.Filter{
		Kinds:   []int{10002},
		Authors: []string{operatorPubkey},
		Limit:   1,
	}

	// Create a timeout context
	timeout := d.client.GetDefaultTimeout()
	fetchCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	events, err := d.client.FetchEvents(fetchCtx, seedRelays, filter)
	if err != nil {
		return fmt.Errorf("failed to fetch relay hints from seeds: %w", err)
	}

	if len(events) == 0 {
		return fmt.Errorf("no relay hints found for operator pubkey %s", operatorPubkey)
	}

	// Parse and store relay hints
	hints, err := ParseRelayHints(events[0])
	if err != nil {
		return fmt.Errorf("failed to parse relay hints: %w", err)
	}

	// Save hints to storage
	for _, hint := range hints {
		if err := d.storage.SaveRelayHint(ctx, hint); err != nil {
			return fmt.Errorf("failed to save relay hint: %w", err)
		}
	}

	return nil
}

// DiscoverRelayHintsForPubkey fetches relay hints for a specific pubkey
// Uses the operator's relays to discover where to find the target pubkey
func (d *Discovery) DiscoverRelayHintsForPubkey(ctx context.Context, targetPubkey string, searchRelays []string) error {
	if len(searchRelays) == 0 {
		return fmt.Errorf("no relays provided for discovery")
	}

	// Fetch the target's NIP-65 relay list (kind 10002)
	filter := nostr.Filter{
		Kinds:   []int{10002},
		Authors: []string{targetPubkey},
		Limit:   1,
	}

	// Create a timeout context
	timeout := d.client.GetDefaultTimeout()
	fetchCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	events, err := d.client.FetchEvents(fetchCtx, searchRelays, filter)
	if err != nil {
		return fmt.Errorf("failed to fetch relay hints: %w", err)
	}

	if len(events) == 0 {
		// No relay hints found - not necessarily an error
		// The pubkey might not publish relay hints
		return nil
	}

	// Parse and store relay hints
	hints, err := ParseRelayHints(events[0])
	if err != nil {
		return fmt.Errorf("failed to parse relay hints: %w", err)
	}

	// Save hints to storage
	for _, hint := range hints {
		if err := d.storage.SaveRelayHint(ctx, hint); err != nil {
			return fmt.Errorf("failed to save relay hint: %w", err)
		}
	}

	return nil
}

// DiscoverRelayHintsForPubkeys discovers relay hints for multiple pubkeys
// This is more efficient than individual requests
func (d *Discovery) DiscoverRelayHintsForPubkeys(ctx context.Context, pubkeys []string, searchRelays []string) error {
	if len(searchRelays) == 0 {
		return fmt.Errorf("no relays provided for discovery")
	}

	if len(pubkeys) == 0 {
		return nil
	}

	// Fetch NIP-65 relay lists for all pubkeys
	filter := nostr.Filter{
		Kinds:   []int{10002},
		Authors: pubkeys,
	}

	// Create a timeout context
	timeout := d.client.GetDefaultTimeout()
	fetchCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	events, err := d.client.FetchEvents(fetchCtx, searchRelays, filter)
	if err != nil {
		return fmt.Errorf("failed to fetch relay hints: %w", err)
	}

	// Process each event
	for _, event := range events {
		hints, err := ParseRelayHints(event)
		if err != nil {
			// Log but don't fail on individual parse errors
			continue
		}

		// Save hints to storage
		for _, hint := range hints {
			if err := d.storage.SaveRelayHint(ctx, hint); err != nil {
				return fmt.Errorf("failed to save relay hint: %w", err)
			}
		}
	}

	return nil
}

// GetOutboxRelays returns where a pubkey PUBLISHES content (write relays)
// This is where you query to read someone's posts
func (d *Discovery) GetOutboxRelays(ctx context.Context, pubkey string) ([]string, error) {
	// First try write relays (outbox) - where they publish
	relays, err := d.storage.GetWriteRelays(ctx, pubkey)
	if err != nil {
		return nil, fmt.Errorf("failed to get write relays: %w", err)
	}

	if len(relays) > 0 {
		return relays, nil
	}

	// Fall back to read relays as backup
	relays, err = d.storage.GetReadRelays(ctx, pubkey)
	if err != nil {
		return nil, fmt.Errorf("failed to get read relays: %w", err)
	}

	return relays, nil
}

// GetInboxRelays returns where a pubkey RECEIVES interactions (read relays)
// This is where you query to find mentions/replies/reactions TO someone
func (d *Discovery) GetInboxRelays(ctx context.Context, pubkey string) ([]string, error) {
	// First try read relays (inbox) - where they receive interactions
	relays, err := d.storage.GetReadRelays(ctx, pubkey)
	if err != nil {
		return nil, fmt.Errorf("failed to get read relays: %w", err)
	}

	if len(relays) > 0 {
		return relays, nil
	}

	// Fall back to write relays as backup
	relays, err = d.storage.GetWriteRelays(ctx, pubkey)
	if err != nil {
		return nil, fmt.Errorf("failed to get write relays: %w", err)
	}

	return relays, nil
}

// GetRelaysForPubkey returns relays for a pubkey (backwards compatibility)
// Deprecated: Use GetOutboxRelays() or GetInboxRelays() for clarity
func (d *Discovery) GetRelaysForPubkey(ctx context.Context, pubkey string) ([]string, error) {
	// For backwards compatibility, return outbox relays (most common use case)
	return d.GetOutboxRelays(ctx, pubkey)
}

// RefreshRelayHints refreshes relay hints that are older than the given duration
func (d *Discovery) RefreshRelayHints(ctx context.Context, operatorPubkey string, maxAge time.Duration) error {
	// Get current relay hints
	hints, err := d.storage.GetRelayHints(ctx, operatorPubkey)
	if err != nil {
		return fmt.Errorf("failed to get relay hints: %w", err)
	}

	if len(hints) == 0 {
		// No hints to refresh, trigger bootstrap
		return d.BootstrapFromSeeds(ctx, operatorPubkey)
	}

	// Check if refresh is needed
	now := time.Now().Unix()
	maxAgeSeconds := int64(maxAge.Seconds())
	needsRefresh := false

	for _, hint := range hints {
		age := now - hint.Freshness
		if age > maxAgeSeconds {
			needsRefresh = true
			break
		}
	}

	if !needsRefresh {
		return nil
	}

	// Refresh from seed relays
	return d.BootstrapFromSeeds(ctx, operatorPubkey)
}

// RelayStatus contains relay status information
type RelayStatus struct {
	URL         string
	Connected   bool
	LastConnect *time.Time
	LastError   error
}

// GetRelays returns status information for all known relays
func (d *Discovery) GetRelays() []RelayStatus {
	// Get seed relays
	seedRelays := d.client.GetSeedRelays()

	relays := make([]RelayStatus, 0, len(seedRelays))
	for _, url := range seedRelays {
		relays = append(relays, RelayStatus{
			URL:       url,
			Connected: false, // TODO: Get actual connection status from client
		})
	}

	return relays
}
