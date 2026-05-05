#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
COMPOSE_FILE="${ROOT_DIR}/docker-compose.integration.yml"
PROJECT_NAME="${COMPOSE_PROJECT_NAME:-miniflux-mcp-integration}"
PORT="${MINIFLUX_INTEGRATION_PORT:-18080}"
MINIFLUX_URL="${MINIFLUX_URL:-http://127.0.0.1:${PORT}}"
MINIFLUX_USERNAME="${MINIFLUX_INTEGRATION_USERNAME:-admin}"
MINIFLUX_PASSWORD="${MINIFLUX_INTEGRATION_PASSWORD:-miniflux-admin}"
SMOKE_FEED_URL="${MINIFLUX_INTEGRATION_FEED_URL:-https://cprss.s3.amazonaws.com/javascriptweekly.com.xml}"
KEEP_MINIFLUX="${KEEP_MINIFLUX:-0}"

if ! command -v docker >/dev/null 2>&1; then
  echo "docker is required for integration smoke testing" >&2
  exit 1
fi

if ! docker compose version >/dev/null 2>&1; then
  echo "docker compose is required for integration smoke testing" >&2
  exit 1
fi

if ! command -v curl >/dev/null 2>&1; then
  echo "curl is required for integration smoke testing" >&2
  exit 1
fi

cleanup() {
  if [[ "${KEEP_MINIFLUX}" != "1" ]]; then
    docker compose -p "${PROJECT_NAME}" -f "${COMPOSE_FILE}" down -v --remove-orphans >/dev/null
  fi
}
trap cleanup EXIT

echo "Starting local Miniflux integration stack at ${MINIFLUX_URL}"
docker compose -p "${PROJECT_NAME}" -f "${COMPOSE_FILE}" up -d

echo "Waiting for Miniflux healthcheck"
for _ in $(seq 1 60); do
  if curl -fsS "${MINIFLUX_URL}/healthcheck" >/dev/null 2>&1; then
    break
  fi
  sleep 2
done

curl -fsS "${MINIFLUX_URL}/healthcheck" >/dev/null

TMP_DIR="$(mktemp -d "${ROOT_DIR}/.integration-smoke.XXXXXX")"
trap 'rm -rf "${TMP_DIR}"; cleanup' EXIT

cat > "${TMP_DIR}/smoke.go" <<'GO'
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	mcpclient "github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
)

func main() {
	rootDir := mustEnv("ROOT_DIR")
	minifluxURL := mustEnv("MINIFLUX_URL")
	username := mustEnv("MINIFLUX_USERNAME")
	password := mustEnv("MINIFLUX_PASSWORD")
	smokeFeedURL := mustEnv("SMOKE_FEED_URL")

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	env := []string{
		"MINIFLUX_URL=" + minifluxURL,
		"MINIFLUX_USERNAME=" + username,
		"MINIFLUX_PASSWORD=" + password,
		"MINIFLUX_API_KEY=",
	}

	c, err := mcpclient.NewStdioMCPClientWithOptions(
		"go",
		env,
		[]string{"run", "."},
		transport.WithCommandFunc(func(ctx context.Context, command string, env []string, args []string) (*exec.Cmd, error) {
			cmd := exec.CommandContext(ctx, command, args...)
			cmd.Dir = rootDir
			cmd.Env = append(filteredEnv("MINIFLUX_URL", "MINIFLUX_USERNAME", "MINIFLUX_PASSWORD", "MINIFLUX_API_KEY"), env...)
			return cmd, nil
		}),
	)
	if err != nil {
		log.Fatalf("start MCP stdio client: %v", err)
	}
	defer c.Close()

	initReq := mcp.InitializeRequest{}
	initReq.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initReq.Params.ClientInfo = mcp.Implementation{Name: "miniflux-mcp integration smoke", Version: "1.0.0"}
	initReq.Params.Capabilities = mcp.ClientCapabilities{}

	if _, err := c.Initialize(ctx, initReq); err != nil {
		log.Fatalf("initialize MCP server: %v", err)
	}

	for _, toolName := range []string{"healthcheck", "get_me", "get_categories", "get_feeds"} {
		callTool(ctx, c, toolName)
	}
	callToolWithArgs(ctx, c, "create_feed", map[string]any{"feed_url": smokeFeedURL})
	callTool(ctx, c, "get_feeds")

	fmt.Printf("integration smoke passed for %s\n", filepath.Base(rootDir))
}

func mustEnv(name string) string {
	value := os.Getenv(name)
	if value == "" {
		log.Fatalf("%s is required", name)
	}
	return value
}

func filteredEnv(names ...string) []string {
	env := []string{}
	for _, entry := range os.Environ() {
		keep := true
		for _, name := range names {
			if strings.HasPrefix(entry, name+"=") {
				keep = false
				break
			}
		}
		if keep {
			env = append(env, entry)
		}
	}
	return env
}

func callTool(ctx context.Context, c *mcpclient.Client, name string) {
	callToolWithArgs(ctx, c, name, map[string]any{})
}

func callToolWithArgs(ctx context.Context, c *mcpclient.Client, name string, args map[string]any) {
	req := mcp.CallToolRequest{}
	req.Params.Name = name
	req.Params.Arguments = args

	result, err := c.CallTool(ctx, req)
	if err != nil {
		log.Fatalf("%s transport error: %v", name, err)
	}
	if result.IsError {
		log.Fatalf("%s returned MCP error: %#v", name, result.Content)
	}

	fmt.Printf("ok %s\n", name)
}
GO

echo "Calling MCP tools against local Miniflux"
ROOT_DIR="${ROOT_DIR}" \
MINIFLUX_URL="${MINIFLUX_URL}" \
MINIFLUX_USERNAME="${MINIFLUX_USERNAME}" \
MINIFLUX_PASSWORD="${MINIFLUX_PASSWORD}" \
SMOKE_FEED_URL="${SMOKE_FEED_URL}" \
go run "${TMP_DIR}/smoke.go"

if [[ "${KEEP_MINIFLUX}" == "1" ]]; then
  echo "Miniflux stack left running with compose project ${PROJECT_NAME}"
fi
