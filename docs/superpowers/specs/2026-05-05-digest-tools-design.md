# Digest Tools Design

## Goal

Add two high-level MCP tools for AI clients: one to fetch Miniflux entries for digest processing, and one to update many entry statuses after a successful downstream action.

## Tool Scope

`get_daily_digest` defaults to unread entries without a time filter, so it can act as a queue for AI digest processing. Callers may provide a `since` timestamp to add a bounded digest window using `date_field`; the MCP server does not choose a timezone or daily boundary.

`update_entries_status` accepts `entry_ids` and a Miniflux status (`read`, `unread`, or `removed`) and updates them in one Miniflux API call. The AI client should call this only after its own delivery, review, or processing step succeeds.

## Content Handling

`get_daily_digest` returns structured JSON with entry metadata and optional content as returned by Miniflux. It must not clean, summarize, rewrite, strip HTML, decode entities, or otherwise transform article content. It supports:

- `content_mode=none`: metadata only.
- `content_mode=feed`: use the feed-provided entry content. This is the default.
- `content_mode=scrape_when_short`: use feed content, but call Miniflux original-content scraping when the content is shorter than `min_content_length`.
- `content_mode=scrape_all`: explicitly scrape original content for every entry.

Returned content is truncated to `max_content_length` per entry. The tool returns content metadata such as `content_source`, `content_available`, and `content_truncated` so the caller can judge whether it is using feed text, scraped text, or missing text.

## Boundaries

The MCP server does not summarize news, decide timezones, calculate daily boundaries, clean article content, or record push delivery state. The AI client owns summarization, delivery formatting, content cleanup, date-window policy, and delivery logs. This server owns Miniflux reads, optional original-content fetches, truncation for payload size, and status changes.

## Verification

Tests should cover tool registration, optional `since` filtering, unread defaults, content modes, explicit scrape behavior, truncation metadata, and batch status request bodies. README tool counts and tool lists must stay aligned with registered tools.
