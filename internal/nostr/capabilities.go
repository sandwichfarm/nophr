package nostr

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/sandwichfarm/nophr/internal/storage"
)

// NIP11RelayInfo represents relay information document (NIP-11)
type NIP11RelayInfo struct {
	Name          string   `json:"name"`
	Description   string   `json:"description"`
	PubKey        string   `json:"pubkey"`
	Contact       string   `json:"contact"`
	SupportedNIPs []int    `json:"supported_nips"`
	Software      string   `json:"software"`
	Version       string   `json:"version"`
	Limitation    struct{} `json:"limitation"`
}

// GetRelayCapabilities retrieves and caches relay capabilities
// Returns cached data if available and not expired, otherwise performs fresh check
func (c *Client) GetRelayCapabilities(ctx context.Context, url string, st *storage.Storage) (*storage.RelayCapabilities, error) {
	// Check cache first
	caps, err := st.GetRelayCapabilities(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("failed to check cached capabilities: %w", err)
	}

	// Return cached data if valid and not expired
	if caps != nil && time.Now().Before(caps.CheckExpiry) {
		return caps, nil
	}

	// Perform fresh capability check
	fmt.Printf("[RELAY] Checking capabilities for %s\n", url)
	caps, err = c.detectRelayCapabilities(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("failed to detect capabilities: %w", err)
	}

	// Cache the result (expires in 7 days)
	caps.LastChecked = time.Now()
	caps.CheckExpiry = caps.LastChecked.Add(7 * 24 * time.Hour)

	if err := st.SaveRelayCapabilities(ctx, caps); err != nil {
		// Log error but return capabilities anyway
		fmt.Printf("[RELAY] ⚠ Failed to cache capabilities: %v\n", err)
	}

	return caps, nil
}

// detectRelayCapabilities performs fresh capability detection
func (c *Client) detectRelayCapabilities(ctx context.Context, url string) (*storage.RelayCapabilities, error) {
	caps := &storage.RelayCapabilities{
		URL:                url,
		SupportsNegentropy: false,
	}

	// Step 1: Try NIP-11 relay information document
	info, err := c.fetchNIP11Info(ctx, url)
	if err == nil && info != nil {
		caps.NIP11Software = info.Software
		caps.NIP11Version = info.Version

		// Check if NIP-77 is listed in supported NIPs
		for _, nip := range info.SupportedNIPs {
			if nip == 77 {
				caps.SupportsNegentropy = true
				fmt.Printf("[RELAY] %s supports NIP-77 (via NIP-11)\n", url)
				return caps, nil
			}
		}
	}

	// Step 2: Try NEG-OPEN handshake as fallback verification
	// This is necessary because not all relays report NIP-77 in NIP-11
	supportsNeg, err := c.testNegentropyHandshake(ctx, url)
	if err == nil {
		caps.SupportsNegentropy = supportsNeg
		if supportsNeg {
			fmt.Printf("[RELAY] %s supports NIP-77 (via NEG-OPEN test)\n", url)
		} else {
			fmt.Printf("[RELAY] %s does not support NIP-77\n", url)
		}
	}

	return caps, nil
}

// fetchNIP11Info fetches relay information document (NIP-11)
func (c *Client) fetchNIP11Info(ctx context.Context, wsURL string) (*NIP11RelayInfo, error) {
	// Convert ws:// or wss:// to http:// or https://
	httpURL := strings.Replace(wsURL, "ws://", "http://", 1)
	httpURL = strings.Replace(httpURL, "wss://", "https://", 1)

	// Create HTTP request with NIP-11 header
	req, err := http.NewRequestWithContext(ctx, "GET", httpURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/nostr+json")

	// Execute request with timeout
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch NIP-11 info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("NIP-11 request failed: status %d", resp.StatusCode)
	}

	// Parse JSON response
	var info NIP11RelayInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("failed to parse NIP-11 response: %w", err)
	}

	return &info, nil
}

// testNegentropyHandshake tests if relay supports NIP-77 by attempting NEG-OPEN
// This is a simplified test - we'll rely more on NIP-11 and runtime fallback
func (c *Client) testNegentropyHandshake(ctx context.Context, url string) (bool, error) {
	// For now, skip the handshake test and rely on:
	// 1. NIP-11 supported_nips list
	// 2. Runtime detection when actually using negentropy (if it fails, update cache)
	//
	// This is safer than trying to parse low-level relay messages
	// which could break with different relay implementations.
	//
	// When we implement negentropy sync in Phase 2, we'll update
	// the cache if we get an "unsupported" error.

	return false, nil // Conservative default: assume not supported unless proven otherwise
}
