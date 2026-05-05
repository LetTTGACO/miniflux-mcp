# Contributing

Thanks for improving `miniflux-mcp`. This project is intentionally small, so contributions should stay focused and easy to verify.

## Local Setup

Required tools:

- Go 1.26 or newer
- Docker, if you want to verify the container image
- A Miniflux instance for manual integration testing

Copy the example environment file when you need to run the server locally:

```bash
cp .env.example .env
```

Then set either `MINIFLUX_API_KEY` or both `MINIFLUX_USERNAME` and `MINIFLUX_PASSWORD`.

## Development Workflow

1. Inspect the existing tool schema and handler patterns before editing.
2. Add or update focused tests for behavior changes.
3. Keep README tool descriptions, group counts, and workflow notes in sync with `tools.go`.
4. Run verification before committing.
5. Commit one logical change at a time.

## Verification

Run these before opening a pull request:

```bash
go test -count=1 ./...
go build ./...
go vet ./...
docker build -t miniflux-mcp:local-check .
```

For small documentation-only changes, `go test -count=1 ./...` is usually enough, but Docker verification is preferred before releases or Dockerfile-related edits.

### Local Miniflux Integration Smoke Test

Use the optional integration stack when you want to verify the MCP server against a real Miniflux API:

```bash
scripts/integration-smoke.sh
```

The script starts Miniflux and Postgres with `docker-compose.integration.yml`, waits for `/healthcheck`, then runs the MCP server over stdio. It calls `healthcheck`, `get_me`, `get_categories`, `get_feeds`, creates a feed from `https://cprss.s3.amazonaws.com/javascriptweekly.com.xml`, and calls `get_feeds` again. It removes the integration containers and database volume when it exits.

To keep the local Miniflux stack running for manual testing:

```bash
KEEP_MINIFLUX=1 scripts/integration-smoke.sh
```

To test a different feed URL:

```bash
MINIFLUX_INTEGRATION_FEED_URL=https://example.com/feed.xml scripts/integration-smoke.sh
```

## API Coverage Changes

When adding Miniflux API coverage:

- Use the pinned client version from `go.mod` as the source of truth.
- Compare request structs in the Miniflux Go client before choosing MCP argument names.
- Prefer expanding an existing tool when the API is the same endpoint with additional options.
- Add a new tool when the client method represents a distinct operation.
- Keep sensitive inputs such as passwords, cookies, and tokens out of logs and docs examples.

## Documentation Changes

When updating docs for existing MCP functionality:

- Treat `minifluxToolDefinitions` in `tools.go` as the registered tool source of truth.
- Check README total and group counts whenever tools move, appear, disappear, or change names.
- Keep `AGENTS.md` aligned with project boundaries that future agents must preserve.
- Update `docs/superpowers/specs/` or `docs/superpowers/plans/` when implementation status or tool workflow guidance changes.
- For digest tools, keep timezone policy, summarization, content cleanup, and delivery state documented as AI-client responsibilities.

## Pull Request Checklist

- [ ] The change has one clear purpose.
- [ ] Tests or documentation were updated where needed.
- [ ] `go test -count=1 ./...` passes.
- [ ] `go build ./...` passes for code changes.
- [ ] `go vet ./...` passes for code changes.
- [ ] Docker build was run for Docker or release-impacting changes.
- [ ] README tool counts still match `tools.go` when tool registration changed.
