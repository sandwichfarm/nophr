package presentation

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/sandwichfarm/nophr/internal/config"
)

// Loader handles loading and caching of headers/footers
type Loader struct {
	config *config.Config
	cache  map[string]*cachedContent
	mu     sync.RWMutex
}

// cachedContent represents cached header/footer content
type cachedContent struct {
	content   string
	loadedAt  time.Time
	filePath  string
}

// NewLoader creates a new presentation loader
func NewLoader(cfg *config.Config) *Loader {
	return &Loader{
		config: cfg,
		cache:  make(map[string]*cachedContent),
	}
}

// GetHeader returns the header for a given page
// If page is empty, returns global header
// If page-specific header exists, returns that (with optional global prepended)
func (l *Loader) GetHeader(page string) (string, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var headers []string

	// Check for global header
	if l.config.Presentation.Headers.Global.Enabled {
		globalHeader, err := l.loadContent("header:global", l.config.Presentation.Headers.Global)
		if err != nil {
			return "", fmt.Errorf("failed to load global header: %w", err)
		}
		if globalHeader != "" {
			headers = append(headers, globalHeader)
		}
	}

	// Check for page-specific header
	if page != "" {
		if pageConfig, ok := l.config.Presentation.Headers.PerPage[page]; ok && pageConfig.Enabled {
			pageHeader, err := l.loadContent(fmt.Sprintf("header:%s", page), pageConfig)
			if err != nil {
				return "", fmt.Errorf("failed to load header for page %s: %w", page, err)
			}
			if pageHeader != "" {
				headers = append(headers, pageHeader)
			}
		}
	}

	return strings.Join(headers, "\n\n"), nil
}

// GetFooter returns the footer for a given page
// If page is empty, returns global footer
// If page-specific footer exists, returns that (with optional global appended)
func (l *Loader) GetFooter(page string) (string, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var footers []string

	// Check for page-specific footer
	if page != "" {
		if pageConfig, ok := l.config.Presentation.Footers.PerPage[page]; ok && pageConfig.Enabled {
			pageFooter, err := l.loadContent(fmt.Sprintf("footer:%s", page), convertToHeaderConfig(pageConfig))
			if err != nil {
				return "", fmt.Errorf("failed to load footer for page %s: %w", page, err)
			}
			if pageFooter != "" {
				footers = append(footers, pageFooter)
			}
		}
	}

	// Check for global footer
	if l.config.Presentation.Footers.Global.Enabled {
		globalFooter, err := l.loadContent("footer:global", convertToHeaderConfig(l.config.Presentation.Footers.Global))
		if err != nil {
			return "", fmt.Errorf("failed to load global footer: %w", err)
		}
		if globalFooter != "" {
			footers = append(footers, globalFooter)
		}
	}

	return strings.Join(footers, "\n\n"), nil
}

// loadContent loads content from either inline config or file
func (l *Loader) loadContent(cacheKey string, cfg config.HeaderConfig) (string, error) {
	// Check cache first
	if cached, ok := l.cache[cacheKey]; ok {
		// Cache is valid for 5 minutes
		if time.Since(cached.loadedAt) < 5*time.Minute {
			return cached.content, nil
		}
	}

	var content string

	// Priority: FilePath > Content
	if cfg.FilePath != "" {
		data, err := os.ReadFile(cfg.FilePath)
		if err != nil {
			return "", fmt.Errorf("failed to read file %s: %w", cfg.FilePath, err)
		}
		content = string(data)
	} else if cfg.Content != "" {
		content = cfg.Content
	}

	// Apply template variables
	content = l.applyTemplateVariables(content)

	// Cache the content
	l.cache[cacheKey] = &cachedContent{
		content:  content,
		loadedAt: time.Now(),
		filePath: cfg.FilePath,
	}

	return content, nil
}

// applyTemplateVariables replaces template variables in content
func (l *Loader) applyTemplateVariables(content string) string {
	replacements := map[string]string{
		"{{site.title}}":       l.config.Site.Title,
		"{{site.description}}": l.config.Site.Description,
		"{{site.operator}}":    l.config.Site.Operator,
		"{{date}}":             time.Now().Format("2006-01-02"),
		"{{datetime}}":         time.Now().Format("2006-01-02 15:04:05 MST"),
		"{{year}}":             time.Now().Format("2006"),
	}

	result := content
	for key, value := range replacements {
		result = strings.ReplaceAll(result, key, value)
	}

	return result
}

// ClearCache clears the content cache
func (l *Loader) ClearCache() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.cache = make(map[string]*cachedContent)
}

// convertToHeaderConfig converts FooterConfig to HeaderConfig for reuse
func convertToHeaderConfig(fc config.FooterConfig) config.HeaderConfig {
	return config.HeaderConfig{
		Enabled:  fc.Enabled,
		Content:  fc.Content,
		FilePath: fc.FilePath,
	}
}
