package main

import (
	"errors"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/ripdajacker/aula-api-client-go"
)

// Contacts tools: the groups ("Kontakter" lists) each child belongs to, and the contact
// list of a group with the guardians' emails and phone numbers where Aula exposes them
// (consent granted).

func registerContactTools(server *mcp.Server, session *aulaSession, count func()) {
	// aula_contact_groups
	type contactGroupsArgs struct {
		ChildIDs []int64 `json:"childIds,omitempty"`
	}
	addTool(server, session, "aula_contact_groups", "List contact groups",
		"Lists the groups (classes, SFO, etc.) each child belongs to. The returned group IDs are needed "+
			"by aula_contacts to fetch a class contact list.",
		`{
		  "type": "object",
		  "properties": {
		    "childIds": {
		      "type": "array",
		      "items": { "type": "integer" },
		      "description": "Optional child IDs to filter by. Empty means all children."
		    }
		  },
		  "additionalProperties": false
		}`,
		func(client *aula.Client, args contactGroupsArgs) (any, error) {
			groups, err := client.ContactGroups(args.ChildIDs)
			if err != nil {
				return nil, err
			}
			return struct {
				Count    int                `json:"count"`
				Children []aula.ChildGroups `json:"children"`
			}{len(groups), groups}, nil
		})
	count()

	// aula_contacts
	type contactsArgs struct {
		GroupID *int64 `json:"groupId,omitempty"`
		Filter  string `json:"filter,omitempty"`
		Name    string `json:"name,omitempty"`
	}
	addTool(server, session, "aula_contacts", "Get a group's contact list",
		"Returns the contact list of a group: for each child, its guardians with relation (e.g. Mor/Far) "+
			"and visible email and phone numbers. Only people who consented to show contact information "+
			"are included.",
		`{
		  "type": "object",
		  "properties": {
		    "groupId": {
		      "type": "integer",
		      "description": "Group ID from aula_contact_groups (e.g. a class like 2A)."
		    },
		    "filter": {
		      "type": "string",
		      "enum": ["child", "parent"],
		      "description": "List children with their guardians (child, default) or the guardians directly."
		    },
		    "name": {
		      "type": "string",
		      "description": "Optional name filter (partial, case-insensitive) on the child or any of its relations."
		    }
		  },
		  "additionalProperties": false
		}`,
		func(client *aula.Client, args contactsArgs) (any, error) {
			if args.GroupID == nil || *args.GroupID < 0 {
				return nil, errors.New("groupId is required (from aula_contact_groups)")
			}
			filter := args.Filter
			if filter == "" {
				filter = "child"
			}
			entries, err := client.ContactList(*args.GroupID, filter)
			if err != nil {
				return nil, err
			}
			var matched []contactEntryView
			for i := range entries {
				if contactMatches(args.Name, &entries[i]) {
					matched = append(matched, serializeContact(&entries[i]))
				}
			}
			return struct {
				Count    int                `json:"count"`
				Contacts []contactEntryView `json:"contacts"`
			}{len(matched), matched}, nil
		})
	count()
}

type contactDetailsView struct {
	Email             string `json:"email,omitempty"`
	HomePhoneNumber   string `json:"homePhoneNumber,omitempty"`
	MobilePhoneNumber string `json:"mobilePhoneNumber,omitempty"`
	WorkPhoneNumber   string `json:"workPhoneNumber,omitempty"`
}

type contactGuardianView struct {
	Name              string `json:"name,omitempty"`
	Relation          string `json:"relation,omitempty"`
	Email             string `json:"email,omitempty"`
	HomePhoneNumber   string `json:"homePhoneNumber,omitempty"`
	MobilePhoneNumber string `json:"mobilePhoneNumber,omitempty"`
	WorkPhoneNumber   string `json:"workPhoneNumber,omitempty"`
}

type contactEntryView struct {
	ChildID      int64                 `json:"childId"`
	ChildName    string                `json:"childName,omitempty"`
	Institution  string                `json:"institution,omitempty"`
	Classes      string                `json:"classes,omitempty"`
	ChildContact contactDetailsView    `json:"childContact"`
	Guardians    []contactGuardianView `json:"guardians"`
}

func contactMatches(nameFilter string, entry *aula.ContactEntry) bool {
	if strings.TrimSpace(nameFilter) == "" {
		return true
	}
	needle := strings.ToLower(nameFilter)
	if strings.Contains(strings.ToLower(entry.FullName), needle) {
		return true
	}
	for _, relation := range entry.VisibleGuardians() {
		if strings.Contains(strings.ToLower(relation.FullName), needle) {
			return true
		}
	}
	return false
}

func serializeContact(entry *aula.ContactEntry) contactEntryView {
	node := contactEntryView{
		ChildID:     entry.ProfileID,
		ChildName:   entry.FullName,
		Institution: entry.InstitutionName,
		Classes:     entry.Metadata,
		ChildContact: contactDetailsView{
			Email:             entry.Email,
			HomePhoneNumber:   entry.HomePhoneNumber,
			MobilePhoneNumber: entry.MobilePhoneNumber,
			WorkPhoneNumber:   entry.WorkPhoneNumber,
		},
		Guardians: []contactGuardianView{},
	}
	for _, relation := range entry.VisibleGuardians() {
		node.Guardians = append(node.Guardians, contactGuardianView{
			Name:              relation.FullName,
			Relation:          relation.Relation,
			Email:             relation.Email,
			HomePhoneNumber:   relation.HomePhoneNumber,
			MobilePhoneNumber: relation.MobilePhoneNumber,
			WorkPhoneNumber:   relation.WorkPhoneNumber,
		})
	}
	return node
}
