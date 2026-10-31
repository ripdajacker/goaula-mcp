package main

import (
	"slices"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/ripdajacker/aula-api-client-go"
)

// Presence tool: daily overview (presence/sleep/meals etc.) for the children.

func registerPresenceTools(server *mcp.Server, session *aulaSession, count func()) {
	type dailyOverviewArgs struct {
		ChildIDs []int64 `json:"childIds,omitempty"`
	}
	addTool(server, session, "aula_daily_overview", "Get daily overview",
		"Fetches the daily presence overview (check-in/check-out status, sleep, meals, activities) for "+
			"each child.",
		`{
		  "type": "object",
		  "properties": {
		    "childIds": {
		      "type": "array",
		      "items": { "type": "integer" },
		      "description": "Optional child IDs. Empty means all children."
		    }
		  },
		  "additionalProperties": false
		}`,
		func(client *aula.Client, args dailyOverviewArgs) (any, error) {
			children := client.AllChildren()
			if len(args.ChildIDs) > 0 {
				var filtered []aula.Child
				for _, child := range children {
					if slices.Contains(args.ChildIDs, child.ID) {
						filtered = append(filtered, child)
					}
				}
				children = filtered
			}

			var overviews []aula.DailyOverview
			for _, child := range children {
				childOverviews, err := client.DailyOverview(child.ID)
				if err != nil {
					return nil, err
				}
				overviews = append(overviews, childOverviews...)
			}
			return overviews, nil
		})
	count()
}
