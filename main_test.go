package main

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"miniflux.app/v2/client"
)

type roundTripperFunc func(req *http.Request) (*http.Response, error)

func (fn roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
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
