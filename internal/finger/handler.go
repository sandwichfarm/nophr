package finger

import (
	"context"
	"fmt"
	"strings"

	"github.com/nbd-wtf/go-nostr"
	"github.com/sandwichfarm/nophr/internal/config"
)

// Handler handles Finger protocol queries
type Handler struct {
	server   *Server
	config   *config.Config
	renderer *Renderer
}

// NewHandler creates a new query handler
func NewHandler(server *Server, cfg *config.Config) *Handler {
	return &Handler{
		server:   server,
		config:   cfg,
		renderer: NewRenderer(),
	}
}

// Query represents a parsed Finger query
type Query struct {
	Verbose  bool   // /W flag
	Username string // username (or pubkey)
	Host     string // hostname for forwarding (not supported)
}

// ParseQuery parses a Finger protocol query
// Format: [/W] [username][@hostname] <CRLF>
func ParseQuery(query string) *Query {
	q := &Query{}

	// Split by @ to check for hostname
	parts := strings.SplitN(query, "@", 2)
	userPart := parts[0]

	if len(parts) > 1 {
		q.Host = parts[1]
	}

	// Check for /W flag
	userPart = strings.TrimSpace(userPart)
	if strings.HasPrefix(userPart, "/W") || strings.HasPrefix(userPart, "/w") {
		q.Verbose = true
		userPart = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(userPart, "/W"), "/w"))
	}

	q.Username = userPart

	return q
}

// Handle processes a Finger query and returns a response
func (h *Handler) Handle(queryStr string) string {
	ctx := context.Background()
	query := ParseQuery(queryStr)

	// Forwarding not supported
	if query.Host != "" {
		return "Forwarding to other hosts not supported.\r\n"
	}

	// Empty query = list all users (if enabled)
	if query.Username == "" {
		return h.handleListUsers(ctx, query.Verbose)
	}

	// User query
	return h.handleUserQuery(ctx, query.Username, query.Verbose)
}

// handleListUsers handles a request to list all users
func (h *Handler) handleListUsers(ctx context.Context, verbose bool) string {
	// Check if listing is enabled
	if h.server.GetConfig().MaxUsers <= 0 {
		return "User listing disabled.\r\n"
	}

	// For Nostr, we only have the owner
	return h.handleUserQuery(ctx, "owner", verbose)
}

// handleUserQuery handles a query for a specific user
func (h *Handler) handleUserQuery(ctx context.Context, username string, verbose bool) string {
	// Normalize username
	username = strings.ToLower(username)

	// Check if querying owner
	if username == "" || username == "owner" || username == h.server.GetOwnerPubkey() {
		return h.renderOwnerInfo(ctx, verbose)
	}

	// Check if querying by pubkey (followed user)
	return h.renderUserInfo(ctx, username, verbose)
}

// renderOwnerInfo renders information about the server owner
func (h *Handler) renderOwnerInfo(ctx context.Context, verbose bool) string {
	queryHelper := h.server.GetQueryHelper()
	ownerPubkey := h.server.GetOwnerPubkey()

	// Get owner's profile
	profile, err := h.server.GetStorage().QueryEvents(ctx, nostr.Filter{
		Kinds:   []int{0},
		Authors: []string{ownerPubkey},
		Limit:   1,
	})

	var profileEvent *nostr.Event
	if err == nil && len(profile) > 0 {
		profileEvent = profile[0]
	}

	// Get recent notes
	notes, err := queryHelper.GetOutboxNotes(ctx, 5)
	if err != nil {
		notes = nil
	}

	// Render
	return h.renderer.RenderUser(ownerPubkey, profileEvent, notes, verbose)
}

// renderUserInfo renders information about a followed user
func (h *Handler) renderUserInfo(ctx context.Context, pubkey string, verbose bool) string {
	// Query profile
	profile, err := h.server.GetStorage().QueryEvents(ctx, nostr.Filter{
		Kinds:   []int{0},
		Authors: []string{pubkey},
		Limit:   1,
	})

	if err != nil || len(profile) == 0 {
		return fmt.Sprintf("User not found: %s\r\n", pubkey)
	}

	profileEvent := profile[0]

	// Get recent notes
	notes, err := h.server.GetStorage().QueryEvents(ctx, nostr.Filter{
		Kinds:   []int{1},
		Authors: []string{pubkey},
		Limit:   5,
	})

	var enrichedNotes []*enrichedNote
	if err == nil {
		for _, note := range notes {
			enrichedNotes = append(enrichedNotes, &enrichedNote{Event: note})
		}
	}

	// Render
	return h.renderer.RenderUser(pubkey, profileEvent, enrichedNotes, verbose)
}

// enrichedNote is a simplified version for finger output
type enrichedNote struct {
	Event *nostr.Event
}
