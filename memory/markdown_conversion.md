Markdown Conversion Strategy

Problem
- Nostr events (especially kind 30023 long-form articles and some kind 1 notes) may contain Markdown formatting.
- Gopher protocol supports only plain text (no formatting).
- Gemini protocol uses gemtext (similar to but incompatible with Markdown).
- Finger protocol requires compact plain text.

Conversion Approaches

1. Markdown → Plain Text (for Gopher)

Strategy: Convert markdown to readable plain text while preserving structure.

Headings
- # Heading → "HEADING" or "== Heading ==" (configurable style)
- ## Subheading → "SUBHEADING" or "-- Subheading --"
- ### Level 3 → "Level 3:" or indent with prefix

Emphasis
- *italic* or _italic_ → leave as-is or convert to *asterisks*
- **bold** or __bold__ → UPPERCASE or leave as **double asterisks**
- ***bold italic*** → UPPERCASE with *asterisks* or leave as-is

Links
- [text](url) → "text (url)" or "text <url>" (configurable)
- <url> → keep as-is
- Bare URLs → keep as-is

Lists
- Unordered: preserve "- " or "* " prefix
- Ordered: preserve "1. " numbering
- Nested: indent with spaces (configurable depth)

Code
- `inline code` → 'inline code' or `keep backticks`
- ```code blocks``` → indent with spaces or wrap with separator lines:
  ```
  ---- CODE ----
  code content here
  --------------
  ```

Quotes
- > quote → indent with "> " prefix or "  | " prefix

Horizontal Rules
- --- or *** → convert to "━━━━━━━━━━" or "----------" (full line width)

Images
- ![alt](url) → "[Image: alt] url" or omit entirely with note

Tables
- Convert to ASCII table with | separators or linearize rows

Line Wrapping
- Wrap at configured max_line_length (default 70 for Gopher)
- Preserve paragraph breaks (double newline)
- Indent continuation lines for lists/quotes

2. Markdown → Gemtext (for Gemini)

Strategy: Map Markdown syntax to Gemini gemtext where possible; simplify where incompatible.

Headings
- # Heading → # Heading (gemtext heading level 1)
- ## Heading → ## Heading (gemtext heading level 2)
- ### Heading → ### Heading (gemtext heading level 3)
- #### and lower → ### Heading (gemtext max is 3 levels)

Emphasis
- *italic*, **bold**, ***both*** → plain text (gemtext has no inline formatting)
- Optional: convert **bold** to UPPERCASE for emphasis

Links
- [text](url) → => url text (gemtext link format)
- Links must be on their own line in gemtext
- Extract inline links and place at end of paragraph or section

Lists
- Unordered: * item → * item (gemtext list)
- Ordered: 1. item → * item (gemtext has no ordered lists; add number in text)
- Nested lists: flatten or indent (gemtext has limited nesting)

Code
- `inline code` → plain text or 'inline code' (gemtext has no inline code)
- ```code blocks``` → ``` code blocks ``` (gemtext preformatted block)

Quotes
- > quote → > quote (gemtext quote line)

Horizontal Rules
- --- → blank line or text separator like "---"

Images
- ![alt](url) → => url Image: alt

Tables
- Linearize or convert to preformatted block (```)

Line Length
- Wrap at configured max_line_length (default 80 for Gemini) to clamp long-form lines

3. Markdown → Plain Text (for Finger)

Strategy: Ultra-compact format; strip all formatting; summarize if needed.

- Remove all markdown syntax
- Convert to single-line or short paragraph
- Truncate at character limit (e.g., 500 chars for .plan)
- Preserve only bare URLs if critical

Implementation

Parser
- Use markdown parsing library (e.g., marked, markdown-it for JS/TS)
- Build AST (abstract syntax tree) from markdown source
- Walk AST and emit protocol-specific output

Renderer Modules
- GopherRenderer: AST → plain text with configurable formatting
- GeminiRenderer: AST → gemtext with link extraction and simplification
- FingerRenderer: AST → compact plain text with truncation

Configuration (per protocol in rendering section)
```yaml
rendering:
  gopher:
    markdown:
      heading_style: "underline"   # underline | uppercase | prefix
      heading_underline_char: "="
      bold_style: "uppercase"      # uppercase | asterisks | none
      italic_style: "asterisks"    # asterisks | none
      link_format: "text <url>"    # "text <url>" | "text (url)"
      code_block_wrap: "separator" # separator | indent
      separator_char: "-"
      max_line_length: 70
      preserve_hard_breaks: true
  gemini:
    markdown:
      flatten_deep_headings: true  # #### → ###
      extract_inline_links: true   # move [text](url) to separate lines
      convert_ordered_lists: true  # 1. → * 1.
      max_line_length: 80          # or null for no wrapping
  finger:
    markdown:
      strip_all: true              # remove all formatting
      max_length: 500              # truncate for .plan
      include_urls: false          # strip URLs entirely
```

Fallback
- If markdown parsing fails, serve raw content with warning comment
- Log parsing errors for diagnostics

Edge Cases
- Malformed markdown: best-effort rendering; fall back to plain text
- Mixed HTML in markdown: strip HTML tags entirely (protocols don't support it)
- Nested formatting: flatten to simplest representation
- Very long URLs: wrap or truncate for Gopher; keep full for Gemini
- Emoji: preserve for Gemini if rendering.gemini.emoji=true; convert to text or remove for Gopher/Finger

Example Transformations

Input (Markdown):
```
# My First Note

This is a **bold** statement with a [link](https://example.com).

Here's some code:
```
function hello() { return "world"; }
```

> A wise quote

- Item 1
- Item 2
```

Output (Gopher plain text):
```
== MY FIRST NOTE ==

This is a BOLD statement with a link <https://example.com>.

Here's some code:
---- CODE ----
function hello() { return "world"; }
--------------

  > A wise quote

- Item 1
- Item 2
```

Output (Gemini gemtext):
```
# My First Note

This is a bold statement with a link.

=> https://example.com link

Here's some code:
```
function hello() { return "world"; }
```

> A wise quote

* Item 1
* Item 2
```

Output (Finger plain text):
```
My First Note - This is a bold statement with a link. Here's some code: function hello() { return "world"; } A wise quote...
```

Testing
- Test suite with markdown samples covering all syntax elements
- Validate Gemini gemtext output with gemtext parser
- Ensure Gopher output is readable in common clients (lynx, VF-1, bombadillo)
- Verify line wrapping doesn't break URLs or code blocks
