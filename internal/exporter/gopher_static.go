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
	"github.com/nbd-wtf/go-nostr/nip19"
	"github.com/sandwichfarm/nophr/internal/config"
	"github.com/sandwichfarm/nophr/internal/gopher"
	"github.com/sandwichfarm/nophr/internal/storage"
)

// GopherExporter writes static gopher holes for owner content.
type GopherExporter struct {
	enabled     bool
	outputDir   string
	host        string
	port        int
	maxItems    int
	ownerPubkey string

	renderer *gopher.Renderer
	storage  *storage.Storage
	mu       sync.Mutex
}

// NewGopherExporter builds an exporter when enabled in config.
func NewGopherExporter(cfg *config.Config, st *storage.Storage) (*GopherExporter, error) {
	if cfg == nil || st == nil {
		return nil, fmt.Errorf("config and storage are required")
	}

	if !cfg.Export.Gopher.Enabled {
		return nil, nil
	}

	ownerHex, err := decodeNpub(cfg.Identity.Npub)
	if err != nil {
		return nil, fmt.Errorf("failed to decode identity.npub: %w", err)
	}

	outputDir := cfg.Export.Gopher.OutputDir
	if outputDir == "" {
		outputDir = "./export/gopher"
	}

	return &GopherExporter{
		enabled:     true,
		outputDir:   outputDir,
		host:        cfg.Export.Gopher.Host,
		port:        cfg.Export.Gopher.Port,
		maxItems:    cfg.Export.Gopher.MaxItems,
		ownerPubkey: ownerHex,
		renderer:    gopher.NewRenderer(cfg, st),
		storage:     st,
	}, nil
}

// HandleEvent triggers an export when a new owner root note/article arrives.
func (g *GopherExporter) HandleEvent(ctx context.Context, event *nostr.Event) {
	if g == nil || !g.enabled {
		return
	}

	if !g.isOwnerRootEvent(event) {
		return
	}

	if err := g.Export(ctx); err != nil {
		fmt.Printf("[EXPORT] ⚠ static gopher export failed: %v\n", err)
	}
}

// Export regenerates the static gopher hole.
func (g *GopherExporter) Export(ctx context.Context) error {
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

	if err := g.writeRootGophermap(notes, articles, now); err != nil {
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

func (g *GopherExporter) ensureDirectories() error {
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

func (g *GopherExporter) writeRootGophermap(notes, articles []*nostr.Event, generatedAt time.Time) error {
	gmap := gopher.NewGophermap(g.host, g.port)
	gmap.AddWelcome("nophr static export", "")

	if len(notes) > 0 {
		gmap.AddDirectory("Notes", "/notes")
	}
	if len(articles) > 0 {
		gmap.AddDirectory("Articles", "/articles")
	}

	gmap.AddSpacer()
	gmap.AddInfo(fmt.Sprintf("Generated: %s", generatedAt.Format(time.RFC3339)))

	return writeFile(filepath.Join(g.outputDir, "gophermap"), gmap.Bytes())
}

func (g *GopherExporter) writeSection(section string, events []*nostr.Event) error {
	sectionPath := filepath.Join(g.outputDir, section, "gophermap")
	gmap := gopher.NewGophermap(g.host, g.port)

	title := capitalize(section)
	gmap.AddWelcome(title, "")

	for _, event := range events {
		display := summarizeContent(event.Content)
		selector := fmt.Sprintf("/%s/%s.txt", section, event.ID)
		gmap.AddTextFile(display, selector)
	}

	return writeFile(sectionPath, gmap.Bytes())
}

func (g *GopherExporter) writeEvents(section string, events []*nostr.Event) error {
	for _, event := range events {
		content := g.renderer.RenderNote(event, nil)
		targetPath := filepath.Join(g.outputDir, section, fmt.Sprintf("%s.txt", event.ID))
		if err := writeFile(targetPath, []byte(content)); err != nil {
			return err
		}
	}

	return nil
}

func (g *GopherExporter) queryOwnerRoots(ctx context.Context, kind int) ([]*nostr.Event, error) {
	filter := nostr.Filter{
		Kinds:   []int{kind},
		Authors: []string{g.ownerPubkey},
		Limit:   g.maxItems * 2, // grab extras; we'll trim after filtering replies
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

func (g *GopherExporter) isOwnerRootEvent(event *nostr.Event) bool {
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

func (g *GopherExporter) isRootEvent(event *nostr.Event) bool {
	if event.Kind == 1 {
		for _, tag := range event.Tags {
			if len(tag) >= 1 && tag[0] == "e" {
				return false
			}
		}
	}
	return event.Kind == 1 || event.Kind == 30023
}

func summarizeContent(content string) string {
	if content == "" {
		return "Untitled"
	}

	firstLine := strings.Split(content, "\n")[0]
	if len(firstLine) > 70 {
		return firstLine[:67] + "..."
	}
	return firstLine
}

func writeFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("failed to create directory for %s: %w", path, err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("failed to write %s: %w", path, err)
	}
	return nil
}

func decodeNpub(npub string) (string, error) {
	if npub == "" {
		return "", fmt.Errorf("npub is empty")
	}

	_, val, err := nip19.Decode(npub)
	if err != nil {
		return "", err
	}

	hex, ok := val.(string)
	if !ok {
		return "", fmt.Errorf("unexpected type for decoded npub")
	}

	return hex, nil
}

func capitalize(input string) string {
	if input == "" {
		return input
	}
	if len(input) == 1 {
		return strings.ToUpper(input)
	}
	return strings.ToUpper(input[:1]) + input[1:]
}
