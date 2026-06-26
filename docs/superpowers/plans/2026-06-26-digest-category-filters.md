# Digest Category Filters Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace `get_daily_digest`'s single `category_id` input with `category_ids` and add `exclude_category_ids` filtering.

**Architecture:** Keep Miniflux API querying bounded by the existing time, status, feed, limit, and order filters. Apply multi-category include/exclude filtering locally before building digest response entries and acknowledgement IDs, because the pinned Miniflux Go client only exposes a single `CategoryID` filter.

**Tech Stack:** Go 1.26, `github.com/mark3labs/mcp-go`, `miniflux.app/v2 v2.2.19`, standard `testing` package.

## Global Constraints

- Keep changes focused on the MCP server, tool schemas, request builders, handlers, tests, and README.
- When adding or expanding a tool, update all relevant places: schema in `tools.go`, handler/request building code, tests, and README.
- `get_daily_digest` must require a caller-provided `since` value.
- Keep digest content modes limited to `none`, `feed`, `scrape_when_short`, and `scrape_all`.
- Do not add article cleanup, summarization, push delivery, or delivery logs.
- Replace `category_id` with `category_ids`; backward compatibility for `category_id` is not required.
- Add `exclude_category_ids`.
- Effective category filtering is `real_category_ids = category_ids - exclude_category_ids`; if `category_ids` is omitted or empty, start from all categories and exclude `exclude_category_ids`.
- Response `count` and `ack_entry_ids` reflect locally filtered entries; response `total` remains the upstream Miniflux API total.

---

### Task 1: Schema and Option Parsing

**Files:**
- Modify: `tools.go`
- Modify: `main.go`
- Test: `main_test.go`

**Interfaces:**
- Consumes: existing `numberArg(args map[string]interface{}, key string) (float64, bool)`.
- Produces: `numberArrayArg(args map[string]interface{}, key string) ([]int64, error)`, `dailyDigestOptions.categoryIDs []int64`, and `dailyDigestOptions.excludeCategoryIDs []int64`.

- [ ] **Step 1: Write failing schema and parser tests**

Add this test after `TestGetDailyDigestRequiresSince` in `main_test.go`:

```go
func TestGetDailyDigestCategoryFilterSchema(t *testing.T) {
	tool := toolDefinitionByName(t, "get_daily_digest")

	if _, ok := tool.InputSchema.Properties["category_id"]; ok {
		t.Fatalf("get_daily_digest schema must not expose category_id")
	}
	for _, property := range []string{"category_ids", "exclude_category_ids"} {
		schema, ok := tool.InputSchema.Properties[property]
		if !ok {
			t.Fatalf("get_daily_digest schema is missing %s", property)
		}
		schemaMap, ok := schema.(map[string]interface{})
		if !ok {
			t.Fatalf("%s schema = %#v, want object", property, schema)
		}
		if schemaMap["type"] != "array" {
			t.Fatalf("%s type = %#v, want array", property, schemaMap["type"])
		}
	}
}

func TestBuildDailyDigestOptionsParsesCategoryFilters(t *testing.T) {
	options, err := buildDailyDigestOptions(map[string]interface{}{
		"since":                float64(1777910400),
		"category_ids":         []interface{}{float64(1), float64(2), float64(4)},
		"exclude_category_ids": []interface{}{float64(4)},
	})
	if err != nil {
		t.Fatalf("buildDailyDigestOptions returned error: %v", err)
	}
	if !sameInt64Set(options.categoryIDs, []int64{1, 2, 4}) {
		t.Fatalf("categoryIDs = %#v, want [1 2 4]", options.categoryIDs)
	}
	if !sameInt64Set(options.excludeCategoryIDs, []int64{4}) {
		t.Fatalf("excludeCategoryIDs = %#v, want [4]", options.excludeCategoryIDs)
	}
}
```

Add this helper near `sameStringSet` in `main_test.go`:

```go
func sameInt64Set(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	seen := make(map[int64]int, len(a))
	for _, value := range a {
		seen[value]++
	}
	for _, value := range b {
		seen[value]--
		if seen[value] < 0 {
			return false
		}
	}
	return true
}
```

- [ ] **Step 2: Run focused tests and verify they fail**

Run: `go test -count=1 ./...`

Expected: FAIL because `category_ids` and `exclude_category_ids` are not in the schema, `category_id` is still present, `dailyDigestOptions` has no new fields, and `sameInt64Set` may be the only passing addition.

- [ ] **Step 3: Update `get_daily_digest` schema**

In `tools.go`, replace the `category_id` property inside the `get_daily_digest` schema with:

```go
"category_ids": map[string]interface{}{
	"type":        "array",
	"description": "Optional category IDs to include before exclusions are applied",
	"items":       map[string]interface{}{"type": "number"},
},
"exclude_category_ids": map[string]interface{}{
	"type":        "array",
	"description": "Optional category IDs to exclude from the digest",
	"items":       map[string]interface{}{"type": "number"},
},
```

- [ ] **Step 4: Add parser fields and helper**

In `main.go`, extend `dailyDigestOptions`:

```go
	categoryIDs         []int64
	excludeCategoryIDs  []int64
```

In `buildDailyDigestOptions`, before `return options, nil`, add:

```go
	categoryIDs, err := numberArrayArg(args, "category_ids")
	if err != nil {
		return options, err
	}
	options.categoryIDs = categoryIDs

	excludeCategoryIDs, err := numberArrayArg(args, "exclude_category_ids")
	if err != nil {
		return options, err
	}
	options.excludeCategoryIDs = excludeCategoryIDs
```

After `numberArg`, add:

```go
func numberArrayArg(args map[string]interface{}, key string) ([]int64, error) {
	value, ok := args[key]
	if !ok || value == nil {
		return nil, nil
	}

	values, ok := value.([]interface{})
	if !ok {
		return nil, fmt.Errorf("%s must be an array of numbers", key)
	}

	result := make([]int64, 0, len(values))
	for _, item := range values {
		number, ok := item.(float64)
		if !ok {
			return nil, fmt.Errorf("%s must be an array of numbers", key)
		}
		result = append(result, int64(number))
	}

	return result, nil
}
```

- [ ] **Step 5: Run focused tests and verify they pass**

Run: `go test -count=1 ./...`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git status --short --branch
git diff
git add tools.go main.go main_test.go
git commit -m "feat: parse digest category filters"
```

### Task 2: Digest Category Filtering Behavior

**Files:**
- Modify: `main.go`
- Test: `main_test.go`

**Interfaces:**
- Consumes: `dailyDigestOptions.categoryIDs []int64` and `dailyDigestOptions.excludeCategoryIDs []int64` from Task 1.
- Produces: `includeDigestEntry(entry *client.Entry, options dailyDigestOptions) bool` and `entryCategoryID(entry *client.Entry) (int64, bool)`.

- [ ] **Step 1: Write failing include/exclude behavior test**

Add this test after `TestBuildDailyDigestOptionsParsesCategoryFilters`:

```go
func TestGetDailyDigestFiltersIncludedAndExcludedCategories(t *testing.T) {
	server := &MinifluxServer{
		client: client.NewClientWithOptions(
			"http://mf",
			client.WithHTTPClient(&http.Client{
				Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
					if req.URL.Path != "/v1/entries" {
						t.Fatalf("path = %s, want /v1/entries", req.URL.Path)
					}
					if got := req.URL.Query().Get("category_id"); got != "" {
						t.Fatalf("category_id query = %q, want empty because filtering is local", got)
					}
					return &http.Response{
						StatusCode: http.StatusOK,
						Body: io.NopCloser(strings.NewReader(`{
							"total": 3,
							"entries": [
								{"id": 11, "title": "One", "url": "https://example.com/1", "status": "unread", "feed_id": 1, "feed": {"id": 1, "title": "Feed 1", "category": {"id": 1, "title": "Keep"}}},
								{"id": 12, "title": "Two", "url": "https://example.com/2", "status": "unread", "feed_id": 2, "feed": {"id": 2, "title": "Feed 2", "category": {"id": 2, "title": "Keep Too"}}},
								{"id": 14, "title": "Four", "url": "https://example.com/4", "status": "unread", "feed_id": 4, "feed": {"id": 4, "title": "Feed 4", "category": {"id": 4, "title": "Drop"}}}
							]
						}`)),
						Header: http.Header{},
					}, nil
				}),
			}),
		),
	}

	result, err := server.GetDailyDigest(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{Arguments: map[string]interface{}{
			"since":                float64(1777910400),
			"category_ids":         []interface{}{float64(1), float64(2), float64(4)},
			"exclude_category_ids": []interface{}{float64(4)},
		}},
	})
	if err != nil {
		t.Fatalf("handler returned transport error: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatalf("result = %#v, want non-error", result)
	}

	var response dailyDigestResponse
	unmarshalToolResultText(t, result, &response)
	if response.Total != 3 {
		t.Fatalf("total = %d, want upstream total 3", response.Total)
	}
	if response.Count != 2 {
		t.Fatalf("count = %d, want locally filtered count 2", response.Count)
	}
	if !sameInt64Set(response.AckEntryIDs, []int64{11, 12}) {
		t.Fatalf("ack_entry_ids = %#v, want [11 12]", response.AckEntryIDs)
	}
}
```

- [ ] **Step 2: Write failing exclude-only behavior test**

Add this test after `TestGetDailyDigestFiltersIncludedAndExcludedCategories`:

```go
func TestGetDailyDigestExcludesCategoriesWithoutIncludeFilter(t *testing.T) {
	server := &MinifluxServer{
		client: client.NewClientWithOptions(
			"http://mf",
			client.WithHTTPClient(&http.Client{
				Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
					if req.URL.Path != "/v1/entries" {
						t.Fatalf("path = %s, want /v1/entries", req.URL.Path)
					}
					return &http.Response{
						StatusCode: http.StatusOK,
						Body: io.NopCloser(strings.NewReader(`{
							"total": 2,
							"entries": [
								{"id": 21, "title": "News", "url": "https://example.com/news", "status": "unread", "feed_id": 1, "feed": {"id": 1, "title": "News Feed", "category": {"id": 1, "title": "News"}}},
								{"id": 22, "title": "Social", "url": "https://example.com/social", "status": "unread", "feed_id": 2, "feed": {"id": 2, "title": "Social Feed", "category": {"id": 4, "title": "Social"}}}
							]
						}`)),
						Header: http.Header{},
					}, nil
				}),
			}),
		),
	}

	result, err := server.GetDailyDigest(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{Arguments: map[string]interface{}{
			"since":                float64(1777910400),
			"exclude_category_ids": []interface{}{float64(4)},
		}},
	})
	if err != nil {
		t.Fatalf("handler returned transport error: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatalf("result = %#v, want non-error", result)
	}

	var response dailyDigestResponse
	unmarshalToolResultText(t, result, &response)
	if response.Count != 1 {
		t.Fatalf("count = %d, want 1", response.Count)
	}
	if !sameInt64Set(response.AckEntryIDs, []int64{21}) {
		t.Fatalf("ack_entry_ids = %#v, want [21]", response.AckEntryIDs)
	}
}
```

- [ ] **Step 3: Run focused tests and verify they fail**

Run: `go test -count=1 ./...`

Expected: FAIL because entries are not locally filtered yet, so excluded categories still appear in `ack_entry_ids` and `count`.

- [ ] **Step 4: Remove old upstream category query**

In `GetDailyDigest`, delete this block:

```go
if categoryID, ok := numberArg(argsMap, "category_id"); ok {
	filter.CategoryID = int64(categoryID)
}
```

- [ ] **Step 5: Add local category filter helpers**

Add these helpers after `buildDailyDigestEntry`:

```go
func includeDigestEntry(entry *client.Entry, options dailyDigestOptions) bool {
	categoryID, ok := entryCategoryID(entry)
	if !ok {
		return len(options.categoryIDs) == 0
	}

	if len(options.categoryIDs) > 0 && !containsInt64(options.categoryIDs, categoryID) {
		return false
	}
	if containsInt64(options.excludeCategoryIDs, categoryID) {
		return false
	}

	return true
}

func entryCategoryID(entry *client.Entry) (int64, bool) {
	if entry.Feed == nil || entry.Feed.Category == nil {
		return 0, false
	}
	return entry.Feed.Category.ID, true
}

func containsInt64(values []int64, target int64) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
```

- [ ] **Step 6: Apply local filtering before building digest entries**

In `GetDailyDigest`, change the response initialization to keep `Total: entries.Total`, `Count: 0`, and empty `AckEntryIDs`/`Entries`.

Replace the loop with:

```go
for _, entry := range entries.Entries {
	if !includeDigestEntry(entry, options) {
		continue
	}
	digestEntry := s.buildDailyDigestEntry(entry, options)
	response.AckEntryIDs = append(response.AckEntryIDs, entry.ID)
	response.Entries = append(response.Entries, digestEntry)
}
response.Count = len(response.Entries)
```

- [ ] **Step 7: Run focused tests and verify they pass**

Run: `go test -count=1 ./...`

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git status --short --branch
git diff
git add main.go main_test.go
git commit -m "feat: filter digest categories locally"
```

### Task 3: README Documentation

**Files:**
- Modify: `README.md`
- Test: `main_test.go` existing README sync test

**Interfaces:**
- Consumes: `get_daily_digest` schema from Tasks 1 and 2.
- Produces: README argument docs matching `category_ids` and `exclude_category_ids`.

- [ ] **Step 1: Update README digest tool summary**

In `README.md`, update the `get_daily_digest` line under Entry Management to mention include/exclude category filters:

```markdown
- `get_daily_digest` - Get a bounded, digest-ready entry set since a caller-provided timestamp with optional feed, category include, and category exclude filters
```

- [ ] **Step 2: Update README digest arguments**

In the AI Digest Workflow argument list, replace:

```markdown
- `feed_id` and `category_id` optionally scope the digest.
```

with:

```markdown
- `feed_id` optionally scopes the digest to one feed.
- `category_ids` optionally scopes the digest to multiple categories.
- `exclude_category_ids` removes categories from the digest after `category_ids` is applied. When both are present, the effective category set is `category_ids - exclude_category_ids`.
```

- [ ] **Step 3: Run tests**

Run: `go test -count=1 ./...`

Expected: PASS, including `TestToolDefinitionsStayInSyncWithREADME`.

- [ ] **Step 4: Commit**

```bash
git status --short --branch
git diff
git add README.md
git commit -m "docs: document digest category filters"
```

### Task 4: Full Verification

**Files:**
- No source files expected.

**Interfaces:**
- Consumes: all implementation and documentation commits from Tasks 1-3.
- Produces: verified working tree ready for user review.

- [ ] **Step 1: Run standard Go tests**

Run: `go test -count=1 ./...`

Expected: PASS.

- [ ] **Step 2: Run build**

Run: `go build ./...`

Expected: PASS with no output.

- [ ] **Step 3: Run vet**

Run: `go vet ./...`

Expected: PASS with no output.

- [ ] **Step 4: Run Docker build**

Run: `docker build -t miniflux-mcp:digest-category-filters .`

Expected: PASS, image built successfully.

- [ ] **Step 5: Confirm final status**

Run: `git status --short --branch`

Expected: clean working tree on the current branch, ahead of origin by the new commits.
