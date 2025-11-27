package finger

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/sandwichfarm/nophr/internal/aggregates"
	"github.com/sandwichfarm/nophr/internal/config"
	"github.com/sandwichfarm/nophr/internal/storage"
)

func TestFingerProtocol(t *testing.T) {
	// Create test config
	cfg := &config.Config{
		Identity: config.Identity{
			Npub: "test-pubkey-1234567890abcdef",
		},
		Storage: config.Storage{
			Driver:     "sqlite",
			SQLitePath: ":memory:",
		},
	}

	fingerCfg := &config.FingerProtocol{
		Enabled:  true,
		Port:     17079, // Use non-standard port for testing
		Bind:     "localhost",
		MaxUsers: 10,
	}

	// Create storage
	ctx := context.Background()
	st, err := storage.New(ctx, &cfg.Storage)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer st.Close()

	// Create aggregates manager
	aggMgr := aggregates.NewManager(st, cfg)

	// Create server
	server := New(fingerCfg, cfg, st, aggMgr)

	// Start server
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Test 1: Query owner by "owner" username
	t.Run("QueryOwner", func(t *testing.T) {
		response := sendFingerRequest(t, fingerCfg.Port, "owner")
		if !strings.Contains(response, "User:") {
			t.Errorf("Owner query should contain 'User:', got: %s", response)
		}
		if !strings.Contains(response, "Pubkey:") {
			t.Errorf("Owner query should contain 'Pubkey:'")
		}
	})

	// Test 2: Empty query (list users)
	t.Run("EmptyQuery", func(t *testing.T) {
		response := sendFingerRequest(t, fingerCfg.Port, "")
		if !strings.Contains(response, "User:") {
			t.Errorf("Empty query should return owner info, got: %s", response)
		}
	})

	// Test 3: Verbose query with /W flag
	t.Run("VerboseQuery", func(t *testing.T) {
		response := sendFingerRequest(t, fingerCfg.Port, "/W owner")
		if !strings.Contains(response, "User:") {
			t.Errorf("Verbose query should contain 'User:'")
		}
		if !strings.Contains(response, "Recent Activity") {
			t.Errorf("Verbose query should show recent activity")
		}
	})

	// Test 4: Non-existent user
	t.Run("NonExistentUser", func(t *testing.T) {
		response := sendFingerRequest(t, fingerCfg.Port, "nonexistent-pubkey-xyz")
		if !strings.Contains(response, "not found") {
			t.Errorf("Non-existent user should return 'not found', got: %s", response)
		}
	})

	// Test 5: Forwarding not supported
	t.Run("ForwardingNotSupported", func(t *testing.T) {
		response := sendFingerRequest(t, fingerCfg.Port, "user@otherhost")
		if !strings.Contains(response, "not supported") {
			t.Errorf("Forwarding should not be supported, got: %s", response)
		}
	})

	// Test 6: CRLF line endings
	t.Run("CRLFLineEndings", func(t *testing.T) {
		response := sendFingerRequest(t, fingerCfg.Port, "owner")
		if !strings.Contains(response, "\r\n") {
			t.Errorf("Response should contain CRLF line endings per RFC 1288")
		}
	})
}

func TestQueryParsing(t *testing.T) {
	tests := []struct {
		input    string
		verbose  bool
		username string
		host     string
	}{
		{"owner", false, "owner", ""},
		{"/W owner", true, "owner", ""},
		{"/w owner", true, "owner", ""},
		{"user@host", false, "user", "host"},
		{"/W user@host", true, "user", "host"},
		{"", false, "", ""},
		{"/W", true, "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			q := ParseQuery(tt.input)
			if q.Verbose != tt.verbose {
				t.Errorf("ParseQuery(%q).Verbose = %v, want %v", tt.input, q.Verbose, tt.verbose)
			}
			if q.Username != tt.username {
				t.Errorf("ParseQuery(%q).Username = %q, want %q", tt.input, q.Username, tt.username)
			}
			if q.Host != tt.host {
				t.Errorf("ParseQuery(%q).Host = %q, want %q", tt.input, q.Host, tt.host)
			}
		})
	}
}

func TestRenderer(t *testing.T) {
	renderer := NewRenderer()

	// Test basic rendering
	t.Run("BasicRendering", func(t *testing.T) {
		result := renderer.RenderUser("pubkey123", nil, []*enrichedNote{}, false)
		if !strings.Contains(result, "User:") {
			t.Errorf("Render should contain 'User:'")
		}
		if !strings.Contains(result, "Pubkey:") {
			t.Errorf("Render should contain 'Pubkey:'")
		}
	})

	// Test verbose rendering
	t.Run("VerboseRendering", func(t *testing.T) {
		result := renderer.RenderUser("pubkey123", nil, []*enrichedNote{}, true)
		if !strings.Contains(result, "Recent Activity") {
			t.Errorf("Verbose render should show recent activity")
		}
	})

	// Test truncatePubkey
	t.Run("TruncatePubkey", func(t *testing.T) {
		short := truncatePubkey("short")
		if short != "short" {
			t.Errorf("Short pubkey should not be truncated")
		}

		long := truncatePubkey("verylongpubkey1234567890abcdef")
		if !strings.Contains(long, "...") {
			t.Errorf("Long pubkey should be truncated with ...")
		}
		if len(long) > 20 {
			t.Errorf("Truncated pubkey too long: %d chars", len(long))
		}
	})
}

// Helper function to send a Finger request
func sendFingerRequest(t *testing.T, port int, query string) string {
	// Connect to server
	conn, err := net.DialTimeout("tcp", net.JoinHostPort("localhost", fmt.Sprintf("%d", port)), 5*time.Second)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	// Send query
	_, err = conn.Write([]byte(query + "\r\n"))
	if err != nil {
		t.Fatalf("Failed to send query: %v", err)
	}

	// Read response
	reader := bufio.NewReader(conn)
	var response strings.Builder

	// Set read deadline
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))

	// Read until connection closes or timeout
	for {
		line, err := reader.ReadString('\n')
		response.WriteString(line)
		if err != nil {
			break
		}
	}

	return response.String()
}
