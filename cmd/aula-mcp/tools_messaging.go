package main

import (
	"errors"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/ripdajacker/aula-api-client-go"
)

// Messaging tools: message search, thread listing and full thread retrieval.

func registerMessagingTools(server *mcp.Server, session *aulaSession, count func()) {
	// aula_messages
	type messagesArgs struct {
		Days     *int    `json:"days,omitempty"`
		Limit    *int    `json:"limit,omitempty"`
		ChildIDs []int64 `json:"childIds,omitempty"`
	}
	addTool(server, session, "aula_messages", "Find messages",
		"Searches Aula messages within the past N days and returns individual messages with thread ID, "+
			"subject, sender and text. Use aula_thread with the threadId for the full conversation.",
		`{
		  "type": "object",
		  "properties": {
		    "days": {
		      "type": "integer",
		      "description": "Number of days back to search for messages. Default: 3."
		    },
		    "limit": {
		      "type": "integer",
		      "description": "Maximum number of messages. Default: 20."
		    },
		    "childIds": {
		      "type": "array",
		      "items": { "type": "integer" },
		      "description": "Optional child IDs to filter by. Empty means all children."
		    }
		  },
		  "additionalProperties": false
		}`,
		func(client *aula.Client, args messagesArgs) (any, error) {
			days := intArg(args.Days, 3)
			limit := max(1, intArg(args.Limit, 20))

			messages, err := client.Messages(days, args.ChildIDs, limit)
			if err != nil {
				return nil, err
			}
			return struct {
				Count    int                 `json:"count"`
				Messages []aula.FoundMessage `json:"messages"`
			}{len(messages), messages}, nil
		})
	count()

	// aula_threads
	type threadsArgs struct {
		Days     *int    `json:"days,omitempty"`
		Page     *int    `json:"page,omitempty"`
		Limit    *int    `json:"limit,omitempty"`
		ChildIDs []int64 `json:"childIds,omitempty"`
	}
	addTool(server, session, "aula_threads", "List message threads",
		"Lists Aula message threads. Either the most recent threads page by page, or (with 'days') all "+
			"threads with activity in the past N days. Threads are scoped to the current child context "+
			"(see aula_set_context).",
		`{
		  "type": "object",
		  "properties": {
		    "days": {
		      "type": "integer",
		      "description": "If set, only lists threads with activity within the past N days (may fetch several pages)."
		    },
		    "page": {
		      "type": "integer",
		      "description": "Page index (0-based) when 'days' is not set. Default: 0."
		    },
		    "limit": {
		      "type": "integer",
		      "description": "Threads per page. Default: 20."
		    },
		    "childIds": {
		      "type": "array",
		      "items": { "type": "integer" },
		      "description": "Optional child IDs to filter by (ignored by the days-based variant)."
		    }
		  },
		  "additionalProperties": false
		}`,
		func(client *aula.Client, args threadsArgs) (any, error) {
			page := intArg(args.Page, 0)
			limit := max(1, intArg(args.Limit, 20))

			var threads any
			var err error
			if args.Days != nil {
				threads, err = client.Threads(*args.Days, args.ChildIDs)
			} else {
				var result *aula.PagedResult[aula.Thread]
				result, err = client.ListThreads(page, limit, args.ChildIDs)
				if err == nil {
					threads = result.Items
				}
			}
			if err != nil {
				return nil, err
			}
			count := 0
			switch t := threads.(type) {
			case []aula.MessagesForThread:
				count = len(t)
			case []aula.Thread:
				count = len(t)
			}
			return struct {
				Count   int `json:"count"`
				Threads any `json:"threads"`
			}{count, threads}, nil
		})
	count()

	// aula_thread
	type threadArgs struct {
		ThreadID any   `json:"threadId"`
		Full     *bool `json:"full,omitempty"`
	}
	addTool(server, session, "aula_thread", "Get a full message thread",
		"Fetches a single Aula message thread by ID, including all messages with sender, text and "+
			"attachments.",
		`{
		  "type": "object",
		  "properties": {
		    "threadId": {
		      "type": ["string", "integer"],
		      "description": "The thread ID (from aula_threads or aula_messages)."
		    },
		    "full": {
		      "type": "boolean",
		      "description": "Fetch all pages of messages in the thread. Default: true."
		    }
		  },
		  "required": ["threadId"],
		  "additionalProperties": false
		}`,
		func(client *aula.Client, args threadArgs) (any, error) {
			threadID := idString(args.ThreadID)
			if threadID == "" {
				return nil, errors.New("threadId is required")
			}
			full := args.Full == nil || *args.Full
			if full {
				return client.ThreadAllPages(threadID)
			}
			return client.Thread(threadID)
		})
	count()
}
