package exporter

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/nbd-wtf/go-nostr"
	"github.com/sandwich/nophr/internal/config"
	"github.com/sandwich/nophr/internal/gemini"
	"github.com/sandwich/nophr/internal/storage"
)

// GeminiExporter writes static gemtext for owner content.
type GeminiExporter struct {
	enabled     bool
	outputDir   string
	host        string
	port        int
	maxItems    int
	ownerPubkey string

	renderer *gemini.Renderer
	storage  *storage.Storage
	mu       sync.Mutex
}

// NewGeminiExporter builds an exporter when enabled in config.
func NewGeminiExporter(cfg *config.Config, st *storage.Storage) (*GeminiExporter, error) {
	if cfg == nil || st == nil {
		return nil, fmt.Errorf("config and storage are required")
	}

	if !cfg.Export.Gemini.Enabled {
		return nil, nil
	}

	ownerHex, err := decodeNpub(cfg.Identity.Npub)
	if err != nil {
		return nil, fmt.Errorf("failed to decode identity.npub: %w", err)
	}

	outputDir := cfg.Export.Gemini.OutputDir
	if outputDir == "" {
		outputDir = "./export/gemini"
	}

	return &GeminiExporter{
		enabled:     true,
		outputDir:   outputDir,
		host:        cfg.Export.Gemini.Host,
		port:        cfg.Export.Gemini.Port,
		maxItems:    cfg.Export.Gemini.MaxItems,
		ownerPubkey: ownerHex,
		renderer:    gemini.NewRenderer(cfg, st),
		storage:     st,
	}, nil
}

// HandleEvent triggers an export when a new owner root note/article arrives.
func (g *GeminiExporter) HandleEvent(ctx context.Context, event *nostr.Event) {
	if g == nil || !g.enabled {
		return
	}

	if !g.isOwnerRootEvent(event) {
		return
	}

	if err := g.Export(ctx); err != nil {
		fmt.Printf("[EXPORT] ⚠ static gemini export failed: %v\n", err)
	}
}

// Export regenerates the static gemtext site.
func (g *GeminiExporter) Export(ctx context.Context) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if err := g.ensureDirectories(); err != nil {
		return err
	}

	notes, err := g.queryOwnerRoots(ctx, 1)
	if err != nil {
		return err
	}

	articles, err := g.queryOwnerRoots(ctx, 30023)
	if err != nil {
		return err
	}

	now := time.Now().UTC()

	if err := g.writeRootIndex(notes, articles, now); err != nil {
		return err
	}

	if err := g.writeSection("notes", notes); err != nil {
		return err
	}

	if err := g.writeSection("articles", articles); err != nil {
		return err
	}

	if err := g.writeEvents("notes", notes); err != nil {
		return err
	}
	if err := g.writeEvents("articles", articles); err != nil {
		return err
	}

	return nil
}

func (g *GeminiExporter) ensureDirectories() error {
	dirs := []string{
		g.outputDir,
		filepath.Join(g.outputDir, "notes"),
		filepath.Join(g.outputDir, "articles"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("failed to create export dir %s: %w", dir, err)
		}
	}

	return nil
}

func (g *GeminiExporter) writeRootIndex(notes, articles []*nostr.Event, generatedAt time.Time) error {
	var sb strings.Builder
	sb.WriteString("# nophr static export\n\n")

	if len(notes) > 0 {
		sb.WriteString(fmt.Sprintf("=> %s Notes\n", g.relativeLink("/notes/index.gmi")))
	}
	if len(articles) > 0 {
		sb.WriteString(fmt.Sprintf("=> %s Articles\n", g.relativeLink("/articles/index.gmi")))
	}

	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("Generated: %s\n", generatedAt.Format(time.RFC3339)))

	return writeFile(filepath.Join(g.outputDir, "index.gmi"), []byte(sb.String()))
}

func (g *GeminiExporter) writeSection(section string, events []*nostr.Event) error {
	var sb strings.Builder

	title := capitalize(section)
	sb.WriteString(fmt.Sprintf("# %s\n\n", title))

	if len(events) == 0 {
		sb.WriteString("No content yet.\n")
		return writeFile(filepath.Join(g.outputDir, section, "index.gmi"), []byte(sb.String()))
	}

	for _, event := range events {
		display := summarizeContent(event.Content)
		sb.WriteString(fmt.Sprintf("=> %s %s\n", g.relativeLink(fmt.Sprintf("/%s/%s.gmi", section, event.ID)), display))
	}

	return writeFile(filepath.Join(g.outputDir, section, "index.gmi"), []byte(sb.String()))
}

func (g *GeminiExporter) writeEvents(section string, events []*nostr.Event) error {
	for _, event := range events {
		homeURL := "/index.gmi"
		threadURL := fmt.Sprintf("/%s/index.gmi", section)
		content := g.renderer.RenderNote(event, nil, threadURL, homeURL)
		targetPath := filepath.Join(g.outputDir, section, fmt.Sprintf("%s.gmi", event.ID))
		if err := writeFile(targetPath, []byte(content)); err != nil {
			return err
		}
	}

	return nil
}

func (g *GeminiExporter) queryOwnerRoots(ctx context.Context, kind int) ([]*nostr.Event, error) {
	filter := nostr.Filter{
		Kinds:   []int{kind},
		Authors: []string{g.ownerPubkey},
		Limit:   g.maxItems * 2,
	}

	events, err := g.storage.QueryEvents(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to query events for export: %w", err)
	}

	var roots []*nostr.Event
	for _, event := range events {
		if !g.isRootEvent(event) {
			continue
		}
		roots = append(roots, event)
	}

	sort.SliceStable(roots, func(i, j int) bool {
		return roots[i].CreatedAt > roots[j].CreatedAt
	})

	if len(roots) > g.maxItems {
		roots = roots[:g.maxItems]
	}

	return roots, nil
}

func (g *GeminiExporter) isOwnerRootEvent(event *nostr.Event) bool {
	if event == nil {
		return false
	}

	if event.PubKey != g.ownerPubkey {
		return false
	}

	if event.Kind != 1 && event.Kind != 30023 {
		return false
	}

	return g.isRootEvent(event)
}

func (g *GeminiExporter) isRootEvent(event *nostr.Event) bool {
	if event.Kind == 1 {
		for _, tag := range event.Tags {
			if len(tag) >= 1 && tag[0] == "e" {
				return false
			}
		}
	}
	return event.Kind == 1 || event.Kind == 30023
}

func (g *GeminiExporter) relativeLink(path string) string {
	if g.port == 1965 {
		return fmt.Sprintf("gemini://%s%s", g.host, path)
	}
	return fmt.Sprintf("gemini://%s:%d%s", g.host, g.port, path)
}
