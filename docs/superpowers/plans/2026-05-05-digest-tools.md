# Digest Tools Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add digest-window entry retrieval and batch entry status updates for AI clients.

**Status:** Completed. The checklist below reflects the current code, docs, and full verification state.

**Architecture:** Follow the existing MCP tool pattern: define schemas in `tools.go`, handlers in `main.go` or `handlers.go`, helper functions near entry filtering, and HTTP-backed behavior tests in `main_test.go`. Keep summarization, content cleanup, timezone policy, and delivery logs outside this MCP server.

**Tech Stack:** Go, `miniflux.app/v2 v2.2.19`, mark3labs MCP Go SDK, standard `go test` HTTP test server.

---

### Task 1: Batch Entry Status Tool

**Files:**
- Modify: `tools.go`
- Modify: `main.go`
- Modify: `main_test.go`
- Modify: `README.md`

- [x] Add a failing test that registered tools include `update_entries_status` and that calling it sends `PUT /v1/entries` with `entry_ids` and `status`.
- [x] Add the tool schema with required `entry_ids` and `status`.
- [x] Implement the handler by validating a non-empty numeric `entry_ids` array and calling `s.client.UpdateEntries(entryIDs, status)`.
- [x] Run the focused test and commit.

### Task 2: Daily Digest Tool

**Files:**
- Modify: `tools.go`
- Modify: `main.go`
- Modify: `main_test.go`
- Modify: `README.md`

- [x] Add failing tests for `get_daily_digest`: optional `since`, unread default status, `published_after` based on caller input when present, and structured response.
- [x] Add failing tests for `content_mode` values, explicit scrape behavior, returned Miniflux content, and truncation metadata.
- [x] Add the tool schema with `since`, `status`, `date_field`, `limit`, `content_mode`, `min_content_length`, and `max_content_length`.
- [x] Implement digest request building and response shaping.
- [x] Verify the digest tool does not expose content-cleaning options such as HTML stripping, entity decoding, or text formatting.
- [x] Run focused tests and commit.

### Task 3: Full Verification

**Files:**
- Modify: `README.md`

- [x] Update README tool counts and document the generic AI digest workflow.
- [x] Run `go test -count=1 ./...`, `go build ./...`, `go vet ./...`, and `docker build -t miniflux-mcp:digest-check .`.
- [x] Commit documentation or verification fixes if needed.
