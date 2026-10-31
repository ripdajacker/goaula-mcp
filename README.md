# goaula-mcp

A Go client library for theaula.dk, plus an MCP server that exposes it to LLM clients.

## Project structure

| Path | Purpose |
|------|---------|
| module root (package `aula`) | Client library: HTTP connector, models, MeeBook client, HTML writers |
| `cmd/goaula-mcp` | MCP server exposing the Aula API as Model Context Protocol tools over stdio |

## Features

- Pure `net/http` client — no headless browser required
- Full feature coverage:
  - Daily overview
  - Message threads and messages (within a timeframe)
  - Posts (within a timeframe)
  - Attachments (images, files) on posts and messages
  - Lookup of parents, children, teachers, etc.
  - Contact groups and contact lists
  - Gallery albums
  - Calendar events
  - MeeBook integration
- Child filtering: filter posts, messages, threads, calendar events, and gallery by specific children

## Requirements

- Go 1.25+ (the `go` toolchain auto-downloads when `GOTOOLCHAIN=auto`)

## Building

```bash
go build ./...
go test ./...
```

## Library usage

```go
package main

import (
	"fmt"
	"log"

	aula "github.com/ripdajacker/goaula-mcp"
)

func main() {
	// Uses ~/.aula/session.txt for session persistence
	client := aula.NewClient(&aula.Config{
		Session: aula.NewFileSessionProvider(),
	})
	defer client.Close()

	if err := client.Login(); err != nil {
		log.Fatal(err)
	}

	// List all children
	for _, child := range client.MyChildren() {
		fmt.Printf("%s (ID: %d)\n", child.Name, child.ID)
	}

	// Latest 20 posts from the last 21 days
	posts, err := client.RecentPosts(21, 20, nil)
	if err != nil {
		log.Fatal(err)
	}
	for _, post := range posts {
		fmt.Println(post.Title)
	}
}
```

All methods return an `error`; handle it as shown above.

```go
// All posts from the last 21 days (auto-paginates), for a specific child
allPosts, err := client.AllPosts(21, []int64{childID}, 20)

// First page of messages, 20 per page
page, err := client.FindMessages(nil, nil, nil, nil, 0, 20)

// A full thread with all pages of messages
thread, err := client.ThreadAllPages("thread id")

// Calendar events for next week
events, err := client.CalendarEvents(3, 7, nil, nil)
```

### Session providers

- `aula.NewFileSessionProvider()`: reads `~/.aula/session.txt`. This is the default when `Config.Session` is nil.
- `aula.StaticSessionProvider{ID: os.Getenv("AULA_SESSION_ID")}`: fixed session ID, e.g. from an environment variable.

Custom providers implement `aula.SessionProvider`:

```go
type SessionProvider interface {
	SessionID() (string, error)    
	SaveSessionID(id string) error 
}
```

## MCP server

`cmd/goaula-mcp` is an [MCP](https://modelcontextprotocol.io) server that exposes Aula as read-only tools over stdio,
so LLM clients (Claude Desktop, OpenCode, etc.) can query posts, messages, threads, calendar, gallery, presence, and
people. It uses the official [MCP Go SDK](https://github.com/modelcontextprotocol/go-sdk).

The server reads the session token from `~/.aula/session.txt`, exactly like the client library. If the session has
expired, the server still starts and every tool then returns an actionable error telling you to refresh the session
file and restart.

### Build and run

```bash
go build -o goaula-mcp ./cmd/goaula-mcp
./goaula-mcp
```

### Tools

| Tool | Description |
|------|-------------|
| `aula_whoami` | Current profile, children, institutions and active context (IDs) |
| `aula_set_context` | Switch the current child/institution context |
| `aula_search_people` | Find parents, children, teachers etc. by name and role |
| `aula_posts` | Recent posts (optionally filtered by days and children) |
| `aula_messages` | Search messages within the past N days |
| `aula_threads` | List message threads (paged, or by activity window) |
| `aula_thread` | A full message thread by ID |
| `aula_calendar` | Calendar events in a date range, optionally by event type |
| `aula_gallery` | Gallery albums with media URLs |
| `aula_daily_overview` | Daily presence overview per child |
| `aula_contact_groups` | Groups (classes, SFO) each child belongs to |
| `aula_contacts` | Contact list of a group (guardians' visible email/phone) |

### Example MCP client configuration

```json
{
  "mcpServers": {
    "aula": {
      "command": "/path/to/goaula-mcp"
    }
  }
}
```

Optional environment variable `AULA_MCP_API_URL` overrides the Aula API base URL
(defaults to `https://www.aula.dk/api/`).

## License

MIT
