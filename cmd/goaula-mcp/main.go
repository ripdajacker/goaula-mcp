// Command goaula-mcp is an MCP server exposing the Aula API (via the aula client library)
// as read-only tools over stdio.
//
// The session token is assumed to be valid and is read from ~/.aula/session.txt,
// exactly like the client library. If login fails (e.g. the session has expired), the
// server still starts so clients can list the tools — every tool call then returns an
// actionable "refresh the session" error.
//
// Optional environment variables:
//   - AULA_MCP_API_URL to override the Aula API base URL (defaults to
//     https://www.aula.dk/api/)
//   - GOAULA_MCP_PING_INTERVAL ping interval, e.g. "15m" or minutes
//     like "15" (default 15m; "0" or "off" disables the ping loop)
package main

import (
	"context"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	aula "github.com/ripdajacker/goaula-mcp"
)

const serverInstructions = "Read-only access to the Danish Aula school portal (aula.dk). Start with " +
	"'aula_whoami' to see the available children and institutions and their IDs, then use " +
	"the aula_posts, aula_messages, aula_threads, aula_thread, aula_calendar, aula_gallery, " +
	"aula_daily_overview, aula_search_people, aula_contact_groups and aula_contacts tools. " +
	"Some tools are scoped to the current child/institution context, which can be changed " +
	"with 'aula_set_context'."

// Build information, overridable via -ldflags "-X ..." (see build.sh).
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	config := &aula.Config{
		Session: aula.NewFileSessionProvider(),
	}
	if apiURL := strings.TrimSpace(os.Getenv("AULA_MCP_API_URL")); apiURL != "" {
		config.APIURL = apiURL
	}

	session := newAulaSession(config)
	session.login()

	server := mcp.NewServer(
		&mcp.Implementation{Name: "aula", Version: "1.0.0"},
		&mcp.ServerOptions{Instructions: serverInstructions})

	toolCount := registerTools(server, session)
	registerAuthTools(server, session)
	slog.Info("Aula MCP server started", "tools", toolCount+3, "sessionReady", session.isReady())

	// Keep the session token alive with periodic pings, like the TS
	// aula-mcp-server. After several consecutive failures the session is
	// marked expired and tools return the actionable "refresh session" error.
	if interval, ok := pingInterval(); ok {
		session.startPing(interval)
		slog.Info("Ping loop started", "interval", interval)
	} else {
		slog.Info("Ping loop disabled via GOAULA_MCP_PING_INTERVAL")
	}

	// Run until stdin is closed (the client disconnects). All logging goes to stderr
	// (slog's default), since stdout carries the JSON-RPC protocol.
	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		slog.Error("MCP server stopped with error", "error", err)
	}
	session.close()
}

// pingInterval reads GOAULA_MCP_PING_INTERVAL. Accepts a Go duration ("15m",
// "1h") or a plain number of minutes ("15"), mirroring the TS server's
// PING_INTERVAL_MINUTES. Returns ok=false when the ping loop is explicitly
// disabled ("0" or "off").
func pingInterval() (time.Duration, bool) {
	const def = 15 * time.Minute

	v := strings.TrimSpace(os.Getenv("GOAULA_MCP_PING_INTERVAL"))
	if v == "" {
		return def, true
	}
	if v == "0" || strings.EqualFold(v, "off") {
		return 0, false
	}
	if d, err := time.ParseDuration(v); err == nil && d > 0 {
		return d, true
	}
	if mins, err := strconv.Atoi(v); err == nil && mins > 0 {
		return time.Duration(mins) * time.Minute, true
	}
	slog.Warn("Invalid GOAULA_MCP_PING_INTERVAL, using default", "value", v, "default", def)
	return def, true
}
