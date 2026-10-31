package main

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/ripdajacker/goaula-mcp"
)

// Gallery tool: photo albums with media.

func registerGalleryTools(server *mcp.Server, session *aulaSession, count func()) {
	type galleryArgs struct {
		AlbumLimit *int    `json:"albumLimit,omitempty"`
		MediaLimit *int    `json:"mediaLimit,omitempty"`
		Days       *int    `json:"days,omitempty"`
		ChildIDs   []int64 `json:"childIds,omitempty"`
	}
	addTool(server, session, "aula_gallery", "List gallery albums",
		"Lists Aula gallery albums with their media (photo URLs). Albums are sorted newest first.",
		`{
		  "type": "object",
		  "properties": {
		    "albumLimit": {
		      "type": "integer",
		      "description": "Maximum number of albums. Default: 12."
		    },
		    "mediaLimit": {
		      "type": "integer",
		      "description": "Maximum number of media items per album. Default: 30."
		    },
		    "days": {
		      "type": "integer",
		      "description": "If set, only albums created within the past N days are returned."
		    },
		    "childIds": {
		      "type": "array",
		      "items": { "type": "integer" },
		      "description": "Optional child IDs to filter by. Empty means the current context."
		    }
		  },
		  "additionalProperties": false
		}`,
		func(client *aula.Client, args galleryArgs) (any, error) {
			albumLimit := max(1, intArg(args.AlbumLimit, 12))
			mediaLimit := max(1, intArg(args.MediaLimit, 30))

			var retrievePastDays *int
			if args.Days != nil && *args.Days > 0 {
				retrievePastDays = args.Days
			}

			return client.GalleryAlbumMedia(albumLimit, mediaLimit, nil, retrievePastDays, args.ChildIDs)
		})
	count()
}
