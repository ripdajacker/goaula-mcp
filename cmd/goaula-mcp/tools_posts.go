package main

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/ripdajacker/goaula-mcp"
)

// Post feed tool: list recent posts. (There is no single-post-by-ID endpoint on the
// current Aula API; the feed is the only post source.)

func registerPostTools(server *mcp.Server, session *aulaSession, count func()) {
	type postsArgs struct {
		Days     *int    `json:"days,omitempty"`
		Limit    *int    `json:"limit,omitempty"`
		ChildIDs []int64 `json:"childIds,omitempty"`
	}
	addTool(server, session, "aula_posts", "List recent posts",
		"Lists Aula posts/announcements from the feed, newest first, with title, content, owner, "+
			"attachment URLs and comment counts.",
		`{
		  "type": "object",
		  "properties": {
		    "days": {
		      "type": "integer",
		      "description": "Number of days back to fetch posts for. 0 disables the date filter. Default: 7."
		    },
		    "limit": {
		      "type": "integer",
		      "description": "Maximum number of posts. Default: 20."
		    },
		    "childIds": {
		      "type": "array",
		      "items": { "type": "integer" },
		      "description": "Optional child IDs to filter by. Empty means the current context."
		    }
		  },
		  "additionalProperties": false
		}`,
		func(client *aula.Client, args postsArgs) (any, error) {
			days := intArg(args.Days, 7)
			limit := max(1, intArg(args.Limit, 20))

			var posts []aula.Post
			var err error
			if days > 0 {
				posts, err = client.AllPosts(days, args.ChildIDs, min(limit, 50))
			} else {
				var page *aula.PagedResult[aula.Post]
				page, err = client.Posts(0, limit, args.ChildIDs)
				if err == nil {
					posts = page.Items
				}
			}
			if err != nil {
				return nil, err
			}
			return struct {
				Count int         `json:"count"`
				Posts []aula.Post `json:"posts"`
			}{len(posts), posts}, nil
		})
	count()
}
