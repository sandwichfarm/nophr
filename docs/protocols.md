# Protocol Servers Guide

Complete guide to nophr's protocol servers: Gopher, Gemini, and Finger.

## Overview

nophr serves your Nostr content via three legacy internet protocols:

| Protocol | Port | TLS | RFC | Purpose |
|----------|------|-----|-----|---------|
| **Gopher** | 70 | No | RFC 1436 | Menu-driven text interface |
| **Gemini** | 1965 | Yes | gemini:// | Modern minimalist web |
| **Finger** | 79 | No | RFC 742/1288 | User information queries |

All three protocols can run simultaneously, serving the same content with protocol-specific rendering.

---

## Table of Contents

- [Gopher](#gopher) - Menu-driven text protocol
- [Gemini](#gemini) - Modern minimalist protocol with TLS
- [Finger](#finger) - User query protocol
- [Common Features](#common-features) - Shared across all protocols
- [Testing](#testing) - How to test each protocol

---

## Gopher

**Port:** 70 (TCP)
**RFC:** [RFC 1436](https://www.rfc-editor.org/rfc/rfc1436.html)
Gopher is a menu-driven protocol from 1991. Content is served as either **gophermaps** (menus) or **text files**.

### Configuration

```yaml
protocols:
  gopher:
    enabled: true
    host: "gopher.example.com"  # Hostname for gopher:// URLs
    port: 70                     # Standard Gopher port
    bind: "0.0.0.0"              # Bind to all interfaces
```

**Port 70 requires root/sudo.** See [Deployment](deployment.md#port-binding) for non-root options.

### Rendering Options

```yaml
rendering:
  gopher:
    max_line_length: 70          # Wrap text at N chars
    show_timestamps: true         # Show event timestamps
    date_format: "2006-01-02 15:04 MST"  # Go time format
    thread_indent: "  "           # Indent for replies
```

**Conventions:**
- 70 characters per line (classic terminal width)
- Primarily ASCII text; Nostr content may include UTF‑8 characters or emoji
- Minimal formatting

### Selectors

Gopher uses "selectors" (paths) to navigate content:

| Selector | Description |
|----------|-------------|
| `/` | Main menu |
| `/notes` | Notes (kind 1, non-replies) |
| `/articles` | Long-form articles (kind 30023) |
| `/replies` | Replies to your content |
| `/mentions` | Posts mentioning you |
| `/search` | Search interface |
| `/search/<query>` | Search results (NIP-50) |
| `/note/<id>` | Individual note/article detail |
| `/thread/<id>` | Thread view |
| `/diagnostics` | System status and statistics |
| `/<custom>` | Custom sections (configured in `sections` config) |

**Legacy selectors** (aliases for compatibility):
| `/inbox` | → `/replies` (backwards compatibility) |
| `/outbox` | alias for `/notes` (backwards compatibility) |

### Gophermap Format

Gophermaps are menus with item types:

```
iWelcome to My Nostr Gopherhole	fake	example.com	70
i	fake	example.com	70
1Notes	/notes	example.com	70
1Articles	/articles	example.com	70
1Replies	/replies	example.com	70
1Mentions	/mentions	example.com	70
1Search	/search	example.com	70
.
```

**Item types:**
- `i` - Informational text (non-clickable)
- `0` - Text file
- `1` - Submenu/directory
- `3` - Error

### Clients

**Command line:**
```bash
# Raw telnet (manual)
telnet gopher.example.com 70
# Type selector and hit enter
/

# Using netcat
echo "/" | nc gopher.example.com 70
```

**Gopher clients:**
- **lynx** - Terminal web browser with Gopher support
  ```bash
  lynx gopher://gopher.example.com
  ```
- **VF-1** - Python Gopher client
  ```bash
  vf1 gopher://gopher.example.com
  ```
- **Bombadillo** - Terminal browser (Gopher, Gemini, Finger)
  ```bash
  bombadillo gopher://gopher.example.com
  ```
- **Lagrange** - GUI client (Gemini + Gopher)

### Example Session

```bash
$ echo "/" | nc localhost 70
iWelcome to Alice's Nostr Archive	fake	localhost	70
i	fake	localhost	70
1Notes (42 items)	/notes	localhost	70
1Articles (7 items)	/articles	localhost	70
1Replies (8 items)	/replies	localhost	70
1Mentions (15 items)	/mentions	localhost	70
1Search	/search	localhost	70
1Archive	/archive	localhost	70   # Example custom section (if configured)
iDiagnostics	/diagnostics	localhost	70
.

$ echo "/notes" | nc localhost 70
1[2025-10-24] Just published my Gopher server!	/note/abc123	localhost	70
1[2025-10-23] Testing markdown conversion	/note/def456	localhost	70
...

$ echo "/search/nostr+protocol" | nc localhost 70
iSearch Results: "nostr protocol"	fake	localhost	70
i==============	fake	localhost	70
i	fake	localhost	70
iFound 5 results:	fake	localhost	70
i	fake	localhost	70
0[Note] Introduction to the Nostr protocol	/note/xyz789	localhost	70
0[Article] Understanding Nostr relays	/note/abc456	localhost	70
...
```

---

## Gemini

**Port:** 1965 (TLS)
**Spec:** [gemini://geminiprotocol.net](gemini://geminiprotocol.net)
Gemini is a modern minimalist protocol (2019) serving content as **gemtext** (lightweight markup).

### Configuration

```yaml
protocols:
  gemini:
    enabled: true
    host: "gemini.example.com"
    port: 1965                    # Standard Gemini port
    bind: "0.0.0.0"
    tls:
      cert_path: "./certs/cert.pem"
      key_path: "./certs/key.pem"
      auto_generate: true          # Generate self-signed if missing
```

### TLS Certificates

**Auto-generate (self-signed):**

If `auto_generate: true` and cert files missing, nophr creates a self-signed certificate.

**Manual generation:**
```bash
mkdir -p certs
openssl req -x509 -newkey rsa:4096 -keyout certs/key.pem \
  -out certs/cert.pem -days 365 -nodes \
  -subj "/CN=gemini.example.com"
```

**Production (Let's Encrypt):**
```bash
# Use certbot or acme.sh
certbot certonly --standalone -d gemini.example.com
# Copy certs
cp /etc/letsencrypt/live/gemini.example.com/fullchain.pem certs/cert.pem
cp /etc/letsencrypt/live/gemini.example.com/privkey.pem certs/key.pem
```

**TOFU (Trust On First Use):**

Gemini clients use TOFU certificate validation. First connection stores certificate fingerprint; subsequent connections verify against stored fingerprint.

### Rendering Options

```yaml
rendering:
  gemini:
    max_line_length: 80           # Wrap at N chars (optional)
    show_timestamps: true          # Show event timestamps
    emoji: true                    # Allow emoji in gemtext
```

**Conventions:**
- UTF-8 text (emoji OK)
- Gemtext markup (headings, links, quotes, preformatted)
- Max line ~1024 chars (but 80 recommended for readability)

### URL Paths

| Path | Description |
|------|-------------|
| `/` | Home page |
| `/notes` | Notes (kind 1, non-replies) |
| `/articles` | Long-form articles (kind 30023) |
| `/replies` | Replies to your content |
| `/mentions` | Posts mentioning you |
| `/search` | Search interface (prompts for query) |
| `/note/<id>` | Individual note/article detail |
| `/thread/<id>` | Thread view |
| `/diagnostics` | System status and statistics |
| `/about` | Your profile (kind 0) |
| `/<custom>` | Custom sections (configured in `sections` config) |

**Legacy paths** (aliases for compatibility):
| `/inbox` | → `/replies` (backwards compatibility) |
| `/outbox` | alias for `/notes` (backwards compatibility) |

### Gemtext Format

Gemtext is line-oriented:

```gemtext
# Heading 1
## Heading 2
### Heading 3

Normal text paragraph.

* Bullet list item
* Another item

> Quote block

=> /notes Notes section
=> gemini://other.site External link

\`\`\`
Preformatted code block
\`\`\`
```

### Clients

**Command line:**
```bash
# OpenSSL (raw)
openssl s_client -connect gemini.example.com:1965 -quiet
gemini://gemini.example.com/
```

**Gemini clients:**
- **amfora** - Terminal Gemini browser
  ```bash
  amfora gemini://gemini.example.com
  ```
- **Lagrange** - GUI Gemini/Gopher client
- **Bombadillo** - Terminal multi-protocol browser
- **Kristall** - Qt-based GUI browser

### Input Queries

Gemini supports input via status code `10`:

**Response:** `10 Enter search query\r\n`

Client prompts user for input, then sends:
```
gemini://gemini.example.com/search?query%20text
```

**Use cases:**
- Search notes
- Filter by tag
- Select date range

### Example Session

```bash
$ amfora gemini://localhost

# Welcome to Alice's Nostr Archive

Notes, articles, and interactions from Nostr

=> /notes Notes (42 items)
=> /articles Articles (7 items)
=> /replies Replies (8 items)
=> /mentions Mentions (15 items)
=> /search Search
=> /archive Archive

# Click /notes

## Notes

=> /note/abc123 [2025-10-24] Just published my Gopher server!
=> /note/def456 [2025-10-23] Testing markdown conversion
...

# Click /search
# Client prompts: "Enter search query:"
# User types: "nostr protocol"

# Search Results

Query: "nostr protocol"

Found 5 results:

=> /profile/xyz123 [Profile] alice
=> /note/abc456 [Note] Introduction to the Nostr protocol
=> /note/def789 [Article] Understanding Nostr relays

=> /search New Search
=> / Back to Home
```

---

## Finger

**Port:** 79 (TCP)
**RFC:** [RFC 742](https://www.rfc-editor.org/rfc/rfc742.html) / [RFC 1288](https://www.rfc-editor.org/rfc/rfc1288.html)
Finger returns user information for a given query.

### Configuration

```yaml
protocols:
  finger:
    enabled: true
    port: 79                      # Standard Finger port
    bind: "0.0.0.0"
    max_users: 100                # Owner + top N followed users
```

**Port 79 requires root/sudo.** See [Deployment](deployment.md#port-binding).

### Rendering Options

```yaml
rendering:
  finger:
    plan_source: "kind_0"         # Use profile about field as .plan
    recent_notes_count: 5         # Show last N notes
```

**Plan source:**
- `kind_0` - Use profile "about" field as .plan
- `kind_1` - Use most recent note as .plan

### Query Format

```bash
finger [username]@host
```

**Username mapping:**
- Owner npub/npub alias
- Followed user npub
- Nostr display name (if unique)

**Examples:**
```bash
finger @gopher.example.com           # Owner info
finger npub1abc@gopher.example.com   # Specific user (hex or npub)
finger alice@gopher.example.com      # By display name
```

### Response Format

```
Login: alice                Name: Alice
Directory: /home/alice      Shell: /bin/bash

Plan:
I'm a Nostr enthusiast building a Gopher gateway. Find me on Nostr!

Recent notes:
  [2025-10-24 12:34] Just published my Gopher server! Check it out.
  [2025-10-23 09:12] Testing markdown conversion for protocols.
  [2025-10-22 15:47] Exploring old-school internet protocols.

Followers: 142  Following: 87  Mutual: 56
Recent zaps: 21,000 sats  Reactions: 342
```

### Clients

**Command line:**
```bash
# Standard finger client
finger @gopher.example.com

# Telnet (manual)
telnet gopher.example.com 79
# Type username and hit enter

# Netcat
echo "" | nc gopher.example.com 79        # Owner info
echo "npub1abc" | nc gopher.example.com 79  # Specific user
```

**Finger clients:**
- **finger** - Standard Unix command (usually pre-installed)
- **Bombadillo** - Multi-protocol terminal browser

### Example Session

```bash
$ finger @localhost
Login: npub1abc...              Name: Alice
Directory: /nostr/npub1abc      Shell: /nostr

Plan:
Building a personal Nostr gateway. Notes, articles, and interactions
served via Gopher, Gemini, and Finger protocols.

Recent notes:
  [2025-10-24] Just published my Gopher server!
  [2025-10-23] Testing markdown conversion.
  [2025-10-22] Exploring old-school protocols.

Followers: 142  Following: 87  Mutual: 56
```

---

## Common Features

### Custom Sections

All protocols support custom sections that can be configured to display filtered content at specific URL paths.

**What are sections?**

Sections allow you to create custom views of your content with specific filters. For example:
- `/diy` - Posts tagged with #diy
- `/art` - Posts tagged with #art
- `/following` - Posts from people you follow
- `/` - Homepage with multiple section previews

**Configuration example:**

```yaml
sections:
  - name: diy-preview
    path: /                    # Show on homepage
    title: "Latest DIY Projects"
    filters:
      tags:
        t: ["diy"]
    limit: 5                   # Show 5 most recent
    order: 0                   # First section on page
    more_link:
      text: "View all DIY posts"
      section_ref: "diy-full"

  - name: diy-full
    path: /diy                 # Dedicated page
    title: "DIY Projects"
    filters:
      tags:
        t: ["diy"]
    limit: 20                  # Full page with 20 items
```

**Features:**
- **Path Mapping**: Map sections to any URL path
- **Multiple Sections**: Show multiple filtered views on one page (e.g., homepage)
- **Ordering**: Control display order when multiple sections share a path
- **"More" Links**: Add navigation to full paginated views
- **Filters**: By kinds, authors, tags, time ranges, scope
- **Sorting**: By date, reactions, zaps, or replies

**Example usage:**

A homepage (`/`) could show:
1. "Latest DIY Projects" (5 items)
2. "Recent Philosophy Posts" (5 items)
3. "Popular Articles" (5 items)

Each with a "View all" link to the full section page.

**Note:** Built-in endpoints (`/notes`, `/replies`, `/mentions`, `/articles`) are NOT sections. They are provided by the router. Sections are for custom filtered views.

### Thread Navigation

All three protocols support thread navigation:

**Gopher:**
```
1Parent: Original post	/event/parent_id	host	70
i	fake	host	70
1  Reply by Bob	/event/reply_id	host	70
1    Reply by Carol	/event/reply2_id	host	70
```

**Gemini:**
```gemtext
## Thread

=> /event/parent_id ↑ Parent: Original post

### Replies

=> /event/reply_id Reply by Bob
=> /event/reply2_id   Reply by Carol (nested)
```

**Finger:**

Finger doesn't support thread navigation (single-query protocol).

### Markdown Conversion

Nostr content (often markdown) is converted to protocol-specific formats:

**Gopher (plain text):**
- Headings: UPPERCASE or underline with ===
- Bold: UPPERCASE or **preserve asterisks**
- Links: `text <url>` or `text (url)`
- Code: indent or wrap with separators
- Line wrap: 70 chars

**Gemini (gemtext):**
- Headings: `# ## ###`
- Links: `=> url text` (separate lines)
- Code: `` ```lang ... ``` ``
- Quotes: `> text`
- Lists: `* item`

**Finger (stripped):**
- Remove all markdown syntax
- Preserve bare URLs optionally
- Truncate to ~500 chars

 

### NIP-19 Entity Resolution

All protocols automatically resolve NIP-19 entities (`nostr:` URIs) in content to human-readable references.

**Supported entities:**
- `nostr:npub1...` - Profile (resolved to display name)
- `nostr:nprofile1...` - Profile with relay hints
- `nostr:note1...` - Event (resolved to title/preview)
- `nostr:nevent1...` - Event with relay hints
- `nostr:naddr1...` - Parameterized replaceable event

**Gopher:**
```
Check out nostr:npub1abc... for more info.

Rendered as:
Check out @alice for more info.
```

**Gemini:**
```gemtext
Check out nostr:npub1abc... for more info.

Rendered as:
Check out [@alice](gemini://host/profile/abc123...) for more info.
```

**Display name resolution:**
- Fetches kind 0 profile metadata from storage
- Priority: display_name > name > nip05 > truncated pubkey
- Falls back gracefully if profile not found

**Title resolution for events:**
- Kind 1 (notes): First line of content (truncated)
- Kind 30023 (articles): "title" tag value
- Fallback: "Note abc123..." or "Event abc123..."

**Performance:**
\- Entity resolution is cached
- Storage lookups only for unknown entities
- Regex matching optimized with compiled pattern

### Aggregates Display

All protocols show interaction aggregates:

**Gopher:**
```
[2025-10-24 12:34] Just published my Gopher server!
  Replies: 5  Reactions: 12 (+10, ❤️2)  Zaps: 21,000 sats
```

**Gemini:**
```gemtext
## [2025-10-24] Just published my Gopher server!

Content here...

💬 5 replies  ❤️ 12 reactions  ⚡ 21,000 sats
```

**Finger:**
```
Recent zaps: 21,000 sats  Reactions: 342
```

---

## Testing

### Test Gopher

**Using telnet:**
```bash
telnet localhost 70
/
# Press Enter
# Should see gophermap menu
```

**Using lynx:**
```bash
lynx gopher://localhost
```

**Expected output:**
```
Welcome to [Site Title]

Notes
Articles
Inbox
Archive
Diagnostics
```

### Test Gemini

**Using openssl:**
```bash
echo "gemini://localhost/" | openssl s_client -connect localhost:1965 -quiet
```

**Using amfora:**
```bash
amfora gemini://localhost
```

**Expected output:**
```
20 text/gemini
# Welcome to [Site Title]

=> /notes Notes
=> /articles Articles
...
```

### Test Finger

**Using finger:**
```bash
finger @localhost
```

**Using netcat:**
```bash
echo "" | nc localhost 79
```

**Expected output:**
```
Login: npub1...
Name: [Operator Name]
Plan: [About field or recent note]
Recent notes: ...
```

### Troubleshooting Tests

**"Connection refused"**
- Server not running
- Wrong port
- Firewall blocking

**"Permission denied" (ports 70, 79)**
- Need sudo/root for ports <1024
- See [Deployment](deployment.md#port-binding)

**"Certificate error" (Gemini)**
- Self-signed cert (expected on first connect)
- Accept certificate in client (TOFU)
- Or generate proper cert

**"Empty response"**
- Check logs: `journalctl -u nophr -f`
- Verify database initialized
- Check config validation

---

## Performance

All protocol servers are lightweight:

| Protocol | Concurrent Connections | Memory per Connection |
|----------|------------------------|----------------------|
| Gopher | 1000+ | ~10KB |
| Gemini | 1000+ (TLS overhead) | ~50KB (TLS buffers) |
| Finger | 1000+ | ~5KB |

**Caching improves performance:**

```yaml
caching:
  enabled: true
  ttl:
    render:
      gopher_menu: 300        # Cache gophermaps 5 min
      gemini_page: 300        # Cache gemtext 5 min
      finger_response: 60     # Cache finger 1 min
```

 

---

## Security

### Port Binding (<1024)

Ports 70, 79, 1965 require root.

**Options:**
1. Run as root (not recommended)
2. Use systemd socket activation (recommended)
3. Port forwarding (iptables)
4. Use higher ports in config (testing only)

See [Deployment Guide](deployment.md#port-binding).

### TLS (Gemini)

**Self-signed certificates:**
- OK for personal use
- Clients must accept on first connect (TOFU)

**Production certificates:**
- Use Let's Encrypt
- Auto-renewal with certbot
- See [Deployment](deployment.md#tls-certificates)

### Rate Limiting

Configure rate limiting in security settings; see `docs/security.md`. You can also complement with firewall rules (iptables, fail2ban).

---

## Implementation Details

**Code locations:**
- Gopher: `internal/gopher/server.go`, `internal/gopher/router.go`, `internal/gopher/gophermap.go`
- Gemini: `internal/gemini/server.go`, `internal/gemini/router.go`, `internal/gemini/renderer.go`
- Finger: `internal/finger/server.go`, `internal/finger/handler.go`, `internal/finger/renderer.go`
- Markdown: `internal/markdown/gopher.go`, `internal/markdown/gemini.go`, `internal/markdown/finger.go`

**Design docs:**
 

---

## References

### Gopher
- RFC 1436: https://www.rfc-editor.org/rfc/rfc1436.html
- Gopher Manifesto: gopher://gopher.floodgap.com/1/gopher/tech/
- Gopher clients: https://github.com/topics/gopher-client

### Gemini
- Gemini spec: gemini://geminiprotocol.net
- Gemini FAQ: https://geminiprotocol.net/docs/faq.html
- Gemini clients: https://geminiprotocol.net/software/

### Finger
- RFC 742: https://www.rfc-editor.org/rfc/rfc742.html
- RFC 1288: https://www.rfc-editor.org/rfc/rfc1288.html

---

**Next:** [Nostr Integration](nostr-integration.md) | [Deployment Guide](deployment.md) | [Architecture](architecture.md)
