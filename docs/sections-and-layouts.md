# Sections and Layouts Guide

Guide to configuring custom sections, archives, and layouts for nophr.

Sections let you define named views over your Nostr data (for example `/diy`, `/philosophy`, or `/art`) that appear in Gopher, Gemini, and Finger navigation.

---

## Overview

Sections are configured in your YAML under the `sections` key. Each section:
- Has a **name** and **path**.
- Applies **filters** to select events.
- Controls **ordering**, **limits**, and **optional archive views**.

Protocol servers use sections to build menus and pages; built-in views like `/notes` and `/articles` are implemented using the same mechanism.

---

## Basic Section Example

```yaml
sections:
  - name: diy-preview
    path: /
    title: "Latest DIY"
    description: "Recent posts tagged #diy"
    filters:
      tags:
        t: ["diy"]
    limit: 5
    order: 0
    more_link:
      text: "More DIY posts"
      section_ref: "diy-full"

  - name: diy-full
    path: /diy
    title: "DIY Projects"
    description: "All posts tagged #diy"
    filters:
      tags:
        t: ["diy"]
    limit: 20
```

**Behavior:**
- `/` shows a short DIY preview section with a "More DIY posts" link.
- `/diy` shows the full DIY section with a larger limit.

---

## Common Fields

While exact fields are defined in the configuration schema, sections generally support:

- `name` – internal identifier.
- `path` – URL path (for example `/`, `/notes`, `/replies`, `/art`).
- `title` – display title for the section.
- `description` – human-readable description.
- `filters` – event selection (kinds, tags, authors, time ranges, etc.).
- `order` – display order when multiple sections share a path.
- `limit` – number of events to show.
- `more_link` – optional link to another section for "see more" behavior.

Sections can also support pagination, grouping, and archive generation as described below.

---

## Filters

Filters let you express which events belong to a section. Common patterns:

### By Kind

```yaml
filters:
  kinds: [1]       # Notes
```

### By Tag

```yaml
filters:
  tags:
    t: ["diy", "build"]
```

### By Author

```yaml
filters:
  authors:
    - "npub1..."   # Owner
    - "npub2..."   # Collaborator
```

### By Time Range

```yaml
filters:
  since_days: 7    # Last 7 days
```

See `docs/nostr-integration.md` and the configuration reference for the full filter schema used in this build.

---

## Archives

Sections can expose time-based archive views such as:

- `/archive/notes` – list of available archives.
- `/archive/notes/2025/10` – October 2025 notes.
- `/archive/notes/2025/10/24` – Notes from a specific day.

Archive behavior is driven by the same section/filter machinery, with helpers that group events by month, year, or day.

If your configuration enables archive generation for a section, the protocol servers will add appropriate menu entries under that section or dedicated `/archive/...` paths.

---

## Built-in vs Custom Sections

Built-in endpoints like:
- `/notes`
- `/articles`
- `/replies`
- `/mentions`

are implemented using the same section framework, backed by query helpers and aggregates. Custom sections add additional views; they do not replace these core endpoints.

Legacy `/inbox` and `/outbox` concepts are deprecated in favor of the more explicit `/replies` and `/notes` views plus configurable sections.

---

## Best Practices

- Keep sections focused – one clear purpose per section.
- Avoid too many sections at the root path; group related content under dedicated paths instead (for example `/dev`, `/art`).
- Use `order` to control menu ordering when multiple sections share a path.
- Combine sections with headers/footers (see presentation guide) for a more polished experience.

---

## Related Documentation

- `docs/configuration.md` – full `sections` configuration reference.
- `docs/protocols.md` – how sections show up in protocol navigation.
- `memory/layouts_sections.md` – internal design for sections and layouts (for developers).

