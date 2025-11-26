package gopher

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/nbd-wtf/go-nostr"
	"github.com/sandwich/nophr/internal/aggregates"
	"github.com/sandwich/nophr/internal/sections"
)

const itemsPerPage = 9 // Gopher clients use single-digit hotkeys (1-9)

// Router handles selector routing for Gopher requests
type Router struct {
	server   *Server
	host     string
	port     int
	renderer *Renderer
}

// NewRouter creates a new router
func NewRouter(server *Server, host string, port int) *Router {
	return &Router{
		server:   server,
		host:     host,
		port:     port,
		renderer: NewRenderer(server.fullConfig, server.storage),
	}
}

// parsePageFromParts extracts page number from URL parts like ["page", "2"]
// Returns page number (1-indexed) and remaining parts
func parsePageFromParts(parts []string) (int, []string) {
	page := 1 // Default to page 1
	remaining := parts

	// Check for /page/N pattern
	for i := 0; i < len(parts)-1; i++ {
		if parts[i] == "page" {
			if pageNum, err := strconv.Atoi(parts[i+1]); err == nil && pageNum > 0 {
				page = pageNum
				// Remove "page" and number from parts
				remaining = append(parts[:i], parts[i+2:]...)
				break
			}
		}
	}

	return page, remaining
}

// addPaginationLinks adds Next/Previous/Home navigation to gophermap
func (r *Router) addPaginationLinks(gmap *Gophermap, basePath string, page, totalItems int) {
	totalPages := (totalItems + itemsPerPage - 1) / itemsPerPage

	gmap.AddSpacer()

	// Previous page link
	if page > 1 {
		prevPath := fmt.Sprintf("%s/page/%d", basePath, page-1)
		gmap.AddDirectory("← Previous Page", prevPath)
	}

	// Next page link
	if page < totalPages {
		nextPath := fmt.Sprintf("%s/page/%d", basePath, page+1)
		gmap.AddDirectory("→ Next Page", nextPath)
	}

	// Page info
	if totalPages > 1 {
		gmap.AddInfo(fmt.Sprintf("Page %d of %d", page, totalPages))
	}

	gmap.AddSpacer()
	gmap.AddDirectory("⌂ Home", "/")
}

// paginateItems returns a subset of items for the current page
func paginateItems[T any](items []T, page int) []T {
	start := (page - 1) * itemsPerPage
	end := start + itemsPerPage

	if start >= len(items) {
		return []T{}
	}

	if end > len(items) {
		end = len(items)
	}

	return items[start:end]
}

// Route routes a selector to the appropriate handler
func (r *Router) Route(selector string) []byte {
	ctx := context.Background()

	// Normalize path
	path := selector
	if path == "" {
		path = "/"
	}

	// Check if sections are registered for this path (sections override defaults)
	if r.server.GetSectionManager() != nil {
		sections := r.server.GetSectionManager().GetSectionsByPath(path)
		if len(sections) > 0 {
			return r.handleSections(ctx, sections, path)
		}
	}

	// Empty selector = root/home (default behavior when no section registered)
	if path == "/" {
		return r.handleRoot(ctx)
	}

	// Parse selector path
	parts := strings.Split(strings.TrimPrefix(selector, "/"), "/")
	if len(parts) == 0 {
		return r.handleRoot(ctx)
	}

	section := parts[0]

	switch section {
	case "notes":
		return r.handleNotes(ctx, parts[1:])

	case "articles":
		return r.handleArticles(ctx, parts[1:])

	case "replies":
		return r.handleReplies(ctx, parts[1:])

	case "mentions":
		return r.handleMentions(ctx, parts[1:])

	case "note":
		if len(parts) >= 2 {
			return r.handleNote(ctx, parts[1])
		}
		return r.errorResponse("Missing note ID")

	case "thread":
		if len(parts) >= 2 {
			return r.handleThread(ctx, parts[1])
		}
		return r.errorResponse("Missing thread ID")

	case "profile":
		if len(parts) >= 2 {
			return r.handleProfile(ctx, parts[1])
		}
		return r.errorResponse("Missing pubkey")

	case "diagnostics":
		return r.handleDiagnostics(ctx)

	case "search":
		return r.handleSearch(ctx, parts[1:])

	// Legacy support - redirect to new endpoints
	case "outbox":
		return r.handleNotes(ctx, parts[1:])

	case "inbox":
		return r.handleReplies(ctx, parts[1:])

	default:
		return r.errorResponse(fmt.Sprintf("Unknown selector: %s", selector))
	}
}

// handleRoot handles the root/home page
func (r *Router) handleRoot(ctx context.Context) []byte {
	gmap := NewGophermap(r.host, r.port)

	// Add header if configured
	r.addHeaderToGophermap(gmap, "home")

	gmap.AddWelcome("nophr - Nostr Gateway", "Browse Nostr content via Gopher protocol")

	gmap.AddDirectory("Notes", "/notes")
	gmap.AddDirectory("Articles", "/articles")
	gmap.AddDirectory("Replies", "/replies")
	gmap.AddDirectory("Mentions", "/mentions")
	gmap.AddSpacer()
	gmap.AddDirectory("Search", "/search")
	gmap.AddDirectory("Diagnostics", "/diagnostics")
	gmap.AddSpacer()
	gmap.AddInfo("Powered by nophr")

	// Add footer if configured
	r.addFooterToGophermap(gmap, "home")

	return gmap.Bytes()
}

// handleOutbox handles outbox listing
func (r *Router) handleOutbox(ctx context.Context, parts []string) []byte {
	gmap := NewGophermap(r.host, r.port)

	// Check if viewing a specific note
	if len(parts) > 0 && parts[0] != "" {
		return r.handleNote(ctx, parts[0])
	}

	// Query outbox notes
	queryHelper := r.server.GetQueryHelper()
	notes, err := queryHelper.GetOutboxNotes(ctx, 50)
	if err != nil {
		gmap.AddError(fmt.Sprintf("Error loading outbox: %v", err))
		gmap.AddSpacer()
		gmap.AddDirectory("← Back to Home", "/")
		return gmap.Bytes()
	}

	gmap.AddInfo("Outbox - My Notes")
	gmap.AddSpacer()

	// Add note links with aggregates
	if len(notes) > 0 {
		for _, note := range notes {
			linkText := eventTitle(note.Event)

			// Add author and timestamp
			gmap.AddInfo(fmt.Sprintf("   By %s - %s",
				truncatePubkey(note.Event.PubKey),
				formatTimestamp(note.Event.CreatedAt)))

			// Add aggregates if available
			if note.Aggregates != nil && note.Aggregates.HasInteractions() {
				aggText := r.renderer.renderAggregates(note.Aggregates)
				if aggText != "" {
					gmap.AddInfo("   " + aggText)
				}
			}

			gmap.AddTextFile(linkText, fmt.Sprintf("/outbox/%s", note.Event.ID))
			gmap.AddSpacer()
		}
	} else {
		gmap.AddInfo("No notes yet.")
		gmap.AddSpacer()
	}

	gmap.AddSpacer()
	gmap.AddDirectory("← Back to Home", "/")

	return gmap.Bytes()
}

// handleInbox handles inbox listing (legacy - redirects to replies)
func (r *Router) handleInbox(ctx context.Context, parts []string) []byte {
	return r.handleReplies(ctx, parts)
}

// handleNotes handles notes listing (kind 1, non-replies)
func (r *Router) handleNotes(ctx context.Context, parts []string) []byte {
	gmap := NewGophermap(r.host, r.port)

	// Parse page number from parts
	page, remaining := parsePageFromParts(parts)

	// Check if viewing a specific note (not "page")
	if len(remaining) > 0 && remaining[0] != "" && remaining[0] != "page" {
		return r.handleNote(ctx, remaining[0])
	}

	// Add header if configured
	r.addHeaderToGophermap(gmap, "notes")

	// Query notes
	queryHelper := r.server.GetQueryHelper()
	notes, err := queryHelper.GetNotes(ctx, 100) // Get more for pagination
	if err != nil {
		gmap.AddError(fmt.Sprintf("Error loading notes: %v", err))
		gmap.AddSpacer()
		gmap.AddDirectory("⌂ Home", "/")
		return gmap.Bytes()
	}

	gmap.AddInfo("Notes")
	gmap.AddSpacer()

	// Paginate notes
	totalNotes := len(notes)
	paginatedNotes := paginateItems(notes, page)

	// Add clickable note links with aggregates
	if len(paginatedNotes) > 0 {
		for _, note := range paginatedNotes {
			linkText := eventTitle(note.Event)

			// Add author and timestamp info line
			gmap.AddInfo(fmt.Sprintf("   By %s - %s",
				truncatePubkey(note.Event.PubKey),
				formatTimestamp(note.Event.CreatedAt)))

			// Add aggregate info if available
			if note.Aggregates != nil && note.Aggregates.HasInteractions() {
				aggText := r.renderer.renderAggregates(note.Aggregates)
				if aggText != "" {
					gmap.AddInfo("   " + aggText)
				}
			}

			// Add the clickable link
			gmap.AddTextFile(linkText, fmt.Sprintf("/note/%s", note.Event.ID))
			gmap.AddSpacer()
		}
	} else {
		gmap.AddInfo("No notes yet.")
		gmap.AddSpacer()
	}

	// Add pagination links
	r.addPaginationLinks(gmap, "/notes", page, totalNotes)

	// Add footer if configured
	r.addFooterToGophermap(gmap, "notes")

	return gmap.Bytes()
}

// handleArticles handles articles listing (kind 30023)
func (r *Router) handleArticles(ctx context.Context, parts []string) []byte {
	gmap := NewGophermap(r.host, r.port)

	// Parse page number from parts
	page, _ := parsePageFromParts(parts)

	// Add header if configured
	r.addHeaderToGophermap(gmap, "articles")

	// Query articles
	queryHelper := r.server.GetQueryHelper()
	articles, err := queryHelper.GetArticles(ctx, 100) // Get more for pagination
	if err != nil {
		gmap.AddError(fmt.Sprintf("Error loading articles: %v", err))
		gmap.AddSpacer()
		gmap.AddDirectory("⌂ Home", "/")
		return gmap.Bytes()
	}

	gmap.AddInfo("Articles")
	gmap.AddSpacer()

	// Paginate articles
	totalArticles := len(articles)
	paginatedArticles := paginateItems(articles, page)

	// Add article links with aggregates
	if len(paginatedArticles) > 0 {
		for _, article := range paginatedArticles {
			linkText := eventTitle(article.Event)

			// Add author and timestamp
			gmap.AddInfo(fmt.Sprintf("   By %s - %s",
				truncatePubkey(article.Event.PubKey),
				formatTimestamp(article.Event.CreatedAt)))

			// Add aggregates if available
			if article.Aggregates != nil && article.Aggregates.HasInteractions() {
				aggText := r.renderer.renderAggregates(article.Aggregates)
				if aggText != "" {
					gmap.AddInfo("   " + aggText)
				}
			}

			gmap.AddTextFile(linkText, fmt.Sprintf("/note/%s", article.Event.ID))
			gmap.AddSpacer()
		}
	} else {
		gmap.AddInfo("No articles yet.")
		gmap.AddSpacer()
	}

	// Add pagination links
	r.addPaginationLinks(gmap, "/articles", page, totalArticles)

	// Add footer if configured
	r.addFooterToGophermap(gmap, "articles")

	return gmap.Bytes()
}

// handleReplies handles replies listing
func (r *Router) handleReplies(ctx context.Context, parts []string) []byte {
	gmap := NewGophermap(r.host, r.port)

	// Parse page number from parts
	page, _ := parsePageFromParts(parts)

	// Add header if configured
	r.addHeaderToGophermap(gmap, "replies")

	// Query replies
	queryHelper := r.server.GetQueryHelper()
	replies, err := queryHelper.GetReplies(ctx, 100) // Get more for pagination
	if err != nil {
		gmap.AddError(fmt.Sprintf("Error loading replies: %v", err))
		gmap.AddSpacer()
		gmap.AddDirectory("⌂ Home", "/")
		return gmap.Bytes()
	}

	gmap.AddInfo("Replies")
	gmap.AddSpacer()

	// Paginate replies
	totalReplies := len(replies)
	paginatedReplies := paginateItems(replies, page)

	// Add reply links with aggregates
	if len(paginatedReplies) > 0 {
		for _, reply := range paginatedReplies {
			linkText := eventTitle(reply.Event)

			// Add author and timestamp
			gmap.AddInfo(fmt.Sprintf("   By %s - %s",
				truncatePubkey(reply.Event.PubKey),
				formatTimestamp(reply.Event.CreatedAt)))

			// Add aggregates if available
			if reply.Aggregates != nil && reply.Aggregates.HasInteractions() {
				aggText := r.renderer.renderAggregates(reply.Aggregates)
				if aggText != "" {
					gmap.AddInfo("   " + aggText)
				}
			}

			gmap.AddTextFile(linkText, fmt.Sprintf("/note/%s", reply.Event.ID))
			gmap.AddSpacer()
		}
	} else {
		gmap.AddInfo("No replies yet.")
		gmap.AddSpacer()
	}

	// Add pagination links
	r.addPaginationLinks(gmap, "/replies", page, totalReplies)

	// Add footer if configured
	r.addFooterToGophermap(gmap, "replies")

	return gmap.Bytes()
}

// handleMentions handles mentions listing
func (r *Router) handleMentions(ctx context.Context, parts []string) []byte {
	gmap := NewGophermap(r.host, r.port)

	// Parse page number from parts
	page, _ := parsePageFromParts(parts)

	// Add header if configured
	r.addHeaderToGophermap(gmap, "mentions")

	// Query mentions
	queryHelper := r.server.GetQueryHelper()
	mentions, err := queryHelper.GetMentions(ctx, 100) // Get more for pagination
	if err != nil {
		gmap.AddError(fmt.Sprintf("Error loading mentions: %v", err))
		gmap.AddSpacer()
		gmap.AddDirectory("⌂ Home", "/")
		return gmap.Bytes()
	}

	gmap.AddInfo("Mentions")
	gmap.AddSpacer()

	// Paginate mentions
	totalMentions := len(mentions)
	paginatedMentions := paginateItems(mentions, page)

	// Add mention links with aggregates
	if len(paginatedMentions) > 0 {
		for _, mention := range paginatedMentions {
			linkText := eventTitle(mention.Event)

			// Add author and timestamp
			gmap.AddInfo(fmt.Sprintf("   By %s - %s",
				truncatePubkey(mention.Event.PubKey),
				formatTimestamp(mention.Event.CreatedAt)))

			// Add aggregates if available
			if mention.Aggregates != nil && mention.Aggregates.HasInteractions() {
				aggText := r.renderer.renderAggregates(mention.Aggregates)
				if aggText != "" {
					gmap.AddInfo("   " + aggText)
				}
			}

			gmap.AddTextFile(linkText, fmt.Sprintf("/note/%s", mention.Event.ID))
			gmap.AddSpacer()
		}
	} else {
		gmap.AddInfo("No mentions yet.")
		gmap.AddSpacer()
	}

	// Add pagination links
	r.addPaginationLinks(gmap, "/mentions", page, totalMentions)

	// Add footer if configured
	r.addFooterToGophermap(gmap, "mentions")

	return gmap.Bytes()
}

// handleNote handles displaying a single note
func (r *Router) handleNote(ctx context.Context, noteID string) []byte {
	// Query the note
	events, err := r.server.GetStorage().QueryEvents(ctx, nostr.Filter{
		IDs: []string{noteID},
	})
	if err != nil || len(events) == 0 {
		gmap := NewGophermap(r.host, r.port)
		gmap.AddError(fmt.Sprintf("Note not found: %s", noteID))
		gmap.AddSpacer()
		gmap.AddDirectory("← Back to Home", "/")
		return gmap.Bytes()
	}

	note := events[0]

	// Get aggregates from storage
	aggData, err := r.server.GetStorage().GetAggregate(ctx, noteID)
	var agg *aggregates.EventAggregates
	if err == nil && aggData != nil {
		agg = &aggregates.EventAggregates{
			EventID:         aggData.EventID,
			ReplyCount:      aggData.ReplyCount,
			ReactionTotal:   aggData.ReactionTotal,
			ReactionCounts:  aggData.ReactionCounts,
			ZapSatsTotal:    aggData.ZapSatsTotal,
			LastInteraction: aggData.LastInteractionAt,
		}
	}

	// Build thread view (includes replies and navigation)
	threadView, err := r.server.GetQueryHelper().GetThreadByEvent(ctx, noteID)
	if err != nil {
		threadView = nil
	}

	// Render the note as plain text
	text := r.renderer.RenderNoteWithThread(note, agg, threadView)

	// Return as plain text with gopher terminator (not gophermap)
	return append([]byte(text), []byte(".\r\n")...)
}

// handleThread handles displaying a thread
func (r *Router) handleThread(ctx context.Context, rootID string) []byte {
	queryHelper := r.server.GetQueryHelper()

	// Query the thread
	thread, err := queryHelper.GetThreadByEvent(ctx, rootID)
	if err != nil || thread == nil {
		gmap := NewGophermap(r.host, r.port)
		gmap.AddError(fmt.Sprintf("Thread not found: %s", rootID))
		gmap.AddSpacer()
		gmap.AddDirectory("← Back to Home", "/")
		return gmap.Bytes()
	}

	// Render the thread
	text := r.renderer.RenderThread(thread)

	// Return as plain text with gopher terminator
	return append([]byte(text), []byte(".\r\n")...)
}

// handleProfile handles displaying a profile
func (r *Router) handleProfile(ctx context.Context, pubkey string) []byte {
	// Query profile metadata (kind 0)
	events, err := r.server.GetStorage().QueryEvents(ctx, nostr.Filter{
		Kinds:   []int{0},
		Authors: []string{pubkey},
		Limit:   1,
	})
	if err != nil || len(events) == 0 {
		gmap := NewGophermap(r.host, r.port)
		gmap.AddError(fmt.Sprintf("Profile not found: %s", pubkey))
		gmap.AddSpacer()
		gmap.AddDirectory("← Back to Home", "/")
		return gmap.Bytes()
	}

	profile := events[0]

	// Render the profile
	text := r.renderer.RenderProfile(profile)

	// Return as plain text with gopher terminator
	return append([]byte(text), []byte(".\r\n")...)
}

// handleDiagnostics handles the diagnostics page
func (r *Router) handleDiagnostics(ctx context.Context) []byte {
	gmap := NewGophermap(r.host, r.port)

	gmap.AddInfo("Diagnostics")
	gmap.AddInfo(strings.Repeat("=", 15))
	gmap.AddSpacer()

	gmap.AddInfo("Server Status: Running")
	gmap.AddInfo(fmt.Sprintf("Host: %s", r.host))
	gmap.AddInfo(fmt.Sprintf("Port: %d", r.port))
	gmap.AddSpacer()

	// TODO: Add storage stats, sync status, etc.
	gmap.AddInfo("Storage: Connected")
	gmap.AddSpacer()

	gmap.AddDirectory("← Back to Home", "/")

	return gmap.Bytes()
}

// handleSearch handles search requests
func (r *Router) handleSearch(ctx context.Context, params []string) []byte {
	gmap := NewGophermap(r.host, r.port)

	// If no search query, show search page
	if len(params) == 0 || params[0] == "" {
		gmap.AddInfo("Search Nostr Content")
		gmap.AddInfo(strings.Repeat("=", 70))
		gmap.AddSpacer()
		gmap.AddInfo("Note: Gopher protocol requires entering full path with query")
		gmap.AddInfo("Format: /search/your+search+terms")
		gmap.AddSpacer()
		gmap.AddInfo("Examples:")
		gmap.AddInfo("  /search/nostr+protocol")
		gmap.AddInfo("  /search/bitcoin")
		gmap.AddSpacer()
		gmap.AddDirectory("← Back to Home", "/")
		return gmap.Bytes()
	}

	// Decode search query (URL encoded, replace + with space)
	query := strings.ReplaceAll(params[0], "+", " ")

	gmap.AddInfo(fmt.Sprintf("Search Results: \"%s\"", query))
	gmap.AddInfo(strings.Repeat("=", 70))
	gmap.AddSpacer()

	// Perform search using NIP-50
	events, err := r.server.storage.QueryEventsWithSearch(ctx, nostr.Filter{
		Search: query,
		Kinds:  []int{0, 1, 30023}, // Profiles, notes, articles
		Limit:  20,
	})

	if err != nil {
		gmap.AddError(fmt.Sprintf("Search failed: %v", err))
		gmap.AddSpacer()
		gmap.AddDirectory("← Back to Search", "/search")
		return gmap.Bytes()
	}

	if len(events) == 0 {
		gmap.AddInfo("No results found")
		gmap.AddSpacer()
		gmap.AddDirectory("← Back to Search", "/search")
		return gmap.Bytes()
	}

	gmap.AddInfo(fmt.Sprintf("Found %d results:", len(events)))
	gmap.AddSpacer()

	for _, event := range events {
		switch event.Kind {
		case 0: // Profile
			gmap.AddTextFile(fmt.Sprintf("[Profile] %s", truncatePubkey(event.PubKey)),
				fmt.Sprintf("/profile/%s", event.PubKey))

		case 1: // Note
			summary := getSummary(event.Content, 80)
			gmap.AddTextFile(fmt.Sprintf("[Note] %s", summary),
				fmt.Sprintf("/note/%s", event.ID))

		case 30023: // Article
			summary := getSummary(event.Content, 80)
			gmap.AddTextFile(fmt.Sprintf("[Article] %s", summary),
				fmt.Sprintf("/note/%s", event.ID))
		}
	}

	gmap.AddSpacer()
	gmap.AddDirectory("← Back to Search", "/search")
	gmap.AddDirectory("← Back to Home", "/")

	return gmap.Bytes()
}

// errorResponse returns an error gophermap
func (r *Router) errorResponse(message string) []byte {
	gmap := NewGophermap(r.host, r.port)
	gmap.AddError(message)
	gmap.AddSpacer()
	gmap.AddDirectory("← Back to Home", "/")
	return gmap.Bytes()
}

// addHeaderToGophermap adds configured header to a gophermap
func (r *Router) addHeaderToGophermap(gmap *Gophermap, page string) {
	header, err := r.renderer.loader.GetHeader(page)
	if err != nil || header == "" {
		return
	}

	// Split header into lines and add as info lines
	lines := strings.Split(header, "\n")
	for _, line := range lines {
		gmap.AddInfo(line)
	}
	gmap.AddSpacer()
}

// addFooterToGophermap adds configured footer to a gophermap
func (r *Router) addFooterToGophermap(gmap *Gophermap, page string) {
	footer, err := r.renderer.loader.GetFooter(page)
	if err != nil || footer == "" {
		return
	}

	// Split footer into lines and add as info lines
	gmap.AddSpacer()
	lines := strings.Split(footer, "\n")
	for _, line := range lines {
		gmap.AddInfo(line)
	}
}

// getSummary creates a summary of content for display
func getSummary(content string, maxLen int) string {
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

func eventTitle(event *nostr.Event) string {
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
	if len(firstLine) > 60 {
		return firstLine[:57] + "..."
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

// handleSection renders a custom section
func (r *Router) handleSection(ctx context.Context, section *sections.Section, path string) []byte {
	gmap := NewGophermap(r.host, r.port)

	// Parse page number from path
	page := 1
	// TODO: Extract page from path if needed

	// Add header if configured
	r.addHeaderToGophermap(gmap, section.Name)

	// Get section page
	sectionPage, err := r.server.GetSectionManager().GetPage(ctx, section.Name, page)
	if err != nil {
		gmap.AddError(fmt.Sprintf("Error loading section: %v", err))
		gmap.AddSpacer()
		gmap.AddDirectory("⌂ Home", "/")
		return gmap.Bytes()
	}

	// Title
	gmap.AddInfo(section.Title)
	if section.Description != "" {
		gmap.AddInfo(section.Description)
	}
	gmap.AddSpacer()

	// Render events
	if len(sectionPage.Events) > 0 {
		for _, event := range sectionPage.Events {
			linkText := eventTitle(event)

			// Add author and timestamp if configured
			if section.ShowAuthors && section.ShowDates {
				gmap.AddInfo(fmt.Sprintf("   By %s - %s",
					truncatePubkey(event.PubKey),
					formatTimestamp(event.CreatedAt)))
			} else if section.ShowAuthors {
				gmap.AddInfo(fmt.Sprintf("   By %s", truncatePubkey(event.PubKey)))
			} else if section.ShowDates {
				gmap.AddInfo(fmt.Sprintf("   %s", formatTimestamp(event.CreatedAt)))
			}

			// Add the clickable link
			gmap.AddTextFile(linkText, fmt.Sprintf("/note/%s", event.ID))
			gmap.AddSpacer()
		}
	} else {
		gmap.AddInfo("No content yet.")
		gmap.AddSpacer()
	}

	// Add "more" link if configured
	if section.MoreLink != nil {
		// Get the target section to determine its path
		targetSection, err := r.server.GetSectionManager().GetSection(section.MoreLink.SectionRef)
		if err == nil && targetSection.Path != "" {
			gmap.AddSpacer()
			gmap.AddDirectory(fmt.Sprintf("→ %s", section.MoreLink.Text), targetSection.Path)
		}
	}

	// Add pagination links
	if sectionPage.TotalPages > 1 {
		r.addPaginationLinks(gmap, path, page, int(sectionPage.TotalItems))
	} else {
		gmap.AddSpacer()
		gmap.AddDirectory("⌂ Home", "/")
	}

	// Add footer if configured
	r.addFooterToGophermap(gmap, section.Name)

	return gmap.Bytes()
}

// handleSections renders multiple sections on a single page (e.g., homepage with multiple filtered views)
func (r *Router) handleSections(ctx context.Context, sections []*sections.Section, path string) []byte {
	gmap := NewGophermap(r.host, r.port)

	// Add header if first section has one configured
	if len(sections) > 0 {
		r.addHeaderToGophermap(gmap, sections[0].Name)
	}

	// Render each section in order
	for i, section := range sections {
		// Get section page (always page 1 for multi-section views)
		sectionPage, err := r.server.GetSectionManager().GetPage(ctx, section.Name, 1)
		if err != nil {
			gmap.AddError(fmt.Sprintf("Error loading section %s: %v", section.Name, err))
			gmap.AddSpacer()
			continue
		}

		// Section title and description
		if section.Title != "" {
			gmap.AddInfo(section.Title)
		}
		if section.Description != "" {
			gmap.AddInfo(section.Description)
		}
		gmap.AddSpacer()

		// Render events from section
		if len(sectionPage.Events) > 0 {
			for _, event := range sectionPage.Events {
				linkText := eventTitle(event)

				// Add author and timestamp if configured
				if section.ShowAuthors && section.ShowDates {
					gmap.AddInfo(fmt.Sprintf("   By %s - %s",
						truncatePubkey(event.PubKey),
						formatTimestamp(event.CreatedAt)))
				} else if section.ShowAuthors {
					gmap.AddInfo(fmt.Sprintf("   By %s", truncatePubkey(event.PubKey)))
				} else if section.ShowDates {
					gmap.AddInfo(fmt.Sprintf("   %s", formatTimestamp(event.CreatedAt)))
				}

				// Add the clickable link
				gmap.AddTextFile(linkText, fmt.Sprintf("/note/%s", event.ID))
				gmap.AddSpacer()
			}
		} else {
			gmap.AddInfo("No content yet.")
			gmap.AddSpacer()
		}

		// Add "more" link if configured
		if section.MoreLink != nil {
			targetSection, err := r.server.GetSectionManager().GetSection(section.MoreLink.SectionRef)
			if err == nil && targetSection.Path != "" {
				gmap.AddDirectory(fmt.Sprintf("→ %s", section.MoreLink.Text), targetSection.Path)
				gmap.AddSpacer()
			}
		}

		// Add separator between sections (except after last)
		if i < len(sections)-1 {
			gmap.AddInfo("─────────────────────────────────────────")
			gmap.AddSpacer()
		}
	}

	// Add home link at bottom
	gmap.AddSpacer()
	gmap.AddDirectory("⌂ Home", "/")

	// Add footer if last section has one configured
	if len(sections) > 0 {
		r.addFooterToGophermap(gmap, sections[len(sections)-1].Name)
	}

	return gmap.Bytes()
}
