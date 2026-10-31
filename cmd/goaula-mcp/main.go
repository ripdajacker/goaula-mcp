// Command goaula-mcp is an MCP server exposing the Aula API (via the aula client library)
// as read-only tools over stdio.
//
// The session token is assumed to be valid and is read from ~/.aula/session.txt,
// exactly like the client library. If login fails (e.g. the session has expired), the
// server still starts so clients can list the tools — every tool call then returns an
// actionable "refresh the session" error.
//
// Optional environment variable: AULA_MCP_API_URL to override the Aula API base URL
// (defaults to https://www.aula.dk/api/).
package main

import (
	"context"
	"log/slog"
	"os"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/ripdajacker/goaula-mcp"
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

	client := aula.NewClient(config)
	session := newAulaSession(client)
	session.login()

	server := mcp.NewServer(
		&mcp.Implementation{Name: "aula", Version: "1.0.0"},
		&mcp.ServerOptions{Instructions: serverInstructions})

	toolCount := registerTools(server, session)
	slog.Info("Aula MCP server started", "tools", toolCount, "sessionReady", session.isReady())

	// Run until stdin is closed (the client disconnects). All logging goes to stderr
	// (slog's default), since stdout carries the JSON-RPC protocol.
	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		slog.Error("MCP server stopped with error", "error", err)
	}
	session.close()
}
