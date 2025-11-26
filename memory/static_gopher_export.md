Static Gopher Export

Goal
- Provide a static gopher hole export alongside the dynamic server.
- Trigger exports automatically when the operator publishes a new root event:
  - kind 1 without any `e` tag (root notes)
  - kind 30023 (long-form)

Configuration (export.gopher)
- enabled: turn static export on/off.
- output_dir: target directory for generated gophermap/files (default: ./export/gopher).
- host/port: host and port used in generated gophermap selectors (defaults from gopher config).
- max_items: cap on items per section to keep gophermaps small.

Generation Behavior
- Triggered from sync engine after storing events that match the owner-root criteria.
- Queries owner content only; no replies (kind 1 with `e` tag are excluded).
- Writes:
  - {output_dir}/gophermap with links to notes/articles sections and generated timestamp.
  - {output_dir}/notes/gophermap with entries for owner root notes.
  - {output_dir}/articles/gophermap with entries for owner articles.
  - {output_dir}/notes/{event_id}.txt and {output_dir}/articles/{event_id}.txt rendered via the Gopher renderer.
- Items are sorted by CreatedAt desc and trimmed to max_items.

Scope / Limitations
- Only owner-authored root notes/articles are exported.
- No archives/pagination yet; single-page listings per section.
- Generation runs synchronously on the ingestion thread—acceptable for low publish frequency; revisit if operator posts at high volume.
