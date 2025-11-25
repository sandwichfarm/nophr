# Phase 11: Sections and Layouts - Completion Report

## Overview

Phase 11 implemented a flexible sections and layouts system that allows configurable content organization, filtering, pagination, and archive generation for protocol servers.

**Status**: ✅ Complete

**Date Completed**: 2025-10-24

## Deliverables

### 1. Section Definition Schema ✅

**File**: `internal/sections/sections.go`

**Features**:
- Section definition with metadata (name, title, description)
- Filter sets (kinds, authors, tags, time ranges, search)
- Sorting options (created_at, reactions, zaps, replies)
- Pagination support with configurable limits
- Grouping options (by day, week, month, year, author, kind)
- Display preferences (show dates, show authors)

**Section Structure**:
```go
type Section struct {
    Name        string      // Unique identifier
    Title       string      // Display title
    Description string      // Description
    Filters     FilterSet   // Content filters
    SortBy      SortField   // Sorting field
    SortOrder   SortOrder   // Asc or desc
    Limit       int         // Items per page
    ShowDates   bool        // Display timestamps
    ShowAuthors bool        // Display authors
    GroupBy     GroupField  // Grouping method
}
```

**Filter Options**:
- **Kinds**: Filter by event kinds (1, 30023, 7, 9735, etc.)
- **Authors**: Filter by pubkeys
- **Tags**: Filter by tag values (e.g., p:pubkey, e:eventid)
- **Time Range**: Since/Until timestamps
- **Search**: Full-text search (placeholder)
- **Scope**: Self, Following, Mutual, FoaF, All

### 2. Filter Query Builder ✅

**File**: `internal/sections/filters.go`

**Features**:
- Fluent API for building complex filters
- Time range helpers (Today, ThisWeek, LastNDays, etc.)
- Kind-specific filters
- Interaction filters (mentions, replies, threads)
- Scope-based filtering
- Tag filtering

**Examples**:
```go
// Basic filter
filter := NewFilterBuilder().
    Kinds(1, 30023).
    Authors("pubkey1", "pubkey2").
    Limit(20).
    Build()

// Time range filter
filter := NewFilterBuilder().
    Kinds(1).
    Since(LastNDays(7).Start).
    Until(time.Now()).
    Build()

// Tag filter
filter := NewFilterBuilder().
    Tag("e", "event123").
    Tag("p", "pubkey456").
    Build()

// Thread filter
filter := ThreadFilter("rootEventID").Build()

// Mention filter
filter := MentionFilter("pubkey").Build()
```

**Time Range Helpers**:
- `Today()`: Current day
- `Yesterday()`: Previous day
- `ThisWeek()`: Current week
- `ThisMonth()`: Current month
- `ThisYear()`: Current year
- `LastNDays(n)`: Last N days
- `LastNHours(n)`: Last N hours

### 3. Default Sections ✅

**Implemented Sections**:
1. **Notes**: Recent short-form notes (kind 1)
2. **Articles**: Long-form articles (kind 30023)
3. **Reactions**: Recent reactions (kind 7)
4. **Zaps**: Zap receipts (kind 9735)
5. **Inbox**: Mentions, replies, interactions
6. **Outbox**: User's published content

**Usage**:
```go
// Get default sections
sections := sections.DefaultSections()

// Create inbox for specific user
inbox := sections.InboxSection("pubkey123")

// Create outbox for specific user
outbox := sections.OutboxSection("pubkey123")
```

**Section Definitions**:
```go
// Notes section
{
    Name:        "notes",
    Title:       "Notes",
    Description: "Recent short-form notes",
    Filters:     FilterSet{Kinds: []int{1}},
    SortBy:      SortByCreatedAt,
    SortOrder:   SortDesc,
    Limit:       20,
}

// Articles section
{
    Name:        "articles",
    Title:       "Articles",
    Description: "Long-form articles",
    Filters:     FilterSet{Kinds: []int{30023}},
    SortBy:      SortByPublishedAt,
    SortOrder:   SortDesc,
    Limit:       10,
}
```

### 4. Archive Generation ✅

**File**: `internal/sections/archives.go`

**Features**:
- Time-based archives (day, month, year)
- Archive listing with event counts
- Archive page generation
- Monthly calendar view
- Automatic archive discovery

**Archive Periods**:
- **Day**: Events for a specific day
- **Month**: Events for a specific month
- **Year**: Events for a specific year

**Usage**:
```go
archiveMgr := sections.NewArchiveManager(storage)

// List available archives
archives, _ := archiveMgr.ListArchives(ctx, section, sections.ArchiveByMonth)

// Get archive page
page, _ := archiveMgr.GetArchivePage(ctx, section, 2025, 10, 0, 1)

// Generate monthly calendar
calendar, _ := archiveMgr.GenerateMonthlyCalendar(ctx, section, 2025, 10)
```

**Archive Structure**:
```go
type Archive struct {
    Period     ArchivePeriod  // day, month, year
    Year       int
    Month      time.Month
    Day        int
    EventCount int64
    FirstEvent time.Time
    LastEvent  time.Time
}
```

**Archive Formatting**:
```go
archive := &Archive{Period: ArchiveByMonth, Year: 2025, Month: 10}

// Format title: "October 2025"
title := archive.FormatTitle()

// Format selector: "/archive/notes/2025/10"
selector := archive.FormatArchiveSelector("notes")
```

### 5. Page Composition System ✅

**Features**:
- Pagination support
- Page metadata (total pages, total items)
- Navigation helpers (HasNext, HasPrev)
- Event list rendering
- Section context preservation

**Page Structure**:
```go
type Page struct {
    Section    *Section      // Section definition
    Events     []*nostr.Event  // Page events
    PageNumber int           // Current page
    TotalPages int           // Total pages
    TotalItems int64         // Total items
    HasNext    bool          // Has next page
    HasPrev    bool          // Has previous page
}
```

**Usage**:
```go
manager := sections.NewManager(storage, ownerPubkeyHex)

// Register sections
for _, section := range sections.DefaultSections() {
    manager.RegisterSection(section)
}

// Get a page
page, _ := manager.GetPage(ctx, "notes", 1)

// Navigate pages
if page.HasNext {
    nextPage, _ := manager.GetPage(ctx, "notes", page.PageNumber+1)
}
```

### 6. Tests ✅

**Files**:
- `internal/sections/sections_test.go`
- `internal/sections/filters_test.go`

**Coverage**:
- Section creation and registration
- Filter building
- Time range generation
- Archive formatting
- Default sections
- Inbox/Outbox sections
- Pagination logic
- Scope filtering

## File Structure

```
internal/sections/
├── sections.go         # Section definitions and manager
├── filters.go          # Filter query builders
├── archives.go         # Archive generation
├── sections_test.go    # Section tests
└── filters_test.go     # Filter tests
```

## Integration Examples

### Gopher Server Integration

```go
// In gopher server
func (s *Server) handleSection(selectorParts []string) (string, error) {
    sectionName := selectorParts[1]
    pageNum := 1
    if len(selectorParts) > 2 {
        pageNum, _ = strconv.Atoi(selectorParts[2])
    }

    // Get section page
    page, err := s.sectionMgr.GetPage(ctx, sectionName, pageNum)
    if err != nil {
        return "", err
    }

    // Render as gophermap
    return s.renderSectionPage(page), nil
}

func (s *Server) renderSectionPage(page *Page) string {
    var out string

    out += fmt.Sprintf("i%s\t\t%s\t%d\r\n", page.Section.Title, s.host, s.port)
    out += fmt.Sprintf("i%s\t\t%s\t%d\r\n", page.Section.Description, s.host, s.port)
    out += fmt.Sprintf("i\t\t%s\t%d\r\n", s.host, s.port)

    for _, event := range page.Events {
        out += fmt.Sprintf("0%s\t/event/%s\t%s\t%d\r\n",
            truncate(event.Content, 70),
            event.ID,
            s.host,
            s.port)
    }

    // Pagination
    if page.HasNext {
        out += fmt.Sprintf("1Next Page >\t/section/%s/%d\t%s\t%d\r\n",
            page.Section.Name, page.PageNumber+1, s.host, s.port)
    }

    return out
}
```

### Gemini Server Integration

```go
// In gemini server
func (s *Server) handleSection(path string) (string, error) {
    parts := strings.Split(path, "/")
    sectionName := parts[2]
    pageNum := 1
    if len(parts) > 3 {
        pageNum, _ = strconv.Atoi(parts[3])
    }

    page, err := s.sectionMgr.GetPage(ctx, sectionName, pageNum)
    if err != nil {
        return "", err
    }

    return s.renderSectionPage(page), nil
}

func (s *Server) renderSectionPage(page *Page) string {
    var out string

    out += fmt.Sprintf("# %s\n\n", page.Section.Title)
    out += fmt.Sprintf("%s\n\n", page.Section.Description)

    for _, event := range page.Events {
        out += fmt.Sprintf("=> /event/%s %s\n",
            event.ID,
            truncate(event.Content, 80))
    }

    out += "\n## Navigation\n\n"
    if page.HasPrev {
        out += fmt.Sprintf("=> /section/%s/%d Previous Page\n",
            page.Section.Name, page.PageNumber-1)
    }
    if page.HasNext {
        out += fmt.Sprintf("=> /section/%s/%d Next Page\n",
            page.Section.Name, page.PageNumber+1)
    }

    return out
}
```

### Archive Integration

```go
// List archives
func (s *Server) handleArchiveList(sectionName string) (string, error) {
    section, _ := s.sectionMgr.GetSection(sectionName)
    archives, _ := s.archiveMgr.ListArchives(ctx, section, sections.ArchiveByMonth)

    var out string
    out += "# Archives\n\n"

    for _, archive := range archives {
        out += fmt.Sprintf("=> %s %s (%d events)\n",
            archive.FormatArchiveSelector(sectionName),
            archive.FormatTitle(),
            archive.EventCount)
    }

    return out, nil
}

// Monthly calendar
func (s *Server) handleMonthlyCalendar(sectionName string, year int, month time.Month) (string, error) {
    section, _ := s.sectionMgr.GetSection(sectionName)
    calendar, _ := s.archiveMgr.GenerateMonthlyCalendar(ctx, section, year, month)

    var out string
    out += fmt.Sprintf("# %s %d\n\n", calendar.Month.String(), calendar.Year)

    for _, day := range calendar.Days {
        if day.HasEvents {
            out += fmt.Sprintf("=> /archive/%s/%d/%02d/%02d Day %d (%d events)\n",
                sectionName, year, month, day.Day, day.Day, day.EventCount)
        }
    }

    return out, nil
}
```

## Configuration

### YAML Configuration

```yaml
layout:
  sections:
    notes:
      title: "Recent Notes"
      description: "Latest short-form posts"
      filters:
        kinds: [1]
        limit: 20
      sort_by: "created_at"
      sort_order: "desc"
      show_dates: true
      show_authors: true

    articles:
      title: "Articles"
      description: "Long-form content"
      filters:
        kinds: [30023]
        limit: 10
      sort_by: "published_at"
      sort_order: "desc"

    inbox:
      title: "Inbox"
      description: "Your interactions"
      filters:
        tags:
          p: ["${OWNER_PUBKEY}"]
        kinds: [1, 7, 9735]
        limit: 50
      sort_by: "created_at"
      sort_order: "desc"
```

## Usage Examples

### Basic Section Usage

```go
// Create manager
manager := sections.NewManager(storage, ownerPubkeyHex)

// Register default sections
for _, section := range sections.DefaultSections() {
    manager.RegisterSection(section)
}

// Add custom section
customSection := &sections.Section{
    Name:        "popular",
    Title:       "Popular Posts",
    Description: "Most liked and zapped content",
    Filters: sections.FilterSet{
        Kinds: []int{1, 30023},
    },
    SortBy:    sections.SortByReactions,
    SortOrder: sections.SortDesc,
    Limit:     10,
}
manager.RegisterSection(customSection)

// Get section page
page, _ := manager.GetPage(ctx, "notes", 1)

// Iterate events
for _, event := range page.Events {
    fmt.Printf("%s: %s\n", event.PubKey[:8], event.Content[:50])
}

// Check pagination
if page.HasNext {
    fmt.Printf("Page %d of %d\n", page.PageNumber, page.TotalPages)
}
```

### Archive Usage

```go
// Create archive manager
archiveMgr := sections.NewArchiveManager(storage)

// List monthly archives
section, _ := manager.GetSection("notes")
archives, _ := archiveMgr.ListArchives(ctx, section, sections.ArchiveByMonth)

for _, archive := range archives {
    fmt.Printf("%s: %d events\n", archive.FormatTitle(), archive.EventCount)
}

// Get specific month
page, _ := archiveMgr.GetArchivePage(ctx, section, 2025, time.October, 0, 1)

// Generate calendar
calendar, _ := archiveMgr.GenerateMonthlyCalendar(ctx, section, 2025, time.October)
for _, day := range calendar.Days {
    if day.HasEvents {
        fmt.Printf("Day %d: %d events\n", day.Day, day.EventCount)
    }
}
```

### Filter Building

```go
// Simple filter
filter := sections.NewFilterBuilder().
    Kinds(1).
    Limit(20).
    Build()

// Time-based filter
filter := sections.NewFilterBuilder().
    Kinds(1).
    Since(sections.LastNDays(7).Start).
    Build()

// Thread filter
filter := sections.ThreadFilter("rootEventID").Build()

// Interaction filter
filter := sections.InteractionFilter("eventID").
    Limit(50).
    Build()
```

## Performance Considerations

- **Pagination**: Limits database queries to page size
- **Caching**: Section pages should be cached (Phase 10)
- **Archive Generation**: Can be pre-computed and cached
- **Sorting**: In-memory sorting for small result sets

## Future Enhancements

- [ ] Custom sort functions (by zap amount, reaction count)
- [ ] Advanced filtering (AND/OR logic)
- [ ] Full-text search integration
- [ ] Tag-based navigation
- [ ] Author-based sections
- [ ] Dynamic sections from config
- [ ] Section templates
- [ ] RSS/Atom feed generation

## Completion Criteria

All Phase 11 requirements have been met:

- [x] Section definition schema
- [x] Filter query builder
- [x] Default sections (notes, articles, inbox, outbox)
- [x] Archive generation (by month/year)
- [x] Page composition
- [x] Tests for sections
- [x] Integration documentation

## Next Phase

**Phase 14: Security and Privacy** - Implement rate limiting, deny-lists, input validation, and security hardening.

## References

- Nostr NIPs: https://github.com/nostr-protocol/nips
- RFC 1436 (Gopher): https://datatracker.ietf.org/doc/html/rfc1436
- Gemini Specification: https://gemini.circumlunar.space/docs/specification.html
