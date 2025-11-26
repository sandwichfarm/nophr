package sections

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip19"
	"github.com/sandwich/nophr/internal/aggregates"
	"github.com/sandwich/nophr/internal/storage"
)

// Section defines a content section with filtering and pagination
type Section struct {
	Name        string
	Path        string // URL path (e.g., "/diy", "/philosophy", "/" for homepage)
	Title       string
	Description string
	Filters     FilterSet
	SortBy      SortField
	SortOrder   SortOrder
	Limit       int
	ShowDates   bool
	ShowAuthors bool
	GroupBy     GroupField
	MoreLink    *MoreLink // Optional link to full paginated view
	Order       int       // Display order when multiple sections share a path (lower numbers first)
}

// MoreLink defines a "more" link to a full paginated section view
type MoreLink struct {
	Text       string // Link text (e.g., "More DIY posts", "View all articles")
	SectionRef string // Name of the section to link to (must be registered)
}

// FilterSet contains multiple filter criteria
type FilterSet struct {
	Kinds   []int
	Authors []string
	Tags    map[string][]string
	Since   *time.Time
	Until   *time.Time
	Search  string
	Scope   Scope
	IsReply *bool
}

// SortField defines how to sort events
type SortField string

const (
	SortByCreatedAt   SortField = "created_at"
	SortByPublishedAt SortField = "published_at"
	SortByReactions   SortField = "reactions"
	SortByZaps        SortField = "zaps"
	SortByReplies     SortField = "replies"
)

// SortOrder defines sort direction
type SortOrder string

const (
	SortAsc  SortOrder = "asc"
	SortDesc SortOrder = "desc"
)

// GroupField defines how to group events
type GroupField string

const (
	GroupNone     GroupField = ""
	GroupByDay    GroupField = "day"
	GroupByWeek   GroupField = "week"
	GroupByMonth  GroupField = "month"
	GroupByYear   GroupField = "year"
	GroupByAuthor GroupField = "author"
	GroupByKind   GroupField = "kind"
)

// Scope defines the author scope for filtering
type Scope string

const (
	ScopeSelf      Scope = "self"
	ScopeFollowing Scope = "following"
	ScopeMutual    Scope = "mutual"
	ScopeFoaf      Scope = "foaf"
	ScopeAll       Scope = "all"
)

// Page represents a paginated section result
type Page struct {
	Section    *Section
	Events     []*nostr.Event
	PageNumber int
	TotalPages int
	TotalItems int64
	HasNext    bool
	HasPrev    bool
}

// Manager manages sections and their content
type Manager struct {
	storage     *storage.Storage
	sections    map[string]*Section
	ownerPubkey string // canonical hex pubkey for owner (default author scope)
}

// NewManager creates a new section manager
func NewManager(st *storage.Storage, ownerPubkey string) *Manager {
	return &Manager{
		storage:     st,
		sections:    make(map[string]*Section),
		ownerPubkey: normalizeAuthorValue(ownerPubkey, ""),
	}
}

// RegisterSection registers a section definition
func (m *Manager) RegisterSection(section *Section) error {
	if section.Name == "" {
		return fmt.Errorf("section name is required")
	}

	if section.Limit == 0 {
		section.Limit = 20 // Default limit
	}

	m.sections[section.Name] = section
	return nil
}

// GetSection retrieves a section by name
func (m *Manager) GetSection(name string) (*Section, error) {
	section, exists := m.sections[name]
	if !exists {
		return nil, fmt.Errorf("section not found: %s", name)
	}
	return section, nil
}

// GetSectionByPath retrieves a section by its URL path (deprecated - use GetSectionsByPath for multiple sections)
func (m *Manager) GetSectionByPath(path string) (*Section, error) {
	for _, section := range m.sections {
		if section.Path == path {
			return section, nil
		}
	}
	return nil, fmt.Errorf("no section registered for path: %s", path)
}

// GetSectionsByPath retrieves all sections for a given path, sorted by Order field
func (m *Manager) GetSectionsByPath(path string) []*Section {
	var matched []*Section
	for _, section := range m.sections {
		if section.Path == path {
			matched = append(matched, section)
		}
	}

	// Sort by Order field (lower numbers first)
	for i := 0; i < len(matched)-1; i++ {
		for j := 0; j < len(matched)-i-1; j++ {
			if matched[j].Order > matched[j+1].Order {
				matched[j], matched[j+1] = matched[j+1], matched[j]
			}
		}
	}

	return matched
}

// ListSections returns all registered sections
func (m *Manager) ListSections() []*Section {
	sections := make([]*Section, 0, len(m.sections))
	for _, section := range m.sections {
		sections = append(sections, section)
	}
	return sections
}

// GetPage retrieves a page of events for a section
func (m *Manager) GetPage(ctx context.Context, sectionName string, pageNum int) (*Page, error) {
	section, err := m.GetSection(sectionName)
	if err != nil {
		return nil, err
	}

	if pageNum < 1 {
		pageNum = 1
	}

	// Build Nostr filter from section filters
	filter := m.buildFilter(section, pageNum)

	// Query events
	events, err := m.storage.QueryEvents(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to query events: %w", err)
	}

	// Apply reply/root filtering if configured
	events = m.applyIsReplyFilter(events, section.Filters.IsReply)

	// Sort events
	m.sortEvents(events, section.SortBy, section.SortOrder)

	// Calculate pagination
	offset := (pageNum - 1) * section.Limit
	totalItems := int64(len(events))
	totalPages := int((totalItems + int64(section.Limit) - 1) / int64(section.Limit))

	// Extract page events
	var pageEvents []*nostr.Event
	if offset < len(events) {
		end := offset + section.Limit
		if end > len(events) {
			end = len(events)
		}
		pageEvents = events[offset:end]
	}

	return &Page{
		Section:    section,
		Events:     pageEvents,
		PageNumber: pageNum,
		TotalPages: totalPages,
		TotalItems: totalItems,
		HasNext:    pageNum < totalPages,
		HasPrev:    pageNum > 1,
	}, nil
}

// buildFilter converts section filters to Nostr filter
func (m *Manager) buildFilter(section *Section, pageNum int) nostr.Filter {
	limit := section.Limit * pageNum // Get all up to this page
	// When post-filtering for replies/roots, fetch extra to avoid empty pages
	if section.Filters.IsReply != nil {
		limit = section.Limit * (pageNum + 2)
	}

	filter := nostr.Filter{
		Limit: limit,
	}

	if len(section.Filters.Kinds) > 0 {
		filter.Kinds = section.Filters.Kinds
	}

	authors := m.resolveAuthors(section.Filters.Authors, section.Filters.Scope)
	if len(authors) > 0 {
		filter.Authors = authors
	}

	if section.Filters.Since != nil {
		since := nostr.Timestamp(section.Filters.Since.Unix())
		filter.Since = &since
	}

	if section.Filters.Until != nil {
		until := nostr.Timestamp(section.Filters.Until.Unix())
		filter.Until = &until
	}

	// Add tag filters
	if len(section.Filters.Tags) > 0 {
		filter.Tags = make(nostr.TagMap)
		for key, values := range section.Filters.Tags {
			filter.Tags[key] = values
		}
	}

	return filter
}

// sortEvents sorts events based on field and order
func (m *Manager) sortEvents(events []*nostr.Event, field SortField, order SortOrder) {
	// Simple bubble sort for now (can optimize later)
	n := len(events)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-i-1; j++ {
			shouldSwap := false

			switch field {
			case SortByCreatedAt, SortByPublishedAt:
				if order == SortDesc {
					shouldSwap = events[j].CreatedAt < events[j+1].CreatedAt
				} else {
					shouldSwap = events[j].CreatedAt > events[j+1].CreatedAt
				}
				// For other sort fields, would need aggregate data
				// This is simplified for now
			}

			if shouldSwap {
				events[j], events[j+1] = events[j+1], events[j]
			}
		}
	}
}

// DefaultSections returns commonly used section definitions
func DefaultSections() []*Section {
	return []*Section{
		{
			Name:        "notes",
			Title:       "Notes",
			Description: "Recent short-form notes",
			Filters: FilterSet{
				Kinds: []int{1},
			},
			SortBy:      SortByCreatedAt,
			SortOrder:   SortDesc,
			Limit:       20,
			ShowDates:   true,
			ShowAuthors: true,
		},
		{
			Name:        "articles",
			Title:       "Articles",
			Description: "Long-form articles",
			Filters: FilterSet{
				Kinds: []int{30023},
			},
			SortBy:      SortByPublishedAt,
			SortOrder:   SortDesc,
			Limit:       10,
			ShowDates:   true,
			ShowAuthors: true,
		},
		{
			Name:        "reactions",
			Title:       "Reactions",
			Description: "Recent reactions and likes",
			Filters: FilterSet{
				Kinds: []int{7},
			},
			SortBy:      SortByCreatedAt,
			SortOrder:   SortDesc,
			Limit:       50,
			ShowDates:   true,
			ShowAuthors: true,
		},
		{
			Name:        "zaps",
			Title:       "Zaps",
			Description: "Recent zap receipts",
			Filters: FilterSet{
				Kinds: []int{9735},
			},
			SortBy:      SortByCreatedAt,
			SortOrder:   SortDesc,
			Limit:       20,
			ShowDates:   true,
			ShowAuthors: true,
		},
	}
}

// NOTE: "inbox" and "outbox" are INTERNAL source identifiers, not user-facing paths.
// The router already provides /notes, /replies, /mentions, /articles paths.
// Sections are for CUSTOM filtered views (e.g., /art, /dev, /following).
//
// These helper functions are deprecated and should not be used.
// They were based on a misunderstanding of the architecture.

// resolveAuthors returns normalized authors for a section, applying defaults:
// - Decode npub values to hex
// - Interpret "owner"/"self" as the owner's pubkey
// - If no authors are provided, default to owner unless scope=all
func (m *Manager) resolveAuthors(authors []string, scope Scope) []string {
	var normalized []string
	seen := make(map[string]struct{})

	for _, author := range authors {
		normalizedAuthor := normalizeAuthorValue(author, m.ownerPubkey)
		if normalizedAuthor == "" {
			continue
		}

		if _, exists := seen[normalizedAuthor]; exists {
			continue
		}
		seen[normalizedAuthor] = struct{}{}
		normalized = append(normalized, normalizedAuthor)
	}

	if len(normalized) > 0 {
		return normalized
	}

	if scope == ScopeAll {
		return nil
	}

	if m.ownerPubkey != "" {
		return []string{m.ownerPubkey}
	}

	return nil
}

func (m *Manager) applyIsReplyFilter(events []*nostr.Event, isReply *bool) []*nostr.Event {
	if isReply == nil {
		return events
	}

	filtered := make([]*nostr.Event, 0, len(events))
	for _, event := range events {
		info, err := aggregates.ParseThreadInfo(event)
		if err != nil {
			// Treat non-threadable kinds as roots
			if !*isReply {
				filtered = append(filtered, event)
			}
			continue
		}

		if info.IsReply() == *isReply {
			filtered = append(filtered, event)
		}
	}

	return filtered
}

// normalizeAuthorValue converts npub → hex and owner/self aliases to the owner pubkey
func normalizeAuthorValue(author string, ownerPubkey string) string {
	if author == "" {
		return ""
	}

	lower := strings.ToLower(author)
	if (lower == "owner" || lower == "self") && ownerPubkey != "" {
		return ownerPubkey
	}

	if strings.HasPrefix(author, "npub1") {
		if _, val, err := nip19.Decode(author); err == nil {
			if hex, ok := val.(string); ok {
				return hex
			}
		}
	}

	return author
}
