package gopher

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip19"
	"github.com/sandwich/nophr/internal/aggregates"
	"github.com/sandwich/nophr/internal/config"
	"github.com/sandwich/nophr/internal/entities"
	"github.com/sandwich/nophr/internal/markdown"
	nostrclient "github.com/sandwich/nophr/internal/nostr"
	"github.com/sandwich/nophr/internal/presentation"
	"github.com/sandwich/nophr/internal/storage"
)

// Renderer renders Nostr events as Gopher text
type Renderer struct {
	parser   *markdown.Parser
	config   *config.Config
	loader   *presentation.Loader
	resolver *entities.Resolver
	storage  *storage.Storage
}

// NewRenderer creates a new event renderer
func NewRenderer(cfg *config.Config, st *storage.Storage) *Renderer {
	return &Renderer{
		parser:   markdown.NewParser(),
		config:   cfg,
		loader:   presentation.NewLoader(cfg),
		resolver: entities.NewResolver(st),
		storage:  st,
	}
}

// RenderNote renders a note event as plain text
func (r *Renderer) RenderNote(event *nostr.Event, agg *aggregates.EventAggregates) string {
	var sb strings.Builder

	// Header
	sb.WriteString(fmt.Sprintf("Note by %s\n", truncatePubkey(event.PubKey)))
	sb.WriteString(fmt.Sprintf("Posted: %s\n", formatTimestamp(event.CreatedAt)))
	sb.WriteString(strings.Repeat("=", 70))
	sb.WriteString("\n\n")

	// Content (resolve NIP-19 entities, then render markdown)
	content := event.Content

	// Resolve NIP-19 entities
	ctx := context.Background()
	content, foundEntities := r.resolver.ReplaceEntitiesWithMetadata(ctx, content, entities.GopherFormatter)
	foundEntities = entities.DedupeEntities(foundEntities)

	// Apply max content length if configured
	if r.config.Display.Limits.MaxContentLength > 0 && len(content) > r.config.Display.Limits.MaxContentLength {
		content = content[:r.config.Display.Limits.MaxContentLength] + r.config.Display.Limits.TruncateIndicator
	}

	rendered, _ := r.parser.RenderGopher([]byte(content), r.gopherRenderOptions())
	rendered = clampWidth(rendered, r.config.Rendering.Gopher.MaxLineLength)
	sb.WriteString(rendered)

	if len(foundEntities) > 0 {
		sb.WriteString("\n")
		sb.WriteString(r.applyConfigSeparator("section"))
		sb.WriteString("\n")
		sb.WriteString(r.renderPortalLinks(foundEntities))
		sb.WriteString("\n")
	}

	// Aggregates footer - only show if configured for detail view
	if r.config.Display.Detail.ShowInteractions && agg != nil && agg.HasInteractions() {
		sb.WriteString("\n")
		sb.WriteString(r.applyConfigSeparator("section"))
		sb.WriteString("\n")
		sb.WriteString(r.renderAggregatesForDetail(agg))
	}

	return sb.String()
}

// RenderProfile renders a profile event
func (r *Renderer) RenderProfile(profileEvent *nostr.Event) string {
	var sb strings.Builder

	// Parse profile metadata
	profile := nostrclient.ParseProfile(profileEvent)
	if profile == nil {
		// Fallback for invalid profile
		sb.WriteString(fmt.Sprintf("Profile: %s\n", truncatePubkey(profileEvent.PubKey)))
		sb.WriteString(strings.Repeat("=", 70))
		sb.WriteString("\n\nInvalid profile data\n")
		return sb.String()
	}

	// Header with display name
	displayName := profile.GetDisplayName()
	if displayName == "" {
		displayName = truncatePubkey(profileEvent.PubKey)
	}

	sb.WriteString(fmt.Sprintf("Profile: %s\n", displayName))
	sb.WriteString(strings.Repeat("=", 70))
	sb.WriteString("\n\n")

	// Pubkey
	sb.WriteString(fmt.Sprintf("Pubkey: %s\n", profileEvent.PubKey))
	sb.WriteString("\n")

	// Name fields
	if profile.Name != "" {
		sb.WriteString(fmt.Sprintf("Name: %s\n", profile.Name))
	}
	if profile.DisplayName != "" && profile.DisplayName != profile.Name {
		sb.WriteString(fmt.Sprintf("Display Name: %s\n", profile.DisplayName))
	}

	// About/Bio
	if profile.About != "" {
		sb.WriteString("\nAbout:\n")
		sb.WriteString(profile.About)
		sb.WriteString("\n")
	}

	// Contact information
	if profile.Website != "" {
		sb.WriteString(fmt.Sprintf("\nWebsite: %s\n", profile.Website))
	}
	if profile.NIP05 != "" {
		sb.WriteString(fmt.Sprintf("NIP-05: %s\n", profile.NIP05))
	}

	// Lightning info
	lightningAddr := profile.GetLightningAddress()
	if lightningAddr != "" {
		sb.WriteString(fmt.Sprintf("Lightning: %s\n", lightningAddr))
	}

	// Media
	if profile.Picture != "" {
		sb.WriteString(fmt.Sprintf("\nPicture: %s\n", profile.Picture))
	}
	if profile.Banner != "" {
		sb.WriteString(fmt.Sprintf("Banner: %s\n", profile.Banner))
	}

	return sb.String()
}

// RenderNoteWithThread renders a note and optionally appends a threaded view
func (r *Renderer) RenderNoteWithThread(event *nostr.Event, agg *aggregates.EventAggregates, thread *aggregates.ThreadView) string {
	base := r.RenderNote(event, agg)

	if thread == nil || !r.config.Display.Detail.ShowThread {
		return base
	}

	var sb strings.Builder
	sb.WriteString(base)
	sb.WriteString("\n")
	sb.WriteString(r.applyConfigSeparator("section"))
	sb.WriteString("\n")
	sb.WriteString(r.RenderThread(thread))

	return sb.String()
}

// RenderThread renders a thread with indentation
func (r *Renderer) RenderThread(thread *aggregates.ThreadView) string {
	if thread == nil || thread.Root == nil {
		return "Thread not found"
	}

	var sb strings.Builder

	sb.WriteString("Thread\n")
	sb.WriteString(strings.Repeat("=", 70))
	sb.WriteString("\n\n")

	// Navigation
	sb.WriteString(fmt.Sprintf("Back to note: /note/%s\n", thread.FocusID))
	sb.WriteString("Home: /\n\n")

	maxDepth := r.config.Display.Limits.MaxThreadDepth
	if maxDepth <= 0 {
		maxDepth = 10
	}

	r.renderThreadNode(&sb, thread.Root, 0, thread.FocusID, maxDepth)

	return sb.String()
}

// renderAggregates renders interaction stats (for feed view - respects feed config)
func (r *Renderer) renderAggregates(agg *aggregates.EventAggregates) string {
	if !r.config.Display.Feed.ShowInteractions {
		return ""
	}
	return r.buildAggregatesString(agg, r.config.Display.Feed.ShowReplies, r.config.Display.Feed.ShowReactions, r.config.Display.Feed.ShowZaps)
}

// renderAggregatesForDetail renders interaction stats for detail view
func (r *Renderer) renderAggregatesForDetail(agg *aggregates.EventAggregates) string {
	return r.buildAggregatesString(agg, r.config.Display.Detail.ShowReplies, r.config.Display.Detail.ShowReactions, r.config.Display.Detail.ShowZaps)
}

// buildAggregatesString builds the aggregates string based on what should be shown
func (r *Renderer) buildAggregatesString(agg *aggregates.EventAggregates, showReplies, showReactions, showZaps bool) string {
	var parts []string

	if showReplies && agg.ReplyCount > 0 {
		parts = append(parts, fmt.Sprintf("%d replies", agg.ReplyCount))
	}

	if showReactions && agg.ReactionTotal > 0 {
		// Show total reactions with breakdown
		if len(agg.ReactionCounts) > 0 {
			var reactionParts []string
			for emoji, count := range agg.ReactionCounts {
				reactionParts = append(reactionParts, fmt.Sprintf("%s %d", emoji, count))
			}
			parts = append(parts, fmt.Sprintf("%d reactions (%s)", agg.ReactionTotal, strings.Join(reactionParts, ", ")))
		} else {
			parts = append(parts, fmt.Sprintf("%d reactions", agg.ReactionTotal))
		}
	}

	if showZaps && agg.ZapSatsTotal > 0 {
		parts = append(parts, fmt.Sprintf("%s zapped", aggregates.FormatSats(agg.ZapSatsTotal)))
	}

	if len(parts) == 0 {
		return ""
	}

	return "Interactions: " + strings.Join(parts, ", ") + "\n"
}

// applyConfigSeparator applies the configured separator for the given type
func (r *Renderer) applyConfigSeparator(separatorType string) string {
	var sep string
	switch separatorType {
	case "item":
		sep = r.config.Presentation.Separators.Item.Gopher
	case "section":
		sep = r.config.Presentation.Separators.Section.Gopher
	default:
		sep = "---"
	}

	// If no custom separator, use default visual separator
	if sep == "" {
		if separatorType == "section" {
			return strings.Repeat("-", 70)
		}
		return ""
	}

	return sep
}

// applyHeadersFooters wraps content with configured headers and footers
func (r *Renderer) applyHeadersFooters(content, page string) string {
	var sb strings.Builder

	// Add header if configured
	if header, err := r.loader.GetHeader(page); err == nil && header != "" {
		sb.WriteString(header)
		sb.WriteString("\n\n")
	}

	// Add main content
	sb.WriteString(content)

	// Add footer if configured
	if footer, err := r.loader.GetFooter(page); err == nil && footer != "" {
		sb.WriteString("\n\n")
		sb.WriteString(footer)
	}

	return sb.String()
}

// truncatePubkey truncates a pubkey for display
func truncatePubkey(pubkey string) string {
	if len(pubkey) <= 16 {
		return pubkey
	}
	return pubkey[:8] + "..." + pubkey[len(pubkey)-8:]
}

// formatTimestamp formats a Nostr timestamp
func formatTimestamp(ts nostr.Timestamp) string {
	t := time.Unix(int64(ts), 0)
	now := time.Now()

	diff := now.Sub(t)

	if diff < time.Minute {
		return "just now"
	} else if diff < time.Hour {
		mins := int(diff.Minutes())
		if mins == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", mins)
	} else if diff < 24*time.Hour {
		hours := int(diff.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	} else if diff < 7*24*time.Hour {
		days := int(diff.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	} else if diff < 30*24*time.Hour {
		weeks := int(diff.Hours() / (24 * 7))
		if weeks == 1 {
			return "1 week ago"
		}
		return fmt.Sprintf("%d weeks ago", weeks)
	} else if diff < 365*24*time.Hour {
		weeks := int(diff.Hours() / (24 * 7))
		if weeks == 1 {
			return "1 week ago"
		}
		return fmt.Sprintf("%d weeks ago", weeks)
	}

	return t.Format("2006-01-02 15:04")
}

// indentText indents each line of text
func indentText(text, indent string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if line != "" {
			lines[i] = indent + line
		}
	}
	return strings.Join(lines, "\n")
}

// RenderNoteList renders a list of notes with summaries
func (r *Renderer) RenderNoteList(notes []*aggregates.EnrichedEvent, title string) []string {
	lines := make([]string, 0)

	lines = append(lines, title)
	lines = append(lines, strings.Repeat("=", len(title)))
	lines = append(lines, "")

	if len(notes) == 0 {
		lines = append(lines, "No notes yet")
		return lines
	}

	summaryLength := r.config.Display.Limits.SummaryLength
	if summaryLength <= 0 {
		summaryLength = 70 // Default fallback
	}

	for i, note := range notes {
		// Extract first line of content as summary
		content := note.Event.Content
		if len(content) > summaryLength {
			content = content[:summaryLength-len(r.config.Display.Limits.TruncateIndicator)] + r.config.Display.Limits.TruncateIndicator
		}
		firstLine := strings.Split(content, "\n")[0]

		lines = append(lines, fmt.Sprintf("%d. %s", i+1, firstLine))
		lines = append(lines, fmt.Sprintf("   by %s - %s",
			truncatePubkey(note.Event.PubKey),
			formatTimestamp(note.Event.CreatedAt)))

		// Only show aggregates if configured for feed view
		if r.config.Display.Feed.ShowInteractions && note.Aggregates != nil && note.Aggregates.HasInteractions() {
			aggStr := r.renderAggregates(note.Aggregates)
			if aggStr != "" {
				lines = append(lines, fmt.Sprintf("   %s", aggStr))
			}
		}

		// Apply item separator if configured
		itemSep := r.applyConfigSeparator("item")
		if itemSep != "" {
			lines = append(lines, itemSep)
		}

		lines = append(lines, "")
	}

	return lines
}

func (r *Renderer) gopherRenderOptions() *markdown.RenderOptions {
	opts := markdown.DefaultGopherOptions()
	if r.config.Rendering.Gopher.MaxLineLength > 0 {
		opts.Width = r.config.Rendering.Gopher.MaxLineLength
	}
	return opts
}

func (r *Renderer) renderPortalLinks(resolved []*entities.Entity) string {
	resolved = entities.DedupeEntities(resolved)
	if len(resolved) == 0 {
		return ""
	}

	portals := []string{"https://njump.me", "https://nostr.at", "https://nostr.eu"}
	lines := []string{"Portal links", strings.Repeat("-", 70)}

	for _, entity := range resolved {
		nip19 := strings.TrimPrefix(entity.OriginalText, "nostr:")
		lines = append(lines, fmt.Sprintf("- %s (%s)", entity.DisplayName, entity.Type))
		for _, portal := range portals {
			lines = append(lines, fmt.Sprintf("  %s/%s", portal, nip19))
		}
	}

	return strings.Join(lines, "\n")
}

func clampWidth(content string, width int) string {
	if width <= 0 {
		return content
	}

	var out []string
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			out = append(out, line)
			continue
		}

		if preserveLine(line) {
			out = append(out, line)
			continue
		}

		wrapped := wrapText(line, width)
		out = append(out, wrapped...)
	}

	return strings.Join(out, "\n")
}

func preserveLine(line string) bool {
	trimmed := strings.TrimLeft(line, " ")
	if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, ">") {
		return true
	}
	if strings.HasPrefix(trimmed, "* ") || strings.HasPrefix(trimmed, "=>") {
		return true
	}
	if strings.HasPrefix(line, "    ") {
		return true
	}
	return false
}

func (r *Renderer) renderThreadNode(sb *strings.Builder, node *aggregates.ThreadNode, depth int, focusID string, maxDepth int) {
	if node == nil {
		return
	}

	if depth >= maxDepth {
		prefix := strings.Repeat(r.threadIndent(), depth)
		sb.WriteString(fmt.Sprintf("%s… additional replies hidden\n", prefix))
		return
	}

	prefix := strings.Repeat(r.threadIndent(), depth)
	markers := make([]string, 0)
	if depth == 0 {
		markers = append(markers, "root")
	}
	if node.Event.ID == focusID {
		markers = append(markers, "you are here")
	}

	summary := r.threadSummary(node.Event.Content)
	line := fmt.Sprintf("%s- %s (%s)", prefix, summary, formatTimestamp(node.Event.CreatedAt))
	if len(markers) > 0 {
		line = fmt.Sprintf("%s [%s]", line, strings.Join(markers, ", "))
	}
	sb.WriteString(line)
	sb.WriteString("\n")

	if portals := r.portalLinks(node.Event); len(portals) > 0 {
		for _, p := range portals {
			sb.WriteString(fmt.Sprintf("%s  portal: %s\n", prefix, p))
		}
	} else {
		sb.WriteString(fmt.Sprintf("%s  selector: /note/%s\n", prefix, node.Event.ID))
	}

	for _, child := range node.Children {
		r.renderThreadNode(sb, child, depth+1, focusID, maxDepth)
	}
}

func (r *Renderer) threadIndent() string {
	indent := r.config.Rendering.Gopher.ThreadIndent
	if indent == "" {
		return "  "
	}
	return indent
}

func (r *Renderer) threadSummary(content string) string {
	limit := r.config.Display.Limits.SummaryLength
	if limit <= 0 {
		limit = 100
	}

	plain := strings.ReplaceAll(content, "\n", " ")
	plain = strings.TrimSpace(plain)
	if len(plain) <= limit {
		return plain
	}

	indicator := r.config.Display.Limits.TruncateIndicator
	if indicator == "" {
		indicator = "..."
	}

	return plain[:limit-len(indicator)] + indicator
}

func (r *Renderer) portalLinks(event *nostr.Event) []string {
	code, ok := r.nostrPointer(event)
	if !ok {
		return nil
	}

	portals := []string{"https://njump.me", "https://nostr.at", "https://nostr.eu"}
	links := make([]string, 0, len(portals))
	for _, base := range portals {
		links = append(links, fmt.Sprintf("%s/%s", base, code))
	}
	return links
}

func (r *Renderer) nostrPointer(event *nostr.Event) (string, bool) {
	relays, _ := r.storage.GetReadRelays(context.Background(), event.PubKey)

	if event.Kind == 30023 {
		if id := dTagValue(event); id != "" {
			if code, err := nip19.EncodeEntity(event.PubKey, event.Kind, id, relays); err == nil {
				return code, true
			}
		}
	}

	if code, err := nip19.EncodeEvent(event.ID, relays, event.PubKey); err == nil {
		return code, true
	}

	return "", false
}

func dTagValue(event *nostr.Event) string {
	for _, tag := range event.Tags {
		if len(tag) >= 2 && tag[0] == "d" && tag[1] != "" {
			return tag[1]
		}
	}
	return ""
}

// RenderThreadGophermap renders a thread as a gophermap with clickable portal links when available.
func (r *Renderer) RenderThreadGophermap(gmap *Gophermap, thread *aggregates.ThreadView) {
	if thread == nil || thread.Root == nil {
		gmap.AddError("Thread not found")
		return
	}

	gmap.AddInfo("Thread")
	gmap.AddInfo(strings.Repeat("=", 20))
	gmap.AddSpacer()
	gmap.AddDirectory("Back to note", fmt.Sprintf("/note/%s", thread.FocusID))
	gmap.AddDirectory("⌂ Home", "/")
	gmap.AddSpacer()

	maxDepth := r.config.Display.Limits.MaxThreadDepth
	if maxDepth <= 0 {
		maxDepth = 10
	}

	r.renderThreadNodeMap(gmap, thread.Root, 0, thread.FocusID, maxDepth)
}

func (r *Renderer) renderThreadNodeMap(gmap *Gophermap, node *aggregates.ThreadNode, depth int, focusID string, maxDepth int) {
	if node == nil {
		return
	}

	if depth >= maxDepth {
		prefix := strings.Repeat(r.threadIndent(), depth)
		gmap.AddInfo(fmt.Sprintf("%s… additional replies hidden", prefix))
		return
	}

	prefix := strings.Repeat(r.threadIndent(), depth)
	markers := make([]string, 0)
	if depth == 0 {
		markers = append(markers, "root")
	}
	if node.Event.ID == focusID {
		markers = append(markers, "you are here")
	}

	summary := r.threadSummary(node.Event.Content)
	line := fmt.Sprintf("%s- %s (%s)", prefix, summary, formatTimestamp(node.Event.CreatedAt))
	if len(markers) > 0 {
		line = fmt.Sprintf("%s [%s]", line, strings.Join(markers, ", "))
	}
	gmap.AddInfo(line)

	if portals := r.portalLinks(node.Event); len(portals) > 0 {
		for _, p := range portals {
			r.addPortalItem(gmap, fmt.Sprintf("%s  portal", prefix), p)
		}
	} else {
		gmap.AddTextFile(fmt.Sprintf("%s  open note", prefix), fmt.Sprintf("/note/%s", node.Event.ID))
	}

	for _, child := range node.Children {
		r.renderThreadNodeMap(gmap, child, depth+1, focusID, maxDepth)
	}
}

func (r *Renderer) addPortalItem(gmap *Gophermap, display, url string) {
	host, port := parseURLHostPort(url)
	gmap.Items = append(gmap.Items, Item{
		Type:     ItemTypeHTML,
		Display:  display,
		Selector: url,
		Host:     host,
		Port:     port,
	})
}

func parseURLHostPort(raw string) (string, int) {
	u, err := url.Parse(raw)
	if err != nil {
		return raw, 70
	}

	port := 0
	if u.Port() != "" {
		if p, err := strconv.Atoi(u.Port()); err == nil {
			port = p
		}
	}
	if port == 0 {
		switch u.Scheme {
		case "https":
			port = 443
		case "http":
			port = 80
		case "gemini":
			port = 1965
		default:
			port = 70
		}
	}

	host := u.Hostname()
	if host == "" {
		host = raw
	}

	return host, port
}
