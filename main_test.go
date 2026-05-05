package main

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"miniflux.app/v2/client"
)

type roundTripperFunc func(req *http.Request) (*http.Response, error)

func (fn roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestToolDefinitionsStayInSyncWithREADME(t *testing.T) {
	toolDefs := minifluxToolDefinitions(&MinifluxServer{})
	if len(toolDefs) != 49 {
		t.Fatalf("registered tools = %d, want 49", len(toolDefs))
	}

	registeredNames := make(map[string]bool, len(toolDefs))
	for _, toolDef := range toolDefs {
		if toolDef.Tool.Name == "" {
			t.Fatalf("tool with empty name: %#v", toolDef.Tool)
		}
		if registeredNames[toolDef.Tool.Name] {
			t.Fatalf("duplicate tool name registered: %s", toolDef.Tool.Name)
		}
		registeredNames[toolDef.Tool.Name] = true
		if toolDef.Handler == nil {
			t.Fatalf("tool %s has nil handler", toolDef.Tool.Name)
		}
	}

	readmeBytes, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatalf("failed to read README.md: %v", err)
	}
	readme := string(readmeBytes)

	totalRe := regexp.MustCompile(`provides \*\*(\d+) tools\*\*`)
	totalMatch := totalRe.FindStringSubmatch(readme)
	if len(totalMatch) != 2 {
		t.Fatalf("README total tool count not found")
	}
	readmeTotal, err := strconv.Atoi(totalMatch[1])
	if err != nil {
		t.Fatalf("invalid README total tool count %q: %v", totalMatch[1], err)
	}
	if readmeTotal != len(toolDefs) {
		t.Fatalf("README total = %d, registered tools = %d", readmeTotal, len(toolDefs))
	}

	readmeNames, groupCounts := readmeToolNamesAndGroupCounts(t, readme)
	if len(readmeNames) != len(toolDefs) {
		t.Fatalf("README lists %d tools, registered tools = %d", len(readmeNames), len(toolDefs))
	}
	for name := range registeredNames {
		if !readmeNames[name] {
			t.Fatalf("registered tool %q is missing from README", name)
		}
	}
	for name := range readmeNames {
		if !registeredNames[name] {
			t.Fatalf("README lists unknown tool %q", name)
		}
	}

	for group, count := range groupCounts {
		if count.declared != count.actual {
			t.Fatalf("README group %q declares %d tools, lists %d", group, count.declared, count.actual)
		}
	}
}

type readmeGroupCount struct {
	declared int
	actual   int
}

func readmeToolNamesAndGroupCounts(t *testing.T, readme string) (map[string]bool, map[string]readmeGroupCount) {
	t.Helper()

	names := map[string]bool{}
	groupCounts := map[string]readmeGroupCount{}
	headingRe := regexp.MustCompile(`^### (.+) \((\d+) tools\)$`)
	toolRe := regexp.MustCompile("^- `([^`]+)` - ")

	currentGroup := ""
	for _, line := range strings.Split(readme, "\n") {
		if match := headingRe.FindStringSubmatch(line); len(match) == 3 {
			declared, err := strconv.Atoi(match[2])
			if err != nil {
				t.Fatalf("invalid README group count %q: %v", match[2], err)
			}
			currentGroup = match[1]
			groupCounts[currentGroup] = readmeGroupCount{declared: declared}
			continue
		}

		match := toolRe.FindStringSubmatch(line)
		if len(match) != 2 {
			continue
		}
		if currentGroup == "" {
			t.Fatalf("README tool %q appears before a group heading", match[1])
		}
		if names[match[1]] {
			t.Fatalf("README lists duplicate tool %q", match[1])
		}
		names[match[1]] = true

		count := groupCounts[currentGroup]
		count.actual++
		groupCounts[currentGroup] = count
	}

	return names, groupCounts
}

func TestToolDefinitionsExposeExpectedRequiredArguments(t *testing.T) {
	toolDefs := minifluxToolDefinitions(&MinifluxServer{})
	toolsByName := make(map[string]mcp.Tool, len(toolDefs))
	for _, toolDef := range toolDefs {
		toolsByName[toolDef.Tool.Name] = toolDef.Tool
	}

	requiredByTool := map[string][]string{
		"get_feed":               {"feed_id"},
		"create_feed":            {"feed_url"},
		"delete_feed":            {"feed_id"},
		"update_feed":            {"feed_id"},
		"refresh_feed":           {"feed_id"},
		"get_feed_entries":       {"feed_id"},
		"get_feed_entry":         {"feed_id", "entry_id"},
		"import_feed_entry":      {"feed_id", "url"},
		"get_feed_icon":          {"feed_id"},
		"mark_feed_as_read":      {"feed_id"},
		"get_entry":              {"entry_id"},
		"update_entry_status":    {"entry_id", "status"},
		"toggle_starred":         {"entry_id"},
		"update_entry":           {"entry_id"},
		"save_entry":             {"entry_id"},
		"fetch_original_content": {"entry_id"},
		"mark_all_as_read":       {"user_id"},
		"create_category":        {"title"},
		"update_category":        {"category_id", "title"},
		"delete_category":        {"category_id"},
		"get_category_feeds":     {"category_id"},
		"get_category_entries":   {"category_id"},
		"get_category_entry":     {"category_id", "entry_id"},
		"mark_category_as_read":  {"category_id"},
		"refresh_category":       {"category_id"},
		"get_user_by_id":         {"user_id"},
		"get_user_by_username":   {"username"},
		"create_user":            {"username", "password"},
		"delete_user":            {"user_id"},
		"discover":               {"url"},
		"import_opml":            {"opml_content"},
		"create_api_key":         {"description"},
		"delete_api_key":         {"api_key_id"},
		"get_icon":               {"icon_id"},
		"get_enclosure":          {"enclosure_id"},
		"update_enclosure":       {"enclosure_id", "media_progression"},
	}

	for toolName, wantRequired := range requiredByTool {
		tool, ok := toolsByName[toolName]
		if !ok {
			t.Fatalf("tool %q is not registered", toolName)
		}
		if !sameStringSet(tool.InputSchema.Required, wantRequired) {
			t.Fatalf("%s required = %#v, want %#v", toolName, tool.InputSchema.Required, wantRequired)
		}
		for _, required := range wantRequired {
			if _, ok := tool.InputSchema.Properties[required]; !ok {
				t.Fatalf("%s requires %q but does not define it in properties", toolName, required)
			}
		}
	}

	for _, toolName := range []string{
		"get_feeds",
		"refresh_all_feeds",
		"get_entries",
		"get_categories",
		"get_users",
		"get_me",
		"get_version",
		"healthcheck",
		"fetch_counters",
		"get_integrations_status",
		"export",
		"flush_history",
		"get_api_keys",
	} {
		tool, ok := toolsByName[toolName]
		if !ok {
			t.Fatalf("tool %q is not registered", toolName)
		}
		if len(tool.InputSchema.Required) != 0 {
			t.Fatalf("%s required = %#v, want none", toolName, tool.InputSchema.Required)
		}
	}
}

func TestEntryFilterSchemaMatchesSupportedArguments(t *testing.T) {
	properties := entryFilterProperties()
	filter := buildEntryFilter(map[string]interface{}{
		"status":           "unread",
		"statuses":         []interface{}{"read"},
		"feed_id":          float64(1),
		"category_id":      float64(2),
		"limit":            float64(3),
		"offset":           float64(4),
		"order":            "published_at",
		"direction":        "desc",
		"starred":          true,
		"before":           float64(5),
		"after":            float64(6),
		"published_before": float64(7),
		"published_after":  float64(8),
		"changed_before":   float64(9),
		"changed_after":    float64(10),
		"before_entry_id":  float64(11),
		"after_entry_id":   float64(12),
		"search":           "rss",
		"globally_visible": true,
	})

	expectedProperties := []string{
		"status",
		"statuses",
		"feed_id",
		"category_id",
		"limit",
		"offset",
		"order",
		"direction",
		"starred",
		"before",
		"after",
		"published_before",
		"published_after",
		"changed_before",
		"changed_after",
		"before_entry_id",
		"after_entry_id",
		"search",
		"globally_visible",
	}

	if len(properties) != len(expectedProperties) {
		t.Fatalf("entry filter schema has %d properties, want %d", len(properties), len(expectedProperties))
	}
	for _, property := range expectedProperties {
		if _, ok := properties[property]; !ok {
			t.Fatalf("entry filter schema is missing %q", property)
		}
	}
	if filter.Status == "" || len(filter.Statuses) == 0 || filter.FeedID == 0 || filter.CategoryID == 0 ||
		filter.Limit == 0 || filter.Offset == 0 || filter.Order == "" || filter.Direction == "" ||
		filter.Starred == "" || filter.Before == 0 || filter.After == 0 || filter.PublishedBefore == 0 ||
		filter.PublishedAfter == 0 || filter.ChangedBefore == 0 || filter.ChangedAfter == 0 ||
		filter.BeforeEntryID == 0 || filter.AfterEntryID == 0 || filter.Search == "" || !filter.GloballyVisible {
		t.Fatalf("buildEntryFilter did not map every argument covered by the schema: %#v", filter)
	}
}

func TestImportOPMLSchemaOnlyAcceptsContent(t *testing.T) {
	tool := toolDefinitionByName(t, "import_opml")

	if !sameStringSet(tool.InputSchema.Required, []string{"opml_content"}) {
		t.Fatalf("import_opml required = %#v, want opml_content", tool.InputSchema.Required)
	}
	if _, ok := tool.InputSchema.Properties["opml_content"]; !ok {
		t.Fatalf("import_opml schema is missing opml_content")
	}
	if _, ok := tool.InputSchema.Properties["file_path"]; ok {
		t.Fatalf("import_opml schema must not accept file_path")
	}
}

func toolDefinitionByName(t *testing.T, name string) mcp.Tool {
	t.Helper()

	for _, toolDef := range minifluxToolDefinitions(&MinifluxServer{}) {
		if toolDef.Tool.Name == name {
			return toolDef.Tool
		}
	}
	t.Fatalf("tool %q is not registered", name)
	return mcp.Tool{}
}

func sameStringSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	seen := make(map[string]int, len(a))
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

func TestImportOPMLPostsProvidedContent(t *testing.T) {
	opmlContent := `<?xml version="1.0"?><opml version="2.0"><body></body></opml>`

	minifluxClient := client.NewClientWithOptions(
		"http://mf",
		client.WithHTTPClient(&http.Client{
			Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodPost {
					t.Fatalf("method = %s, want POST", req.Method)
				}
				if req.URL.String() != "http://mf/v1/import" {
					t.Fatalf("url = %s, want http://mf/v1/import", req.URL.String())
				}

				body, err := io.ReadAll(req.Body)
				if err != nil {
					t.Fatalf("failed to read request body: %v", err)
				}
				if string(body) != opmlContent {
					t.Fatalf("body = %q, want OPML content", string(body))
				}

				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewBufferString("")),
					Header:     http.Header{},
				}, nil
			}),
		}),
	)
	server := &MinifluxServer{client: minifluxClient}

	result, err := server.ImportOPML(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "import_opml",
			Arguments: map[string]interface{}{
				"opml_content": opmlContent,
			},
		},
	})

	if err != nil {
		t.Fatalf("ImportOPML returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("ImportOPML returned MCP error: %#v", result.Content)
	}
	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok || textContent.Text != "OPML imported successfully" {
		t.Fatalf("result content = %#v, want success text", result.Content)
	}
}

func TestImportOPMLRequiresContent(t *testing.T) {
	server := &MinifluxServer{}

	result, err := server.ImportOPML(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "import_opml",
			Arguments: map[string]interface{}{},
		},
	})

	if err != nil {
		t.Fatalf("ImportOPML returned error: %v", err)
	}
	if !result.IsError {
		t.Fatalf("ImportOPML IsError = false, want true")
	}
}

func TestNewMinifluxServerFromConfigChecksStartupWithDeadline(t *testing.T) {
	requestedPaths := []string{}
	timeout := 250 * time.Millisecond

	httpClient := &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			deadline, ok := req.Context().Deadline()
			if !ok {
				t.Fatalf("request to %s has no context deadline", req.URL.Path)
			}
			if time.Until(deadline) > timeout {
				t.Fatalf("deadline for %s exceeds startup timeout", req.URL.Path)
			}

			requestedPaths = append(requestedPaths, req.URL.Path)
			switch req.URL.Path {
			case "/healthcheck":
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewBufferString("OK")),
					Header:     http.Header{},
				}, nil
			case "/v1/me":
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewBufferString(`{"id":1,"username":"tester"}`)),
					Header:     http.Header{},
				}, nil
			default:
				t.Fatalf("unexpected path: %s", req.URL.Path)
			}
			return nil, nil
		}),
	}

	server, err := newMinifluxServerFromConfig("http://mf", "api-key", "", "", timeout, httpClient)
	if err != nil {
		t.Fatalf("newMinifluxServerFromConfig returned error: %v", err)
	}
	if server == nil || server.client == nil {
		t.Fatalf("server/client is nil")
	}
	if len(requestedPaths) != 2 || requestedPaths[0] != "/healthcheck" || requestedPaths[1] != "/v1/me" {
		t.Fatalf("requested paths = %#v, want healthcheck then me", requestedPaths)
	}
}

func TestNewMinifluxServerFromConfigRequiresCredentials(t *testing.T) {
	_, err := newMinifluxServerFromConfig("http://mf", "", "", "", time.Second, nil)
	if err == nil {
		t.Fatalf("error = nil, want missing credentials error")
	}
}

func TestGetFeedEntriesUsesFullEntryFilter(t *testing.T) {
	server := &MinifluxServer{
		client: client.NewClientWithOptions(
			"http://mf",
			client.WithHTTPClient(&http.Client{
				Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
					if req.URL.Path != "/v1/feeds/12/entries" {
						t.Fatalf("path = %s, want /v1/feeds/12/entries", req.URL.Path)
					}
					query := req.URL.Query()
					if query.Get("search") != "rss" {
						t.Fatalf("search = %q, want rss", query.Get("search"))
					}
					if query.Get("starred") != "1" {
						t.Fatalf("starred = %q, want 1", query.Get("starred"))
					}
					if query.Get("published_after") != "1700000000" {
						t.Fatalf("published_after = %q, want 1700000000", query.Get("published_after"))
					}
					if query.Get("order") != "published_at" || query.Get("direction") != "desc" {
						t.Fatalf("order/direction = %q/%q, want published_at/desc", query.Get("order"), query.Get("direction"))
					}
					if query.Get("feed_id") != "" {
						t.Fatalf("feed_id query = %q, want empty because feed_id is already in path", query.Get("feed_id"))
					}

					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(bytes.NewBufferString(`{"total":0,"entries":[]}`)),
						Header:     http.Header{},
					}, nil
				}),
			}),
		),
	}

	_, err := server.GetFeedEntries(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]interface{}{
				"feed_id":          float64(12),
				"search":           "rss",
				"starred":          true,
				"published_after":  float64(1700000000),
				"order":            "published_at",
				"direction":        "desc",
				"globally_visible": true,
			},
		},
	})
	if err != nil {
		t.Fatalf("GetFeedEntries returned error: %v", err)
	}
}

func TestGetCategoryEntriesUsesFullEntryFilter(t *testing.T) {
	server := &MinifluxServer{
		client: client.NewClientWithOptions(
			"http://mf",
			client.WithHTTPClient(&http.Client{
				Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
					if req.URL.Path != "/v1/categories/34/entries" {
						t.Fatalf("path = %s, want /v1/categories/34/entries", req.URL.Path)
					}
					query := req.URL.Query()
					if query.Get("after_entry_id") != "99" {
						t.Fatalf("after_entry_id = %q, want 99", query.Get("after_entry_id"))
					}
					if query.Get("changed_before") != "1800000000" {
						t.Fatalf("changed_before = %q, want 1800000000", query.Get("changed_before"))
					}
					if got := query["status"]; len(got) != 2 || got[0] != "read" || got[1] != "unread" {
						t.Fatalf("status query = %#v, want read/unread", got)
					}
					if query.Get("category_id") != "" {
						t.Fatalf("category_id query = %q, want empty because category_id is already in path", query.Get("category_id"))
					}

					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(bytes.NewBufferString(`{"total":0,"entries":[]}`)),
						Header:     http.Header{},
					}, nil
				}),
			}),
		),
	}

	_, err := server.GetCategoryEntries(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]interface{}{
				"category_id":      float64(34),
				"statuses":         []interface{}{"read", "unread"},
				"after_entry_id":   float64(99),
				"changed_before":   float64(1800000000),
				"globally_visible": true,
			},
		},
	})
	if err != nil {
		t.Fatalf("GetCategoryEntries returned error: %v", err)
	}
}

func TestBuildEntryFilterMapsSupportedArguments(t *testing.T) {
	filter := buildEntryFilter(map[string]interface{}{
		"status":           "unread",
		"statuses":         []interface{}{"read", "unread"},
		"feed_id":          float64(12),
		"category_id":      float64(34),
		"limit":            float64(20),
		"offset":           float64(5),
		"order":            "published_at",
		"direction":        "desc",
		"starred":          true,
		"before":           float64(1700000000),
		"after":            float64(1600000000),
		"published_before": float64(1700000100),
		"published_after":  float64(1600000100),
		"changed_before":   float64(1700000200),
		"changed_after":    float64(1600000200),
		"before_entry_id":  float64(99),
		"after_entry_id":   float64(88),
		"search":           "miniflux",
		"globally_visible": true,
	})

	if filter.Status != "unread" {
		t.Fatalf("Status = %q, want unread", filter.Status)
	}
	if len(filter.Statuses) != 2 || filter.Statuses[0] != "read" || filter.Statuses[1] != "unread" {
		t.Fatalf("Statuses = %#v, want read/unread", filter.Statuses)
	}
	if filter.FeedID != 12 || filter.CategoryID != 34 {
		t.Fatalf("FeedID/CategoryID = %d/%d, want 12/34", filter.FeedID, filter.CategoryID)
	}
	if filter.Limit != 20 || filter.Offset != 5 {
		t.Fatalf("Limit/Offset = %d/%d, want 20/5", filter.Limit, filter.Offset)
	}
	if filter.Order != "published_at" || filter.Direction != "desc" {
		t.Fatalf("Order/Direction = %q/%q, want published_at/desc", filter.Order, filter.Direction)
	}
	if filter.Starred != "1" {
		t.Fatalf("Starred = %q, want 1", filter.Starred)
	}
	if filter.Before != 1700000000 || filter.After != 1600000000 {
		t.Fatalf("Before/After = %d/%d, want 1700000000/1600000000", filter.Before, filter.After)
	}
	if filter.PublishedBefore != 1700000100 || filter.PublishedAfter != 1600000100 {
		t.Fatalf("PublishedBefore/PublishedAfter = %d/%d, want 1700000100/1600000100", filter.PublishedBefore, filter.PublishedAfter)
	}
	if filter.ChangedBefore != 1700000200 || filter.ChangedAfter != 1600000200 {
		t.Fatalf("ChangedBefore/ChangedAfter = %d/%d, want 1700000200/1600000200", filter.ChangedBefore, filter.ChangedAfter)
	}
	if filter.BeforeEntryID != 99 || filter.AfterEntryID != 88 {
		t.Fatalf("BeforeEntryID/AfterEntryID = %d/%d, want 99/88", filter.BeforeEntryID, filter.AfterEntryID)
	}
	if filter.Search != "miniflux" {
		t.Fatalf("Search = %q, want miniflux", filter.Search)
	}
	if !filter.GloballyVisible {
		t.Fatalf("GloballyVisible = false, want true")
	}
}

func TestBuildFeedModificationRequestMapsSupportedArguments(t *testing.T) {
	req := buildFeedModificationRequest(map[string]interface{}{
		"feed_url":                       "https://example.com/feed.xml",
		"site_url":                       "https://example.com",
		"title":                          "Example",
		"scraper_rules":                  "article",
		"rewrite_rules":                  "rewrite",
		"urlrewrite_rules":               "url rewrite",
		"blocklist_rules":                "block",
		"keeplist_rules":                 "keep",
		"block_filter_entry_rules":       "entry block",
		"keep_filter_entry_rules":        "entry keep",
		"crawler":                        true,
		"ignore_entry_updates":           true,
		"user_agent":                     "Agent",
		"cookie":                         "k=v",
		"username":                       "user",
		"password":                       "pass",
		"category_id":                    float64(7),
		"disabled":                       true,
		"ignore_http_cache":              true,
		"allow_self_signed_certificates": true,
		"fetch_via_proxy":                true,
		"hide_globally":                  true,
		"disable_http2":                  true,
		"proxy_url":                      "socks5://localhost:1080",
	})

	if req.FeedURL == nil || *req.FeedURL != "https://example.com/feed.xml" {
		t.Fatalf("FeedURL = %#v, want feed URL", req.FeedURL)
	}
	if req.SiteURL == nil || *req.SiteURL != "https://example.com" {
		t.Fatalf("SiteURL = %#v, want site URL", req.SiteURL)
	}
	if req.Title == nil || *req.Title != "Example" {
		t.Fatalf("Title = %#v, want Example", req.Title)
	}
	if req.CategoryID == nil || *req.CategoryID != 7 {
		t.Fatalf("CategoryID = %#v, want 7", req.CategoryID)
	}
	if req.Crawler == nil || !*req.Crawler || req.Disabled == nil || !*req.Disabled {
		t.Fatalf("Crawler/Disabled not mapped")
	}
	if req.ProxyURL == nil || *req.ProxyURL != "socks5://localhost:1080" {
		t.Fatalf("ProxyURL = %#v, want proxy URL", req.ProxyURL)
	}
}

func TestBuildFeedCreationRequestMapsSupportedArguments(t *testing.T) {
	req := buildFeedCreationRequest(map[string]interface{}{
		"feed_url":                       "https://example.com/feed.xml",
		"category_id":                    float64(7),
		"user_agent":                     "Agent",
		"cookie":                         "k=v",
		"username":                       "user",
		"password":                       "pass",
		"crawler":                        true,
		"ignore_entry_updates":           true,
		"disabled":                       true,
		"ignore_http_cache":              true,
		"allow_self_signed_certificates": true,
		"fetch_via_proxy":                true,
		"scraper_rules":                  "article",
		"rewrite_rules":                  "rewrite",
		"urlrewrite_rules":               "url rewrite",
		"blocklist_rules":                "block",
		"keeplist_rules":                 "keep",
		"block_filter_entry_rules":       "entry block",
		"keep_filter_entry_rules":        "entry keep",
		"hide_globally":                  true,
		"disable_http2":                  true,
		"proxy_url":                      "socks5://localhost:1080",
	})

	if req.FeedURL != "https://example.com/feed.xml" {
		t.Fatalf("FeedURL = %q, want feed URL", req.FeedURL)
	}
	if req.CategoryID != 7 {
		t.Fatalf("CategoryID = %d, want 7", req.CategoryID)
	}
	if req.UserAgent != "Agent" || req.Cookie != "k=v" {
		t.Fatalf("UserAgent/Cookie = %q/%q, want Agent/k=v", req.UserAgent, req.Cookie)
	}
	if req.Username != "user" || req.Password != "pass" {
		t.Fatalf("Username/Password = %q/%q, want user/pass", req.Username, req.Password)
	}
	if !req.Crawler || !req.IgnoreEntryUpdates || !req.Disabled || !req.IgnoreHTTPCache {
		t.Fatalf("primary booleans not mapped")
	}
	if !req.AllowSelfSignedCertificates || !req.FetchViaProxy || !req.HideGlobally || !req.DisableHTTP2 {
		t.Fatalf("advanced booleans not mapped")
	}
	if req.ScraperRules != "article" || req.RewriteRules != "rewrite" || req.UrlRewriteRules != "url rewrite" {
		t.Fatalf("scraper/rewrite rules not mapped")
	}
	if req.BlocklistRules != "block" || req.KeeplistRules != "keep" {
		t.Fatalf("blocklist/keeplist rules not mapped")
	}
	if req.BlockFilterEntryRules != "entry block" || req.KeepFilterEntryRules != "entry keep" {
		t.Fatalf("entry filter rules not mapped")
	}
	if req.ProxyURL != "socks5://localhost:1080" {
		t.Fatalf("ProxyURL = %q, want proxy URL", req.ProxyURL)
	}
}

func TestBuildEntryModificationRequestMapsSupportedArguments(t *testing.T) {
	req := buildEntryModificationRequest(map[string]interface{}{
		"title":   "New title",
		"content": "New content",
	})

	if req.Title == nil || *req.Title != "New title" {
		t.Fatalf("Title = %#v, want New title", req.Title)
	}
	if req.Content == nil || *req.Content != "New content" {
		t.Fatalf("Content = %#v, want New content", req.Content)
	}
}

func TestBuildImportFeedEntryPayloadMapsSupportedArguments(t *testing.T) {
	payload := buildImportFeedEntryPayload(map[string]interface{}{
		"url":          "https://example.com/article",
		"title":        "Entry",
		"author":       "Author",
		"content":      "<p>Content</p>",
		"published_at": float64(1736200000),
		"status":       "unread",
		"starred":      true,
		"tags":         []interface{}{"go", "rss"},
		"external_id":  "unique-id",
		"comments_url": "https://example.com/article#comments",
	})

	if payload["url"] != "https://example.com/article" || payload["title"] != "Entry" {
		t.Fatalf("payload URL/title not mapped: %#v", payload)
	}
	if payload["published_at"] != int64(1736200000) {
		t.Fatalf("published_at = %#v, want int64 timestamp", payload["published_at"])
	}
	if payload["starred"] != true {
		t.Fatalf("starred = %#v, want true", payload["starred"])
	}
	tags, ok := payload["tags"].([]string)
	if !ok || len(tags) != 2 || tags[0] != "go" || tags[1] != "rss" {
		t.Fatalf("tags = %#v, want go/rss", payload["tags"])
	}
}
