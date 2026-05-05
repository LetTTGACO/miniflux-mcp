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
