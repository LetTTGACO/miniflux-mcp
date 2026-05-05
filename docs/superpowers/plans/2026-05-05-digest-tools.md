# Digest Tools Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add digest-window entry retrieval and batch entry status updates for AI clients.

**Architecture:** Follow the existing MCP tool pattern: define schemas in `tools.go`, handlers in `main.go` or `handlers.go`, helper functions near entry filtering, and HTTP-backed behavior tests in `main_test.go`. Keep summarization, content cleanup, timezone policy, and delivery logs outside this MCP server.

**Tech Stack:** Go, `miniflux.app/v2 v2.2.19`, mark3labs MCP Go SDK, standard `go test` HTTP test server.

---

### Task 1: Batch Entry Status Tool

**Files:**
- Modify: `tools.go`
- Modify: `main.go`
- Modify: `main_test.go`
- Modify: `README.md`

- [ ] Add a failing test that registered tools include `update_entries_status` and that calling it sends `PUT /v1/entries` with `entry_ids` and `status`.
- [ ] Add the tool schema with required `entry_ids` and `status`.
- [ ] Implement the handler by validating a non-empty numeric `entry_ids` array and calling `s.client.UpdateEntries(entryIDs, status)`.
- [ ] Run the focused test and commit.

### Task 2: Daily Digest Tool

**Files:**
- Modify: `tools.go`
- Modify: `main.go`
- Modify: `main_test.go`
- Modify: `README.md`

- [ ] Add failing tests for `get_daily_digest`: required `since`, unread default status, `published_after` based on caller input, and structured response.
- [ ] Add failing tests for `content_mode` values, explicit scrape behavior, returned Miniflux content, and truncation metadata.
- [ ] Add the tool schema with `since`, `status`, `date_field`, `limit`, `content_mode`, `min_content_length`, and `max_content_length`.
- [ ] Implement digest request building and response shaping.
- [ ] Verify the digest tool does not expose content-cleaning options such as HTML stripping, entity decoding, or text formatting.
- [ ] Run focused tests and commit.

### Task 3: Full Verification

**Files:**
- Modify: `README.md`

- [ ] Update README tool counts and document the generic AI digest workflow.
- [ ] Run `go test -count=1 ./...`, `go build ./...`, `go vet ./...`, and `docker build -t miniflux-mcp:digest-check .`.
- [ ] Commit documentation or verification fixes if needed.
