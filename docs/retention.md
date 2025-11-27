# Retention Guide

User-facing guide to nophr's event retention and pruning behavior.

This document explains how retention works today and how to configure it safely for your deployment.

## Overview

nophr stores Nostr events locally and periodically prunes old or low-priority data to keep the database healthy.

Retention has two layers:
- **Simple retention** – keep events for a fixed number of days.
- **Advanced retention** – rule-based engine that can keep or prune events based on kind, age, engagement, and social distance.

All retention is enforced locally only; no data is deleted from remote relays.

---

## Simple Retention

Simple retention uses a single `keep_days` value to prune older events.

```yaml
sync:
  retention:
    keep_days: 365
    prune_on_start: true
```

**Fields:**
- `keep_days` – minimum number of days to keep events.
- `prune_on_start` – when `true`, runs a pruning pass during startup.

**Behavior:**
- Events older than `keep_days` can be pruned.
- Profile and contact data (kinds 0 and 3) are generally preserved so identity and graph remain usable.

Simple retention is a good default for small personal deployments.

---

## Advanced Retention

Advanced retention allows fine-grained rules based on event properties and engagement.

```yaml
sync:
  retention:
    advanced:
      enabled: true
      default_keep_days: 365
      rules:
        - name: "keep-owner-profile"
          priority: 100
          kinds: [0, 3]
          action: "keep"

        - name: "prune-old-low-engagement-notes"
          priority: 10
          kinds: [1]
          min_age_days: 365
          min_engagement: 0
          max_keep_days: 365
          action: "prune"
```

Exact field names and options follow the configuration schema; this guide focuses on concepts and safe usage.

### Concepts

- **Rules** – evaluated from highest to lowest `priority`.
- **Conditions** – match on kind, age, engagement, social distance, references, and more.
- **Actions** – typically "keep" or "prune" with optional time limits.

If multiple rules match, the highest-priority rule wins. A default catch-all rule should exist with the lowest priority.

---

## Practical Configuration Patterns

### 1. Keep Identity and Graph Forever

Ensure owner profile and contact graph are never pruned:

```yaml
sync:
  retention:
    advanced:
      enabled: true
      rules:
        - name: "keep-core-metadata"
          priority: 100
          kinds: [0, 3]
          action: "keep"
```

### 2. Prune Old Low-Engagement Notes

Keep recent or popular notes longer, drop old low-engagement ones:

```yaml
sync:
  retention:
    advanced:
      enabled: true
      default_keep_days: 365
      rules:
        - name: "prune-old-quiet-notes"
          priority: 10
          kinds: [1]
          min_age_days: 365
          max_keep_days: 365
          min_engagement: 0
          action: "prune"
```

### 3. Protect Important Events

You can mark key events as protected using metadata (for example via tags or dedicated kinds) and write rules that always keep them. Refer to your configuration schema for the exact fields available in this build.

---

## Operational Guidelines

- **Start simple** – begin with `keep_days` only. Enable advanced rules after you are comfortable with the data shape.
- **Monitor size** – watch `nophr.db` size and event counts (see `docs/storage.md`) to validate retention is doing what you expect.
- **Be conservative** – prefer pruning less rather than more until you have good backups.
- **Back up before big changes** – take a database backup before enabling new or aggressive rules.

Retention runs in the background and during startup (if configured). It is designed to keep nophr responsive over long periods without manual cleanup.

---

## Related Documentation

- `docs/storage.md` – storage backend and database maintenance.
- `docs/troubleshooting.md` – investigating database size and performance.
- `memory/retention_advanced.md` – internal design details (for developers).

