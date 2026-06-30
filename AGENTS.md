# Agent Instructions

## Project Scope

This repository is a Go MCP server for Miniflux. Keep changes focused on the MCP server, tool schemas, request builders, handlers, tests, Docker image, and project documentation.

Do not make unrelated behavior changes while doing documentation, metadata, or audit work. When expanding API coverage, compare against the pinned Go client in `go.mod` first, currently `miniflux.app/v2 v2.2.19`.

## Development Rules

- Prefer small, staged changes with one clear purpose per commit.
- Use existing naming conventions in `tools.go`, `main.go`, `handlers.go`, and `main_test.go`.
- Keep MCP tool argument names aligned with Miniflux JSON field names unless the repo already uses a different name.
- When adding or expanding a tool, update all relevant places: schema in `tools.go`, handler/request building code, tests, and README.
- Do not commit credentials, API keys, cookies, private URLs, or local `.env` files.
- For OPML import, keep the MCP input as `opml_content` string. Do not switch it back to reading local file paths.
- Startup currently fails fast when healthcheck or auth fails. Do not weaken that behavior unless explicitly requested.

## Current MCP Capability Map

`tools.go` is the source of truth for registered MCP tools. The current public surface is 51 tools across 7 README groups:

- Feed Management: 12 tools for feed CRUD, refresh, feed entry access/import, feed icons, and marking a feed as read.
- Entry Management: 10 tools for entry listing/filtering, digest retrieval, single and batch status updates, starring, saving, editing, original-content fetch, and marking all entries as read for a user.
- Category Management: 9 tools for category CRUD, category feeds/entries, refresh, and mark-as-read operations.
- User Management: 6 tools for listing users, reading the current user, user lookup, user creation, and user deletion.
- System & Utility: 8 tools for version, healthcheck, counters, integrations, feed discovery, OPML export/import, and history flushing.
- API Key Management: 3 tools for listing, creating, and deleting API keys.
- Icons & Media: 3 tools for icons, enclosures, and enclosure media progression.

When updating docs, verify both the total count and every group count against `minifluxToolDefinitions`.

## Digest Tool Boundaries

`get_daily_digest` and `update_entries_status` are high-level tools for AI clients, but this server should stay a Miniflux MCP server rather than a summarizer or delivery system.

- `get_daily_digest` should default to unread entries when `since` is omitted. If `since` is provided, use it as the caller-owned time window; the server must not choose a timezone, daily boundary, or schedule.
- `get_daily_digest` uses `category_ids` for optional multi-category inclusion and `exclude_category_ids` for exclusion. Do not reintroduce the old single `category_id` argument for this tool unless explicitly requested.
- Keep digest content modes limited to Miniflux-backed content retrieval: `none`, `feed`, `scrape_when_short`, and `scrape_all`.
- Do not add article cleanup, summarization, push delivery, or delivery logs to this server.
- The digest response should keep acknowledgement metadata such as `ack_entry_ids` so clients can call `update_entries_status` only after their downstream processing succeeds.

## Testing And Verification

Run focused tests first when changing a helper or handler, then run the full suite before committing.

Standard verification:

```bash
go test -count=1 ./...
go build ./...
go vet ./...
docker build -t miniflux-mcp:local-check .
```

Use a more specific Docker tag when it helps describe the change being verified.

## Docker Publishing

- The Docker publish workflow is triggered by `v*` tags.
- Use Docker Hub access-token authentication, not account passwords.
- The required GitHub Actions repository secrets are `DOCKERHUB_USERNAME` and `DOCKERHUB_TOKEN`.
- Keep the public Docker image name documented in README, and let `.github/workflows/docker.yml` derive the namespace from `DOCKERHUB_USERNAME`.
- If changing the image namespace or registry, update `.github/workflows/docker.yml` and README in the same change.

## Documentation Rules

- Keep README tool counts and group counts aligned with `RegisterAllTools` in `tools.go`.
- If a tool schema changes, update the README description in the same stage.
- If a tool changes arguments, update any related workflow docs or plan/spec notes under `docs/`.
- Keep README usage examples credential-free and avoid private Miniflux URLs.
- Prefer concise docs that help a future human or agent run, verify, and safely modify the project.

## Git Workflow

The user prefers phase-by-phase commits:

1. Make one scoped change.
2. Verify it.
3. Commit it.
4. Move to the next scoped change.

Before committing, check `git status --short --branch` and review the diff for unrelated changes.
