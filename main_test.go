package main

import "testing"

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
