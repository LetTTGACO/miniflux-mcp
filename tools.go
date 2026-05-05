package main

import (
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type ToolDefinition struct {
	Tool    mcp.Tool
	Handler server.ToolHandlerFunc
}

func entryFilterProperties() map[string]interface{} {
	return map[string]interface{}{
		"status": map[string]interface{}{
			"type":        "string",
			"description": "Filter by entry status (read, unread, removed)",
		},
		"statuses": map[string]interface{}{
			"type":        "array",
			"description": "Filter by multiple entry statuses",
			"items": map[string]interface{}{
				"type": "string",
			},
		},
		"feed_id": map[string]interface{}{
			"type":        "number",
			"description": "Filter by specific feed ID",
		},
		"category_id": map[string]interface{}{
			"type":        "number",
			"description": "Filter by specific category ID",
		},
		"limit": map[string]interface{}{
			"type":        "number",
			"description": "Limit the number of entries returned",
		},
		"offset": map[string]interface{}{
			"type":        "number",
			"description": "Offset for pagination",
		},
		"order": map[string]interface{}{
			"type":        "string",
			"description": "Sort field for entries",
		},
		"direction": map[string]interface{}{
			"type":        "string",
			"description": "Sort direction (asc or desc)",
		},
		"starred": map[string]interface{}{
			"type":        "boolean",
			"description": "Filter starred entries",
		},
		"before": map[string]interface{}{
			"type":        "number",
			"description": "Return entries before this Unix timestamp",
		},
		"after": map[string]interface{}{
			"type":        "number",
			"description": "Return entries after this Unix timestamp",
		},
		"published_before": map[string]interface{}{
			"type":        "number",
			"description": "Return entries published before this Unix timestamp",
		},
		"published_after": map[string]interface{}{
			"type":        "number",
			"description": "Return entries published after this Unix timestamp",
		},
		"changed_before": map[string]interface{}{
			"type":        "number",
			"description": "Return entries changed before this Unix timestamp",
		},
		"changed_after": map[string]interface{}{
			"type":        "number",
			"description": "Return entries changed after this Unix timestamp",
		},
		"before_entry_id": map[string]interface{}{
			"type":        "number",
			"description": "Return entries before this entry ID",
		},
		"after_entry_id": map[string]interface{}{
			"type":        "number",
			"description": "Return entries after this entry ID",
		},
		"search": map[string]interface{}{
			"type":        "string",
			"description": "Search query",
		},
		"globally_visible": map[string]interface{}{
			"type":        "boolean",
			"description": "Filter globally visible entries",
		},
	}
}

func entryFilterPropertiesWithRouteID(routeIDKey, description string) map[string]interface{} {
	properties := entryFilterProperties()
	properties[routeIDKey] = map[string]interface{}{
		"type":        "number",
		"description": description,
	}
	return properties
}

func minifluxToolDefinitions(s *MinifluxServer) []ToolDefinition {
	return []ToolDefinition{
		// Feed Operations
		{
			Tool: mcp.Tool{
				Name:        "get_feeds",
				Description: "Get all RSS/Atom feeds from Miniflux",
				InputSchema: mcp.ToolInputSchema{
					Type:       "object",
					Properties: map[string]interface{}{},
				},
			},
			Handler: s.GetFeeds,
		},
		{
			Tool: mcp.Tool{
				Name:        "get_feed",
				Description: "Get a specific feed by ID",
				InputSchema: mcp.ToolInputSchema{
					Type: "object",
					Properties: map[string]interface{}{
						"feed_id": map[string]interface{}{
							"type":        "number",
							"description": "The ID of the feed to retrieve",
						},
					},
					Required: []string{"feed_id"},
				},
			},
			Handler: s.GetFeed,
		},
		{
			Tool: mcp.Tool{
				Name:        "create_feed",
				Description: "Add a new RSS/Atom feed to Miniflux",
				InputSchema: mcp.ToolInputSchema{
					Type: "object",
					Properties: map[string]interface{}{
						"feed_url": map[string]interface{}{
							"type":        "string",
							"description": "The URL of the RSS/Atom feed to add",
						},
						"category_id": map[string]interface{}{
							"type":        "number",
							"description": "The category ID to assign the feed to (default: 1)",
						},
						"crawler": map[string]interface{}{
							"type":        "boolean",
							"description": "Enable web scraper for full content",
						},
						"ignore_entry_updates": map[string]interface{}{
							"type":        "boolean",
							"description": "Ignore entry updates for this feed",
						},
						"user_agent": map[string]interface{}{
							"type":        "string",
							"description": "Custom user agent for feed fetching",
						},
						"cookie": map[string]interface{}{
							"type":        "string",
							"description": "HTTP cookie for feed fetching",
						},
						"username": map[string]interface{}{
							"type":        "string",
							"description": "Username for HTTP basic authentication",
						},
						"password": map[string]interface{}{
							"type":        "string",
							"description": "Password for HTTP basic authentication",
						},
						"disabled": map[string]interface{}{
							"type":        "boolean",
							"description": "Disable feed refresh",
						},
						"ignore_http_cache": map[string]interface{}{
							"type":        "boolean",
							"description": "Ignore HTTP cache",
						},
						"allow_self_signed_certificates": map[string]interface{}{
							"type":        "boolean",
							"description": "Allow self-signed certificates",
						},
						"fetch_via_proxy": map[string]interface{}{
							"type":        "boolean",
							"description": "Fetch via configured proxy",
						},
						"scraper_rules": map[string]interface{}{
							"type":        "string",
							"description": "Scraper rules",
						},
						"rewrite_rules": map[string]interface{}{
							"type":        "string",
							"description": "Rewrite rules",
						},
						"urlrewrite_rules": map[string]interface{}{
							"type":        "string",
							"description": "URL rewrite rules",
						},
						"blocklist_rules": map[string]interface{}{
							"type":        "string",
							"description": "Blocklist rules",
						},
						"keeplist_rules": map[string]interface{}{
							"type":        "string",
							"description": "Keeplist rules",
						},
						"block_filter_entry_rules": map[string]interface{}{
							"type":        "string",
							"description": "Entry block filter rules",
						},
						"keep_filter_entry_rules": map[string]interface{}{
							"type":        "string",
							"description": "Entry keep filter rules",
						},
						"hide_globally": map[string]interface{}{
							"type":        "boolean",
							"description": "Hide entries from global views",
						},
						"disable_http2": map[string]interface{}{
							"type":        "boolean",
							"description": "Disable HTTP/2",
						},
						"proxy_url": map[string]interface{}{
							"type":        "string",
							"description": "Per-feed proxy URL",
						},
					},
					Required: []string{"feed_url"},
				},
			},
			Handler: s.CreateFeed,
		},
		{
			Tool: mcp.Tool{
				Name:        "delete_feed",
				Description: "Delete a specific feed",
				InputSchema: mcp.ToolInputSchema{
					Type: "object",
					Properties: map[string]interface{}{
						"feed_id": map[string]interface{}{
							"type":        "number",
							"description": "The ID of the feed to delete",
						},
					},
					Required: []string{"feed_id"},
				},
			},
			Handler: s.DeleteFeed,
		},
		{
			Tool: mcp.Tool{
				Name:        "update_feed",
				Description: "Update a feed",
				InputSchema: mcp.ToolInputSchema{
					Type: "object",
					Properties: map[string]interface{}{
						"feed_id":                        map[string]interface{}{"type": "number", "description": "The ID of the feed"},
						"feed_url":                       map[string]interface{}{"type": "string", "description": "Feed URL"},
						"site_url":                       map[string]interface{}{"type": "string", "description": "Site URL"},
						"title":                          map[string]interface{}{"type": "string", "description": "Feed title"},
						"scraper_rules":                  map[string]interface{}{"type": "string", "description": "Scraper rules"},
						"rewrite_rules":                  map[string]interface{}{"type": "string", "description": "Rewrite rules"},
						"urlrewrite_rules":               map[string]interface{}{"type": "string", "description": "URL rewrite rules"},
						"blocklist_rules":                map[string]interface{}{"type": "string", "description": "Blocklist rules"},
						"keeplist_rules":                 map[string]interface{}{"type": "string", "description": "Keeplist rules"},
						"block_filter_entry_rules":       map[string]interface{}{"type": "string", "description": "Entry block filter rules"},
						"keep_filter_entry_rules":        map[string]interface{}{"type": "string", "description": "Entry keep filter rules"},
						"crawler":                        map[string]interface{}{"type": "boolean", "description": "Enable web scraper"},
						"ignore_entry_updates":           map[string]interface{}{"type": "boolean", "description": "Ignore entry updates"},
						"user_agent":                     map[string]interface{}{"type": "string", "description": "Custom user agent"},
						"cookie":                         map[string]interface{}{"type": "string", "description": "HTTP cookie"},
						"username":                       map[string]interface{}{"type": "string", "description": "HTTP basic auth username"},
						"password":                       map[string]interface{}{"type": "string", "description": "HTTP basic auth password"},
						"category_id":                    map[string]interface{}{"type": "number", "description": "Category ID"},
						"disabled":                       map[string]interface{}{"type": "boolean", "description": "Disable feed refresh"},
						"ignore_http_cache":              map[string]interface{}{"type": "boolean", "description": "Ignore HTTP cache"},
						"allow_self_signed_certificates": map[string]interface{}{"type": "boolean", "description": "Allow self-signed certificates"},
						"fetch_via_proxy":                map[string]interface{}{"type": "boolean", "description": "Fetch via configured proxy"},
						"hide_globally":                  map[string]interface{}{"type": "boolean", "description": "Hide entries from global views"},
						"disable_http2":                  map[string]interface{}{"type": "boolean", "description": "Disable HTTP/2"},
						"proxy_url":                      map[string]interface{}{"type": "string", "description": "Per-feed proxy URL"},
					},
					Required: []string{"feed_id"},
				},
			},
			Handler: s.UpdateFeed,
		},
		{
			Tool: mcp.Tool{
				Name:        "refresh_feed",
				Description: "Manually refresh a specific feed",
				InputSchema: mcp.ToolInputSchema{
					Type: "object",
					Properties: map[string]interface{}{
						"feed_id": map[string]interface{}{
							"type":        "number",
							"description": "The ID of the feed to refresh",
						},
					},
					Required: []string{"feed_id"},
				},
			},
			Handler: s.RefreshFeed,
		},
		{
			Tool: mcp.Tool{
				Name:        "refresh_all_feeds",
				Description: "Refresh all feeds",
				InputSchema: mcp.ToolInputSchema{
					Type:       "object",
					Properties: map[string]interface{}{},
				},
			},
			Handler: s.RefreshAllFeeds,
		},
		{
			Tool: mcp.Tool{
				Name:        "get_feed_entries",
				Description: "Get entries from a specific feed",
				InputSchema: mcp.ToolInputSchema{
					Type:       "object",
					Properties: entryFilterPropertiesWithRouteID("feed_id", "The ID of the feed"),
					Required:   []string{"feed_id"},
				},
			},
			Handler: s.GetFeedEntries,
		},
		{
			Tool: mcp.Tool{
				Name:        "get_feed_entry",
				Description: "Get a specific entry from a feed",
				InputSchema: mcp.ToolInputSchema{
					Type: "object",
					Properties: map[string]interface{}{
						"feed_id": map[string]interface{}{
							"type":        "number",
							"description": "The ID of the feed",
						},
						"entry_id": map[string]interface{}{
							"type":        "number",
							"description": "The ID of the entry",
						},
					},
					Required: []string{"feed_id", "entry_id"},
				},
			},
			Handler: s.GetFeedEntry,
		},
		{
			Tool: mcp.Tool{
				Name:        "import_feed_entry",
				Description: "Import an entry into a feed",
				InputSchema: mcp.ToolInputSchema{
					Type: "object",
					Properties: map[string]interface{}{
						"feed_id":      map[string]interface{}{"type": "number", "description": "The ID of the feed"},
						"url":          map[string]interface{}{"type": "string", "description": "Entry URL"},
						"title":        map[string]interface{}{"type": "string", "description": "Entry title"},
						"author":       map[string]interface{}{"type": "string", "description": "Entry author"},
						"content":      map[string]interface{}{"type": "string", "description": "Entry content"},
						"published_at": map[string]interface{}{"type": "number", "description": "Published Unix timestamp"},
						"status":       map[string]interface{}{"type": "string", "description": "Entry status"},
						"starred":      map[string]interface{}{"type": "boolean", "description": "Whether the entry is starred"},
						"tags": map[string]interface{}{
							"type":        "array",
							"description": "Entry tags",
							"items":       map[string]interface{}{"type": "string"},
						},
						"external_id":  map[string]interface{}{"type": "string", "description": "External unique ID"},
						"comments_url": map[string]interface{}{"type": "string", "description": "Comments URL"},
					},
					Required: []string{"feed_id", "url"},
				},
			},
			Handler: s.ImportFeedEntry,
		},
		{
			Tool: mcp.Tool{
				Name:        "get_feed_icon",
				Description: "Get the icon of a specific feed",
				InputSchema: mcp.ToolInputSchema{
					Type: "object",
					Properties: map[string]interface{}{
						"feed_id": map[string]interface{}{
							"type":        "number",
							"description": "The ID of the feed",
						},
					},
					Required: []string{"feed_id"},
				},
			},
			Handler: s.GetFeedIcon,
		},
		{
			Tool: mcp.Tool{
				Name:        "mark_feed_as_read",
				Description: "Mark all entries in a feed as read",
				InputSchema: mcp.ToolInputSchema{
					Type: "object",
					Properties: map[string]interface{}{
						"feed_id": map[string]interface{}{
							"type":        "number",
							"description": "The ID of the feed",
						},
					},
					Required: []string{"feed_id"},
				},
			},
			Handler: s.MarkFeedAsRead,
		},

		// Entry Operations
		{
			Tool: mcp.Tool{
				Name:        "get_entries",
				Description: "Get entries (articles) from Miniflux with optional filtering",
				InputSchema: mcp.ToolInputSchema{
					Type:       "object",
					Properties: entryFilterProperties(),
				},
			},
			Handler: s.GetEntries,
		},
		{
			Tool: mcp.Tool{
				Name:        "get_daily_digest",
				Description: "Get entries since a caller-provided timestamp with digest-ready metadata and content",
				InputSchema: mcp.ToolInputSchema{
					Type: "object",
					Properties: map[string]interface{}{
						"since": map[string]interface{}{
							"description": "Unix timestamp or RFC3339 timestamp for the start of the digest window",
						},
						"status": map[string]interface{}{
							"type":        "string",
							"description": "Entry status to fetch (default: unread)",
							"enum":        []string{"read", "unread", "removed"},
						},
						"date_field": map[string]interface{}{
							"type":        "string",
							"description": "Date field used for the since filter (published or changed)",
							"enum":        []string{"published", "changed"},
						},
						"limit": map[string]interface{}{
							"type":        "number",
							"description": "Maximum number of entries to fetch (default: 50)",
						},
						"feed_id": map[string]interface{}{
							"type":        "number",
							"description": "Optional feed ID filter",
						},
						"category_id": map[string]interface{}{
							"type":        "number",
							"description": "Optional category ID filter",
						},
						"content_mode": map[string]interface{}{
							"type":        "string",
							"description": "Content retrieval mode (none, feed, scrape_when_short, scrape_all; default: feed)",
							"enum":        []string{"none", "feed", "scrape_when_short", "scrape_all"},
						},
						"min_content_length": map[string]interface{}{
							"type":        "number",
							"description": "Minimum feed content length before scraping is attempted (default: 500)",
						},
						"max_content_length": map[string]interface{}{
							"type":        "number",
							"description": "Maximum content length per entry (default: 6000)",
						},
					},
					Required: []string{"since"},
				},
			},
			Handler: s.GetDailyDigest,
		},
		{
			Tool: mcp.Tool{
				Name:        "get_entry",
				Description: "Get a specific entry by ID",
				InputSchema: mcp.ToolInputSchema{
					Type: "object",
					Properties: map[string]interface{}{
						"entry_id": map[string]interface{}{
							"type":        "number",
							"description": "The ID of the entry to retrieve",
						},
					},
					Required: []string{"entry_id"},
				},
			},
			Handler: s.GetEntry,
		},
		{
			Tool: mcp.Tool{
				Name:        "update_entry_status",
				Description: "Update the status of an entry (mark as read/unread/removed)",
				InputSchema: mcp.ToolInputSchema{
					Type: "object",
					Properties: map[string]interface{}{
						"entry_id": map[string]interface{}{
							"type":        "number",
							"description": "The ID of the entry to update",
						},
						"status": map[string]interface{}{
							"type":        "string",
							"description": "New status for the entry (read, unread, removed)",
							"enum":        []string{"read", "unread", "removed"},
						},
					},
					Required: []string{"entry_id", "status"},
				},
			},
			Handler: s.UpdateEntryStatus,
		},
		{
			Tool: mcp.Tool{
				Name:        "update_entries_status",
				Description: "Update the status of multiple entries (mark as read/unread/removed)",
				InputSchema: mcp.ToolInputSchema{
					Type: "object",
					Properties: map[string]interface{}{
						"entry_ids": map[string]interface{}{
							"type":        "array",
							"description": "The IDs of the entries to update",
							"items": map[string]interface{}{
								"type": "number",
							},
						},
						"status": map[string]interface{}{
							"type":        "string",
							"description": "New status for the entries (read, unread, removed)",
							"enum":        []string{"read", "unread", "removed"},
						},
					},
					Required: []string{"entry_ids", "status"},
				},
			},
			Handler: s.UpdateEntriesStatus,
		},
		{
			Tool: mcp.Tool{
				Name:        "toggle_starred",
				Description: "Toggle starred status of an entry",
				InputSchema: mcp.ToolInputSchema{
					Type: "object",
					Properties: map[string]interface{}{
						"entry_id": map[string]interface{}{
							"type":        "number",
							"description": "The ID of the entry",
						},
					},
					Required: []string{"entry_id"},
				},
			},
			Handler: s.ToggleStarred,
		},
		{
			Tool: mcp.Tool{
				Name:        "update_entry",
				Description: "Update entry title or content",
				InputSchema: mcp.ToolInputSchema{
					Type: "object",
					Properties: map[string]interface{}{
						"entry_id": map[string]interface{}{"type": "number", "description": "The ID of the entry"},
						"title":    map[string]interface{}{"type": "string", "description": "Entry title"},
						"content":  map[string]interface{}{"type": "string", "description": "Entry content"},
					},
					Required: []string{"entry_id"},
				},
			},
			Handler: s.UpdateEntry,
		},
		{
			Tool: mcp.Tool{
				Name:        "save_entry",
				Description: "Save an entry",
				InputSchema: mcp.ToolInputSchema{
					Type: "object",
					Properties: map[string]interface{}{
						"entry_id": map[string]interface{}{
							"type":        "number",
							"description": "The ID of the entry",
						},
					},
					Required: []string{"entry_id"},
				},
			},
			Handler: s.SaveEntry,
		},
		{
			Tool: mcp.Tool{
				Name:        "fetch_original_content",
				Description: "Fetch the original content of an entry",
				InputSchema: mcp.ToolInputSchema{
					Type: "object",
					Properties: map[string]interface{}{
						"entry_id": map[string]interface{}{
							"type":        "number",
							"description": "The ID of the entry",
						},
					},
					Required: []string{"entry_id"},
				},
			},
			Handler: s.FetchEntryOriginalContent,
		},
		{
			Tool: mcp.Tool{
				Name:        "mark_all_as_read",
				Description: "Mark all entries as read for a user",
				InputSchema: mcp.ToolInputSchema{
					Type: "object",
					Properties: map[string]interface{}{
						"user_id": map[string]interface{}{
							"type":        "number",
							"description": "The ID of the user",
						},
					},
					Required: []string{"user_id"},
				},
			},
			Handler: s.MarkAllAsRead,
		},

		// Category Operations
		{
			Tool: mcp.Tool{
				Name:        "get_categories",
				Description: "Get all feed categories from Miniflux",
				InputSchema: mcp.ToolInputSchema{
					Type:       "object",
					Properties: map[string]interface{}{},
				},
			},
			Handler: s.GetCategories,
		},
		{
			Tool: mcp.Tool{
				Name:        "create_category",
				Description: "Create a new category",
				InputSchema: mcp.ToolInputSchema{
					Type: "object",
					Properties: map[string]interface{}{
						"title": map[string]interface{}{
							"type":        "string",
							"description": "The title of the category",
						},
					},
					Required: []string{"title"},
				},
			},
			Handler: s.CreateCategory,
		},
		{
			Tool: mcp.Tool{
				Name:        "update_category",
				Description: "Update a category title",
				InputSchema: mcp.ToolInputSchema{
					Type: "object",
					Properties: map[string]interface{}{
						"category_id": map[string]interface{}{
							"type":        "number",
							"description": "The ID of the category",
						},
						"title": map[string]interface{}{
							"type":        "string",
							"description": "The new title of the category",
						},
					},
					Required: []string{"category_id", "title"},
				},
			},
			Handler: s.UpdateCategory,
		},
		{
			Tool: mcp.Tool{
				Name:        "delete_category",
				Description: "Delete a category",
				InputSchema: mcp.ToolInputSchema{
					Type: "object",
					Properties: map[string]interface{}{
						"category_id": map[string]interface{}{
							"type":        "number",
							"description": "The ID of the category",
						},
					},
					Required: []string{"category_id"},
				},
			},
			Handler: s.DeleteCategory,
		},
		{
			Tool: mcp.Tool{
				Name:        "get_category_feeds",
				Description: "Get all feeds in a specific category",
				InputSchema: mcp.ToolInputSchema{
					Type: "object",
					Properties: map[string]interface{}{
						"category_id": map[string]interface{}{
							"type":        "number",
							"description": "The ID of the category",
						},
					},
					Required: []string{"category_id"},
				},
			},
			Handler: s.GetCategoryFeeds,
		},
		{
			Tool: mcp.Tool{
				Name:        "get_category_entries",
				Description: "Get all entries in a specific category",
				InputSchema: mcp.ToolInputSchema{
					Type:       "object",
					Properties: entryFilterPropertiesWithRouteID("category_id", "The ID of the category"),
					Required:   []string{"category_id"},
				},
			},
			Handler: s.GetCategoryEntries,
		},
		{
			Tool: mcp.Tool{
				Name:        "get_category_entry",
				Description: "Get a specific entry from a category",
				InputSchema: mcp.ToolInputSchema{
					Type: "object",
					Properties: map[string]interface{}{
						"category_id": map[string]interface{}{
							"type":        "number",
							"description": "The ID of the category",
						},
						"entry_id": map[string]interface{}{
							"type":        "number",
							"description": "The ID of the entry",
						},
					},
					Required: []string{"category_id", "entry_id"},
				},
			},
			Handler: s.GetCategoryEntry,
		},
		{
			Tool: mcp.Tool{
				Name:        "mark_category_as_read",
				Description: "Mark all entries in a category as read",
				InputSchema: mcp.ToolInputSchema{
					Type: "object",
					Properties: map[string]interface{}{
						"category_id": map[string]interface{}{
							"type":        "number",
							"description": "The ID of the category",
						},
					},
					Required: []string{"category_id"},
				},
			},
			Handler: s.MarkCategoryAsRead,
		},
		{
			Tool: mcp.Tool{
				Name:        "refresh_category",
				Description: "Refresh all feeds in a category",
				InputSchema: mcp.ToolInputSchema{
					Type: "object",
					Properties: map[string]interface{}{
						"category_id": map[string]interface{}{
							"type":        "number",
							"description": "The ID of the category",
						},
					},
					Required: []string{"category_id"},
				},
			},
			Handler: s.RefreshCategory,
		},

		// User Management
		{
			Tool: mcp.Tool{
				Name:        "get_users",
				Description: "Get all users",
				InputSchema: mcp.ToolInputSchema{
					Type:       "object",
					Properties: map[string]interface{}{},
				},
			},
			Handler: s.GetUsers,
		},
		{
			Tool: mcp.Tool{
				Name:        "get_me",
				Description: "Get current user information",
				InputSchema: mcp.ToolInputSchema{
					Type:       "object",
					Properties: map[string]interface{}{},
				},
			},
			Handler: s.GetMe,
		},
		{
			Tool: mcp.Tool{
				Name:        "get_user_by_id",
				Description: "Get a specific user by ID",
				InputSchema: mcp.ToolInputSchema{
					Type: "object",
					Properties: map[string]interface{}{
						"user_id": map[string]interface{}{
							"type":        "number",
							"description": "The ID of the user",
						},
					},
					Required: []string{"user_id"},
				},
			},
			Handler: s.GetUserByID,
		},
		{
			Tool: mcp.Tool{
				Name:        "get_user_by_username",
				Description: "Get a specific user by username",
				InputSchema: mcp.ToolInputSchema{
					Type: "object",
					Properties: map[string]interface{}{
						"username": map[string]interface{}{
							"type":        "string",
							"description": "The username of the user",
						},
					},
					Required: []string{"username"},
				},
			},
			Handler: s.GetUserByUsername,
		},
		{
			Tool: mcp.Tool{
				Name:        "create_user",
				Description: "Create a new user",
				InputSchema: mcp.ToolInputSchema{
					Type: "object",
					Properties: map[string]interface{}{
						"username": map[string]interface{}{
							"type":        "string",
							"description": "The username for the new user",
						},
						"password": map[string]interface{}{
							"type":        "string",
							"description": "The password for the new user",
						},
						"is_admin": map[string]interface{}{
							"type":        "boolean",
							"description": "Whether the user should be an admin",
						},
					},
					Required: []string{"username", "password"},
				},
			},
			Handler: s.CreateUser,
		},
		{
			Tool: mcp.Tool{
				Name:        "delete_user",
				Description: "Delete a user",
				InputSchema: mcp.ToolInputSchema{
					Type: "object",
					Properties: map[string]interface{}{
						"user_id": map[string]interface{}{
							"type":        "number",
							"description": "The ID of the user",
						},
					},
					Required: []string{"user_id"},
				},
			},
			Handler: s.DeleteUser,
		},

		// System & Utility
		{
			Tool: mcp.Tool{
				Name:        "get_version",
				Description: "Get Miniflux version information",
				InputSchema: mcp.ToolInputSchema{
					Type:       "object",
					Properties: map[string]interface{}{},
				},
			},
			Handler: s.GetVersion,
		},
		{
			Tool: mcp.Tool{
				Name:        "healthcheck",
				Description: "Perform a health check",
				InputSchema: mcp.ToolInputSchema{
					Type:       "object",
					Properties: map[string]interface{}{},
				},
			},
			Handler: s.Healthcheck,
		},
		{
			Tool: mcp.Tool{
				Name:        "fetch_counters",
				Description: "Fetch feed counters",
				InputSchema: mcp.ToolInputSchema{
					Type:       "object",
					Properties: map[string]interface{}{},
				},
			},
			Handler: s.FetchCounters,
		},
		{
			Tool: mcp.Tool{
				Name:        "get_integrations_status",
				Description: "Get integrations status for the signed-in user",
				InputSchema: mcp.ToolInputSchema{
					Type:       "object",
					Properties: map[string]interface{}{},
				},
			},
			Handler: s.GetIntegrationsStatus,
		},
		{
			Tool: mcp.Tool{
				Name:        "discover",
				Description: "Discover feeds from a URL",
				InputSchema: mcp.ToolInputSchema{
					Type: "object",
					Properties: map[string]interface{}{
						"url": map[string]interface{}{
							"type":        "string",
							"description": "The URL to discover feeds from",
						},
					},
					Required: []string{"url"},
				},
			},
			Handler: s.Discover,
		},
		{
			Tool: mcp.Tool{
				Name:        "export",
				Description: "Export feeds as OPML",
				InputSchema: mcp.ToolInputSchema{
					Type:       "object",
					Properties: map[string]interface{}{},
				},
			},
			Handler: s.Export,
		},
		{
			Tool: mcp.Tool{
				Name:        "import_opml",
				Description: "Import feeds from OPML content",
				InputSchema: mcp.ToolInputSchema{
					Type: "object",
					Properties: map[string]interface{}{
						"opml_content": map[string]interface{}{
							"type":        "string",
							"description": "The OPML document content to import",
						},
					},
					Required: []string{"opml_content"},
				},
			},
			Handler: s.ImportOPML,
		},
		{
			Tool: mcp.Tool{
				Name:        "flush_history",
				Description: "Flush the read history",
				InputSchema: mcp.ToolInputSchema{
					Type:       "object",
					Properties: map[string]interface{}{},
				},
			},
			Handler: s.FlushHistory,
		},

		// API Key Management
		{
			Tool: mcp.Tool{
				Name:        "get_api_keys",
				Description: "Get all API keys",
				InputSchema: mcp.ToolInputSchema{
					Type:       "object",
					Properties: map[string]interface{}{},
				},
			},
			Handler: s.GetAPIKeys,
		},
		{
			Tool: mcp.Tool{
				Name:        "create_api_key",
				Description: "Create a new API key",
				InputSchema: mcp.ToolInputSchema{
					Type: "object",
					Properties: map[string]interface{}{
						"description": map[string]interface{}{
							"type":        "string",
							"description": "Description for the API key",
						},
					},
					Required: []string{"description"},
				},
			},
			Handler: s.CreateAPIKey,
		},
		{
			Tool: mcp.Tool{
				Name:        "delete_api_key",
				Description: "Delete an API key",
				InputSchema: mcp.ToolInputSchema{
					Type: "object",
					Properties: map[string]interface{}{
						"api_key_id": map[string]interface{}{
							"type":        "number",
							"description": "The ID of the API key",
						},
					},
					Required: []string{"api_key_id"},
				},
			},
			Handler: s.DeleteAPIKey,
		},

		// Icons & Media
		{
			Tool: mcp.Tool{
				Name:        "get_icon",
				Description: "Get an icon by ID",
				InputSchema: mcp.ToolInputSchema{
					Type: "object",
					Properties: map[string]interface{}{
						"icon_id": map[string]interface{}{
							"type":        "number",
							"description": "The ID of the icon",
						},
					},
					Required: []string{"icon_id"},
				},
			},
			Handler: s.GetIcon,
		},
		{
			Tool: mcp.Tool{
				Name:        "get_enclosure",
				Description: "Get an enclosure by ID",
				InputSchema: mcp.ToolInputSchema{
					Type: "object",
					Properties: map[string]interface{}{
						"enclosure_id": map[string]interface{}{
							"type":        "number",
							"description": "The ID of the enclosure",
						},
					},
					Required: []string{"enclosure_id"},
				},
			},
			Handler: s.GetEnclosure,
		},
		{
			Tool: mcp.Tool{
				Name:        "update_enclosure",
				Description: "Update enclosure media progression",
				InputSchema: mcp.ToolInputSchema{
					Type: "object",
					Properties: map[string]interface{}{
						"enclosure_id":      map[string]interface{}{"type": "number", "description": "The ID of the enclosure"},
						"media_progression": map[string]interface{}{"type": "number", "description": "Media progression in seconds"},
					},
					Required: []string{"enclosure_id", "media_progression"},
				},
			},
			Handler: s.UpdateEnclosure,
		},
	}
}

func (s *MinifluxServer) RegisterAllTools(mcpServer *server.MCPServer) {
	// Register all tools
	for _, toolDef := range minifluxToolDefinitions(s) {
		mcpServer.AddTool(toolDef.Tool, toolDef.Handler)
	}
}
