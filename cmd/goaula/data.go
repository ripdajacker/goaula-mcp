package main

// Data commands: profile/context, people search, posts, messages/threads,
// contacts, calendar and the daily overview. All accept --json; the human
// renderers keep to short, scriptable line formats.

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	aula "github.com/ripdajacker/goaula-mcp"
)

// ── whoami / set-context ────────────────────────────────────────────────

// whoamiResult is the JSON payload of the whoami command.
type whoamiResult struct {
	ProfileName    string           `json:"profileName"`
	CurrentChildID int64            `json:"currentChildId,omitempty"`
	Children       []childSummary   `json:"children"`
	Institutions   []institutionUI  `json:"institutions"`
	Context        persistedContext `json:"persistedContext"`
	APIVersion     int              `json:"apiVersion"`
}

type childSummary struct {
	ID                 int64            `json:"id"`
	Name               string           `json:"name"`
	InstitutionCode    string           `json:"institutionCode,omitempty"`
	UserID             string           `json:"userId,omitempty"`
	InstitutionProfile *instProfileInfo `json:"institutionProfile,omitempty"`
}

type institutionUI struct {
	ID   int64  `json:"institutionProfileId"`
	Name string `json:"name"`
	Code string `json:"institutionCode,omitempty"`
}

type instProfileInfo struct {
	ProfileID   int64  `json:"profileId"`
	Institution string `json:"institution"`
	Code        string `json:"institutionCode"`
}

// cmdWhoami renders the profile, children and institutions with the IDs that
// the set-context and data commands take. --json includes all details.
func cmdWhoami(args []string) int {
	_, jsonOut, _ := flagSet(args, "whoami", nil)

	client, err := sessionClient(context{})
	if err != nil {
		failSession(err)
		return exitFail
	}
	defer client.Close()

	profile := client.CurrentProfile()
	var csums []childSummary
	if profile != nil {
		for _, c := range profile.Children {
			sum := childSummary{
				ID:              c.ID,
				Name:            c.Name,
				InstitutionCode: c.InstitutionCode,
				UserID:          c.UserID,
			}
			if c.InstitutionProfile != nil {
				sum.InstitutionProfile = &instProfileInfo{
					ProfileID:   c.InstitutionProfile.ID,
					Institution: c.InstitutionProfile.InstitutionName,
					Code:        c.InstitutionProfile.InstitutionCode,
				}
			}
			csums = append(csums, sum)
		}
	}
	var insts []institutionUI
	profileName := ""
	if profile != nil {
		profileName = firstNonEmpty(profile.FullName, profile.DisplayName)
		for _, i := range profile.InstitutionProfiles {
			insts = append(insts, institutionUI{ID: i.ID, Name: i.InstitutionName, Code: i.InstitutionCode})
		}
	}
	persisted := loadContext()
	currentChild := int64(0)
	if client.CurrentChild() != nil {
		currentChild = client.CurrentChild().ID
	}
	result := whoamiResult{
		ProfileName:    profileName,
		CurrentChildID: currentChild,
		Children:       csums,
		Institutions:   insts,
		Context:        persisted,
		APIVersion:     client.ActiveAPIVersion(),
	}
	emit(*jsonOut, result, func(b *strings.Builder) {
		fmt.Fprintf(b, "Profile: %s (API v%d)\n", profileName, client.ActiveAPIVersion())
		for _, c := range csums {
			marker := " "
			if c.ID == currentChild {
				marker = "*"
			}
			instID := int64(0)
			if c.InstitutionProfile != nil {
				instID = c.InstitutionProfile.ProfileID
			}
			fmt.Fprintf(b, "%s child %d: %s (%s, institution profile %d)\n", marker, c.ID, c.Name, c.InstitutionCode, instID)
		}
		for _, i := range insts {
			fmt.Fprintf(b, "  institution profile %d: %s (%s)\n", i.ID, i.Name, i.Code)
		}
		fmt.Fprintf(b, "Persisted context: --child %d --institution %d (%s)\n",
			persisted.ChildID, persisted.InstitutionID, contextFile())
		b.WriteString("Tip: goaula set-context --child <child ID> to make that child the default.\n")
	})
	return exitOK
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// cmdSetContext persists the child/institution selection used by data
// commands. Only given values are changed; --clear resets both.
func cmdSetContext(args []string) int {
	var clear bool
	_, jsonOut, ctx := flagSet(args, "set-context", func(fs *flag.FlagSet) {
		fs.BoolVar(&clear, "clear", false, "remove the persisted context (commands fall back to the client defaults)")
	})

	if clear {
		persisted := persistedContext{}
		if err := saveContext(persisted); err != nil {
			fmt.Fprintf(os.Stderr, "goaula: failed to save context: %v\n", err)
			return exitFail
		}
		emit(*jsonOut, persisted, func(b *strings.Builder) {
			fmt.Fprintln(b, "Context cleared; commands now use the client defaults (first child/institution).")
		})
		return exitOK
	}

	persisted := loadContext()
	changed := ctx.ChildID != 0 || ctx.InstitutionID != 0
	if ctx.ChildID != 0 {
		persisted.ChildID = ctx.ChildID
	}
	if ctx.InstitutionID != 0 {
		persisted.InstitutionID = ctx.InstitutionID
	}
	if !changed {
		emit(*jsonOut, persisted, func(b *strings.Builder) {
			fmt.Fprintf(b, "Persisted context unchanged: --child %d --institution %d (%s)\n",
				persisted.ChildID, persisted.InstitutionID, contextFile())
			b.WriteString("Nothing to change: pass --child and/or --institution, or --clear.\n")
		})
		return exitOK
	}

	// Validate the IDs against the profile so a typo does not poison every
	// later command.
	if persisted.ChildID != 0 || persisted.InstitutionID != 0 {
		client, err := sessionClient(persistedContext{})
		if err != nil {
			failSession(err)
			return exitFail
		}
		defer client.Close()
		if persisted.ChildID != 0 && !client.SetMyCurrentChild(persisted.ChildID) {
			fmt.Fprintf(os.Stderr, "goaula: child %d not found — check the IDs with 'goaula whoami'\n", persisted.ChildID)
			return exitFail
		}
		if persisted.InstitutionID != 0 && !client.SetMyCurrentInstitution(persisted.InstitutionID) {
			fmt.Fprintf(os.Stderr, "goaula: institution profile %d not found — check the IDs with 'goaula whoami'\n", persisted.InstitutionID)
			return exitFail
		}
	}
	if err := saveContext(persisted); err != nil {
		fmt.Fprintf(os.Stderr, "goaula: failed to save context: %v\n", err)
		return exitFail
	}
	emit(*jsonOut, persisted, func(b *strings.Builder) {
		fmt.Fprintf(b, "Context persisted to %s: --child %d --institution %d\n",
			contextFile(), persisted.ChildID, persisted.InstitutionID)
		b.WriteString("Data commands now use that selection; override single commands with --child/--institution.\n")
	})
	return exitOK
}

// ── children ────────────────────────────────────────────────────────────

// cmdChildren lists the children of the current profile — the IDs the
// context flags and daily-overview take.
func cmdChildren(args []string) int {
	_, jsonOut, ctx := flagSet(args, "children", nil)

	client, err := sessionClient(ctx)
	if err != nil {
		failSession(err)
		return exitFail
	}
	defer client.Close()

	children := client.AllChildren()
	var currentChildID int64
	if client.CurrentChild() != nil {
		currentChildID = client.CurrentChild().ID
	}
	emit(*jsonOut, children, func(b *strings.Builder) {
		if len(children) == 0 {
			b.WriteString("No children on the profile.\n")
		}
		for i := range children {
			c := &children[i]
			marker := " "
			if c.ID == currentChildID {
				marker = "*"
			}
			instID, instName, instCode := int64(0), "", ""
			if c.InstitutionProfile != nil {
				instID = c.InstitutionProfile.ID
				instName = c.InstitutionProfile.InstitutionName
				instCode = c.InstitutionProfile.InstitutionCode
			}
			fmt.Fprintf(b, "%s child %d: %s (%s, %s, institution profile %d)\n",
				marker, c.ID, c.Name, instName, instCode, instID)
		}
	})
	return exitOK
}

// ── people ──────────────────────────────────────────────────────────────

// cmdPeople searches people by name and optional role.
func cmdPeople(args []string) int {
	var name, role string
	_, jsonOut, _ := flagSet(args, "people", func(fs *flag.FlagSet) {
		fs.StringVar(&name, "name", "", "name (or partial name) to search for")
		fs.StringVar(&role, "role", "any", "role filter: parent, child, teacher, preschool-teacher, leader, employee or any")
	})

	client, err := sessionClient(context{})
	if err != nil {
		failSession(err)
		return exitFail
	}
	defer client.Close()

	var people []aula.Recipient
	switch role {
	case "any", "":
		people, err = client.FindPeople(name)
	case "parent", "parents":
		people, err = client.FindParents(name)
	case "child", "children":
		people, err = client.FindChildren(name)
	case "teacher", "teachers":
		people, err = client.FindTeachers(name)
	case "preschool-teacher", "sfo":
		people, err = client.FindPreschoolTeachers(name)
	case "leader", "leaders":
		people, err = client.FindLeaders(name)
	case "employee", "employees":
		people, err = client.FindEmployees(name)
	default:
		fmt.Fprintf(os.Stderr, "goaula: unknown role %q (use parent, child, teacher, preschool-teacher, leader, employee or any)\n", role)
		return exitUsage
	}
	if err != nil {
		failSession(err)
		return exitFail
	}
	emit(*jsonOut, people, func(b *strings.Builder) {
		for _, p := range people {
			line := fmt.Sprintf("%s [%s", p.Name, displayRole(p))
			if p.InstitutionRole != "" {
				line += "/" + p.InstitutionRole
			}
			line += fmt.Sprintf("] profileId=%s", p.ProfileID)
			if p.AulaEmail != "" {
				line += " " + p.AulaEmail
			}
			fmt.Fprintln(b, line)
		}
		if len(people) == 0 {
			b.WriteString("No matching people.\n")
		}
	})
	return exitOK
}

// displayRole reduces the portal role to a short tag.
func displayRole(r aula.Recipient) string {
	switch r.PortalRole {
	case "guardian":
		return "parent"
	default:
		return r.PortalRole
	}
}

// ── posts ───────────────────────────────────────────────────────────────

// cmdPosts lists recent posts for the current context.
func cmdPosts(args []string) int {
	var days, limit int
	var full bool
	_, jsonOut, ctx := flagSet(args, "posts", func(fs *flag.FlagSet) {
		fs.IntVar(&days, "days", 0, "only posts within the past N days (0 = no filter)")
		fs.IntVar(&limit, "limit", 20, "maximum number of posts")
		fs.BoolVar(&full, "full", false, "human output: show the full HTML content instead of a snippet")
	})

	client, err := sessionClient(ctx)
	if err != nil {
		failSession(err)
		return exitFail
	}
	defer client.Close()

	posts, err := client.RecentPosts(days, limit, nil)
	if err != nil {
		failSession(err)
		return exitFail
	}
	emit(*jsonOut, posts, func(b *strings.Builder) {
		for _, p := range posts {
			owner := ""
			if p.OwnerProfile != nil {
				owner = p.OwnerProfile.FullName
			}
			fmt.Fprintf(b, "%s | %s | by %s\n", tsOrDefault(p.Timestamp, "?"), p.Title, owner)
			text := ""
			if p.Content != nil {
				text = p.Content.HTML
			}
			content := snippet(text, 200)
			if content == "" && p.HasFileAttachments() {
				content = "(attachments only)"
			}
			fmt.Fprintf(b, "  %s\n", content)
			if full {
				fmt.Fprintln(b, "  "+strings.TrimSpace(text))
			}
			if p.CommentCount > 0 {
				fmt.Fprintf(b, "  %d comments\n", p.CommentCount)
			}
		}
		if len(posts) == 0 {
			b.WriteString("No posts in the requested window.\n")
		}
	})
	return exitOK
}

// tsOrDefault renders a timestamp for human output (local, minute precision).
func tsOrDefault(ts, def string) string {
	if t, err := time.Parse(time.RFC3339Nano, ts); err == nil {
		return t.Local().Format("2006-01-02 15:04")
	}
	return def
}

// snippet strips HTML tags and truncates to a prefix (max < = no truncation).
func snippet(html string, max int) string {
	text := strings.Join(strings.Fields(stripTags(html)), " ")
	if max > 0 && len(text) > max {
		text = text[:max] + "…"
	}
	return text
}

// stripTags removes <...> sequences.
func stripTags(html string) string {
	var b strings.Builder
	depth := 0
	for _, r := range html {
		switch {
		case r == '<':
			depth++
		case r == '>':
			if depth > 0 {
				depth--
			}
		case depth == 0:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// ── messages & threads ──────────────────────────────────────────────────

// cmdMessages searches messages for the current context.
func cmdMessages(args []string) int {
	var days, limit, page int
	_, jsonOut, ctx := flagSet(args, "messages", func(fs *flag.FlagSet) {
		fs.IntVar(&days, "days", 3, "how many days back to search")
		fs.IntVar(&limit, "limit", 50, "maximum number of messages")
		fs.IntVar(&page, "page", 0, "0-based page index")
	})

	client, err := sessionClient(ctx)
	if err != nil {
		failSession(err)
		return exitFail
	}
	defer client.Close()

	from := time.Now().AddDate(0, 0, -days)
	result, err := client.FindMessages(&from, nil, nil, nil, page, limit)
	if err != nil {
		failSession(err)
		return exitFail
	}
	emit(*jsonOut, result, func(b *strings.Builder) {
		for _, m := range result.Items {
			subject := m.ThreadSubject
			if subject == "" {
				subject = "(no subject)"
			}
			fmt.Fprintf(b, "%s | %s | %s\n", tsOrDefault(m.SendDateTime, "?"), subject, m.ID)
			fmt.Fprintf(b, "  %s\n", snippet(m.TextHTML(), 200))
		}
		more := ""
		if result.HasMore {
			more = fmt.Sprintf(" (more available, next page: %d)", page+1)
		}
		fmt.Fprintf(b, "%d message(s)%s\n", len(result.Items), more)
	})
	return exitOK
}

// cmdThreads lists threads from the current context.
func cmdThreads(args []string) int {
	var limit int
	_, jsonOut, ctx := flagSet(args, "threads", func(fs *flag.FlagSet) {
		fs.IntVar(&limit, "limit", 25, "maximum number of threads (paged server-side, 20 per page)")
	})

	client, err := sessionClient(ctx)
	if err != nil {
		failSession(err)
		return exitFail
	}
	defer client.Close()

	var threads []aula.Thread
	for page := 0; len(threads) < limit; page++ {
		result, err := client.ListThreads(page, 20, nil)
		if err != nil {
			failSession(err)
			return exitFail
		}
		threads = append(threads, result.Items...)
		if !result.HasMore || len(result.Items) == 0 {
			break
		}
	}
	if len(threads) > limit {
		threads = threads[:limit]
	}
	emit(*jsonOut, threads, func(b *strings.Builder) {
		for _, t := range threads {
			last := "—"
			if t.LatestMessage != nil {
				last = tsOrDefault(t.LatestMessage.SendDateTime, "—")
			}
			fmt.Fprintf(b, "%s | ID %d | %d recipient(s)\n", last, t.ID, len(t.Recipients))
			fmt.Fprintf(b, "  %s\n", t.Subject)
		}
		if len(threads) == 0 {
			b.WriteString("No threads.\n")
		}
	})
	return exitOK
}

// cmdThread renders one full thread (all pages of messages).
func cmdThread(args []string) int {
	var full bool
	fs, jsonOut, _ := flagSet(args, "thread", func(fs *flag.FlagSet) {
		fs.BoolVar(&full, "full", false, "human output: show the full HTML content instead of a snippet")
	})
	requireArgs(fs, 1)
	threadID := fs.Arg(0)

	client, err := sessionClient(context{})
	if err != nil {
		failSession(err)
		return exitFail
	}
	defer client.Close()

	thread, err := client.ThreadAllPages(threadID)
	if err != nil {
		failSession(err)
		return exitFail
	}
	emit(*jsonOut, thread, func(b *strings.Builder) {
		fmt.Fprintf(b, "Thread: %s (ID %d, %d message(s))\n", thread.Subject, thread.ID, len(thread.Messages))
		for _, m := range thread.Messages {
			from := ""
			if m.Sender != nil {
				from = m.Sender.FullName
			}
			fmt.Fprintf(b, "\n%s | %s\n", tsOrDefault(m.SendDateTime, "?"), from)
			text := snippet(m.TextHTML(), 0)
			if !full {
				text = snippet(m.TextHTML(), 400)
			}
			fmt.Fprintln(b, "  "+text)
			if m.HasFileAttachments() {
				fmt.Fprintf(b, "  attachments: %s\n", strings.Join(fileNames(m), ", "))
			}
		}
	})
	return exitOK
}

func fileNames(m aula.Message) []string {
	var names []string
	for _, a := range m.FileAttachments() {
		names = append(names, a.FileName())
	}
	return names
}

// ── contacts ────────────────────────────────────────────────────────────

// cmdContactGroups lists the groups (classes, SFO) each child of the context
// belongs to — the group IDs `contacts` takes.
func cmdContactGroups(args []string) int {
	_, jsonOut, ctx := flagSet(args, "contact-groups", nil)

	client, err := sessionClient(ctx)
	if err != nil {
		failSession(err)
		return exitFail
	}
	defer client.Close()

	var childIDs []int64
	if ctx.ChildID != 0 {
		childIDs = []int64{ctx.ChildID}
	}
	groups, err := client.ContactGroups(childIDs)
	if err != nil {
		failSession(err)
		return exitFail
	}
	emit(*jsonOut, groups, func(b *strings.Builder) {
		for _, cg := range groups {
			fmt.Fprintf(b, "%s (profile %d):\n", cg.DisplayName, cg.ProfileID)
			for _, g := range cg.SortedGroups() {
				marker := " "
				if g.ShowAsDefault {
					marker = "*"
				}
				fmt.Fprintf(b, "%s group %d: %s (%s)\n", marker, g.ID, g.Name, g.InstitutionCode)
			}
		}
		if len(groups) == 0 {
			b.WriteString("No groups found for the current context.\n")
		}
	})
	return exitOK
}

// cmdContacts lists the contact list of a group.
func cmdContacts(args []string) int {
	var filter string
	fs, jsonOut, _ := flagSet(args, "contacts", func(fs *flag.FlagSet) {
		fs.StringVar(&filter, "filter", "child", "filter=child lists children with their guardians, filter=parent lists guardians directly")
	})
	requireArgs(fs, 1)
	groupID, err := strconv.ParseInt(fs.Arg(0), 10, 64)
	if err != nil {
		fmt.Fprintf(os.Stderr, "goaula: invalid group ID %q (get IDs from 'goaula contact-groups')\n", fs.Arg(0))
		return exitUsage
	}

	client, err := sessionClient(context{})
	if err != nil {
		failSession(err)
		return exitFail
	}
	defer client.Close()

	entries, err := client.ContactList(groupID, filter)
	if err != nil {
		failSession(err)
		return exitFail
	}
	emit(*jsonOut, entries, func(b *strings.Builder) {
		for _, e := range entries {
			fmt.Fprintf(b, "%s [%s]", e.FullName, e.Role)
			if e.Email != "" {
				fmt.Fprintf(b, " | %s", e.Email)
			}
			if e.MobilePhoneNumber != "" {
				fmt.Fprintf(b, " | %s", e.MobilePhoneNumber)
			}
			b.WriteString("\n")
			for _, g := range e.VisibleGuardians() {
				fmt.Fprintf(b, "  %s (%s)", g.FullName, g.Relation)
				if g.Email != "" {
					fmt.Fprintf(b, " | %s", g.Email)
				}
				if g.MobilePhoneNumber != "" {
					fmt.Fprintf(b, " | %s", g.MobilePhoneNumber)
				}
				b.WriteString("\n")
			}
		}
	})
	return exitOK
}

// ── calendar ────────────────────────────────────────────────────────────

// cmdCalendar lists calendar events, optionally filtered by type.
func cmdCalendar(args []string) int {
	var past, future int
	var eventType string
	_, jsonOut, ctx := flagSet(args, "calendar", func(fs *flag.FlagSet) {
		fs.IntVar(&past, "past", 3, "days in the past to include")
		fs.IntVar(&future, "future", 7, "days in the future to include")
		fs.StringVar(&eventType, "type", "all", "event filter: all, lesson, lessons, excursions, special")
	})

	client, err := sessionClient(ctx)
	if err != nil {
		failSession(err)
		return exitFail
	}
	defer client.Close()

	var events []aula.CalendarEvent
	switch eventType {
	case "all", "":
		events, err = client.CalendarEvents(past, future, nil, nil)
	case "lesson", "lessons":
		events, err = client.CalendarEventsOnlyLessons(past, future)
	case "excursions", "excursion":
		events, err = client.CalendarEventsOnlyExcursions(past, future)
	case "special":
		events, err = client.CalendarEventsSpecialEvents(past, future)
	default:
		et, ok := aula.ParseCalendarEventType(eventType)
		if !ok || et == "" {
			fmt.Fprintf(os.Stderr, "goaula: unknown event type %q (use all, lesson, excursions or special)\n", eventType)
			return exitUsage
		}
		events, err = client.CalendarEvents(past, future, nil, &et)
	}
	if err != nil {
		failSession(err)
		return exitFail
	}
	emit(*jsonOut, events, func(b *strings.Builder) {
		for _, e := range events {
			fmt.Fprintf(b, "%s → %s | %s [%s]\n",
				tsOrDefault(e.StartDateTime, "?"), tsOrDefault(e.EndDateTime, "—"), e.Title, e.Type)
			detail := ""
			if e.CreatorName != "" {
				detail += "by " + e.CreatorName
			}
			if e.InstitutionName != "" {
				if detail != "" {
					detail += " "
				}
				detail += "at " + e.InstitutionName
			}
			if e.HasAttachments {
				if detail != "" {
					detail += " | "
				}
				detail += "has attachments"
			}
			if detail != "" {
				fmt.Fprintf(b, "  %s\n", detail)
			}
		}
		if len(events) == 0 {
			b.WriteString("No events in the requested window.\n")
		}
	})
	return exitOK
}

// ── daily overview ──────────────────────────────────────────────────────

// cmdDailyOverview prints the daily presence overview of one child. The
// positional argument or --child wins; otherwise the persisted context's
// child, and finally the client's default child.
func cmdDailyOverview(args []string) int {
	fs, jsonOut, ctx := flagSet(args, "daily-overview", nil)
	childID := ctx.ChildID
	if fs.NArg() == 1 {
		id, perr := strconv.ParseInt(fs.Arg(0), 10, 64)
		if perr != nil {
			fmt.Fprintf(os.Stderr, "goaula: invalid child ID %q (IDs via 'goaula whoami')\n", fs.Arg(0))
			return exitUsage
		}
		childID = id
	} else if fs.NArg() > 1 {
		requireArgs(fs, 1)
	}
	if childID == 0 {
		childID = loadContext().ChildID
	}

	client, err := sessionClient(context{ChildID: childID})
	if err != nil {
		failSession(err)
		return exitFail
	}
	defer client.Close()

	if childID == 0 && client.CurrentChild() != nil {
		childID = client.CurrentChild().ID
	}
	if childID == 0 {
		fmt.Fprintln(os.Stderr, `goaula: no child specified: pass "--child N" or a child ID argument (IDs via 'goaula whoami')`)
		return exitUsage
	}

	overviews, err := client.DailyOverview(childID)
	if err != nil {
		failSession(err)
		return exitFail
	}
	emit(*jsonOut, overviews, func(b *strings.Builder) {
		for _, o := range overviews {
			name, code := "", ""
			if o.InstitutionProfile != nil {
				name = o.InstitutionProfile.InstitutionName
				code = o.InstitutionProfile.InstitutionCode
			}
			fmt.Fprintf(b, "%s (%s):\n", firstNonEmpty(name, "—"), code)
			fmt.Fprintf(b, "  presence status: %d\n", o.Status)
			if o.CheckInTime != "" {
				fmt.Fprintf(b, "  checked in %s, checked out %s\n", o.CheckInTime, o.CheckOutTime)
			}
			if o.EntryTime != "" {
				fmt.Fprintf(b, "  entry %s, exit %s (with %s)\n", o.EntryTime, o.ExitTime, o.ExitWith)
			}
			if o.Location != "" {
				fmt.Fprintf(b, "  location: %s\n", o.Location)
			}
		}
		if len(overviews) == 0 {
			b.WriteString("No daily overview entries.\n")
		}
	})
	return exitOK
}
