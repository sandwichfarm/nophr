package ops

import (
	"bytes"
	"strings"
	"testing"

	"github.com/sandwichfarm/nophr/internal/config"
)

func TestNewLogger(t *testing.T) {
	tests := []struct {
		name   string
		config *config.Logging
	}{
		{
			name: "text format",
			config: &config.Logging{
				Level:  "info",
				Format: "text",
			},
		},
		{
			name: "json format",
			config: &config.Logging{
				Level:  "debug",
				Format: "json",
			},
		},
		{
			name: "warn level",
			config: &config.Logging{
				Level:  "warn",
				Format: "text",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := NewLogger(tt.config)
			if logger == nil {
				t.Fatal("expected logger to be created")
			}

			if logger.format != tt.config.Format {
				t.Errorf("expected format %s, got %s", tt.config.Format, logger.format)
			}
		})
	}
}

func TestLoggerWithComponent(t *testing.T) {
	var buf bytes.Buffer
	cfg := &config.Logging{
		Level:  "info",
		Format: "text",
	}

	logger := NewLoggerWithWriter(cfg, &buf)
	componentLogger := logger.WithComponent("test-component")

	componentLogger.Info("test message")

	output := buf.String()
	if !strings.Contains(output, "test message") {
		t.Errorf("expected log output to contain 'test message', got: %s", output)
	}

	if !strings.Contains(output, "component") {
		t.Errorf("expected log output to contain 'component', got: %s", output)
	}
}

func TestIsDebugEnabled(t *testing.T) {
	tests := []struct {
		name     string
		level    string
		expected bool
	}{
		{"debug level", "debug", true},
		{"info level", "info", false},
		{"warn level", "warn", false},
		{"error level", "error", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := NewLogger(&config.Logging{
				Level:  tt.level,
				Format: "text",
			})

			if logger.IsDebugEnabled() != tt.expected {
				t.Errorf("expected IsDebugEnabled to be %v, got %v", tt.expected, logger.IsDebugEnabled())
			}
		})
	}
}

func TestLoggerHelpers(t *testing.T) {
	var buf bytes.Buffer
	cfg := &config.Logging{
		Level:  "debug",
		Format: "text",
	}

	logger := NewLoggerWithWriter(cfg, &buf)

	// Test all helper methods don't panic
	logger.LogStorageOperation("test", 100, nil)
	logger.LogRelayConnection("wss://relay.test", true, nil)
	logger.LogSyncProgress("wss://relay.test", 1, 10, 12345)
	logger.LogProtocolRequest("gopher", "/test", 50, nil)
	logger.LogCacheOperation("get", "test-key", true)
	logger.LogAggregateUpdate("event123", 1, 5, 3, 2)
	logger.LogRetentionPrune(100, 5000, nil)
	logger.LogBackupOperation("backup", "/tmp/backup.db", 1024, nil)
	logger.LogStartup("v1.0.0", "abc123", map[string]interface{}{"key": "value"})
	logger.LogShutdown("test shutdown")

	output := buf.String()
	if output == "" {
		t.Error("expected log output, got empty string")
	}
}
