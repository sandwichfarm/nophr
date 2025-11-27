package gemini

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sandwichfarm/nophr/internal/aggregates"
	"github.com/sandwichfarm/nophr/internal/config"
	"github.com/sandwichfarm/nophr/internal/storage"
)

func TestGeminiProtocol(t *testing.T) {
	// Create test config
	cfg := &config.Config{
		Identity: config.Identity{
			Npub: "npub1nq3zgtqruwhnz0xx40gh4a4fkamlr2sc7ke5wqs2s3nyv2fpy9esg4hdwq",
		},
		Storage: config.Storage{
			Driver:     "sqlite",
			SQLitePath: ":memory:",
		},
	}

	geminiCfg := &config.GeminiProtocol{
		Enabled: true,
		Host:    "localhost",
		Port:    11965, // Use non-standard port for testing
		TLS: config.GeminiTLS{
			AutoGenerate: true,
		},
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
	server, err := New(geminiCfg, cfg, st, "localhost", aggMgr)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Start server
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	// Give server time to start
	time.Sleep(200 * time.Millisecond)

	// Test 1: Root path
	t.Run("RootPath", func(t *testing.T) {
		response := sendGeminiRequest(t, geminiCfg.Port, "gemini://localhost/")
		if !strings.Contains(response, "20 ") {
			t.Errorf("Root response should have status 20, got: %s", response[:20])
		}
		if !strings.Contains(response, "nophr") {
			t.Errorf("Root response should contain 'nophr'")
		}
	})

	// Test 2: Notes path (was Outbox in Phase 16)
	t.Run("NotesPath", func(t *testing.T) {
		response := sendGeminiRequest(t, geminiCfg.Port, "gemini://localhost/notes")
		if !strings.Contains(response, "20 ") {
			t.Errorf("Notes response should have status 20, got: %s", response[:min(200, len(response))])
		}
		if !strings.Contains(response, "Notes") && !strings.Contains(response, "notes") {
			t.Errorf("Notes response should contain 'Notes' or 'notes'")
		}
	})

	// Test 3: Replies path (was Inbox in Phase 16)
	t.Run("RepliesPath", func(t *testing.T) {
		response := sendGeminiRequest(t, geminiCfg.Port, "gemini://localhost/replies")
		if !strings.Contains(response, "20 ") {
			t.Errorf("Replies response should have status 20, got: %s", response[:min(200, len(response))])
		}
		if !strings.Contains(response, "Replies") && !strings.Contains(response, "replies") {
			t.Errorf("Replies response should contain 'Replies' or 'replies'")
		}
	})

	// Test 4: Diagnostics path
	t.Run("DiagnosticsPath", func(t *testing.T) {
		response := sendGeminiRequest(t, geminiCfg.Port, "gemini://localhost/diagnostics")
		if !strings.Contains(response, "20 ") {
			t.Errorf("Diagnostics response should have status 20")
		}
		if !strings.Contains(response, "Diagnostics") {
			t.Errorf("Diagnostics response should contain 'Diagnostics'")
		}
	})

	// Test 5: Search path (should request input)
	t.Run("SearchPath", func(t *testing.T) {
		response := sendGeminiRequest(t, geminiCfg.Port, "gemini://localhost/search")
		if !strings.Contains(response, "10 ") {
			t.Errorf("Search without query should request input (status 10), got: %s", response[:20])
		}
	})

	// Test 6: Invalid path
	t.Run("InvalidPath", func(t *testing.T) {
		response := sendGeminiRequest(t, geminiCfg.Port, "gemini://localhost/invalid")
		if !strings.Contains(response, "51 ") {
			t.Errorf("Invalid path should return status 51 (not found), got: %s", response[:20])
		}
	})

	// Test 7: Invalid URL
	t.Run("InvalidURL", func(t *testing.T) {
		response := sendGeminiRequest(t, geminiCfg.Port, "not-a-url")
		if !strings.Contains(response, "59 ") {
			t.Errorf("Invalid URL should return status 59 (bad request), got: %s", response[:20])
		}
	})

	// Test 8: Non-gemini scheme
	t.Run("NonGeminiScheme", func(t *testing.T) {
		response := sendGeminiRequest(t, geminiCfg.Port, "http://localhost/")
		if !strings.Contains(response, "53 ") {
			t.Errorf("Non-gemini scheme should return status 53 (proxy refused), got: %s", response[:20])
		}
	})
}

func TestGeminiResponseFormat(t *testing.T) {
	// Test success response
	t.Run("SuccessResponse", func(t *testing.T) {
		response := FormatSuccessResponse("# Hello\n\nTest content")
		responseStr := string(response)

		if !strings.HasPrefix(responseStr, "20 ") {
			t.Errorf("Success response should start with '20 '")
		}
		if !strings.Contains(responseStr, "text/gemini") {
			t.Errorf("Success response should contain 'text/gemini'")
		}
		if !strings.Contains(responseStr, "\r\n") {
			t.Errorf("Response should contain CRLF")
		}
	})

	// Test error response
	t.Run("ErrorResponse", func(t *testing.T) {
		response := FormatErrorResponse(StatusNotFound, "Not found")
		responseStr := string(response)

		if !strings.HasPrefix(responseStr, "51 ") {
			t.Errorf("Error response should start with '51 '")
		}
		if !strings.Contains(responseStr, "Not found") {
			t.Errorf("Error response should contain error message")
		}
	})

	// Test input response
	t.Run("InputResponse", func(t *testing.T) {
		response := FormatInputResponse("Enter query:", false)
		responseStr := string(response)

		if !strings.HasPrefix(responseStr, "10 ") {
			t.Errorf("Input response should start with '10 '")
		}
		if !strings.Contains(responseStr, "Enter query:") {
			t.Errorf("Input response should contain prompt")
		}
	})

	// Test sensitive input response
	t.Run("SensitiveInputResponse", func(t *testing.T) {
		response := FormatInputResponse("Enter password:", true)
		responseStr := string(response)

		if !strings.HasPrefix(responseStr, "11 ") {
			t.Errorf("Sensitive input response should start with '11 '")
		}
	})

	// Test redirect response
	t.Run("RedirectResponse", func(t *testing.T) {
		response := FormatRedirectResponse("gemini://new.host/path", false)
		responseStr := string(response)

		if !strings.HasPrefix(responseStr, "30 ") {
			t.Errorf("Temporary redirect should start with '30 '")
		}

		response = FormatRedirectResponse("gemini://new.host/path", true)
		responseStr = string(response)

		if !strings.HasPrefix(responseStr, "31 ") {
			t.Errorf("Permanent redirect should start with '31 '")
		}
	})
}

func TestRendererOutput(t *testing.T) {
	cfg := &config.Config{
		Storage: config.Storage{
			Driver:     "sqlite",
			SQLitePath: ":memory:",
		},
		Display: config.Display{
			Feed: config.FeedDisplay{
				ShowInteractions: true,
			},
			Detail: config.DetailDisplay{
				ShowInteractions: true,
			},
		},
	}

	// Create storage for renderer
	ctx := context.Background()
	st, err := storage.New(ctx, &cfg.Storage)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer st.Close()

	renderer := NewRenderer(cfg, st)

	// Test home rendering
	t.Run("HomeRendering", func(t *testing.T) {
		home := renderer.RenderHome()

		if !strings.Contains(home, "# nophr") {
			t.Errorf("Home should contain title")
		}
		if !strings.Contains(home, "=> /notes") {
			t.Errorf("Home should contain notes link")
		}
		if !strings.Contains(home, "=> /replies") {
			t.Errorf("Home should contain replies link")
		}
	})

	// Test note list rendering
	t.Run("NoteListRendering", func(t *testing.T) {
		notes := []*aggregates.EnrichedEvent{}
		gemtext := renderer.RenderNoteList(notes, "Test List", "gemini://localhost/")

		if !strings.Contains(gemtext, "# Test List") {
			t.Errorf("Note list should contain title")
		}
		if !strings.Contains(gemtext, "No notes yet") {
			t.Errorf("Empty note list should say 'No notes yet'")
		}
	})
}

func TestGenerateSelfSignedCertFallsBackOnPersistError(t *testing.T) {
	// Create a path that cannot be used as a directory (parent is a file)
	blocker, err := os.CreateTemp(t.TempDir(), "nophr-gemini-cert-block")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	blocker.Close()

	certPath := filepath.Join(blocker.Name(), "cert.pem") // parent is a file, so persist should fail
	keyPath := filepath.Join(blocker.Name(), "key.pem")

	server := &Server{
		config: &config.GeminiProtocol{
			TLS: config.GeminiTLS{
				CertPath:     certPath,
				KeyPath:      keyPath,
				AutoGenerate: true,
			},
		},
		host: "localhost",
	}

	if err := server.generateSelfSignedCert(); err != nil {
		t.Fatalf("generateSelfSignedCert should succeed even when persistence fails: %v", err)
	}

	if server.tlsConfig == nil || len(server.tlsConfig.Certificates) == 0 {
		t.Fatalf("tlsConfig should be initialized even when certificate persistence fails")
	}
}

// Helper function to send a Gemini request
func sendGeminiRequest(t *testing.T, port int, url string) string {
	// Create TLS config that accepts self-signed certs
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
	}

	// Connect to server
	conn, err := tls.DialWithDialer(
		&net.Dialer{Timeout: 5 * time.Second},
		"tcp",
		net.JoinHostPort("localhost", fmt.Sprintf("%d", port)),
		tlsConfig,
	)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	// Send URL
	_, err = conn.Write([]byte(url + "\r\n"))
	if err != nil {
		t.Fatalf("Failed to send URL: %v", err)
	}

	// Read response
	reader := bufio.NewReader(conn)
	var response strings.Builder

	// Read until connection closes or timeout
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	for {
		line, err := reader.ReadString('\n')
		response.WriteString(line)
		if err != nil {
			break
		}
	}

	return response.String()
}
