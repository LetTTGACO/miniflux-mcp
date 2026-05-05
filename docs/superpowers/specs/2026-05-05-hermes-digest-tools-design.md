# Hermes Digest Tools Design

## Goal

Add two high-level MCP tools for Hermes Agent: one to fetch daily unread Miniflux entries with summary-ready content, and one to update many entry statuses after a successful push.

## Tool Scope

`get_daily_digest` returns entries published since the start of the current day in a caller-selected timezone, defaulting to `Asia/Shanghai`. It defaults to unread entries so articles read in the Miniflux web UI are not pushed again by Hermes.

`update_entries_status` accepts `entry_ids` and a Miniflux status (`read`, `unread`, or `removed`) and updates them in one Miniflux API call. Hermes should call this only after its own delivery succeeds.

## Content Handling

`get_daily_digest` returns structured JSON with entry metadata and optional plain-text content. It supports:

- `content_mode=none`: metadata only.
- `content_mode=feed`: use the feed-provided entry content.
- `content_mode=scrape_when_short`: use feed content, but call Miniflux original-content scraping when the text is shorter than `min_content_length`.
- `content_mode=scrape_all`: scrape original content for every entry.

Returned content is truncated to `max_content_length` per entry. The tool returns content metadata such as `content_source`, `content_available`, and `content_truncated` so Hermes can judge whether it is summarizing full text, feed text, or missing text.

## Boundaries

The MCP server does not summarize news and does not record Hermes push delivery state. Hermes owns summarization, delivery formatting, and delivery logs. This server owns Miniflux reads, content extraction, and status changes.

## Verification

Tests should cover tool registration, default daily filtering, content modes, scrape fallback, truncation metadata, and batch status request bodies. README tool counts and tool lists must stay aligned with registered tools.
