# Presentation and Theming Guide

Guide to controlling visual presentation and theming for nophr across Gopher, Gemini, and Finger.

Presentation settings customize headers, footers, separators, and simple template variables without changing code.

---

## Overview

Presentation is configured under the `presentation` key:

```yaml
presentation:
  headers:
    global:
      enabled: false
      content: ""
      file_path: ""
    per_page: {}
  footers:
    global:
      enabled: false
      content: ""
      file_path: ""
    per_page: {}
  separators:
    item:
      gopher: ""
      gemini: ""
      finger: ""
    section:
      gopher: "---"
      gemini: "---"
      finger: "---"
```

Use `display` and `behavior` settings (documented in `docs/configuration.md`) together with `presentation` for complete control over what content is shown and how it is arranged.

---

## Headers and Footers

Headers and footers can be defined inline or loaded from files.

### Global Header

```yaml
presentation:
  headers:
    global:
      enabled: true
      content: "Welcome to {{site.title}}"
      file_path: ""
```

### File-based Footer

```yaml
presentation:
  footers:
    global:
      enabled: true
      content: ""
      file_path: "./templates/footer.txt"
```

When `file_path` is set, nophr loads the file content and uses it in preference to `content`. Loaded content is cached for a short period to avoid repeated disk reads.

Per-page headers and footers can be configured via the `per_page` maps (for example keyed by path or section); see the configuration reference for the exact keys supported in this build.

---

## Template Variables

Presentation content supports simple template variables such as:

- `{{site.title}}`
- `{{site.description}}`
- `{{site.operator}}`
- `{{date}}` (YYYY-MM-DD)
- `{{datetime}}` (current date and time)
- `{{year}}`

These variables are expanded at render time using the current configuration and time.

Example:

```yaml
presentation:
  headers:
    global:
      enabled: true
      content: "{{site.title}} — {{date}}"
```

---

## Separators

Separators control how items and sections are visually separated in each protocol.

```yaml
presentation:
  separators:
    item:
      gopher: ""
      gemini: ""
      finger: ""
    section:
      gopher: "────────────────────────────────"
      gemini: "────────────────────────────────"
      finger: "────────────────────────────────"
```

Use:
- Short separators for subtle spacing.
- Longer separators for clear breaks between sections.

Each protocol can have its own separator style.

---

## Working with Display and Behavior

The `display` and `behavior` sections (documented in `docs/configuration.md`) complement `presentation`:

- `display` – controls whether to show replies, reactions, zaps, threads, and how much content to show in feeds or detail views.
- `behavior` – tunes filtering, sort order, and (future) pagination behavior.

Together with `presentation`, you can:
- Hide or show interaction counts.
- Limit summary length and thread depth.
- Change section separators per protocol.
- Add site-wide or per-page banners and footers.

---

## Example: Minimalist Gemini Theme

```yaml
display:
  feed:
    show_interactions: false
    show_reactions: false
    show_zaps: false
    show_replies: false

presentation:
  headers:
    global:
      enabled: true
      content: "# {{site.title}}\n\n{{site.description}}"
  separators:
    section:
      gemini: ""
```

This configuration hides interaction details in feeds and uses a simple global Gemini header.

---

## Related Documentation

- `docs/configuration.md` – `display`, `behavior`, and `presentation` configuration details.
- `docs/protocols.md` – how content appears in each protocol.
- `memory/ui_export.md` – internal rendering design (for developers).

