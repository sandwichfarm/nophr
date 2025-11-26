package gemini

import (
	"context"
	"fmt"
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

// Renderer renders Nostr events as Gemtext
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

// RenderHome renders the home page
func (r *Renderer) RenderHome() string {
	var sb strings.Builder

	sb.WriteString("# nophr - Nostr Gateway\n\n")
	sb.WriteString("Browse Nostr content via Gemini protocol\n\n")
	sb.WriteString("## Navigation\n\n")
	sb.WriteString("=> /notes Notes\n")
	sb.WriteString("=> /articles Articles\n")
	sb.WriteString("=> /replies Replies\n")
	sb.WriteString("=> /mentions Mentions\n")
	sb.WriteString("=> /search Search\n")
	sb.WriteString("=> /diagnostics Diagnostics\n")
	sb.WriteString("\n")
	sb.WriteString("Powered by nophr\n")

	return r.applyHeadersFooters(sb.String(), "home")
}

// RenderNote renders a note event as gemtext
func (r *Renderer) RenderNote(event *nostr.Event, agg *aggregates.EventAggregates, threadURL, homeURL string) string {
	var sb strings.Builder

	// Header
	sb.WriteString(fmt.Sprintf("# Note by %s\n", truncatePubkey(event.PubKey)))
	sb.WriteString(fmt.Sprintf("Posted: %s\n\n", formatTimestamp(event.CreatedAt)))

	// Content (resolve NIP-19 entities, then render markdown as gemtext)
	content := event.Content
	ctx := context.Background()
	content, foundEntities := r.resolver.ReplaceEntitiesWithMetadata(ctx, content, entities.PlainTextFormatter)
	foundEntities = entities.DedupeEntities(foundEntities)

	rendered, _ := r.parser.RenderGemini([]byte(content), r.geminiRenderOptions())
	rendered = clampWidth(rendered, r.config.Rendering.Gemini.MaxLineLength)
	sb.WriteString(rendered)
	sb.WriteString("\n")

	if len(foundEntities) > 0 {
		sb.WriteString("## Portal Links\n\n")
		sb.WriteString(r.renderPortalLinks(foundEntities))
		sb.WriteString("\n")
	}

	// Aggregates
	if agg != nil && agg.HasInteractions() {
		sb.WriteString("## Interactions\n\n")
		sb.WriteString(r.renderAggregates(agg))
		sb.WriteString("\n")
	}

	// Navigation
	sb.WriteString("## Actions\n\n")
	sb.WriteString(fmt.Sprintf("=> %s View Thread\n", threadURL))
	sb.WriteString(fmt.Sprintf("=> %s Back to Home\n", homeURL))

	return sb.String()
}

// RenderNoteWithThread renders a note and optionally appends a thread view
func (r *Renderer) RenderNoteWithThread(event *nostr.Event, agg *aggregates.EventAggregates, thread *aggregates.ThreadView, threadURL, homeURL string) string {
	base := r.RenderNote(event, agg, threadURL, homeURL)

	if thread == nil || !r.config.Display.Detail.ShowThread {
		return base
	}

	var sb strings.Builder
	sb.WriteString(base)
	sb.WriteString("\n## Thread\n\n")
	sb.WriteString(r.RenderThread(thread, homeURL))

	return sb.String()
}

// RenderProfile renders a profile event
func (r *Renderer) RenderProfile(profileEvent *nostr.Event, homeURL string) string {
	var sb strings.Builder

	// Parse profile metadata
	profile := nostrclient.ParseProfile(profileEvent)
	if profile == nil {
		// Fallback for invalid profile
		sb.WriteString(fmt.Sprintf("# Profile: %s\n\n", truncatePubkey(profileEvent.PubKey)))
		sb.WriteString("Invalid profile data\n\n")
		sb.WriteString(fmt.Sprintf("=> %s Back to Home\n", homeURL))
		return sb.String()
	}

	// Header with display name
	displayName := profile.GetDisplayName()
	if displayName == "" {
		displayName = truncatePubkey(profileEvent.PubKey)
	}

	sb.WriteString(fmt.Sprintf("# %s\n\n", displayName))

	// Pubkey
	sb.WriteString(fmt.Sprintf("Pubkey: %s\n\n", profileEvent.PubKey))

	// Name fields
	if profile.Name != "" {
		sb.WriteString(fmt.Sprintf("**Name:** %s\n", profile.Name))
	}
	if profile.DisplayName != "" && profile.DisplayName != profile.Name {
		sb.WriteString(fmt.Sprintf("**Display Name:** %s\n", profile.DisplayName))
	}
	if profile.Name != "" || profile.DisplayName != "" {
		sb.WriteString("\n")
	}

	// About/Bio
	if profile.About != "" {
		sb.WriteString("## About\n\n")
		sb.WriteString(profile.About)
		sb.WriteString("\n\n")
	}

	// Contact information section
	hasContact := profile.Website != "" || profile.NIP05 != "" || profile.GetLightningAddress() != ""
	if hasContact {
		sb.WriteString("## Contact & Links\n\n")
		if profile.Website != "" {
			sb.WriteString(fmt.Sprintf("=> %s Website\n", profile.Website))
		}
		if profile.NIP05 != "" {
			sb.WriteString(fmt.Sprintf("**NIP-05:** %s\n", profile.NIP05))
		}
		lightningAddr := profile.GetLightningAddress()
		if lightningAddr != "" {
			sb.WriteString(fmt.Sprintf("**Lightning:** %s\n", lightningAddr))
		}
		sb.WriteString("\n")
	}

	// Media section
	hasMedia := profile.Picture != "" || profile.Banner != ""
	if hasMedia {
		sb.WriteString("## Media\n\n")
		if profile.Picture != "" {
			sb.WriteString(fmt.Sprintf("=> %s Profile Picture\n", profile.Picture))
		}
		if profile.Banner != "" {
			sb.WriteString(fmt.Sprintf("=> %s Banner Image\n", profile.Banner))
		}
		sb.WriteString("\n")
	}

	// Navigation
	sb.WriteString(fmt.Sprintf("=> %s Back to Home\n", homeURL))

	return sb.String()
}

// RenderThread renders a thread with replies
func (r *Renderer) RenderThread(thread *aggregates.ThreadView, homeURL string) string {
	if thread == nil || thread.Root == nil {
		return "Thread not found\n"
	}

	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("=> /note/%s Back to note\n\n", thread.FocusID))

	maxDepth := r.config.Display.Limits.MaxThreadDepth
	if maxDepth <= 0 {
		maxDepth = 10
	}

	r.renderThreadNode(&sb, thread.Root, 0, thread.FocusID, maxDepth)

	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("=> %s Back to Home\n", homeURL))

	return sb.String()
}

// RenderNoteList renders a list of notes with summaries
func (r *Renderer) RenderNoteList(notes []*aggregates.EnrichedEvent, title, homeURL string) string {
	var sb strings.Builder

	// Determine page name from title for headers/footers
	// Map common titles to page names
	pageName := "notes" // default
	titleLower := strings.ToLower(title)
	if strings.Contains(titleLower, "article") {
		pageName = "articles"
	} else if strings.Contains(titleLower, "repl") {
		pageName = "replies"
	} else if strings.Contains(titleLower, "mention") {
		pageName = "mentions"
	}

	sb.WriteString(fmt.Sprintf("# %s\n\n", title))

	if len(notes) == 0 {
		sb.WriteString("No notes yet.\n\n")
		sb.WriteString(fmt.Sprintf("=> %s Back to Home\n", homeURL))
		return r.applyHeadersFooters(sb.String(), pageName)
	}

	for i, note := range notes {
		entryTitle := r.titleForEvent(note.Event)
		ts := formatTimestamp(note.Event.CreatedAt)
		linkText := fmt.Sprintf("%d. %s — %s", i+1, entryTitle, ts)
		sb.WriteString(fmt.Sprintf("=> /note/%s %s\n", note.Event.ID, linkText))

		if note.Aggregates != nil && note.Aggregates.HasInteractions() {
			if agg := strings.TrimSpace(r.renderAggregates(note.Aggregates)); agg != "" {
				sb.WriteString(fmt.Sprintf("   %s\n", agg))
			}
		}
	}

	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("=> %s Back to Home\n", homeURL))

	return r.applyHeadersFooters(sb.String(), pageName)
}

// renderAggregates renders interaction stats (for feed view)
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

// GetSummary creates a summary of content for display
func (r *Renderer) GetSummary(content string, maxLen int) string {
	// Remove newlines
	summary := strings.ReplaceAll(content, "\n", " ")
	summary = strings.ReplaceAll(summary, "\r", "")

	// Trim whitespace
	summary = strings.TrimSpace(summary)

	// Truncate if needed
	if len(summary) > maxLen {
		return summary[:maxLen] + "..."
	}

	return summary
}

func (r *Renderer) geminiRenderOptions() *markdown.RenderOptions {
	opts := markdown.DefaultGeminiOptions()
	if r.config.Rendering.Gemini.MaxLineLength > 0 {
		opts.Width = r.config.Rendering.Gemini.MaxLineLength
	}
	return opts
}

func (r *Renderer) renderPortalLinks(resolved []*entities.Entity) string {
	resolved = entities.DedupeEntities(resolved)
	if len(resolved) == 0 {
		return ""
	}

	portals := []string{"https://njump.me", "https://nostr.at", "https://nostr.eu"}
	var lines []string

	for _, entity := range resolved {
		nip19 := strings.TrimPrefix(entity.OriginalText, "nostr:")
		lines = append(lines, fmt.Sprintf("* %s (%s)", entity.DisplayName, entity.Type))
		for _, portal := range portals {
			lines = append(lines, fmt.Sprintf("=> %s/%s %s", portal, nip19, entity.DisplayName))
		}
		lines = append(lines, "")
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

		if preserveGeminiLine(line) {
			out = append(out, line)
			continue
		}

		wrapped := wrapLineToWidth(line, width)
		out = append(out, wrapped...)
	}

	return strings.Join(out, "\n")
}

func preserveGeminiLine(line string) bool {
	trimmed := strings.TrimLeft(line, " ")
	if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "=>") {
		return true
	}
	if strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "* ") {
		return true
	}
	return false
}

func wrapLineToWidth(text string, width int) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{""}
	}

	var lines []string
	var current strings.Builder

	for _, word := range words {
		if current.Len() == 0 {
			current.WriteString(word)
			continue
		}

		if current.Len()+1+len(word) <= width {
			current.WriteString(" ")
			current.WriteString(word)
			continue
		}

		lines = append(lines, current.String())
		current.Reset()
		current.WriteString(word)
	}

	if current.Len() > 0 {
		lines = append(lines, current.String())
	}

	return lines
}

func (r *Renderer) titleForEvent(event *nostr.Event) string {
	// Prefer explicit title tag for long-form
	if event.Kind == 30023 {
		if title := titleFromTags(event); title != "" {
			return title
		}
	}

	content := strings.TrimSpace(event.Content)
	if content == "" {
		if len(event.ID) > 8 {
			return fmt.Sprintf("Event %s...", event.ID[:8])
		}
		return fmt.Sprintf("Event %s", event.ID)
	}

	firstLine := strings.Split(content, "\n")[0]
	if len(firstLine) > 120 {
		return firstLine[:117] + "..."
	}
	return firstLine
}

func titleFromTags(event *nostr.Event) string {
	for _, tag := range event.Tags {
		if len(tag) >= 2 && tag[0] == "title" && tag[1] != "" {
			return tag[1]
		}
	}
	return ""
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
	line := fmt.Sprintf("%s* %s (%s)", prefix, summary, formatTimestamp(node.Event.CreatedAt))
	if len(markers) > 0 {
		line = fmt.Sprintf("%s [%s]", line, strings.Join(markers, ", "))
	}
	sb.WriteString(line)
	sb.WriteString("\n")
	if portals := r.portalLinksForEvent(node.Event); len(portals) > 0 {
		for _, p := range portals {
			sb.WriteString(fmt.Sprintf("%s=> %s Open\n", prefix, p))
		}
	} else {
		sb.WriteString(fmt.Sprintf("%s=> /note/%s Open note\n", prefix, node.Event.ID))
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

func (r *Renderer) portalLinksForEvent(event *nostr.Event) []string {
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
