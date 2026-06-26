package main

import (
	"bytes"
	"context"
	"encoding/json"
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
	if len(toolDefs) != 51 {
		t.Fatalf("registered tools = %d, want 51", len(toolDefs))
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

func TestToolDefinitionsExposeAgentFacingDescriptions(t *testing.T) {
	for _, toolDef := range minifluxToolDefinitions(&MinifluxServer{}) {
		if strings.TrimSpace(toolDef.Tool.Description) == "" {
			t.Fatalf("tool %q has no description", toolDef.Tool.Name)
		}
		for propertyName, propertySchema := range toolDef.Tool.InputSchema.Properties {
			assertSchemaDescription(t, toolDef.Tool.Name, propertyName, propertySchema)
		}
	}
}

func assertSchemaDescription(t *testing.T, toolName, propertyPath string, schema interface{}) {
	t.Helper()

	schemaMap, ok := schema.(map[string]interface{})
	if !ok {
		t.Fatalf("%s.%s schema = %#v, want object schema", toolName, propertyPath, schema)
	}
	if description, ok := schemaMap["description"].(string); !ok || strings.TrimSpace(description) == "" {
		t.Fatalf("%s.%s has no agent-facing description", toolName, propertyPath)
	}
	if items, ok := schemaMap["items"]; ok {
		itemMap, ok := items.(map[string]interface{})
		if !ok {
			t.Fatalf("%s.%s.items schema = %#v, want object schema", toolName, propertyPath, items)
		}
		if _, hasProperties := itemMap["properties"]; hasProperties {
			assertSchemaDescription(t, toolName, propertyPath+".items", items)
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
		"get_daily_digest":       {"since"},
		"update_entry_status":    {"entry_id", "status"},
		"update_entries_status":  {"entry_ids", "status"},
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

func TestGetDailyDigestUsesCallerProvidedSince(t *testing.T) {
	requests := make([]*http.Request, 0, 1)
	server := &MinifluxServer{
		client: client.NewClientWithOptions(
			"http://mf",
			client.WithHTTPClient(&http.Client{
				Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
					requests = append(requests, req)
					if req.Method != http.MethodGet {
						t.Fatalf("method = %s, want GET", req.Method)
					}
					if req.URL.Path != "/v1/entries" {
						t.Fatalf("path = %s, want /v1/entries", req.URL.Path)
					}

					return &http.Response{
						StatusCode: http.StatusOK,
						Body: io.NopCloser(strings.NewReader(`{
							"total": 1,
							"entries": [{
								"id": 42,
								"title": "Morning news",
								"url": "https://example.com/news",
								"status": "unread",
								"content": "<p>This is a complete feed story with enough useful text for a summary.</p>",
								"published_at": "2026-05-05T01:30:00Z",
								"changed_at": "2026-05-05T01:40:00Z",
								"feed_id": 7,
								"feed": {"id": 7, "title": "Example Feed", "category": {"id": 3, "title": "News"}}
							}]
						}`)),
						Header: http.Header{},
					}, nil
				}),
			}),
		),
	}

	result, err := server.GetDailyDigest(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{Arguments: map[string]interface{}{
			"since":        float64(1777910400),
			"content_mode": "feed",
		}},
	})
	if err != nil {
		t.Fatalf("handler returned transport error: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatalf("result = %#v, want non-error", result)
	}
	if len(requests) != 1 {
		t.Fatalf("requests = %d, want 1", len(requests))
	}

	query := requests[0].URL.Query()
	if got := query.Get("status"); got != "unread" {
		t.Fatalf("status query = %q, want unread", got)
	}
	if got := query.Get("published_after"); got != "1777910400" {
		t.Fatalf("published_after query = %q, want 1777910400", got)
	}
	if got := query.Get("order"); got != "published_at" {
		t.Fatalf("order query = %q, want published_at", got)
	}
	if got := query.Get("direction"); got != "desc" {
		t.Fatalf("direction query = %q, want desc", got)
	}
	if got := query.Get("limit"); got != "50" {
		t.Fatalf("limit query = %q, want 50", got)
	}

	var response dailyDigestResponse
	unmarshalToolResultText(t, result, &response)
	if response.Count != 1 || len(response.AckEntryIDs) != 1 || response.AckEntryIDs[0] != 42 {
		t.Fatalf("response count/ack ids = %d/%#v, want 1/[42]", response.Count, response.AckEntryIDs)
	}
	if response.Entries[0].Content != "<p>This is a complete feed story with enough useful text for a summary.</p>" {
		t.Fatalf("content = %q, want feed content as returned by Miniflux", response.Entries[0].Content)
	}
	if response.Entries[0].ContentSource != "feed" {
		t.Fatalf("content_source = %q, want feed", response.Entries[0].ContentSource)
	}
}

func TestGetDailyDigestScrapesOnlyWhenExplicitlyRequestedAndTruncates(t *testing.T) {
	requestPaths := make([]string, 0, 2)
	server := &MinifluxServer{
		client: client.NewClientWithOptions(
			"http://mf",
			client.WithHTTPClient(&http.Client{
				Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
					requestPaths = append(requestPaths, req.URL.Path)
					switch req.URL.Path {
					case "/v1/entries":
						return &http.Response{
							StatusCode: http.StatusOK,
							Body: io.NopCloser(strings.NewReader(`{
								"total": 1,
								"entries": [{
									"id": 42,
									"title": "Short feed",
									"url": "https://example.com/news",
									"status": "unread",
									"content": "<p>Short.</p>",
									"published_at": "2026-05-05T01:30:00Z"
								}]
							}`)),
							Header: http.Header{},
						}, nil
					case "/v1/entries/42/fetch-content":
						return &http.Response{
							StatusCode: http.StatusOK,
							Body:       io.NopCloser(strings.NewReader(`{"content":"<article>Scraped content is much longer than the feed summary.</article>"}`)),
							Header:     http.Header{},
						}, nil
					default:
						t.Fatalf("unexpected path: %s", req.URL.Path)
					}
					return nil, nil
				}),
			}),
		),
	}

	result, err := server.GetDailyDigest(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{Arguments: map[string]interface{}{
			"since":              float64(1777910400),
			"content_mode":       "scrape_all",
			"max_content_length": float64(36),
		}},
	})
	if err != nil {
		t.Fatalf("handler returned transport error: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatalf("result = %#v, want non-error", result)
	}
	if !sameStringSet(requestPaths, []string{"/v1/entries", "/v1/entries/42/fetch-content"}) {
		t.Fatalf("request paths = %#v, want entries and fetch-content", requestPaths)
	}

	var response dailyDigestResponse
	unmarshalToolResultText(t, result, &response)
	entry := response.Entries[0]
	if entry.Content != "<article>Scraped content is much lon" {
		t.Fatalf("content = %q, want truncated scraped content", entry.Content)
	}
	if entry.ContentSource != "scraped" {
		t.Fatalf("content_source = %q, want scraped", entry.ContentSource)
	}
	if !entry.ContentTruncated {
		t.Fatalf("content_truncated = false, want true")
	}
	if !entry.ContentAvailable {
		t.Fatalf("content_available = false, want true")
	}
}

func TestGetDailyDigestRequiresSince(t *testing.T) {
	server := &MinifluxServer{}

	result, err := server.GetDailyDigest(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{Arguments: map[string]interface{}{}},
	})
	if err != nil {
		t.Fatalf("handler returned transport error: %v", err)
	}
	assertToolErrorContains(t, result, "since is required")
}

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

func TestHandlersRejectMissingRequiredArgumentsBeforeCallingClient(t *testing.T) {
	server := &MinifluxServer{}
	tests := []struct {
		name    string
		handler func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)
		wantErr string
	}{
		{"GetFeed", server.GetFeed, "feed_id is required"},
		{"DeleteFeed", server.DeleteFeed, "feed_id is required"},
		{"UpdateFeed", server.UpdateFeed, "feed_id is required"},
		{"ImportFeedEntry", server.ImportFeedEntry, "feed_id and url are required"},
		{"GetFeedEntries", server.GetFeedEntries, "feed_id is required"},
		{"GetFeedEntry", server.GetFeedEntry, "feed_id and entry_id are required"},
		{"GetFeedIcon", server.GetFeedIcon, "feed_id is required"},
		{"MarkFeedAsRead", server.MarkFeedAsRead, "feed_id is required"},
		{"GetEntry", server.GetEntry, "entry_id is required"},
		{"UpdateEntryStatus", server.UpdateEntryStatus, "entry_id and status are required"},
		{"CreateFeed", server.CreateFeed, "feed_url is required"},
		{"RefreshFeed", server.RefreshFeed, "feed_id is required"},
		{"GetUserByID", server.GetUserByID, "user_id is required"},
		{"GetUserByUsername", server.GetUserByUsername, "username is required"},
		{"CreateUser", server.CreateUser, "username and password are required"},
		{"DeleteUser", server.DeleteUser, "user_id is required"},
		{"CreateCategory", server.CreateCategory, "title is required"},
		{"UpdateCategory", server.UpdateCategory, "category_id and title are required"},
		{"DeleteCategory", server.DeleteCategory, "category_id is required"},
		{"GetCategoryFeeds", server.GetCategoryFeeds, "category_id is required"},
		{"GetCategoryEntries", server.GetCategoryEntries, "category_id is required"},
		{"GetCategoryEntry", server.GetCategoryEntry, "category_id and entry_id are required"},
		{"MarkCategoryAsRead", server.MarkCategoryAsRead, "category_id is required"},
		{"RefreshCategory", server.RefreshCategory, "category_id is required"},
		{"ToggleStarred", server.ToggleStarred, "entry_id is required"},
		{"SaveEntry", server.SaveEntry, "entry_id is required"},
		{"UpdateEntry", server.UpdateEntry, "entry_id is required"},
		{"FetchEntryOriginalContent", server.FetchEntryOriginalContent, "entry_id is required"},
		{"MarkAllAsRead", server.MarkAllAsRead, "user_id is required"},
		{"Discover", server.Discover, "url is required"},
		{"ImportOPML", server.ImportOPML, "opml_content is required"},
		{"CreateAPIKey", server.CreateAPIKey, "description is required"},
		{"DeleteAPIKey", server.DeleteAPIKey, "api_key_id is required"},
		{"GetIcon", server.GetIcon, "icon_id is required"},
		{"GetEnclosure", server.GetEnclosure, "enclosure_id is required"},
		{"UpdateEnclosure", server.UpdateEnclosure, "enclosure_id and media_progression are required"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.handler(context.Background(), mcp.CallToolRequest{})
			if err != nil {
				t.Fatalf("handler returned transport error: %v", err)
			}
			assertToolErrorContains(t, result, tt.wantErr)
		})
	}
}

func TestHandlersRejectInvalidArgumentContainers(t *testing.T) {
	server := &MinifluxServer{}
	tests := []struct {
		name    string
		handler func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)
	}{
		{"GetFeed", server.GetFeed},
		{"CreateFeed", server.CreateFeed},
		{"GetEntry", server.GetEntry},
		{"CreateCategory", server.CreateCategory},
		{"GetUserByID", server.GetUserByID},
		{"Discover", server.Discover},
		{"ImportOPML", server.ImportOPML},
		{"CreateAPIKey", server.CreateAPIKey},
		{"GetIcon", server.GetIcon},
		{"UpdateEnclosure", server.UpdateEnclosure},
	}

	request := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: []interface{}{"not", "an", "object"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.handler(context.Background(), request)
			if err != nil {
				t.Fatalf("handler returned transport error: %v", err)
			}
			assertToolErrorContains(t, result, "Invalid arguments format")
		})
	}
}

func assertToolErrorContains(t *testing.T, result *mcp.CallToolResult, want string) {
	t.Helper()

	if result == nil {
		t.Fatalf("result is nil, want MCP tool error containing %q", want)
	}
	if !result.IsError {
		t.Fatalf("result IsError = false, want true; content = %#v", result.Content)
	}
	if len(result.Content) == 0 {
		t.Fatalf("error result has no content")
	}
	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("error content = %#v, want mcp.TextContent", result.Content[0])
	}
	if !strings.Contains(textContent.Text, want) {
		t.Fatalf("error text = %q, want to contain %q", textContent.Text, want)
	}
}

func unmarshalToolResultText(t *testing.T, result *mcp.CallToolResult, target interface{}) {
	t.Helper()

	if result == nil {
		t.Fatalf("result is nil")
	}
	if len(result.Content) == 0 {
		t.Fatalf("result has no content")
	}
	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("result content = %#v, want mcp.TextContent", result.Content[0])
	}
	if err := json.Unmarshal([]byte(textContent.Text), target); err != nil {
		t.Fatalf("failed to unmarshal result text: %v\n%s", err, textContent.Text)
	}
}

func TestCategoryHandlersSendExpectedRequests(t *testing.T) {
	tests := []struct {
		name         string
		handler      func(*MinifluxServer) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)
		args         map[string]interface{}
		wantMethod   string
		wantPath     string
		wantJSONBody map[string]interface{}
		responseCode int
		responseBody string
	}{
		{
			name: "create category",
			handler: func(s *MinifluxServer) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				return s.CreateCategory
			},
			args:         map[string]interface{}{"title": "Reading"},
			wantMethod:   http.MethodPost,
			wantPath:     "/v1/categories",
			wantJSONBody: map[string]interface{}{"title": "Reading", "hide_globally": false},
			responseCode: http.StatusOK,
			responseBody: `{"id":7,"title":"Reading"}`,
		},
		{
			name: "update category",
			handler: func(s *MinifluxServer) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				return s.UpdateCategory
			},
			args:         map[string]interface{}{"category_id": float64(7), "title": "Updated"},
			wantMethod:   http.MethodPut,
			wantPath:     "/v1/categories/7",
			wantJSONBody: map[string]interface{}{"title": "Updated", "hide_globally": nil},
			responseCode: http.StatusOK,
			responseBody: `{"id":7,"title":"Updated"}`,
		},
		{
			name: "delete category",
			handler: func(s *MinifluxServer) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				return s.DeleteCategory
			},
			args:         map[string]interface{}{"category_id": float64(7)},
			wantMethod:   http.MethodDelete,
			wantPath:     "/v1/categories/7",
			responseCode: http.StatusNoContent,
		},
		{
			name: "mark category as read",
			handler: func(s *MinifluxServer) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				return s.MarkCategoryAsRead
			},
			args:         map[string]interface{}{"category_id": float64(7)},
			wantMethod:   http.MethodPut,
			wantPath:     "/v1/categories/7/mark-all-as-read",
			responseCode: http.StatusNoContent,
		},
		{
			name: "refresh category",
			handler: func(s *MinifluxServer) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				return s.RefreshCategory
			},
			args:         map[string]interface{}{"category_id": float64(7)},
			wantMethod:   http.MethodPut,
			wantPath:     "/v1/categories/7/refresh",
			responseCode: http.StatusNoContent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := newHTTPAssertionServer(t, tt.wantMethod, tt.wantPath, tt.wantJSONBody, tt.responseCode, tt.responseBody)
			result, err := tt.handler(server)(context.Background(), mcp.CallToolRequest{
				Params: mcp.CallToolParams{Arguments: tt.args},
			})
			if err != nil {
				t.Fatalf("handler returned transport error: %v", err)
			}
			if result == nil || result.IsError {
				t.Fatalf("result = %#v, want non-error", result)
			}
		})
	}
}

func TestUserAndAPIKeyHandlersSendExpectedRequests(t *testing.T) {
	tests := []struct {
		name         string
		handler      func(*MinifluxServer) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)
		args         map[string]interface{}
		wantMethod   string
		wantPath     string
		wantJSONBody map[string]interface{}
		responseCode int
		responseBody string
	}{
		{
			name: "create user",
			handler: func(s *MinifluxServer) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				return s.CreateUser
			},
			args:       map[string]interface{}{"username": "alice", "password": "secret", "is_admin": true},
			wantMethod: http.MethodPost,
			wantPath:   "/v1/users",
			wantJSONBody: map[string]interface{}{
				"username":          "alice",
				"password":          "secret",
				"is_admin":          true,
				"google_id":         "",
				"openid_connect_id": "",
			},
			responseCode: http.StatusOK,
			responseBody: `{"id":3,"username":"alice","is_admin":true}`,
		},
		{
			name: "delete user",
			handler: func(s *MinifluxServer) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				return s.DeleteUser
			},
			args:         map[string]interface{}{"user_id": float64(3)},
			wantMethod:   http.MethodDelete,
			wantPath:     "/v1/users/3",
			responseCode: http.StatusNoContent,
		},
		{
			name: "create api key",
			handler: func(s *MinifluxServer) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				return s.CreateAPIKey
			},
			args:         map[string]interface{}{"description": "MCP"},
			wantMethod:   http.MethodPost,
			wantPath:     "/v1/api-keys",
			wantJSONBody: map[string]interface{}{"description": "MCP"},
			responseCode: http.StatusOK,
			responseBody: `{"id":9,"description":"MCP","token":"token"}`,
		},
		{
			name: "delete api key",
			handler: func(s *MinifluxServer) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				return s.DeleteAPIKey
			},
			args:         map[string]interface{}{"api_key_id": float64(9)},
			wantMethod:   http.MethodDelete,
			wantPath:     "/v1/api-keys/9",
			responseCode: http.StatusNoContent,
		},
		{
			name: "mark all as read",
			handler: func(s *MinifluxServer) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				return s.MarkAllAsRead
			},
			args:         map[string]interface{}{"user_id": float64(3)},
			wantMethod:   http.MethodPut,
			wantPath:     "/v1/users/3/mark-all-as-read",
			responseCode: http.StatusNoContent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := newHTTPAssertionServer(t, tt.wantMethod, tt.wantPath, tt.wantJSONBody, tt.responseCode, tt.responseBody)
			result, err := tt.handler(server)(context.Background(), mcp.CallToolRequest{
				Params: mcp.CallToolParams{Arguments: tt.args},
			})
			if err != nil {
				t.Fatalf("handler returned transport error: %v", err)
			}
			if result == nil || result.IsError {
				t.Fatalf("result = %#v, want non-error", result)
			}
		})
	}
}

func TestEntryAndEnclosureMutationHandlersSendExpectedRequests(t *testing.T) {
	tests := []struct {
		name         string
		handler      func(*MinifluxServer) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)
		args         map[string]interface{}
		wantMethod   string
		wantPath     string
		wantJSONBody map[string]interface{}
		responseCode int
		responseBody string
	}{
		{
			name: "update entry status",
			handler: func(s *MinifluxServer) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				return s.UpdateEntryStatus
			},
			args:         map[string]interface{}{"entry_id": float64(42), "status": "read"},
			wantMethod:   http.MethodPut,
			wantPath:     "/v1/entries",
			wantJSONBody: map[string]interface{}{"entry_ids": []interface{}{float64(42)}, "status": "read"},
			responseCode: http.StatusNoContent,
		},
		{
			name: "update entries status",
			handler: func(s *MinifluxServer) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				return s.UpdateEntriesStatus
			},
			args:         map[string]interface{}{"entry_ids": []interface{}{float64(42), float64(43)}, "status": "read"},
			wantMethod:   http.MethodPut,
			wantPath:     "/v1/entries",
			wantJSONBody: map[string]interface{}{"entry_ids": []interface{}{float64(42), float64(43)}, "status": "read"},
			responseCode: http.StatusNoContent,
		},
		{
			name: "toggle starred",
			handler: func(s *MinifluxServer) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				return s.ToggleStarred
			},
			args:         map[string]interface{}{"entry_id": float64(42)},
			wantMethod:   http.MethodPut,
			wantPath:     "/v1/entries/42/star",
			responseCode: http.StatusNoContent,
		},
		{
			name: "save entry",
			handler: func(s *MinifluxServer) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				return s.SaveEntry
			},
			args:         map[string]interface{}{"entry_id": float64(42)},
			wantMethod:   http.MethodPost,
			wantPath:     "/v1/entries/42/save",
			responseCode: http.StatusNoContent,
		},
		{
			name: "update entry",
			handler: func(s *MinifluxServer) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				return s.UpdateEntry
			},
			args:         map[string]interface{}{"entry_id": float64(42), "title": "New", "content": "Body"},
			wantMethod:   http.MethodPut,
			wantPath:     "/v1/entries/42",
			wantJSONBody: map[string]interface{}{"title": "New", "content": "Body"},
			responseCode: http.StatusOK,
			responseBody: `{"id":42,"title":"New","content":"Body"}`,
		},
		{
			name: "update enclosure",
			handler: func(s *MinifluxServer) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				return s.UpdateEnclosure
			},
			args:         map[string]interface{}{"enclosure_id": float64(99), "media_progression": float64(120)},
			wantMethod:   http.MethodPut,
			wantPath:     "/v1/enclosures/99",
			wantJSONBody: map[string]interface{}{"media_progression": float64(120)},
			responseCode: http.StatusNoContent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := newHTTPAssertionServer(t, tt.wantMethod, tt.wantPath, tt.wantJSONBody, tt.responseCode, tt.responseBody)
			result, err := tt.handler(server)(context.Background(), mcp.CallToolRequest{
				Params: mcp.CallToolParams{Arguments: tt.args},
			})
			if err != nil {
				t.Fatalf("handler returned transport error: %v", err)
			}
			if result == nil || result.IsError {
				t.Fatalf("result = %#v, want non-error", result)
			}
		})
	}
}

func TestFeedMutationHandlersSendExpectedRequests(t *testing.T) {
	tests := []struct {
		name         string
		handler      func(*MinifluxServer) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)
		args         map[string]interface{}
		wantMethod   string
		wantPath     string
		wantJSONBody map[string]interface{}
		responseCode int
		responseBody string
	}{
		{
			name: "create feed",
			handler: func(s *MinifluxServer) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				return s.CreateFeed
			},
			args: map[string]interface{}{
				"feed_url":      "https://example.com/feed.xml",
				"category_id":   float64(7),
				"crawler":       true,
				"hide_globally": true,
				"proxy_url":     "socks5://localhost:1080",
			},
			wantMethod: http.MethodPost,
			wantPath:   "/v1/feeds",
			wantJSONBody: map[string]interface{}{
				"feed_url":      "https://example.com/feed.xml",
				"category_id":   float64(7),
				"crawler":       true,
				"hide_globally": true,
				"proxy_url":     "socks5://localhost:1080",
			},
			responseCode: http.StatusOK,
			responseBody: `{"feed_id":12}`,
		},
		{
			name: "update feed",
			handler: func(s *MinifluxServer) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				return s.UpdateFeed
			},
			args: map[string]interface{}{
				"feed_id":       float64(12),
				"title":         "Updated",
				"category_id":   float64(7),
				"hide_globally": false,
			},
			wantMethod: http.MethodPut,
			wantPath:   "/v1/feeds/12",
			wantJSONBody: map[string]interface{}{
				"title":         "Updated",
				"category_id":   float64(7),
				"hide_globally": false,
			},
			responseCode: http.StatusOK,
			responseBody: `{"id":12,"title":"Updated"}`,
		},
		{
			name: "import feed entry",
			handler: func(s *MinifluxServer) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				return s.ImportFeedEntry
			},
			args: map[string]interface{}{
				"feed_id":      float64(12),
				"url":          "https://example.com/article",
				"title":        "Article",
				"published_at": float64(1736200000),
				"starred":      true,
				"tags":         []interface{}{"go", "rss"},
			},
			wantMethod: http.MethodPost,
			wantPath:   "/v1/feeds/12/entries/import",
			wantJSONBody: map[string]interface{}{
				"url":          "https://example.com/article",
				"title":        "Article",
				"published_at": float64(1736200000),
				"starred":      true,
				"tags":         []interface{}{"go", "rss"},
			},
			responseCode: http.StatusOK,
			responseBody: `{"id":42}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := newHTTPBodySubsetAssertionServer(t, tt.wantMethod, tt.wantPath, tt.wantJSONBody, tt.responseCode, tt.responseBody)
			result, err := tt.handler(server)(context.Background(), mcp.CallToolRequest{
				Params: mcp.CallToolParams{Arguments: tt.args},
			})
			if err != nil {
				t.Fatalf("handler returned transport error: %v", err)
			}
			if result == nil || result.IsError {
				t.Fatalf("result = %#v, want non-error", result)
			}
		})
	}
}

func TestFeedCommandHandlersSendExpectedRequests(t *testing.T) {
	tests := []struct {
		name         string
		handler      func(*MinifluxServer) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)
		args         map[string]interface{}
		wantMethod   string
		wantPath     string
		responseCode int
	}{
		{
			name: "delete feed",
			handler: func(s *MinifluxServer) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				return s.DeleteFeed
			},
			args:         map[string]interface{}{"feed_id": float64(12)},
			wantMethod:   http.MethodDelete,
			wantPath:     "/v1/feeds/12",
			responseCode: http.StatusNoContent,
		},
		{
			name: "refresh feed",
			handler: func(s *MinifluxServer) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				return s.RefreshFeed
			},
			args:         map[string]interface{}{"feed_id": float64(12)},
			wantMethod:   http.MethodPut,
			wantPath:     "/v1/feeds/12/refresh",
			responseCode: http.StatusNoContent,
		},
		{
			name: "refresh all feeds",
			handler: func(s *MinifluxServer) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				return s.RefreshAllFeeds
			},
			args:         nil,
			wantMethod:   http.MethodPut,
			wantPath:     "/v1/feeds/refresh",
			responseCode: http.StatusNoContent,
		},
		{
			name: "mark feed as read",
			handler: func(s *MinifluxServer) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				return s.MarkFeedAsRead
			},
			args:         map[string]interface{}{"feed_id": float64(12)},
			wantMethod:   http.MethodPut,
			wantPath:     "/v1/feeds/12/mark-all-as-read",
			responseCode: http.StatusNoContent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := newHTTPAssertionServer(t, tt.wantMethod, tt.wantPath, nil, tt.responseCode, "")
			result, err := tt.handler(server)(context.Background(), mcp.CallToolRequest{
				Params: mcp.CallToolParams{Arguments: tt.args},
			})
			if err != nil {
				t.Fatalf("handler returned transport error: %v", err)
			}
			if result == nil || result.IsError {
				t.Fatalf("result = %#v, want non-error", result)
			}
		})
	}
}

func TestReadOnlyHandlersSendExpectedRequests(t *testing.T) {
	tests := []struct {
		name         string
		handler      func(*MinifluxServer) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)
		args         map[string]interface{}
		wantPath     string
		responseBody string
	}{
		{"get feeds", func(s *MinifluxServer) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return s.GetFeeds
		}, nil, "/v1/feeds", `[]`},
		{"get feed", func(s *MinifluxServer) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return s.GetFeed
		}, map[string]interface{}{"feed_id": float64(12)}, "/v1/feeds/12", `{"id":12,"title":"Feed"}`},
		{"get feed entry", func(s *MinifluxServer) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return s.GetFeedEntry
		}, map[string]interface{}{"feed_id": float64(12), "entry_id": float64(42)}, "/v1/feeds/12/entries/42", `{"id":42,"title":"Entry"}`},
		{"get feed icon", func(s *MinifluxServer) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return s.GetFeedIcon
		}, map[string]interface{}{"feed_id": float64(12)}, "/v1/feeds/12/icon", `{"id":5,"mime_type":"image/png","data":"abc"}`},
		{"get entries", func(s *MinifluxServer) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return s.GetEntries
		}, nil, "/v1/entries", `{"total":0,"entries":[]}`},
		{"get entry", func(s *MinifluxServer) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return s.GetEntry
		}, map[string]interface{}{"entry_id": float64(42)}, "/v1/entries/42", `{"id":42,"title":"Entry"}`},
		{"fetch original content", func(s *MinifluxServer) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return s.FetchEntryOriginalContent
		}, map[string]interface{}{"entry_id": float64(42)}, "/v1/entries/42/fetch-content", `{"content":"<article>full</article>"}`},
		{"get categories", func(s *MinifluxServer) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return s.GetCategories
		}, nil, "/v1/categories", `[]`},
		{"get category feeds", func(s *MinifluxServer) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return s.GetCategoryFeeds
		}, map[string]interface{}{"category_id": float64(7)}, "/v1/categories/7/feeds", `[]`},
		{"get category entry", func(s *MinifluxServer) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return s.GetCategoryEntry
		}, map[string]interface{}{"category_id": float64(7), "entry_id": float64(42)}, "/v1/categories/7/entries/42", `{"id":42,"title":"Entry"}`},
		{"get users", func(s *MinifluxServer) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return s.GetUsers
		}, nil, "/v1/users", `[]`},
		{"get me", func(s *MinifluxServer) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return s.GetMe
		}, nil, "/v1/me", `{"id":1,"username":"me"}`},
		{"get user by id", func(s *MinifluxServer) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return s.GetUserByID
		}, map[string]interface{}{"user_id": float64(3)}, "/v1/users/3", `{"id":3,"username":"alice"}`},
		{"get user by username", func(s *MinifluxServer) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return s.GetUserByUsername
		}, map[string]interface{}{"username": "alice"}, "/v1/users/alice", `{"id":3,"username":"alice"}`},
		{"get version", func(s *MinifluxServer) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return s.GetVersion
		}, nil, "/v1/version", `{"version":"2.2.19"}`},
		{"fetch counters", func(s *MinifluxServer) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return s.FetchCounters
		}, nil, "/v1/feeds/counters", `{"reads":{"12":1},"unreads":{"12":2}}`},
		{"get integrations status", func(s *MinifluxServer) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return s.GetIntegrationsStatus
		}, nil, "/v1/integrations/status", `{"has_integrations":true}`},
		{"get api keys", func(s *MinifluxServer) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return s.GetAPIKeys
		}, nil, "/v1/api-keys", `[]`},
		{"get icon", func(s *MinifluxServer) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return s.GetIcon
		}, map[string]interface{}{"icon_id": float64(5)}, "/v1/icons/5", `{"id":5,"mime_type":"image/png","data":"abc"}`},
		{"get enclosure", func(s *MinifluxServer) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return s.GetEnclosure
		}, map[string]interface{}{"enclosure_id": float64(99)}, "/v1/enclosures/99", `{"id":99,"entry_id":42,"url":"https://example.com/audio.mp3"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := newHTTPAssertionServer(t, http.MethodGet, tt.wantPath, nil, http.StatusOK, tt.responseBody)
			result, err := tt.handler(server)(context.Background(), mcp.CallToolRequest{
				Params: mcp.CallToolParams{Arguments: tt.args},
			})
			if err != nil {
				t.Fatalf("handler returned transport error: %v", err)
			}
			if result == nil || result.IsError {
				t.Fatalf("result = %#v, want non-error", result)
			}
		})
	}
}

func TestUtilityCommandHandlersSendExpectedRequests(t *testing.T) {
	tests := []struct {
		name         string
		handler      func(*MinifluxServer) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)
		wantMethod   string
		wantPath     string
		responseCode int
		responseBody string
	}{
		{
			name: "healthcheck",
			handler: func(s *MinifluxServer) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				return s.Healthcheck
			},
			wantMethod:   http.MethodGet,
			wantPath:     "/healthcheck",
			responseCode: http.StatusOK,
			responseBody: "OK",
		},
		{
			name: "export",
			handler: func(s *MinifluxServer) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				return s.Export
			},
			wantMethod:   http.MethodGet,
			wantPath:     "/v1/export",
			responseCode: http.StatusOK,
			responseBody: `<?xml version="1.0"?><opml version="2.0"></opml>`,
		},
		{
			name: "flush history",
			handler: func(s *MinifluxServer) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				return s.FlushHistory
			},
			wantMethod:   http.MethodPut,
			wantPath:     "/v1/flush-history",
			responseCode: http.StatusNoContent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := newHTTPAssertionServer(t, tt.wantMethod, tt.wantPath, nil, tt.responseCode, tt.responseBody)
			result, err := tt.handler(server)(context.Background(), mcp.CallToolRequest{})
			if err != nil {
				t.Fatalf("handler returned transport error: %v", err)
			}
			if result == nil || result.IsError {
				t.Fatalf("result = %#v, want non-error", result)
			}
		})
	}
}

func TestDiscoverSendsExpectedRequest(t *testing.T) {
	server := newHTTPAssertionServer(
		t,
		http.MethodPost,
		"/v1/discover",
		map[string]interface{}{"url": "https://example.com"},
		http.StatusOK,
		`[{"title":"Example","url":"https://example.com/feed.xml","type":"rss"}]`,
	)

	result, err := server.Discover(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]interface{}{"url": "https://example.com"},
		},
	})
	if err != nil {
		t.Fatalf("Discover returned transport error: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatalf("result = %#v, want non-error", result)
	}
}

func newHTTPAssertionServer(t *testing.T, wantMethod, wantPath string, wantJSONBody map[string]interface{}, responseCode int, responseBody string) *MinifluxServer {
	t.Helper()
	return newHTTPAssertionServerWithBodyCheck(t, wantMethod, wantPath, responseCode, responseBody, func(req *http.Request) {
		assertJSONBody(t, req, wantJSONBody)
	})
}

func newHTTPBodySubsetAssertionServer(t *testing.T, wantMethod, wantPath string, wantJSONBodySubset map[string]interface{}, responseCode int, responseBody string) *MinifluxServer {
	t.Helper()
	return newHTTPAssertionServerWithBodyCheck(t, wantMethod, wantPath, responseCode, responseBody, func(req *http.Request) {
		assertJSONBodyContains(t, req, wantJSONBodySubset)
	})
}

func newHTTPAssertionServerWithBodyCheck(t *testing.T, wantMethod, wantPath string, responseCode int, responseBody string, checkBody func(*http.Request)) *MinifluxServer {
	t.Helper()

	return &MinifluxServer{
		client: client.NewClientWithOptions(
			"http://mf",
			client.WithHTTPClient(&http.Client{
				Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
					if req.Method != wantMethod {
						t.Fatalf("method = %s, want %s", req.Method, wantMethod)
					}
					if req.URL.Path != wantPath {
						t.Fatalf("path = %s, want %s", req.URL.Path, wantPath)
					}
					checkBody(req)

					return &http.Response{
						StatusCode: responseCode,
						Body:       io.NopCloser(bytes.NewBufferString(responseBody)),
						Header:     http.Header{},
					}, nil
				}),
			}),
		),
	}
}

func assertJSONBodyContains(t *testing.T, req *http.Request, wantSubset map[string]interface{}) {
	t.Helper()

	if req.Body == nil {
		t.Fatalf("body is nil, want JSON body containing %#v", wantSubset)
	}
	body, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("failed to read request body: %v", err)
	}
	var got map[string]interface{}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("failed to decode JSON body %q: %v", body, err)
	}
	for key, wantValue := range wantSubset {
		gotValue, ok := got[key]
		if !ok {
			t.Fatalf("body = %#v, want key %q", got, key)
		}
		if !jsonValuesEqual(gotValue, wantValue) {
			t.Fatalf("body[%q] = %#v, want %#v; full body = %#v", key, gotValue, wantValue, got)
		}
	}
}

func assertJSONBody(t *testing.T, req *http.Request, want map[string]interface{}) {
	t.Helper()

	if want == nil {
		if req.Body == nil {
			return
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}
		if len(body) != 0 {
			t.Fatalf("body = %s, want empty body", body)
		}
		return
	}
	if req.Body == nil {
		t.Fatalf("body is nil, want JSON body %#v", want)
	}
	body, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("failed to read request body: %v", err)
	}
	var got map[string]interface{}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("failed to decode JSON body %q: %v", body, err)
	}
	if !jsonMapsEqual(got, want) {
		t.Fatalf("body = %#v, want %#v", got, want)
	}
}

func jsonMapsEqual(got, want map[string]interface{}) bool {
	if len(got) != len(want) {
		return false
	}
	for key, wantValue := range want {
		gotValue, ok := got[key]
		if !ok {
			return false
		}
		if !jsonValuesEqual(gotValue, wantValue) {
			return false
		}
	}
	return true
}

func jsonValuesEqual(got, want interface{}) bool {
	switch wantValue := want.(type) {
	case []interface{}:
		gotValues, ok := got.([]interface{})
		if !ok || len(gotValues) != len(wantValue) {
			return false
		}
		for i := range wantValue {
			if !jsonValuesEqual(gotValues[i], wantValue[i]) {
				return false
			}
		}
		return true
	case nil:
		return got == nil
	case float64:
		gotFloat, ok := got.(float64)
		return ok && gotFloat == wantValue
	default:
		return got == wantValue
	}
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
