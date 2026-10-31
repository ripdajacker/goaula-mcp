package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/ripdajacker/goaula-mcp"
)

func addTool[In any](server *mcp.Server, session *aulaSession, name, title, description, inputSchema string,
	handler func(client *aula.Client, args In) (any, error)) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        name,
		Title:       title,
		Description: description,
		InputSchema: json.RawMessage(inputSchema),
		Annotations: &mcp.ToolAnnotations{
			Title:          title,
			ReadOnlyHint:   true,
			IdempotentHint: false,
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args In) (*mcp.CallToolResult, any, error) {
		result, err := session.call(func(client *aula.Client) (any, error) {
			return handler(client, args)
		})
		if err != nil {
			var invalidSession *aula.InvalidSessionError
			if errors.As(err, &invalidSession) {
				slog.Error("Session error while running tool", "tool", name, "error", err)
				return nil, nil, errors.New(invalidSession.Message)
			}
			slog.Error("Tool failed", "tool", name, "error", err)
			return nil, nil, fmt.Errorf("Tool '%s' failed: %w", name, err)
		}
		data, err := aula.MarshalJSON(result)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to serialize tool result: %w", err)
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: string(data)}},
		}, nil, nil
	})
}

// registerTools registers all Aula tools on the server and returns the tool count.
func registerTools(server *mcp.Server, session *aulaSession) int {
	n := 0
	count := func() { n++ }
	registerProfileTools(server, session, count)
	registerPostTools(server, session, count)
	registerMessagingTools(server, session, count)
	registerContactTools(server, session, count)
	registerCalendarTools(server, session, count)
	registerGalleryTools(server, session, count)
	registerPresenceTools(server, session, count)
	return n
}

// intArg returns *v when set, otherwise the default.
func intArg(v *int, defaultValue int) int {
	if v == nil {
		return defaultValue
	}
	return *v
}

// idString converts a JSON string-or-integer argument to a string.
func idString(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case json.Number:
		return t.String()
	default:
		return fmt.Sprint(v)
	}
}
