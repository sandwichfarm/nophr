Composable Layouts and Sections

Concept
- Sections are query-driven views over indexed data. Pages are composed of sections.
- Each section defines what to show (filters) and how to show it (template), plus archives and feeds.
- **Multiple sections can be registered for the same path** (e.g., homepage with multiple topic previews)
- Sections are **completely optional** - default routes work without configuration

Section fields (Current Implementation)
- Name: stable identifier (required)
- Path: URL path like "/diy" or "/" (required)
- Title: display name (optional)
- Description: section description (optional)
- Filters: FilterSet with kinds, authors, tags, time ranges (required)
- SortBy: created_at|published_at|reactions|zaps|replies (default: created_at)
- SortOrder: asc|desc (default: desc)
- Limit: max items to show (default: 20)
- ShowDates: display timestamps (default: false)
- ShowAuthors: display author info (default: false)
- GroupBy: none|day|week|month|year|author|kind (default: none)
- MoreLink: optional link to full paginated section (for previews)
- Order: display order when multiple sections on same path (lower first)

Section fields (Future/Planned)
- template: list|cards|threaded|gallery|table (pluggable) - NOT YET IMPLEMENTED
- archive: by month|year with route pattern - NOT YET IMPLEMENTED
- feeds: enable rss/json - NOT YET IMPLEMENTED
- hide_when_empty: true|false - NOT YET IMPLEMENTED

Filter spec (examples)
- kinds: [1, 30023, 7, 9735]
- authors: [owner] | following | mutual | foaf:2 | allowlist:[npub...] | denylist:[...]
- is_reply: true|false
- thread_of: <event id>
- replies_to: owner|section:<id>
- mentions: owner | [npub...]
- p_tags: [npub...]
- e_tags: [<event id>...]
- hashtag: ["art", "dev"]
- d_tag: ["slug"]
- lang: ["en"]
- has_reactions: true|false
- min_zap_sats: N
- reaction_chars: ["+"]
- since/until: timestamps; or last_days: N
- include_direct_mentions: true|false
- include_threads_of_mine: true|false

Current Implementation Status

Homepage (/ or empty selector):
- **Default behavior**: Auto-generated menu (gophermap/gemtext) with links to default routes
- **✅ Implemented**: Sections can override any path including "/"
- **✅ Implemented**: Multiple sections can be registered for the same path
- **✅ Implemented**: Sections are sorted by Order field (lower numbers first)

Default routes (hardcoded in router, work without sections):
- `/notes` - Owner's notes (kind 1, is_reply:false, limit:9, paginated)
- `/articles` - Owner's articles (kind 30023, limit:9, paginated)
- `/replies` - Replies to owner (kind 1, replies_to:owner, limit:9, paginated)
- `/mentions` - Mentions of owner (mentions:owner, limit:9, paginated)
- `/profile/<pubkey>` - Profile display (kind 0)
- `/note/<id>` - Individual note display
- `/diagnostics` - System status

Sections System (Optional Customization):
- **✅ Sections override default routes** - If section registered at path, it takes precedence
- **✅ Multiple sections per path** - Compose homepage with multiple topic previews
- **✅ Section ordering** - Control display order with Order field
- **✅ "More" links** - Preview sections can link to full paginated views
- **✅ Completely optional** - Default routes work without any section configuration

IMPORTANT: "inbox" and "outbox" naming
- Historically these terms refer to relay selection strategy (inbox relays vs outbox relays).
- Current implementation exposes legacy endpoints:
  - `/inbox` (legacy) maps to Replies (redirects/forwards to `/replies`).
  - `/outbox` (legacy) lists the operator's notes (owner’s posts).
- Prefer the canonical routes `/notes` and `/replies` for new configurations. Do not create sections named "inbox" or "outbox".

Example 1: Go Code (Current Implementation)

```go
// Homepage with multiple topic previews
sectionManager.RegisterSection(&sections.Section{
    Name:  "diy-preview",
    Path:  "/",
    Title: "Recent DIY Projects",
    Order: 1,  // Show first
    Limit: 5,
    Filters: sections.FilterSet{
        Tags: map[string][]string{"t": {"diy"}},
    },
    ShowDates:   true,
    ShowAuthors: true,
    MoreLink: &sections.MoreLink{
        Text:       "More DIY posts",
        SectionRef: "diy-full",
    },
})

sectionManager.RegisterSection(&sections.Section{
    Name:  "philosophy-preview",
    Path:  "/",
    Title: "Recent Philosophy",
    Order: 2,  // Show second
    Limit: 5,
    Filters: sections.FilterSet{
        Tags: map[string][]string{"t": {"philosophy"}},
    },
    ShowDates: true,
    MoreLink: &sections.MoreLink{
        Text:       "More philosophy posts",
        SectionRef: "philosophy-full",
    },
})

// Full sections (paginated)
sectionManager.RegisterSection(&sections.Section{
    Name:  "diy-full",
    Path:  "/diy",
    Title: "All DIY Projects",
    Limit: 9,  // Gopher pagination limit
    Filters: sections.FilterSet{
        Tags: map[string][]string{"t": {"diy"}},
    },
    ShowDates:   true,
    ShowAuthors: true,
})

sectionManager.RegisterSection(&sections.Section{
    Name:  "philosophy-full",
    Path:  "/philosophy",
    Title: "All Philosophy Posts",
    Limit: 9,
    Filters: sections.FilterSet{
        Tags: map[string][]string{"t": {"philosophy"}},
    },
    ShowDates: true,
})
```

Example 2: YAML Configuration (IMPLEMENTED)

```yaml
# Sections can be configured via YAML in your nophr config file
sections:
  diy-preview:
    path: "/"
    title: "Recent DIY Projects"
    order: 1
    limit: 5
    filters:
      tags:
        t: ["diy"]
    show_dates: true
    show_authors: true
    more_link:
      text: "More DIY posts"
      section_ref: "diy-full"

  philosophy-preview:
    path: "/"
    title: "Recent Philosophy"
    order: 2
    limit: 5
    filters:
      tags:
        t: ["philosophy"]
    show_dates: true
    more_link:
      text: "More philosophy posts"
      section_ref: "philosophy-full"

  diy-full:
    path: "/diy"
    title: "All DIY Projects"
    limit: 9
    filters:
      tags:
        t: ["diy"]
    show_dates: true
    show_authors: true

  philosophy-full:
    path: "/philosophy"
    title: "All Philosophy Posts"
    limit: 9
    filters:
      tags:
        t: ["philosophy"]
    show_dates: true
```

Implementation Notes

✅ **Currently Implemented**:
- Sections can override any path (including `/`)
- Multiple sections per path supported
- Section ordering via Order field
- "More" links for preview → full view navigation
- Sections are completely optional
- Default routes work without configuration
- FilterSet supports kinds, authors, tags, time ranges
- `is_reply` filter to explicitly include or exclude replies
- **Full protocol parity**: Gopher and Gemini both support sections
- **YAML configuration**: Sections can be configured via YAML (no Go code required)
- **Owner default context**: If no authors are provided, sections default to the operator’s canonical pubkey (decoded from `identity.npub`). Set `scope: all` to disable the default. Authors accept `npub` or hex, and the literal `owner`/`self` aliases resolve to the operator.

🚧 **Not Yet Implemented** (Future):
- Template system (list, cards, threaded, etc.)
- Archive generation (by month/year)
- Feed generation (RSS/JSON)
- hide_when_empty field
- Scope-based filtering (following, mutual, foaf)
- Advanced filter fields (mentions, etc.)

📝 **Usage Pattern**:
1. Define sections in YAML config file (or register in Go code for advanced use cases)
2. Sections with same Path are rendered together, sorted by Order
3. Preview sections use small Limit (3-5) and MoreLink
4. Full sections use larger Limit (9 for Gopher pagination)
5. Sections override default routes if registered at that path
6. If no sections registered, default routes are used
