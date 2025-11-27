nophr: Config-First Personal Nostr-to-Gopher/Gemini/Finger Gateway

Overview
- Serves Nostr content via Gopher (RFC 1436), Gemini (gemini://), and Finger (RFC 742) protocols only.
- Single-tenant by default; shows one operator's notes and articles from Nostr.
- Everything configurable via file and env overrides; no hard-coded relays.
- Inbox/Outbox model to aggregate replies, reactions, and zaps from Nostr.
- Seed-relay bootstrap; dynamic relay discovery via NIP-65 (kind 10002).
- Controlled synchronization scope (self/following/mutual/FOAF depth) with caps and allow/deny lists.
- Embedded Khatru relay for event storage; SQLite backend in this build (LMDB planned; no PostgreSQL).
- Protocol-specific rendering: Gopher menus/text, Gemini gemtext, Finger user info.
- Composable layouts: sections defined by filters; sensible defaults and archives.
- Privacy and safety: env-only secrets, pruning, deny-lists, and diagnostics.

Status
- This directory documents the design decisions and plan. The codebase implements many of these phases already; memory/ remains the source of truth and must stay in sync with implementation.

Implementation Guide:
- PHASES.md (phased implementation plan with deliverables and completion criteria)
- ../AGENTS.md (instructions for AI agents working on this project)

Architecture and Design:
- architecture.md (system design and component overview)
- glossary.md (terminology reference)

Development Process:
- cicd.md (CI/CD pipeline and release automation)
- distribution.md (installation and packaging strategy)
- testing.md (testing strategy)

Core Systems:
- storage_model.md (Khatru integration and database schema)
- configuration.md (config system and options)
- sequence_seed_discovery_sync.md (Nostr sync flow)

Features:
- ui_export.md (Gopher/Gemini/Finger protocol rendering)
- markdown_conversion.md (markdown to protocol conversion)
- layouts_sections.md (configurable sections and pages)
- inbox_outbox.md (interaction aggregation)
- caching.md (caching strategy)

Nostr Integration:
- relay_discovery.md (NIP-65 relay discovery)
- sync_scope.md (social graph and scope control)
- nips.md (Nostr protocol specifications)

Operations:
- operations.md (operational procedures)
- diagnostics.md (monitoring and diagnostics)
- security_privacy.md (security and privacy features)
- retention_advanced.md (Advanced configurable retention system; implemented in Phase 20 — see PHASE20_COMPLETION.md)

Design References
- architecture.md — Overall system design
- storage_model.md — Storage model and custom tables
- configuration.md — Config schema and validation notes
- layouts_sections.md — Sections, filters, pages
- ui_export.md — Protocol rendering design (Gopher/Gemini/Finger)
- markdown_conversion.md — Markdown→protocol conversion rules
- relay_discovery.md — NIP-65 relay discovery
- sync_scope.md — Social graph and scope
- sequence_seed_discovery_sync.md — Discovery→sync flow
- inbox_outbox.md — Aggregates and interaction model
- static_gopher_export.md — Static export (Gopher/Gemini) triggers/config
- caching.md, cache_keys_hashing.md — Caching strategy and keys
- diagnostics.md, operations.md, distribution.md — Ops and packaging
- security_privacy.md — Security and privacy
- nips.md — Nostr specs used
- roadmap.md — High-level roadmap (internal)
- PHASES.md — Implementation phases (internal)
