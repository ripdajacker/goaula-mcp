package aula

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPagedResult(t *testing.T) {
	total := 42
	p := NewPagedResult([]string{"a", "b"}, 1, 2, &total, true)

	if !p.HasMore || p.NextPage() != 2 || p.IsLastPage() || p.IsFirstPage() {
		t.Errorf("unexpected paged result state: %+v", p)
	}

	last := NewPagedResult([]string{"a"}, 2, 2, &total, false)
	if last.NextPage() != -1 || !last.IsLastPage() {
		t.Errorf("unexpected last page state: %+v", last)
	}

	first := NewPagedResult[string](nil, 0, 2, nil, false)
	if !first.IsFirstPage() || first.TotalSize != nil {
		t.Errorf("unexpected first page state: %+v", first)
	}
}

func TestPostUnmarshal(t *testing.T) {
	body := `{
		"id": 123,
		"title": "Hello",
		"content": {"html": "<p>World</p>"},
		"timestamp": "2024-05-01T10:00:00+02:00",
		"ownerProfile": {"id": 5, "fullName": "Jane Teacher"},
		"attachments": [
			{"id": 1, "name": "photo", "media": {"mediaType": "image", "file": {"url": "https://s3/img.png"}, "thumbnailUrl": "https://s3/thumb.png"}},
			{"id": 2, "name": "doc", "file": {"url": "https://s3/doc.pdf", "name": "doc.pdf"}}
		],
		"unknownField": {"should": "be ignored"}
	}`
	var post Post
	if err := json.Unmarshal([]byte(body), &post); err != nil {
		t.Fatal(err)
	}
	if post.ID != 123 || post.Title != "Hello" || post.Content.HTML != "<p>World</p>" {
		t.Errorf("unexpected post: %+v", post)
	}
	if !post.HasImageAttachments() || !post.HasFileAttachments() {
		t.Error("expected both image and file attachments")
	}
	images := post.ImageAttachments()
	if len(images) != 1 || images[0].FullSizeURL() != "https://s3/img.png" || images[0].ThumbnailURL() != "https://s3/thumb.png" {
		t.Errorf("unexpected image attachments: %+v", images)
	}
	files := post.FileAttachments()
	if len(files) != 1 || files[0].FileURL() != "https://s3/doc.pdf" || files[0].FileName() != "doc.pdf" {
		t.Errorf("unexpected file attachments: %+v", files)
	}
	if got := post.ImageAttachmentURLs(); len(got) != 1 || got[0] != "https://s3/img.png" {
		t.Errorf("unexpected image URLs: %v", got)
	}
}

func TestProfilesUnmarshalAndFind(t *testing.T) {
	body := `{
		"profiles": [{
			"profileId": 10,
			"displayName": "John Doe",
			"portalRole": "guardian",
			"institutionProfiles": [{"id": 100, "institutionName": "Sunny School", "institutionCode": "S1"}],
			"children": [{"id": 1000, "name": "Kid Doe", "userId": "kid1", "institutionProfile": {"id": 200}}]
		}]
	}`
	var profiles Profiles
	if err := json.Unmarshal([]byte(body), &profiles); err != nil {
		t.Fatal(err)
	}
	p := profiles.ProfileByDisplayName("John")
	if p == nil || p.ProfileID != 10 {
		t.Fatalf("profile not found: %+v", p)
	}
	if c := p.ChildByName("Kid"); c == nil || c.ID != 1000 {
		t.Errorf("child not found: %+v", c)
	}
	if i := p.InstitutionByName("Sunny"); i == nil || i.ID != 100 {
		t.Errorf("institution not found: %+v", i)
	}
}

func TestFindMessageRequestJSON(t *testing.T) {
	req := NewFindMessageRequest([]int64{1, 2}, []string{"S1"})
	req.Limit = 50
	req.Offset = 50

	data, err := MarshalJSON(req)
	if err != nil {
		t.Fatal(err)
	}
	js := string(data)

	for _, want := range []string{
		`"activeChildrenInstitutionProfileIds":[1,2]`,
		`"commonInboxId":null`,
		`"exactTerm":true`,
		`"filterBy":"all"`,
		`"hasAttachments":null`,
		`"institutionCodes":["S1"]`,
		`"limit":50`,
		`"offset":50`,
		`"participants":[]`,
		`"sortBy":"date"`,
		`"sortDirection":"desc"`,
		`"threadCreators":[]`,
	} {
		if !strings.Contains(js, want) {
			t.Errorf("request JSON missing %s:\n%s", want, js)
		}
	}
}

func TestParseCalendarEventType(t *testing.T) {
	if got, ok := ParseCalendarEventType("excursion"); !ok || got != EventTypeExcursion {
		t.Errorf("excursion: got %v, %v", got, ok)
	}
	if _, ok := ParseCalendarEventType("bogus"); ok {
		t.Error("bogus should not parse")
	}
	e := &CalendarEvent{Type: "holiday"}
	if e.EventType() != EventTypeHoliday {
		t.Errorf("EventType = %v", e.EventType())
	}
}

func TestEnumParsing(t *testing.T) {
	if ParsePortalRole("guardian") != PortalRoleParent {
		t.Error("guardian should be parent")
	}
	if ParsePortalRole("nope") != "" {
		t.Error("nope should be empty")
	}
	if ParseInstitutionRole("preschool-teacher") != InstitutionRolePreschoolTeacher {
		t.Error("preschool-teacher should parse")
	}
	if ParseRelationType("child") != RelationTypeChild {
		t.Error("child should parse")
	}
}

func TestContactEntryVisibleGuardians(t *testing.T) {
	entry := ContactEntry{
		FullName: "Kid",
		Relations: []ContactRelation{
			{FullName: "Mom", CurrentUserCanViewContactInformation: true, UserHasGivenConsentToShowContactInformation: true},
			{FullName: "Dad", CurrentUserCanViewContactInformation: true},
		},
	}
	visible := entry.VisibleGuardians()
	if len(visible) != 1 || visible[0].FullName != "Mom" {
		t.Errorf("unexpected visible guardians: %+v", visible)
	}
}

func TestWeekPlanTaskLines(t *testing.T) {
	task := WeekPlanTask{Content: "Do a thing\n- \n--\n  \nWe go on a tur tomorrow"}
	lines := task.ContentLines()
	if len(lines) != 2 {
		t.Errorf("content lines = %v", lines)
	}
	if !task.RelatedToATrip() {
		t.Error("expected related to trip")
	}
	if got := (&WeekPlanTask{Pill: "a, b, c"}).PillTags(); len(got) != 3 {
		t.Errorf("pill tags = %v", got)
	}
}

func TestWeekPlanDayFullDate(t *testing.T) {
	day := WeekPlanDay{Date: "tir 15. okt"}
	full := day.FullDate()
	if full.Day() != 15 || full.Month() != 10 {
		t.Errorf("full date = %v", full)
	}
}
