package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"miniflux.app/v2/client"
)

type MinifluxServer struct {
	client *client.Client
}

const minifluxStartupTimeout = 10 * time.Second

// dailyDigestResponse is shaped for downstream summarizers: it includes a stable
// acknowledgement list plus enough entry metadata to let an agent cite or mark
// the fetched items after it has processed the digest.
type dailyDigestResponse struct {
	GeneratedAt time.Time          `json:"generated_at"`
	Since       int64              `json:"since"`
	Status      string             `json:"status"`
	DateField   string             `json:"date_field"`
	Total       int                `json:"total"`
	Count       int                `json:"count"`
	AckEntryIDs []int64            `json:"ack_entry_ids"`
	Entries     []dailyDigestEntry `json:"entries"`
}

type dailyDigestEntry struct {
	ID               int64     `json:"id"`
	Title            string    `json:"title"`
	URL              string    `json:"url"`
	Status           string    `json:"status"`
	PublishedAt      time.Time `json:"published_at"`
	ChangedAt        time.Time `json:"changed_at"`
	FeedID           int64     `json:"feed_id"`
	FeedTitle        string    `json:"feed_title,omitempty"`
	CategoryID       int64     `json:"category_id,omitempty"`
	CategoryTitle    string    `json:"category_title,omitempty"`
	Author           string    `json:"author,omitempty"`
	Tags             []string  `json:"tags,omitempty"`
	Starred          bool      `json:"starred"`
	ReadingTime      int       `json:"reading_time,omitempty"`
	Content          string    `json:"content,omitempty"`
	ContentSource    string    `json:"content_source"`
	ContentAvailable bool      `json:"content_available"`
	ContentTruncated bool      `json:"content_truncated"`
	ContentLength    int       `json:"content_length"`
	ContentError     string    `json:"content_error,omitempty"`
}

// NewMinifluxServer builds the production server from environment variables and
// fails fast when Miniflux is unreachable or credentials are invalid.
func NewMinifluxServer() *MinifluxServer {
	baseURL := os.Getenv("MINIFLUX_URL")
	apiKey := os.Getenv("MINIFLUX_API_KEY")
	username := os.Getenv("MINIFLUX_USERNAME")
	password := os.Getenv("MINIFLUX_PASSWORD")

	server, err := newMinifluxServerFromConfig(baseURL, apiKey, username, password, minifluxStartupTimeout, nil)
	if err != nil {
		log.Fatal(err)
	}

	return server
}

// newMinifluxServerFromConfig exists so tests can inject timeouts and HTTP
// transports while production still reads configuration from the environment.
func newMinifluxServerFromConfig(baseURL, apiKey, username, password string, startupTimeout time.Duration, httpClient *http.Client) (*MinifluxServer, error) {
	if baseURL == "" {
		return nil, fmt.Errorf("MINIFLUX_URL environment variable is required")
	}
	if apiKey == "" && (username == "" || password == "") {
		return nil, fmt.Errorf("either MINIFLUX_API_KEY or both MINIFLUX_USERNAME and MINIFLUX_PASSWORD must be set")
	}

	if startupTimeout <= 0 {
		startupTimeout = minifluxStartupTimeout
	}
	if httpClient == nil {
		httpClient = &http.Client{}
	}

	options := []client.Option{client.WithHTTPClient(httpClient)}
	var minifluxClient *client.Client
	if apiKey != "" {
		options = append(options, client.WithAPIKey(apiKey))
	} else {
		options = append(options, client.WithCredentials(username, password))
	}
	minifluxClient = client.NewClientWithOptions(baseURL, options...)

	ctx, cancel := context.WithTimeout(context.Background(), startupTimeout)
	defer cancel()

	if err := minifluxClient.HealthcheckContext(ctx); err != nil {
		return nil, fmt.Errorf("healthcheck failed: %w", err)
	}
	log.Printf("Healthcheck passed")

	if _, err := minifluxClient.MeContext(ctx); err != nil {
		return nil, fmt.Errorf("auth failed: %w", err)
	}
	log.Printf("Auth passed")

	return &MinifluxServer{
		client: minifluxClient,
	}, nil
}

// GetFeeds returns the Go client's raw feed response as formatted JSON because
// MCP clients can consume text results directly and Miniflux already owns the
// response shape.
func (s *MinifluxServer) GetFeeds(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	feeds, err := s.client.Feeds()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to fetch feeds: %v", err)), nil
	}

	feedsJSON, err := json.MarshalIndent(feeds, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal feeds: %v", err)), nil
	}

	return mcp.NewToolResultText(string(feedsJSON)), nil
}

func (s *MinifluxServer) GetEntries(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.Params.Arguments

	var filter *client.Filter
	if args != nil {
		argsMap, ok := args.(map[string]interface{})
		if ok {
			filter = buildEntryFilter(argsMap)
		}
	}

	entries, err := s.client.Entries(filter)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to fetch entries: %v", err)), nil
	}

	entriesJSON, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal entries: %v", err)), nil
	}

	return mcp.NewToolResultText(string(entriesJSON)), nil
}

// buildEntryFilter translates MCP JSON arguments into the Miniflux client's
// filter struct. Numeric JSON values arrive as float64 through mcp-go.
func buildEntryFilter(args map[string]interface{}) *client.Filter {
	filter := &client.Filter{}

	if statusStr, ok := args["status"].(string); ok {
		filter.Status = statusStr
	}
	if statuses, ok := args["statuses"].([]interface{}); ok {
		for _, status := range statuses {
			if statusStr, ok := status.(string); ok {
				filter.Statuses = append(filter.Statuses, statusStr)
			}
		}
	}
	if feedIDFloat, ok := args["feed_id"].(float64); ok {
		filter.FeedID = int64(feedIDFloat)
	}
	if categoryIDFloat, ok := args["category_id"].(float64); ok {
		filter.CategoryID = int64(categoryIDFloat)
	}
	if limitFloat, ok := args["limit"].(float64); ok {
		filter.Limit = int(limitFloat)
	}
	if offsetFloat, ok := args["offset"].(float64); ok {
		filter.Offset = int(offsetFloat)
	}
	if order, ok := args["order"].(string); ok {
		filter.Order = order
	}
	if direction, ok := args["direction"].(string); ok {
		filter.Direction = direction
	}
	if starred, ok := args["starred"].(bool); ok {
		if starred {
			filter.Starred = client.FilterOnlyStarred
		} else {
			filter.Starred = client.FilterNotStarred
		}
	}
	if starred, ok := args["starred"].(string); ok {
		filter.Starred = starred
	}
	if before, ok := args["before"].(float64); ok {
		filter.Before = int64(before)
	}
	if after, ok := args["after"].(float64); ok {
		filter.After = int64(after)
	}
	if publishedBefore, ok := args["published_before"].(float64); ok {
		filter.PublishedBefore = int64(publishedBefore)
	}
	if publishedAfter, ok := args["published_after"].(float64); ok {
		filter.PublishedAfter = int64(publishedAfter)
	}
	if changedBefore, ok := args["changed_before"].(float64); ok {
		filter.ChangedBefore = int64(changedBefore)
	}
	if changedAfter, ok := args["changed_after"].(float64); ok {
		filter.ChangedAfter = int64(changedAfter)
	}
	if beforeEntryID, ok := args["before_entry_id"].(float64); ok {
		filter.BeforeEntryID = int64(beforeEntryID)
	}
	if afterEntryID, ok := args["after_entry_id"].(float64); ok {
		filter.AfterEntryID = int64(afterEntryID)
	}
	if search, ok := args["search"].(string); ok {
		filter.Search = search
	}
	if globallyVisible, ok := args["globally_visible"].(bool); ok {
		filter.GloballyVisible = globallyVisible
	}

	return filter
}

// GetDailyDigest builds a bounded, summarizer-friendly entry list. Unlike
// get_entries, it requires the caller to provide the time window explicitly so
// scheduled agents do not accidentally reprocess an unbounded backlog.
func (s *MinifluxServer) GetDailyDigest(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	argsMap := map[string]interface{}{}
	if request.Params.Arguments != nil {
		var ok bool
		argsMap, ok = request.Params.Arguments.(map[string]interface{})
		if !ok {
			return mcp.NewToolResultError("Invalid arguments format"), nil
		}
	}

	options, err := buildDailyDigestOptions(argsMap)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	filter := &client.Filter{
		Status:    options.status,
		Limit:     options.limit,
		Order:     "published_at",
		Direction: "desc",
	}
	if options.dateField == "changed" {
		filter.ChangedAfter = options.since
		filter.Order = "changed_at"
	} else {
		filter.PublishedAfter = options.since
	}
	if feedID, ok := numberArg(argsMap, "feed_id"); ok {
		filter.FeedID = int64(feedID)
	}

	entries, err := s.client.Entries(filter)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to fetch daily digest entries: %v", err)), nil
	}

	response := dailyDigestResponse{
		GeneratedAt: options.now,
		Since:       options.since,
		Status:      options.status,
		DateField:   options.dateField,
		Total:       entries.Total,
		Count:       len(entries.Entries),
		AckEntryIDs: make([]int64, 0, len(entries.Entries)),
		Entries:     make([]dailyDigestEntry, 0, len(entries.Entries)),
	}

	for _, entry := range entries.Entries {
		digestEntry := s.buildDailyDigestEntry(entry, options)
		response.AckEntryIDs = append(response.AckEntryIDs, entry.ID)
		response.Entries = append(response.Entries, digestEntry)
	}

	digestJSON, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal daily digest: %v", err)), nil
	}

	return mcp.NewToolResultText(string(digestJSON)), nil
}

type dailyDigestOptions struct {
	now                time.Time
	since              int64
	status             string
	dateField          string
	limit              int
	categoryIDs        []int64
	excludeCategoryIDs []int64
	contentMode        string
	minContentLength   int
	maxContentLength   int
}

func buildDailyDigestOptions(args map[string]interface{}) (dailyDigestOptions, error) {
	options := dailyDigestOptions{
		status:           "unread",
		dateField:        "published",
		limit:            50,
		contentMode:      "feed",
		minContentLength: 500,
		maxContentLength: 6000,
		now:              time.Now(),
	}

	if since, ok := numberArg(args, "since"); ok {
		options.since = int64(since)
	} else if sinceString, ok := args["since"].(string); ok && sinceString != "" {
		since, err := time.Parse(time.RFC3339, sinceString)
		if err != nil {
			return options, fmt.Errorf("since must be a Unix timestamp or RFC3339 timestamp")
		}
		options.since = since.Unix()
	} else {
		return options, fmt.Errorf("since is required")
	}

	if status, ok := args["status"].(string); ok && status != "" {
		options.status = status
	}
	if dateField, ok := args["date_field"].(string); ok && dateField != "" {
		if dateField != "published" && dateField != "changed" {
			return options, fmt.Errorf("date_field must be published or changed")
		}
		options.dateField = dateField
	}
	if limit, ok := numberArg(args, "limit"); ok {
		options.limit = int(limit)
	}
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
	if contentMode, ok := args["content_mode"].(string); ok && contentMode != "" {
		switch contentMode {
		case "none", "feed", "scrape_when_short", "scrape_all":
			options.contentMode = contentMode
		default:
			return options, fmt.Errorf("content_mode must be none, feed, scrape_when_short, or scrape_all")
		}
	}
	if minContentLength, ok := numberArg(args, "min_content_length"); ok {
		options.minContentLength = int(minContentLength)
	}
	if maxContentLength, ok := numberArg(args, "max_content_length"); ok {
		options.maxContentLength = int(maxContentLength)
	}

	return options, nil
}

// buildDailyDigestEntry flattens Miniflux's nested entry/feed/category structs
// into one JSON object per entry so downstream agents do not need Miniflux-
// specific traversal logic.
func (s *MinifluxServer) buildDailyDigestEntry(entry *client.Entry, options dailyDigestOptions) dailyDigestEntry {
	digestEntry := dailyDigestEntry{
		ID:          entry.ID,
		Title:       entry.Title,
		URL:         entry.URL,
		Status:      entry.Status,
		PublishedAt: entry.Date,
		ChangedAt:   entry.ChangedAt,
		FeedID:      entry.FeedID,
		Author:      entry.Author,
		Tags:        entry.Tags,
		Starred:     entry.Starred,
		ReadingTime: entry.ReadingTime,
	}
	if entry.Feed != nil {
		digestEntry.FeedID = entry.Feed.ID
		digestEntry.FeedTitle = entry.Feed.Title
		if entry.Feed.Category != nil {
			digestEntry.CategoryID = entry.Feed.Category.ID
			digestEntry.CategoryTitle = entry.Feed.Category.Title
		}
	}

	content, source, contentErr := s.digestEntryContent(entry, options)
	digestEntry.ContentSource = source
	digestEntry.ContentError = contentErr
	digestEntry.ContentAvailable = strings.TrimSpace(content) != ""

	content, truncated := truncateContent(content, options.maxContentLength)
	digestEntry.Content = content
	digestEntry.ContentLength = len(content)
	digestEntry.ContentAvailable = strings.TrimSpace(content) != ""
	digestEntry.ContentTruncated = truncated

	return digestEntry
}

func (s *MinifluxServer) digestEntryContent(entry *client.Entry, options dailyDigestOptions) (string, string, string) {
	switch options.contentMode {
	case "none":
		return "", "none", ""
	case "scrape_all":
		content, err := s.client.FetchEntryOriginalContent(entry.ID)
		if err != nil {
			return entry.Content, "feed", err.Error()
		}
		return content, "scraped", ""
	case "scrape_when_short":
		if len(strings.TrimSpace(entry.Content)) >= options.minContentLength {
			return entry.Content, "feed", ""
		}
		content, err := s.client.FetchEntryOriginalContent(entry.ID)
		if err != nil {
			return entry.Content, "feed", err.Error()
		}
		return content, "scraped", ""
	default:
		return entry.Content, "feed", ""
	}
}

// truncateContent caps content by bytes rather than runes because the tool is
// mainly guarding model/context payload size, not rendering display text.
func truncateContent(content string, maxLength int) (string, bool) {
	content = strings.TrimSpace(content)
	if maxLength <= 0 || len(content) <= maxLength {
		return content, false
	}
	return strings.TrimSpace(content[:maxLength]), true
}

func numberArg(args map[string]interface{}, key string) (float64, bool) {
	value, ok := args[key].(float64)
	return value, ok
}

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

// buildScopedEntryFilter removes the path-scoped ID before passing the remaining
// arguments to Miniflux, preventing duplicate feed_id/category_id query params.
func buildScopedEntryFilter(args map[string]interface{}, routeIDKey string) *client.Filter {
	filterArgs := make(map[string]interface{}, len(args))
	for key, value := range args {
		if key == routeIDKey {
			continue
		}
		filterArgs[key] = value
	}
	if len(filterArgs) == 0 {
		return nil
	}

	return buildEntryFilter(filterArgs)
}

// buildFeedModificationRequest uses pointer fields so Miniflux can distinguish
// omitted values from explicit zero values during partial updates.
func buildFeedModificationRequest(args map[string]interface{}) *client.FeedModificationRequest {
	request := &client.FeedModificationRequest{}

	setStringPtr(args, "feed_url", &request.FeedURL)
	setStringPtr(args, "site_url", &request.SiteURL)
	setStringPtr(args, "title", &request.Title)
	setStringPtr(args, "scraper_rules", &request.ScraperRules)
	setStringPtr(args, "rewrite_rules", &request.RewriteRules)
	setStringPtr(args, "urlrewrite_rules", &request.UrlRewriteRules)
	setStringPtr(args, "blocklist_rules", &request.BlocklistRules)
	setStringPtr(args, "keeplist_rules", &request.KeeplistRules)
	setStringPtr(args, "block_filter_entry_rules", &request.BlockFilterEntryRules)
	setStringPtr(args, "keep_filter_entry_rules", &request.KeepFilterEntryRules)
	setBoolPtr(args, "crawler", &request.Crawler)
	setBoolPtr(args, "ignore_entry_updates", &request.IgnoreEntryUpdates)
	setStringPtr(args, "user_agent", &request.UserAgent)
	setStringPtr(args, "cookie", &request.Cookie)
	setStringPtr(args, "username", &request.Username)
	setStringPtr(args, "password", &request.Password)
	setInt64Ptr(args, "category_id", &request.CategoryID)
	setBoolPtr(args, "disabled", &request.Disabled)
	setBoolPtr(args, "ignore_http_cache", &request.IgnoreHTTPCache)
	setBoolPtr(args, "allow_self_signed_certificates", &request.AllowSelfSignedCertificates)
	setBoolPtr(args, "fetch_via_proxy", &request.FetchViaProxy)
	setBoolPtr(args, "hide_globally", &request.HideGlobally)
	setBoolPtr(args, "disable_http2", &request.DisableHTTP2)
	setStringPtr(args, "proxy_url", &request.ProxyURL)

	return request
}

// buildFeedCreationRequest fills Miniflux creation defaults and maps only the
// optional fields the MCP schema exposes.
func buildFeedCreationRequest(args map[string]interface{}) *client.FeedCreationRequest {
	request := &client.FeedCreationRequest{CategoryID: 1}

	setString(args, "feed_url", &request.FeedURL)
	setInt64(args, "category_id", &request.CategoryID)
	setString(args, "user_agent", &request.UserAgent)
	setString(args, "cookie", &request.Cookie)
	setString(args, "username", &request.Username)
	setString(args, "password", &request.Password)
	setBool(args, "crawler", &request.Crawler)
	setBool(args, "ignore_entry_updates", &request.IgnoreEntryUpdates)
	setBool(args, "disabled", &request.Disabled)
	setBool(args, "ignore_http_cache", &request.IgnoreHTTPCache)
	setBool(args, "allow_self_signed_certificates", &request.AllowSelfSignedCertificates)
	setBool(args, "fetch_via_proxy", &request.FetchViaProxy)
	setString(args, "scraper_rules", &request.ScraperRules)
	setString(args, "rewrite_rules", &request.RewriteRules)
	setString(args, "urlrewrite_rules", &request.UrlRewriteRules)
	setString(args, "blocklist_rules", &request.BlocklistRules)
	setString(args, "keeplist_rules", &request.KeeplistRules)
	setString(args, "block_filter_entry_rules", &request.BlockFilterEntryRules)
	setString(args, "keep_filter_entry_rules", &request.KeepFilterEntryRules)
	setBool(args, "hide_globally", &request.HideGlobally)
	setBool(args, "disable_http2", &request.DisableHTTP2)
	setString(args, "proxy_url", &request.ProxyURL)

	return request
}

func buildEntryModificationRequest(args map[string]interface{}) *client.EntryModificationRequest {
	request := &client.EntryModificationRequest{}

	setStringPtr(args, "title", &request.Title)
	setStringPtr(args, "content", &request.Content)

	return request
}

func buildImportFeedEntryPayload(args map[string]interface{}) map[string]interface{} {
	payload := map[string]interface{}{}

	for _, key := range []string{"title", "url", "author", "content", "status", "external_id", "comments_url"} {
		if value, ok := args[key].(string); ok {
			payload[key] = value
		}
	}
	if publishedAt, ok := args["published_at"].(float64); ok {
		payload["published_at"] = int64(publishedAt)
	}
	if starred, ok := args["starred"].(bool); ok {
		payload["starred"] = starred
	}
	if tags, ok := args["tags"].([]interface{}); ok {
		tagStrings := make([]string, 0, len(tags))
		for _, tag := range tags {
			if tagString, ok := tag.(string); ok {
				tagStrings = append(tagStrings, tagString)
			}
		}
		payload["tags"] = tagStrings
	}

	return payload
}

func setStringPtr(args map[string]interface{}, key string, dest **string) {
	if value, ok := args[key].(string); ok {
		*dest = &value
	}
}

func setBoolPtr(args map[string]interface{}, key string, dest **bool) {
	if value, ok := args[key].(bool); ok {
		*dest = &value
	}
}

func setInt64Ptr(args map[string]interface{}, key string, dest **int64) {
	if value, ok := args[key].(float64); ok {
		intValue := int64(value)
		*dest = &intValue
	}
}

func setString(args map[string]interface{}, key string, dest *string) {
	if value, ok := args[key].(string); ok {
		*dest = value
	}
}

func setBool(args map[string]interface{}, key string, dest *bool) {
	if value, ok := args[key].(bool); ok {
		*dest = value
	}
}

func setInt64(args map[string]interface{}, key string, dest *int64) {
	if value, ok := args[key].(float64); ok {
		*dest = int64(value)
	}
}

func (s *MinifluxServer) GetEntry(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.Params.Arguments
	if args == nil {
		return mcp.NewToolResultError("entry_id is required"), nil
	}

	argsMap, ok := args.(map[string]interface{})
	if !ok {
		return mcp.NewToolResultError("Invalid arguments format"), nil
	}

	entryIDFloat, ok := argsMap["entry_id"].(float64)
	if !ok {
		return mcp.NewToolResultError("entry_id must be a number"), nil
	}

	entryID := int64(entryIDFloat)
	entry, err := s.client.Entry(entryID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to fetch entry: %v", err)), nil
	}

	entryJSON, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal entry: %v", err)), nil
	}

	return mcp.NewToolResultText(string(entryJSON)), nil
}

func (s *MinifluxServer) UpdateEntryStatus(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.Params.Arguments
	if args == nil {
		return mcp.NewToolResultError("entry_id and status are required"), nil
	}

	argsMap, ok := args.(map[string]interface{})
	if !ok {
		return mcp.NewToolResultError("Invalid arguments format"), nil
	}

	entryIDFloat, ok := argsMap["entry_id"].(float64)
	if !ok {
		return mcp.NewToolResultError("entry_id must be a number"), nil
	}

	status, ok := argsMap["status"].(string)
	if !ok {
		return mcp.NewToolResultError("status must be a string"), nil
	}

	entryID := int64(entryIDFloat)
	err := s.client.UpdateEntries([]int64{entryID}, status)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to update entry status: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Entry %d status updated to: %s", entryID, status)), nil
}

func (s *MinifluxServer) UpdateEntriesStatus(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.Params.Arguments
	if args == nil {
		return mcp.NewToolResultError("entry_ids and status are required"), nil
	}

	argsMap, ok := args.(map[string]interface{})
	if !ok {
		return mcp.NewToolResultError("Invalid arguments format"), nil
	}

	entryIDsArg, ok := argsMap["entry_ids"].([]interface{})
	if !ok {
		return mcp.NewToolResultError("entry_ids must be an array"), nil
	}
	if len(entryIDsArg) == 0 {
		return mcp.NewToolResultError("entry_ids must not be empty"), nil
	}

	entryIDs := make([]int64, 0, len(entryIDsArg))
	for _, entryIDArg := range entryIDsArg {
		entryIDFloat, ok := entryIDArg.(float64)
		if !ok {
			return mcp.NewToolResultError("entry_ids must contain only numbers"), nil
		}
		entryIDs = append(entryIDs, int64(entryIDFloat))
	}

	status, ok := argsMap["status"].(string)
	if !ok {
		return mcp.NewToolResultError("status must be a string"), nil
	}

	err := s.client.UpdateEntries(entryIDs, status)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to update entries status: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("%d entries status updated to: %s", len(entryIDs), status)), nil
}

func (s *MinifluxServer) CreateFeed(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.Params.Arguments
	if args == nil {
		return mcp.NewToolResultError("feed_url is required"), nil
	}

	argsMap, ok := args.(map[string]interface{})
	if !ok {
		return mcp.NewToolResultError("Invalid arguments format"), nil
	}

	if _, ok := argsMap["feed_url"].(string); !ok {
		return mcp.NewToolResultError("feed_url must be a string"), nil
	}

	feedRequest := buildFeedCreationRequest(argsMap)

	createdFeed, err := s.client.CreateFeed(feedRequest)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create feed: %v", err)), nil
	}

	feedJSON, err := json.MarshalIndent(createdFeed, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal created feed: %v", err)), nil
	}

	return mcp.NewToolResultText(string(feedJSON)), nil
}

func (s *MinifluxServer) GetCategories(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	categories, err := s.client.Categories()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to fetch categories: %v", err)), nil
	}

	categoriesJSON, err := json.MarshalIndent(categories, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal categories: %v", err)), nil
	}

	return mcp.NewToolResultText(string(categoriesJSON)), nil
}

func (s *MinifluxServer) RefreshFeed(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.Params.Arguments
	if args == nil {
		return mcp.NewToolResultError("feed_id is required"), nil
	}

	argsMap, ok := args.(map[string]interface{})
	if !ok {
		return mcp.NewToolResultError("Invalid arguments format"), nil
	}

	feedIDFloat, ok := argsMap["feed_id"].(float64)
	if !ok {
		return mcp.NewToolResultError("feed_id must be a number"), nil
	}

	feedID := int64(feedIDFloat)
	err := s.client.RefreshFeed(feedID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to refresh feed: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Feed %d refreshed successfully", feedID)), nil
}

// User Management Methods
func (s *MinifluxServer) GetUsers(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	users, err := s.client.Users()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to fetch users: %v", err)), nil
	}

	usersJSON, err := json.MarshalIndent(users, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal users: %v", err)), nil
	}

	return mcp.NewToolResultText(string(usersJSON)), nil
}

func (s *MinifluxServer) GetMe(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	user, err := s.client.Me()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to fetch current user: %v", err)), nil
	}

	userJSON, err := json.MarshalIndent(user, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal user: %v", err)), nil
	}

	return mcp.NewToolResultText(string(userJSON)), nil
}

func (s *MinifluxServer) GetUserByID(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.Params.Arguments
	if args == nil {
		return mcp.NewToolResultError("user_id is required"), nil
	}

	argsMap, ok := args.(map[string]interface{})
	if !ok {
		return mcp.NewToolResultError("Invalid arguments format"), nil
	}

	userIDFloat, ok := argsMap["user_id"].(float64)
	if !ok {
		return mcp.NewToolResultError("user_id must be a number"), nil
	}

	userID := int64(userIDFloat)
	user, err := s.client.UserByID(userID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to fetch user: %v", err)), nil
	}

	userJSON, err := json.MarshalIndent(user, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal user: %v", err)), nil
	}

	return mcp.NewToolResultText(string(userJSON)), nil
}

func (s *MinifluxServer) GetUserByUsername(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.Params.Arguments
	if args == nil {
		return mcp.NewToolResultError("username is required"), nil
	}

	argsMap, ok := args.(map[string]interface{})
	if !ok {
		return mcp.NewToolResultError("Invalid arguments format"), nil
	}

	username, ok := argsMap["username"].(string)
	if !ok {
		return mcp.NewToolResultError("username must be a string"), nil
	}

	user, err := s.client.UserByUsername(username)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to fetch user: %v", err)), nil
	}

	userJSON, err := json.MarshalIndent(user, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal user: %v", err)), nil
	}

	return mcp.NewToolResultText(string(userJSON)), nil
}

func (s *MinifluxServer) CreateUser(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.Params.Arguments
	if args == nil {
		return mcp.NewToolResultError("username and password are required"), nil
	}

	argsMap, ok := args.(map[string]interface{})
	if !ok {
		return mcp.NewToolResultError("Invalid arguments format"), nil
	}

	username, ok := argsMap["username"].(string)
	if !ok {
		return mcp.NewToolResultError("username must be a string"), nil
	}

	password, ok := argsMap["password"].(string)
	if !ok {
		return mcp.NewToolResultError("password must be a string"), nil
	}

	var isAdmin bool
	if adminVal, ok := argsMap["is_admin"].(bool); ok {
		isAdmin = adminVal
	}

	user, err := s.client.CreateUser(username, password, isAdmin)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create user: %v", err)), nil
	}

	userJSON, err := json.MarshalIndent(user, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal user: %v", err)), nil
	}

	return mcp.NewToolResultText(string(userJSON)), nil
}

func (s *MinifluxServer) DeleteUser(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.Params.Arguments
	if args == nil {
		return mcp.NewToolResultError("user_id is required"), nil
	}

	argsMap, ok := args.(map[string]interface{})
	if !ok {
		return mcp.NewToolResultError("Invalid arguments format"), nil
	}

	userIDFloat, ok := argsMap["user_id"].(float64)
	if !ok {
		return mcp.NewToolResultError("user_id must be a number"), nil
	}

	userID := int64(userIDFloat)
	err := s.client.DeleteUser(userID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to delete user: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("User %d deleted successfully", userID)), nil
}

// Category Management Methods
func (s *MinifluxServer) CreateCategory(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.Params.Arguments
	if args == nil {
		return mcp.NewToolResultError("title is required"), nil
	}

	argsMap, ok := args.(map[string]interface{})
	if !ok {
		return mcp.NewToolResultError("Invalid arguments format"), nil
	}

	title, ok := argsMap["title"].(string)
	if !ok {
		return mcp.NewToolResultError("title must be a string"), nil
	}

	category, err := s.client.CreateCategory(title)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create category: %v", err)), nil
	}

	categoryJSON, err := json.MarshalIndent(category, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal category: %v", err)), nil
	}

	return mcp.NewToolResultText(string(categoryJSON)), nil
}

func (s *MinifluxServer) UpdateCategory(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.Params.Arguments
	if args == nil {
		return mcp.NewToolResultError("category_id and title are required"), nil
	}

	argsMap, ok := args.(map[string]interface{})
	if !ok {
		return mcp.NewToolResultError("Invalid arguments format"), nil
	}

	categoryIDFloat, ok := argsMap["category_id"].(float64)
	if !ok {
		return mcp.NewToolResultError("category_id must be a number"), nil
	}

	title, ok := argsMap["title"].(string)
	if !ok {
		return mcp.NewToolResultError("title must be a string"), nil
	}

	categoryID := int64(categoryIDFloat)
	category, err := s.client.UpdateCategory(categoryID, title)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to update category: %v", err)), nil
	}

	categoryJSON, err := json.MarshalIndent(category, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal category: %v", err)), nil
	}

	return mcp.NewToolResultText(string(categoryJSON)), nil
}

func (s *MinifluxServer) DeleteCategory(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.Params.Arguments
	if args == nil {
		return mcp.NewToolResultError("category_id is required"), nil
	}

	argsMap, ok := args.(map[string]interface{})
	if !ok {
		return mcp.NewToolResultError("Invalid arguments format"), nil
	}

	categoryIDFloat, ok := argsMap["category_id"].(float64)
	if !ok {
		return mcp.NewToolResultError("category_id must be a number"), nil
	}

	categoryID := int64(categoryIDFloat)
	err := s.client.DeleteCategory(categoryID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to delete category: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Category %d deleted successfully", categoryID)), nil
}

func (s *MinifluxServer) GetCategoryFeeds(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.Params.Arguments
	if args == nil {
		return mcp.NewToolResultError("category_id is required"), nil
	}

	argsMap, ok := args.(map[string]interface{})
	if !ok {
		return mcp.NewToolResultError("Invalid arguments format"), nil
	}

	categoryIDFloat, ok := argsMap["category_id"].(float64)
	if !ok {
		return mcp.NewToolResultError("category_id must be a number"), nil
	}

	categoryID := int64(categoryIDFloat)
	feeds, err := s.client.CategoryFeeds(categoryID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to fetch category feeds: %v", err)), nil
	}

	feedsJSON, err := json.MarshalIndent(feeds, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal feeds: %v", err)), nil
	}

	return mcp.NewToolResultText(string(feedsJSON)), nil
}

func (s *MinifluxServer) GetCategoryEntries(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.Params.Arguments
	if args == nil {
		return mcp.NewToolResultError("category_id is required"), nil
	}

	argsMap, ok := args.(map[string]interface{})
	if !ok {
		return mcp.NewToolResultError("Invalid arguments format"), nil
	}

	categoryIDFloat, ok := argsMap["category_id"].(float64)
	if !ok {
		return mcp.NewToolResultError("category_id must be a number"), nil
	}

	categoryID := int64(categoryIDFloat)
	filter := buildScopedEntryFilter(argsMap, "category_id")

	entries, err := s.client.CategoryEntries(categoryID, filter)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to fetch category entries: %v", err)), nil
	}

	entriesJSON, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal entries: %v", err)), nil
	}

	return mcp.NewToolResultText(string(entriesJSON)), nil
}

func (s *MinifluxServer) MarkCategoryAsRead(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.Params.Arguments
	if args == nil {
		return mcp.NewToolResultError("category_id is required"), nil
	}

	argsMap, ok := args.(map[string]interface{})
	if !ok {
		return mcp.NewToolResultError("Invalid arguments format"), nil
	}

	categoryIDFloat, ok := argsMap["category_id"].(float64)
	if !ok {
		return mcp.NewToolResultError("category_id must be a number"), nil
	}

	categoryID := int64(categoryIDFloat)
	err := s.client.MarkCategoryAsRead(categoryID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to mark category as read: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Category %d marked as read", categoryID)), nil
}

func (s *MinifluxServer) RefreshCategory(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.Params.Arguments
	if args == nil {
		return mcp.NewToolResultError("category_id is required"), nil
	}

	argsMap, ok := args.(map[string]interface{})
	if !ok {
		return mcp.NewToolResultError("Invalid arguments format"), nil
	}

	categoryIDFloat, ok := argsMap["category_id"].(float64)
	if !ok {
		return mcp.NewToolResultError("category_id must be a number"), nil
	}

	categoryID := int64(categoryIDFloat)
	err := s.client.RefreshCategory(categoryID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to refresh category: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Category %d refreshed successfully", categoryID)), nil
}

func main() {
	minifluxServer := NewMinifluxServer()

	s := server.NewMCPServer(
		"miniflux-mcp",
		"0.1.0",
		server.WithLogging(),
	)

	// Register all tools
	minifluxServer.RegisterAllTools(s)

	if err := server.ServeStdio(s); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
