package main

import (
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/ripdajacker/goaula-mcp"
)

// Profile and context tools: who the logged-in user is, which children/institutions
// exist, and switching the current child/institution context. Also people search.

func registerProfileTools(server *mcp.Server, session *aulaSession, count func()) {
	// aula_whoami
	addTool(server, session, "aula_whoami", "Show current Aula context",
		"Lists the logged-in Aula user's profile, their children (with IDs used by other tools), "+
			"institutions, and the currently selected child/institution context.",
		`{
		  "type": "object",
		  "properties": {},
		  "additionalProperties": false
		}`,
		func(client *aula.Client, args struct{}) (any, error) {
			return contextSummary(client), nil
		})
	count()

	// aula_set_context
	type setContextArgs struct {
		ChildID       *int64 `json:"childId,omitempty"`
		InstitutionID *int64 `json:"institutionId,omitempty"`
	}
	addTool(server, session, "aula_set_context", "Set current child/institution context",
		"Sets the current child and/or institution context. Some Aula endpoints (e.g. message thread "+
			"listing) are scoped to the selected context, so call this before those tools.",
		`{
		  "type": "object",
		  "properties": {
		    "childId": {
		      "type": "integer",
		      "description": "Child ID to select as the current child context (from aula_whoami). Optional."
		    },
		    "institutionId": {
		      "type": "integer",
		      "description": "Institution profile ID to select as the current institution context (from aula_whoami). Optional."
		    }
		  },
		  "additionalProperties": false
		}`,
		func(client *aula.Client, args setContextArgs) (any, error) {
			if args.ChildID != nil && *args.ChildID >= 0 {
				client.SetMyCurrentChild(*args.ChildID)
			}
			if args.InstitutionID != nil && *args.InstitutionID >= 0 {
				client.SetMyCurrentInstitution(*args.InstitutionID)
			}
			return contextSummary(client), nil
		})
	count()

	// aula_search_people
	type searchPeopleArgs struct {
		Name string `json:"name"`
		Role string `json:"role,omitempty"`
	}
	addTool(server, session, "aula_search_people", "Search for people",
		"Searches Aula for people (parents, children, employees) by name and returns their IDs, roles "+
			"and contact details. Use the returned profileId/institutionProfileId for messaging and "+
			"recipient lookups.",
		`{
		  "type": "object",
		  "properties": {
		    "name": {
		      "type": "string",
		      "description": "Name or partial name to search for. Empty matches broadly."
		    },
		    "role": {
		      "type": "string",
		      "enum": ["any", "parent", "child", "employee", "teacher", "preschool_teacher", "leader"],
		      "description": "Optional role filter. Default: any."
		    }
		  },
		  "required": ["name"],
		  "additionalProperties": false
		}`,
		func(client *aula.Client, args searchPeopleArgs) (any, error) {
			var results []aula.Recipient
			var err error
			switch strings.ToLower(args.Role) {
			case "parent":
				results, err = client.FindParents(args.Name)
			case "child":
				results, err = client.FindChildren(args.Name)
			case "teacher":
				results, err = client.FindTeachers(args.Name)
			case "preschool_teacher":
				results, err = client.FindPreschoolTeachers(args.Name)
			case "leader":
				results, err = client.FindLeaders(args.Name)
			case "employee":
				results, err = client.FindEmployees(args.Name)
			default:
				results, err = client.FindPeople(args.Name)
			}
			if err != nil {
				return nil, err
			}
			return results, nil
		})
	count()
}

type contextChildInfo struct {
	ChildID              int64  `json:"childId"`
	Name                 string `json:"name"`
	UserID               string `json:"userId,omitempty"`
	InstitutionCode      string `json:"institutionCode,omitempty"`
	InstitutionProfileID int64  `json:"institutionProfileId,omitempty"`
	InstitutionName      string `json:"institutionName,omitempty"`
}

type contextInstitutionInfo struct {
	InstitutionProfileID int64  `json:"institutionProfileId"`
	InstitutionName      string `json:"institutionName,omitempty"`
	InstitutionCode      string `json:"institutionCode,omitempty"`
}

type contextSummaryResult struct {
	ProfileID              int64                    `json:"profileId,omitempty"`
	DisplayName            string                   `json:"displayName,omitempty"`
	PortalRole             string                   `json:"portalRole,omitempty"`
	Children               []contextChildInfo       `json:"children"`
	Institutions           []contextInstitutionInfo `json:"institutions"`
	CurrentChildID         int64                    `json:"currentChildId,omitempty"`
	CurrentChildName       string                   `json:"currentChildName,omitempty"`
	CurrentInstitutionID   int64                    `json:"currentInstitutionId,omitempty"`
	CurrentInstitutionName string                   `json:"currentInstitutionName,omitempty"`
}

func contextSummary(client *aula.Client) *contextSummaryResult {
	summary := &contextSummaryResult{
		Children:     []contextChildInfo{},
		Institutions: []contextInstitutionInfo{},
	}

	if profile := client.CurrentProfile(); profile != nil {
		summary.ProfileID = profile.ProfileID
		summary.DisplayName = profile.DisplayName
		summary.PortalRole = profile.PortalRole
	}

	for _, child := range client.AllChildren() {
		node := contextChildInfo{
			ChildID:         child.ID,
			Name:            child.Name,
			UserID:          child.UserID,
			InstitutionCode: child.InstitutionCode,
		}
		if child.InstitutionProfile != nil {
			node.InstitutionProfileID = child.InstitutionProfile.ID
			node.InstitutionName = child.InstitutionProfile.InstitutionName
		}
		summary.Children = append(summary.Children, node)
	}

	for _, institution := range client.AllInstitutions() {
		summary.Institutions = append(summary.Institutions, contextInstitutionInfo{
			InstitutionProfileID: institution.ID,
			InstitutionName:      institution.InstitutionName,
			InstitutionCode:      institution.InstitutionCode,
		})
	}

	if child := client.CurrentChild(); child != nil {
		summary.CurrentChildID = child.ID
		summary.CurrentChildName = child.Name
	}
	if institution := client.CurrentInstitution(); institution != nil {
		summary.CurrentInstitutionID = institution.ID
		summary.CurrentInstitutionName = institution.InstitutionName
	}
	return summary
}
