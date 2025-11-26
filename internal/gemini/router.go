package gemini

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/nbd-wtf/go-nostr"
	"github.com/sandwich/nophr/internal/aggregates"
	"github.com/sandwich/nophr/internal/sections"
)

// Router handles URL routing for Gemini requests
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

// Route routes a URL to the appropriate handler
func (r *Router) Route(u *url.URL) []byte {
	ctx := context.Background()

	// Extract path
	path := u.Path
	if path == "" {
		path = "/"
	}

	// Check if sections are registered for this path (sections override defaults)
	if r.server.GetSectionManager() != nil {
		sectionsList := r.server.GetSectionManager().GetSectionsByPath(path)
		if len(sectionsList) > 0 {
			return r.handleSections(ctx, sectionsList, path, u.Query())
		}
	}

	// Parse path
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		return r.handleRoot(ctx, u.Query())
	}

	section := parts[0]

	switch section {
	case "notes":
		return r.handleNotes(ctx, parts[1:], u.Query())

	case "articles":
		return r.handleArticles(ctx, parts[1:], u.Query())

	case "replies":
		return r.handleReplies(ctx, parts[1:], u.Query())

	case "mentions":
		return r.handleMentions(ctx, parts[1:], u.Query())

	case "note":
		if len(parts) >= 2 {
			return r.handleNote(ctx, parts[1])
		}
		return FormatErrorResponse(StatusNotFound, "Missing note ID")

	case "thread":
		if len(parts) >= 2 {
			return r.handleThread(ctx, parts[1])
		}
		return FormatErrorResponse(StatusNotFound, "Missing thread ID")

	case "profile":
		if len(parts) >= 2 {
			return r.handleProfile(ctx, parts[1])
		}
		return FormatErrorResponse(StatusNotFound, "Missing pubkey")

	case "search":
		return r.handleSearch(ctx, u.Query())

	case "diagnostics":
		return r.handleDiagnostics(ctx)

	// Legacy support - redirect to new endpoints
	case "outbox":
		return r.handleNotes(ctx, parts[1:], u.Query())

	case "inbox":
		return r.handleReplies(ctx, parts[1:], u.Query())

	default:
		return FormatErrorResponse(StatusNotFound, fmt.Sprintf("Unknown path: %s", path))
	}
}

// handleRoot handles the root/home page
func (r *Router) handleRoot(ctx context.Context, query url.Values) []byte {
	gemtext := r.renderer.RenderHome()
	return FormatSuccessResponse(gemtext)
}

// handleOutbox handles outbox listing
func (r *Router) handleOutbox(ctx context.Context, parts []string, query url.Values) []byte {
	// Check if viewing a specific note
	if len(parts) > 0 && parts[0] != "" {
		return r.handleNote(ctx, parts[0])
	}

	// Query outbox notes
	queryHelper := r.server.GetQueryHelper()
	notes, err := queryHelper.GetOutboxNotes(ctx, 50)
	if err != nil {
		return FormatErrorResponse(StatusTemporaryFailure, fmt.Sprintf("Error loading outbox: %v", err))
	}

	// Render note list
	gemtext := r.renderer.RenderNoteList(notes, "Outbox - My Notes", r.geminiURL("/"))
	return FormatSuccessResponse(gemtext)
}

// handleInbox handles inbox listing (legacy - redirects to replies)
func (r *Router) handleInbox(ctx context.Context, parts []string, query url.Values) []byte {
	return r.handleReplies(ctx, parts, query)
}

// handleNotes handles notes listing (kind 1, non-replies)
func (r *Router) handleNotes(ctx context.Context, parts []string, query url.Values) []byte {
	// Check if viewing a specific note
	if len(parts) > 0 && parts[0] != "" {
		return r.handleNote(ctx, parts[0])
	}

	// Query notes
	queryHelper := r.server.GetQueryHelper()
	notes, err := queryHelper.GetNotes(ctx, 50)
	if err != nil {
		return FormatErrorResponse(StatusTemporaryFailure, fmt.Sprintf("Error loading notes: %v", err))
	}

	// Render note list
	gemtext := r.renderer.RenderNoteList(notes, "Notes", r.geminiURL("/"))
	return FormatSuccessResponse(gemtext)
}

// handleArticles handles articles listing (kind 30023)
func (r *Router) handleArticles(ctx context.Context, parts []string, query url.Values) []byte {
	// Query articles
	queryHelper := r.server.GetQueryHelper()
	articles, err := queryHelper.GetArticles(ctx, 50)
	if err != nil {
		return FormatErrorResponse(StatusTemporaryFailure, fmt.Sprintf("Error loading articles: %v", err))
	}

	// Render article list
	gemtext := r.renderer.RenderNoteList(articles, "Articles", r.geminiURL("/"))
	return FormatSuccessResponse(gemtext)
}

// handleReplies handles replies listing
func (r *Router) handleReplies(ctx context.Context, parts []string, query url.Values) []byte {
	// Query replies
	queryHelper := r.server.GetQueryHelper()
	replies, err := queryHelper.GetReplies(ctx, 50)
	if err != nil {
		return FormatErrorResponse(StatusTemporaryFailure, fmt.Sprintf("Error loading replies: %v", err))
	}

	// Render reply list
	gemtext := r.renderer.RenderNoteList(replies, "Replies", r.geminiURL("/"))
	return FormatSuccessResponse(gemtext)
}

// handleMentions handles mentions listing
func (r *Router) handleMentions(ctx context.Context, parts []string, query url.Values) []byte {
	// Query mentions
	queryHelper := r.server.GetQueryHelper()
	mentions, err := queryHelper.GetMentions(ctx, 50)
	if err != nil {
		return FormatErrorResponse(StatusTemporaryFailure, fmt.Sprintf("Error loading mentions: %v", err))
	}

	// Render mention list
	gemtext := r.renderer.RenderNoteList(mentions, "Mentions", r.geminiURL("/"))
	return FormatSuccessResponse(gemtext)
}

// handleNote handles displaying a single note
func (r *Router) handleNote(ctx context.Context, noteID string) []byte {
	// Query the note
	events, err := r.server.GetStorage().QueryEvents(ctx, nostr.Filter{
		IDs: []string{noteID},
	})
	if err != nil || len(events) == 0 {
		return FormatErrorResponse(StatusNotFound, fmt.Sprintf("Note not found: %s", noteID))
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

	// Render the note
	gemtext := r.renderer.RenderNoteWithThread(note, agg, threadView, r.geminiURL("/thread/"+noteID), r.geminiURL("/"))
	return FormatSuccessResponse(gemtext)
}

// handleThread handles displaying a thread
func (r *Router) handleThread(ctx context.Context, rootID string) []byte {
	queryHelper := r.server.GetQueryHelper()

	// Query the thread
	thread, err := queryHelper.GetThreadByEvent(ctx, rootID)
	if err != nil || thread == nil {
		return FormatErrorResponse(StatusNotFound, fmt.Sprintf("Thread not found: %s", rootID))
	}

	// Render the thread
	gemtext := r.renderer.RenderThread(thread, r.geminiURL("/"))
	return FormatSuccessResponse(gemtext)
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
		return FormatErrorResponse(StatusNotFound, fmt.Sprintf("Profile not found: %s", pubkey))
	}

	profile := events[0]

	// Render the profile
	gemtext := r.renderer.RenderProfile(profile, r.geminiURL("/"))
	return FormatSuccessResponse(gemtext)
}

// handleSearch handles search functionality
func (r *Router) handleSearch(ctx context.Context, query url.Values) []byte {
	searchQuery := query.Get("q")

	// If no query provided, request input
	if searchQuery == "" {
		return FormatInputResponse("Enter search query:", false)
	}

	// Perform NIP-50 search
	events, err := r.server.GetStorage().QueryEventsWithSearch(ctx, nostr.Filter{
		Search: searchQuery,
		Kinds:  []int{0, 1, 30023}, // Profiles, notes, articles
		Limit:  50,
	})

	gemtext := "# Search Results\n\n"
	gemtext += fmt.Sprintf("Query: \"%s\"\n\n", searchQuery)

	if err != nil {
		gemtext += fmt.Sprintf("Error: %v\n\n", err)
		gemtext += fmt.Sprintf("=> %s Try Again\n", r.geminiURL("/search"))
		gemtext += fmt.Sprintf("=> %s Back to Home\n", r.geminiURL("/"))
		return FormatSuccessResponse(gemtext)
	}

	if len(events) == 0 {
		gemtext += "No results found.\n\n"
		gemtext += fmt.Sprintf("=> %s Try Another Search\n", r.geminiURL("/search"))
		gemtext += fmt.Sprintf("=> %s Back to Home\n", r.geminiURL("/"))
		return FormatSuccessResponse(gemtext)
	}

	gemtext += fmt.Sprintf("Found %d results:\n\n", len(events))

	for _, event := range events {
		switch event.Kind {
		case 0: // Profile
			gemtext += fmt.Sprintf("=> %s [Profile] %s\n",
				r.geminiURL(fmt.Sprintf("/profile/%s", event.PubKey)),
				truncatePubkey(event.PubKey))

		case 1: // Note
			summary := r.renderer.GetSummary(event.Content, 100)
			gemtext += fmt.Sprintf("=> %s [Note] %s\n",
				r.geminiURL(fmt.Sprintf("/note/%s", event.ID)),
				summary)

		case 30023: // Article
			summary := r.renderer.GetSummary(event.Content, 100)
			gemtext += fmt.Sprintf("=> %s [Article] %s\n",
				r.geminiURL(fmt.Sprintf("/note/%s", event.ID)),
				summary)
		}
	}

	gemtext += "\n"
	gemtext += fmt.Sprintf("=> %s New Search\n", r.geminiURL("/search"))
	gemtext += fmt.Sprintf("=> %s Back to Home\n", r.geminiURL("/"))

	return FormatSuccessResponse(gemtext)
}

// handleDiagnostics handles the diagnostics page
func (r *Router) handleDiagnostics(ctx context.Context) []byte {
	gemtext := "# Diagnostics\n\n"
	gemtext += "## Server Status\n\n"
	gemtext += "* Server: Running\n"
	gemtext += fmt.Sprintf("* Host: %s\n", r.host)
	gemtext += fmt.Sprintf("* Port: %d\n", r.port)
	gemtext += "\n## Storage\n\n"
	gemtext += "* Status: Connected\n"
	gemtext += "\n"
	gemtext += fmt.Sprintf("=> %s Back to Home\n", r.geminiURL("/"))

	return FormatSuccessResponse(gemtext)
}

// geminiURL constructs a gemini:// URL for the given path
func (r *Router) geminiURL(path string) string {
	if r.port == 1965 {
		return fmt.Sprintf("gemini://%s%s", r.host, path)
	}
	return fmt.Sprintf("gemini://%s:%d%s", r.host, r.port, path)
}

// handleSections renders multiple sections on a single page (e.g., homepage with multiple filtered views)
func (r *Router) handleSections(ctx context.Context, sectionsList []*sections.Section, path string, query url.Values) []byte {
	var gemtext strings.Builder

	// Render each section in order
	for i, section := range sectionsList {
		// Get section page (always page 1 for multi-section views)
		sectionPage, err := r.server.GetSectionManager().GetPage(ctx, section.Name, 1)
		if err != nil {
			gemtext.WriteString(fmt.Sprintf("# Error loading section %s\n\n", section.Name))
			gemtext.WriteString(fmt.Sprintf("Error: %v\n\n", err))
			continue
		}

		// Section title and description
		if section.Title != "" {
			gemtext.WriteString(fmt.Sprintf("## %s\n\n", section.Title))
		}
		if section.Description != "" {
			gemtext.WriteString(fmt.Sprintf("%s\n\n", section.Description))
		}

		// Render events from section
		if len(sectionPage.Events) > 0 {
			for _, event := range sectionPage.Events {
				// Extract first line for display
				content := event.Content
				if len(content) > 80 {
					content = content[:77] + "..."
				}
				firstLine := strings.Split(content, "\n")[0]

				linkText := firstLine

				// Add author and timestamp if configured
				if section.ShowAuthors && section.ShowDates {
					gemtext.WriteString(fmt.Sprintf("%s - %s\n",
						truncatePubkey(event.PubKey),
						formatTimestamp(event.CreatedAt)))
				} else if section.ShowAuthors {
					gemtext.WriteString(fmt.Sprintf("%s\n", truncatePubkey(event.PubKey)))
				} else if section.ShowDates {
					gemtext.WriteString(fmt.Sprintf("%s\n", formatTimestamp(event.CreatedAt)))
				}

				// Add the clickable link
				gemtext.WriteString(fmt.Sprintf("=> %s %s\n\n", r.geminiURL(fmt.Sprintf("/note/%s", event.ID)), linkText))
			}
		} else {
			gemtext.WriteString("No content yet.\n\n")
		}

		// Add "more" link if configured
		if section.MoreLink != nil {
			targetSection, err := r.server.GetSectionManager().GetSection(section.MoreLink.SectionRef)
			if err == nil && targetSection.Path != "" {
				gemtext.WriteString(fmt.Sprintf("=> %s → %s\n\n", r.geminiURL(targetSection.Path), section.MoreLink.Text))
			}
		}

		// Add separator between sections (except after last)
		if i < len(sectionsList)-1 {
			gemtext.WriteString("---\n\n")
		}
	}

	// Add home link at bottom
	gemtext.WriteString("\n=> / ⌂ Home\n")

	return FormatSuccessResponse(gemtext.String())
}
