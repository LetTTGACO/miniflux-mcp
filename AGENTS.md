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

## Documentation Rules

- Keep README tool counts and group counts aligned with `RegisterAllTools` in `tools.go`.
- If a tool schema changes, update the README description in the same stage.
- Prefer concise docs that help a future human or agent run, verify, and safely modify the project.

## Git Workflow

The user prefers phase-by-phase commits:

1. Make one scoped change.
2. Verify it.
3. Commit it.
4. Move to the next scoped change.

Before committing, check `git status --short --branch` and review the diff for unrelated changes.
