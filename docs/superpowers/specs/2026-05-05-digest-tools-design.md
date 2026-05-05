# Digest Tools Design

## Goal

Add two high-level MCP tools for AI clients: one to fetch Miniflux entries for a caller-defined digest window, and one to update many entry statuses after a successful downstream action.

## Tool Scope

`get_daily_digest` returns entries published since a caller-provided `since` timestamp. It defaults to unread entries so articles read in the Miniflux web UI are not returned again unless the caller chooses another status. The MCP server does not choose a timezone or daily boundary.

`update_entries_status` accepts `entry_ids` and a Miniflux status (`read`, `unread`, or `removed`) and updates them in one Miniflux API call. The AI client should call this only after its own delivery, review, or processing step succeeds.

## Content Handling

`get_daily_digest` returns structured JSON with entry metadata and optional content as returned by Miniflux. It supports:

- `content_mode=none`: metadata only.
- `content_mode=feed`: use the feed-provided entry content. This is the default.
- `content_mode=scrape_when_short`: use feed content, but call Miniflux original-content scraping when the content is shorter than `min_content_length`.
- `content_mode=scrape_all`: explicitly scrape original content for every entry.

Content defaults to `content_format=raw`, which returns Miniflux content unchanged. Callers may request `content_format=text` for lightweight HTML tag stripping and entity decoding before truncation.

Returned content is truncated to `max_content_length` per entry. The tool returns content metadata such as `content_source`, `content_available`, and `content_truncated` so the caller can judge whether it is using feed text, scraped text, or missing text.

## Boundaries

The MCP server does not summarize news, decide timezones, calculate daily boundaries, or record push delivery state. The AI client owns summarization, delivery formatting, date-window policy, and delivery logs. This server owns Miniflux reads, optional original-content fetches, and status changes.

## Verification

Tests should cover tool registration, required `since` filtering, content modes, explicit scrape behavior, truncation metadata, and batch status request bodies. README tool counts and tool lists must stay aligned with registered tools.
