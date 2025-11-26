Protocol Rendering

Gopher Server (RFC 1436)
- Serves on port 70; responds to selectors with gophermaps or text files.
- Homepage (selector "/" or empty): fully configurable via layout.pages.home or layout.sections
  - Default: Auto-generated gophermap menu with links to all sections
  - Customizable: Can show composed sections, single section, or custom content
- Default sections (all configurable via layout.sections):
  - /notes - Owner's notes (kind 1, non-replies)
  - /articles - Owner's long-form articles (kind 30023)
  - /replies - Replies to owner's content
  - /mentions - Posts mentioning the owner
  - /archive - Time-based archives (by year/month)
  - /about - Owner profile (kind 0)
  - /diagnostics - System status and statistics
- Per-section views: gophermap with item type '0' (text) or '1' (submenu) for each event.
- List item rendering: Each event shows title/preview with aggregate stats (reply count, reaction total, cumulative zap sats).
  - Format: "X replies, Y reactions, Z sats zapped" (shown when interactions > 0)
  - Aggregates retrieved from cached aggregates table for performance
- Text rendering: converts Nostr event content to wrapped plain text; shows metadata (author, timestamp, reactions/zaps).
- Thread navigation: parent/replies linked via selectors; indented display.
- Event detail: /event/<id> shows full event with:
  - Full event content (rendered as plain text)
  - Aggregate stats footer: reply count, reaction total (with breakdown by emoji/char if >1 type), cumulative zap sats
  - Threaded replies: Full thread tree with indentation, each reply showing its own aggregates
  - Navigation: Links to parent event, individual reply events
  - NIP-19 entities resolved inline with display names and portal references appended (njump.me, nostr.at, nostr.eu)
- Archives: gophermap by year/month with links to individual posts.
- Diagnostics: text file showing relay status, sync cursors, author counts.

Note: "inbox" and "outbox" are internal concepts for data organization, not exposed as paths.

Aggregate Rendering Details (applies to both Gopher and Gemini):
- List views: Compact format - "X replies, Y reactions, Z sats zapped" (only shown if interactions > 0)
- Detail views: Expanded format with breakdown:
  - Reply count with link to view all replies in thread
  - Reaction breakdown: If multiple reaction types, show each (e.g., "👍 5, ❤️ 3, 🔥 2" or "+ 5, heart 3")
  - Cumulative zap total with formatted sats (e.g., "21,000 sats" or "21k sats")
- Interactive expansion: Not supported by Gopher/Gemini protocols - all details shown inline in detail view
- Threading: Replies are fully expanded in detail view, indented for hierarchy, each with own aggregates

Gemini Server (gemini://)
- Serves on port 1965 with TLS (self-signed or custom cert).
- Homepage (gemini://host/ or gemini://host): fully configurable via layout.pages.home or layout.sections
  - Default: Auto-generated gemtext with links to all sections
  - Customizable: Can show composed sections, single section, or custom content
- Default sections (same as Gopher, all configurable via layout.sections):
  - /notes - Owner's notes (kind 1, non-replies)
  - /articles - Owner's long-form articles (kind 30023)
  - /replies - Replies to owner's content
  - /mentions - Posts mentioning the owner
  - /archive - Time-based archives (by year/month)
  - /about - Owner profile (kind 0)
  - /diagnostics - System status and statistics
- Per-section views: gemtext document with event links (=> /event/<id>).
- List item rendering: Each event link shows preview with aggregate stats (reply count, reaction total, cumulative zap sats).
  - Format: "X replies, Y reactions, Z sats" (inline with event preview)
  - Aggregates retrieved from cached aggregates table for performance
- Event rendering: gemtext formatting with headings, quotes, preformatted blocks; reactions/zaps shown as text.
- Event detail: gemini://host/event/<id> shows full event with:
  - Full event content (rendered as gemtext)
  - Aggregate stats section: reply count, reaction total (with breakdown by emoji if >1 type), cumulative zap sats
  - Threaded replies: Full thread tree with links, each reply showing its own aggregates
  - Navigation: Links to parent event, individual reply events
  - NIP-19 entities resolved inline with portal links appended (njump.me, nostr.at, nostr.eu) for quick off-site navigation
- Thread navigation: links to parent and child replies.
- Input support: search queries, filter selection via Gemini input (status 10).
- Archives: gemtext index by year/month with links.
- Diagnostics: gemtext page with relay/sync status.

Note: "inbox" and "outbox" are internal concepts for data organization, not exposed as paths.

Finger Server (RFC 742)
- Serves on port 79; responds to finger queries.
- Query format: "npub@host" or "username@host" (maps to followed users).
- Response: plain text with owner profile (from kind 0), .plan field (about/bio), recent notes (last 5 kind 1 events).
- Limited to owner + top N followed users (configured via protocols.finger.max_users).
- Shows interaction counts (followers, following, recent zaps/reactions if available).

Content Transformation
- Nostr events → Gopher text: convert markdown to plain text with configurable formatting (see markdown_conversion.md).
  - Headings: underline or uppercase style
  - Bold: UPPERCASE or **preserve asterisks**
  - Links: "text <url>" or "text (url)" format
  - Code blocks: indent or wrap with separators
  - Line wrapping at 70 chars (configurable)
- Nostr events → Gemtext: convert markdown to gemtext format (see markdown_conversion.md).
  - Map headings: # ## ### (flatten deeper levels)
  - Extract inline links to separate => lines
  - Convert ordered lists to unordered (* 1. item)
  - Preserve code blocks and quotes
  - Optional line wrapping at configured max_line_length (default 80) to clamp long-form article width
- Nostr events → Finger response: strip all markdown, compact format with timestamps and summaries.
  - Remove all formatting syntax
  - Truncate to max length (default 500 chars)
  - Preserve bare URLs optionally

See markdown_conversion.md for detailed conversion rules and configuration options.

Caching
- Cache rendered gophermaps, gemtext pages, and finger responses per TTL.
- Invalidate on new events, profile updates, or interaction changes.
- Serve stale content if sync is temporarily unavailable.
