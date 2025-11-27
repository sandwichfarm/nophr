package presentation

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sandwichfarm/nophr/internal/config"
)

func TestGetHeader(t *testing.T) {
	tests := []struct {
		name           string
		cfg            *config.Config
		page           string
		expectedText   string
		shouldContain  []string
		shouldNotExist bool
	}{
		{
			name: "global header with inline content",
			cfg: &config.Config{
				Site: config.Site{
					Title:       "Test Site",
					Description: "Test Description",
					Operator:    "Test Operator",
				},
				Presentation: config.Presentation{
					Headers: config.Headers{
						Global: config.HeaderConfig{
							Enabled: true,
							Content: "Welcome to {{site.title}}!",
						},
					},
				},
			},
			page:          "",
			expectedText:  "Welcome to Test Site!",
			shouldContain: []string{"Test Site"},
		},
		{
			name: "page-specific header",
			cfg: &config.Config{
				Site: config.Site{
					Title: "Test Site",
				},
				Presentation: config.Presentation{
					Headers: config.Headers{
						Global: config.HeaderConfig{
							Enabled: true,
							Content: "Global Header",
						},
						PerPage: map[string]config.HeaderConfig{
							"notes": {
								Enabled: true,
								Content: "Notes Page Header",
							},
						},
					},
				},
			},
			page:          "notes",
			shouldContain: []string{"Global Header", "Notes Page Header"},
		},
		{
			name: "disabled global header",
			cfg: &config.Config{
				Presentation: config.Presentation{
					Headers: config.Headers{
						Global: config.HeaderConfig{
							Enabled: false,
							Content: "Should not appear",
						},
					},
				},
			},
			page:           "",
			shouldNotExist: true,
		},
		{
			name: "template variables replacement",
			cfg: &config.Config{
				Site: config.Site{
					Title:       "My Site",
					Description: "Site Description",
					Operator:    "Alice",
				},
				Presentation: config.Presentation{
					Headers: config.Headers{
						Global: config.HeaderConfig{
							Enabled: true,
							Content: "{{site.title}} by {{site.operator}} - {{date}}",
						},
					},
				},
			},
			page:          "",
			shouldContain: []string{"My Site", "Alice"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loader := NewLoader(tt.cfg)
			header, err := loader.GetHeader(tt.page)

			if tt.shouldNotExist {
				if err == nil && header != "" {
					t.Errorf("Expected no header, got: %s", header)
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if tt.expectedText != "" && !strings.Contains(header, tt.expectedText) {
				t.Errorf("Expected header to contain '%s', got: %s", tt.expectedText, header)
			}

			for _, text := range tt.shouldContain {
				if !strings.Contains(header, text) {
					t.Errorf("Expected header to contain '%s', got: %s", text, header)
				}
			}
		})
	}
}

func TestGetFooter(t *testing.T) {
	tests := []struct {
		name          string
		cfg           *config.Config
		page          string
		shouldContain []string
	}{
		{
			name: "global footer",
			cfg: &config.Config{
				Site: config.Site{
					Title: "Test Site",
				},
				Presentation: config.Presentation{
					Footers: config.Footers{
						Global: config.FooterConfig{
							Enabled: true,
							Content: "© {{year}} {{site.title}}",
						},
					},
				},
			},
			page:          "",
			shouldContain: []string{"Test Site"},
		},
		{
			name: "page-specific footer",
			cfg: &config.Config{
				Presentation: config.Presentation{
					Footers: config.Footers{
						PerPage: map[string]config.FooterConfig{
							"articles": {
								Enabled: true,
								Content: "End of Articles",
							},
						},
					},
				},
			},
			page:          "articles",
			shouldContain: []string{"End of Articles"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loader := NewLoader(tt.cfg)
			footer, err := loader.GetFooter(tt.page)

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			for _, text := range tt.shouldContain {
				if !strings.Contains(footer, text) {
					t.Errorf("Expected footer to contain '%s', got: %s", text, footer)
				}
			}
		})
	}
}

func TestLoadContentFromFile(t *testing.T) {
	// Create temporary directory for test files
	tmpDir, err := os.MkdirTemp("", "nophr-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test file
	testContent := "Header content from file with {{site.title}}"
	testFile := filepath.Join(tmpDir, "header.txt")
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	cfg := &config.Config{
		Site: config.Site{
			Title: "File Test",
		},
		Presentation: config.Presentation{
			Headers: config.Headers{
				Global: config.HeaderConfig{
					Enabled:  true,
					FilePath: testFile,
				},
			},
		},
	}

	loader := NewLoader(cfg)
	header, err := loader.GetHeader("")

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !strings.Contains(header, "File Test") {
		t.Errorf("Expected header to contain 'File Test', got: %s", header)
	}

	if !strings.Contains(header, "Header content from file") {
		t.Errorf("Expected header to contain file content, got: %s", header)
	}
}

func TestCaching(t *testing.T) {
	cfg := &config.Config{
		Site: config.Site{
			Title: "Cache Test",
		},
		Presentation: config.Presentation{
			Headers: config.Headers{
				Global: config.HeaderConfig{
					Enabled: true,
					Content: "Test Header {{datetime}}",
				},
			},
		},
	}

	loader := NewLoader(cfg)

	// Load first time
	header1, err := loader.GetHeader("")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Load second time (should be cached)
	header2, err := loader.GetHeader("")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Both should be identical (datetime should be the same from cache)
	if header1 != header2 {
		t.Errorf("Expected cached content to be identical:\nFirst:  %s\nSecond: %s", header1, header2)
	}
}

func TestClearCache(t *testing.T) {
	cfg := &config.Config{
		Site: config.Site{
			Title: "Cache Clear Test",
		},
		Presentation: config.Presentation{
			Headers: config.Headers{
				Global: config.HeaderConfig{
					Enabled: true,
					Content: "Test Content",
				},
			},
		},
	}

	loader := NewLoader(cfg)

	// Load to populate cache
	_, err := loader.GetHeader("")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Verify cache is populated
	if len(loader.cache) == 0 {
		t.Error("Expected cache to be populated")
	}

	// Clear cache
	loader.ClearCache()

	// Verify cache is empty
	if len(loader.cache) != 0 {
		t.Errorf("Expected cache to be empty, has %d items", len(loader.cache))
	}
}

func TestTemplateVariables(t *testing.T) {
	now := time.Now()
	cfg := &config.Config{
		Site: config.Site{
			Title:       "Template Test",
			Description: "Test Description",
			Operator:    "Test Operator",
		},
		Presentation: config.Presentation{
			Headers: config.Headers{
				Global: config.HeaderConfig{
					Enabled: true,
					Content: "Title: {{site.title}}\nDescription: {{site.description}}\nOperator: {{site.operator}}\nDate: {{date}}\nYear: {{year}}",
				},
			},
		},
	}

	loader := NewLoader(cfg)
	header, err := loader.GetHeader("")

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expectedTexts := []string{
		"Title: Template Test",
		"Description: Test Description",
		"Operator: Test Operator",
		"Date: " + now.Format("2006-01-02"),
		"Year: " + now.Format("2006"),
	}

	for _, expected := range expectedTexts {
		if !strings.Contains(header, expected) {
			t.Errorf("Expected header to contain '%s', got: %s", expected, header)
		}
	}
}

func TestFilePriorityOverContent(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "nophr-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	fileContent := "Content from file"
	testFile := filepath.Join(tmpDir, "priority.txt")
	if err := os.WriteFile(testFile, []byte(fileContent), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	cfg := &config.Config{
		Presentation: config.Presentation{
			Headers: config.Headers{
				Global: config.HeaderConfig{
					Enabled:  true,
					Content:  "Inline content (should be ignored)",
					FilePath: testFile,
				},
			},
		},
	}

	loader := NewLoader(cfg)
	header, err := loader.GetHeader("")

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !strings.Contains(header, "Content from file") {
		t.Errorf("Expected file content, got: %s", header)
	}

	if strings.Contains(header, "Inline content") {
		t.Errorf("File should take priority over inline content, got: %s", header)
	}
}
