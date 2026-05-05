# Miniflux MCP Server

A Model Context Protocol (MCP) server for interacting with Miniflux RSS reader. This server provides tools to manage feeds, entries, users, and categories through the MCP protocol using [Miniflux Client](https://github.com/miniflux/v2/tree/main/client).

## Features

- **Feed Management**: List, create, and refresh RSS/Atom feeds
- **Entry Operations**: Read entries, update status (read/unread/removed)
- **Category Management**: List and organize feed categories
- **Flexible Authentication**: Support for both API key and username/password authentication

## Setup Instructions

### Getting a Miniflux API Key

1. Log into your Miniflux instance
2. Go to Settings → API Keys
3. Create a new API key
4. Copy the generated key to your configuration

### Using Docker

```bash
docker build -t miniflux-mcp .
docker run --env-file .env miniflux-mcp
```

### Environment Variables

| Variable | Description | Required |
|----------|-------------|----------|
| `MINIFLUX_URL` | Your Miniflux instance URL | Yes |
| `MINIFLUX_API_KEY` | API key for authentication | Yes* |
| `MINIFLUX_USERNAME` | Username for basic auth | Yes* |
| `MINIFLUX_PASSWORD` | Password for basic auth | Yes* |

*Either use `MINIFLUX_API_KEY` OR both `MINIFLUX_USERNAME` and `MINIFLUX_PASSWORD`

### Getting a Miniflux API Key

1. Log into your Miniflux instance
2. Go to Settings → API Keys
3. Create a new API key
4. Copy the generated key to your configuration

### Docker Run

```bash
# Setup .env file
# Run
docker run -i --rm --env-file .env letttgaco/miniflux-mcp:latest
```

### Docker Hub Release Workflow

The Docker publish workflow runs when a `v*` tag is pushed, for example `v1.0.0`.

Configure these repository secrets before publishing:

| Secret | Description |
|--------|-------------|
| `DOCKERHUB_USERNAME` | Docker Hub username used for the image namespace |
| `DOCKERHUB_TOKEN` | Docker Hub access token used to push the image |

The published image name is `letttgaco/miniflux-mcp`.

### Local Integration Smoke Test

To test this MCP server against a disposable local Miniflux instance:

```bash
scripts/integration-smoke.sh
```

This starts Miniflux and Postgres through `docker-compose.integration.yml`, then calls the MCP tools `healthcheck`, `get_me`, `get_categories`, `get_feeds`, `create_feed`, and `get_feeds` again over stdio. The default feed is `https://cprss.s3.amazonaws.com/javascriptweekly.com.xml`; set `MINIFLUX_INTEGRATION_FEED_URL` to test another RSS/Atom URL. Set `KEEP_MINIFLUX=1` to leave the local stack running after the smoke test.

### Integration with Claude Desktop

To use this MCP server with Claude Desktop, add the following to your Claude Desktop configuration:

```json5
{
  "mcpServers": {
    "miniflux": {
      "command": "docker",
      "args": [
        "run",
        "-i",
        "--rm",
        "-e",
        "MINIFLUX_URL",
        "-e",
        "MINIFLUX_API_KEY",
        "letttgaco/miniflux-mcp:latest"
      ],
      "env": {
        "MINIFLUX_URL": "https://your-miniflux-instance.com",
        "MINIFLUX_API_KEY": "your_api_key_here"
        // Or use username/password instead of API key
        // "MINIFLUX_USERNAME": "your_username_here",
        // "MINIFLUX_PASSWORD": "your_password_here"
      }
    }
  }
}
```

## Available Tools

The Miniflux MCP Server provides **51 tools** covering Miniflux API functionality, which can be found in the [Miniflux API Reference](https://miniflux.app/docs/api.html#go-client).

### Feed Management (12 tools)
- `get_feeds` - Get all RSS/Atom feeds
- `get_feed` - Get a specific feed by ID
- `create_feed` - Add a new RSS/Atom feed with optional Miniflux feed settings such as category, scraper/rewrite rules, auth, cookies, proxy, HTTP cache, HTTP/2, and visibility flags
- `delete_feed` - Delete a specific feed
- `update_feed` - Update feed settings
- `refresh_feed` - Manually refresh a specific feed
- `refresh_all_feeds` - Refresh all feeds
- `get_feed_entries` - Get entries from a specific feed
- `get_feed_entry` - Get a specific entry from a feed
- `import_feed_entry` - Import an entry into a feed
- `get_feed_icon` - Get the icon of a specific feed
- `mark_feed_as_read` - Mark all entries in a feed as read

### Entry Management (10 tools)
- `get_entries` - Get entries with optional filtering
- `get_daily_digest` - Get entries since a caller-provided timestamp with digest-ready metadata and content
- `get_entry` - Get a specific entry by ID
- `update_entry_status` - Update entry status (read/unread/removed)
- `update_entries_status` - Update multiple entry statuses (read/unread/removed) in one request
- `toggle_starred` - Toggle starred status of an entry
- `update_entry` - Update entry title or content
- `save_entry` - Save an entry
- `fetch_original_content` - Fetch original content of an entry
- `mark_all_as_read` - Mark all entries as read for a user

### Category Management (9 tools)
- `get_categories` - Get all feed categories
- `create_category` - Create a new category
- `update_category` - Update a category title
- `delete_category` - Delete a category
- `get_category_feeds` - Get all feeds in a specific category
- `get_category_entries` - Get all entries in a specific category
- `get_category_entry` - Get a specific entry from a category
- `mark_category_as_read` - Mark all entries in a category as read
- `refresh_category` - Refresh all feeds in a category

### User Management (6 tools)
- `get_users` - Get all users
- `get_me` - Get current user information
- `get_user_by_id` - Get a specific user by ID
- `get_user_by_username` - Get a specific user by username
- `create_user` - Create a new user
- `delete_user` - Delete a user

### System & Utility (8 tools)
- `get_version` - Get Miniflux version information
- `healthcheck` - Perform a health check
- `fetch_counters` - Fetch feed counters
- `get_integrations_status` - Get integrations status
- `discover` - Discover feeds from a URL
- `export` - Export feeds as OPML
- `import_opml` - Import feeds from OPML content
- `flush_history` - Flush the read history

### API Key Management (3 tools)
- `get_api_keys` - Get all API keys
- `create_api_key` - Create a new API key
- `delete_api_key` - Delete an API key

### Icons & Media (3 tools)
- `get_icon` - Get an icon by ID
- `get_enclosure` - Get an enclosure by ID
- `update_enclosure` - Update enclosure media progression

## AI Digest Workflow

Use `get_daily_digest` when an AI client needs bounded Miniflux input for a daily report or similar digest. The caller must pass `since` as a Unix timestamp or RFC3339 timestamp; the caller owns timezone and date-boundary decisions. Content is returned as Miniflux provides it by default; pass `content_format: "text"` to do lightweight HTML tag stripping and entity decoding.

Recommended flow:

1. The AI client decides the digest window and calls `get_daily_digest` with `since` and an optional bounded `limit`.
2. The AI client summarizes, displays, or pushes the returned entries.
3. After successful delivery or review, the AI client may call `update_entries_status` with `ack_entry_ids` from the digest response and `status: "read"`.

Keep summarization, delivery logs, and timezone policy in the AI client. This MCP server only reads Miniflux data, returns entry content, and updates Miniflux entry status.

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Support

For issues and questions:
1. Check the Miniflux documentation: https://miniflux.app/docs/
2. Review the MCP specification: https://spec.modelcontextprotocol.io/
3. Open an issue in this repository
