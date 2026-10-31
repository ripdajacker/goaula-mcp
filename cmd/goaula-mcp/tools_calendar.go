package main

import (
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/ripdajacker/goaula-mcp"
)

// Calendar tool: events within a date range, optionally filtered by event type.

func registerCalendarTools(server *mcp.Server, session *aulaSession, count func()) {
	type calendarArgs struct {
		DaysInPast   *int    `json:"daysInPast,omitempty"`
		DaysInFuture *int    `json:"daysInFuture,omitempty"`
		EventType    string  `json:"eventType,omitempty"`
		ChildIDs     []int64 `json:"childIds,omitempty"`
	}
	addTool(server, session, "aula_calendar", "Get calendar events",
		"Fetches Aula calendar events (lessons, holidays, birthdays, excursions, meetings) for a date "+
			"range, optionally filtered by event type and children.",
		`{
		  "type": "object",
		  "properties": {
		    "daysInPast": {
		      "type": "integer",
		      "description": "Days back from today. Default: 7."
		    },
		    "daysInFuture": {
		      "type": "integer",
		      "description": "Days forward from today. Default: 14."
		    },
		    "eventType": {
		      "type": "string",
		      "enum": ["event", "holiday", "presence_holiday", "birthday", "excursion", "other",
		               "school_home_meeting", "parental_meeting", "performance_meeting", "lesson"],
		      "description": "Optional event type filter."
		    },
		    "childIds": {
		      "type": "array",
		      "items": { "type": "integer" },
		      "description": "Optional child IDs to filter by. Empty means the current context."
		    }
		  },
		  "additionalProperties": false
		}`,
		func(client *aula.Client, args calendarArgs) (any, error) {
			daysInPast := intArg(args.DaysInPast, 7)
			daysInFuture := intArg(args.DaysInFuture, 14)

			var eventType *aula.CalendarEventType
			if t := strings.TrimSpace(args.EventType); t != "" && !strings.EqualFold(t, "any") {
				parsed, ok := aula.ParseCalendarEventType(t)
				if !ok {
					return nil, fmt.Errorf("Unknown event type: %s", t)
				}
				eventType = &parsed
			}

			events, err := client.CalendarEvents(daysInPast, daysInFuture, args.ChildIDs, eventType)
			if err != nil {
				return nil, err
			}
			return struct {
				Count  int                  `json:"count"`
				Events []aula.CalendarEvent `json:"events"`
			}{len(events), events}, nil
		})
	count()
}
