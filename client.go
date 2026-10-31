package aula

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
)

// aulaDateFormat is the date format Aula expects in POST bodies
const aulaDateFormat = "2006-01-02 15:04:05.000Z07:00"

// Config is the configuration for the Client.
type Config struct {
	// APIURL is the URL of the Aula API, defaults to "https://www.aula.dk/api/".
	APIURL string
	// Session provides the session ID (PHPSESSID).
	Session SessionProvider
}

// Client is the main client for communicating with the Aula API (Aula.dk).
//
// Usage:
//
//	client := aula.NewClient(&aula.Config{Session: aula.NewFileSessionProvider()})
//	if err := client.Login(); err != nil { ... }
//	posts, err := client.RecentPosts(21, 20, nil)
//
// The client is NOT safe for concurrent use; serialize access (e.g. with a mutex)
// when sharing it between goroutines.
type Client struct {
	config Config
	conn   *connector

	loggedIn         bool
	activeAPIVersion int

	allProfiles     *Profiles
	allChildren     []Child
	allInstitutions []InstitutionProfile

	currentProfile     *Profile
	currentChild       *Child
	currentInstitution *InstitutionProfile
}

// NewClient creates a new Aula API client with the given configuration.
func NewClient(config *Config) *Client {
	cfg := *config
	if cfg.APIURL == "" {
		cfg.APIURL = DefaultAPIURL
	}
	if cfg.Session == nil {
		cfg.Session = NewFileSessionProvider()
	}
	return &Client{
		config: cfg,
		conn:   newConnector(cfg.Session),
	}
}

// Login logs the user in and sets up the session for further API calls.
func (c *Client) Login() error {
	if _, err := c.conn.initializeAPIUse(c.config.APIURL); err != nil {
		return err
	}
	c.activeAPIVersion = c.conn.activeAPIVersion
	if err := c.setupInitialAulaData(); err != nil {
		return err
	}
	c.loggedIn = true
	return nil
}

// PingAula does the most lightweight thing to test the API is currently active.
func (c *Client) PingAula() error {
	_, err := c.profilesByLogin()
	return err
}

// LoggedIn reports whether Login has completed successfully.
func (c *Client) LoggedIn() bool { return c.loggedIn }

// ActiveAPIVersion returns the negotiated Aula API version.
func (c *Client) ActiveAPIVersion() int { return c.activeAPIVersion }

// AllProfiles returns all profiles of the logged-in user.
func (c *Client) AllProfiles() *Profiles { return c.allProfiles }

// AllChildren returns all children of the current profile.
func (c *Client) AllChildren() []Child { return c.allChildren }

// AllInstitutions returns all institution profiles of the current profile.
func (c *Client) AllInstitutions() []InstitutionProfile { return c.allInstitutions }

// CurrentProfile returns the currently selected profile.
func (c *Client) CurrentProfile() *Profile { return c.currentProfile }

// CurrentChild returns the currently selected child.
func (c *Client) CurrentChild() *Child { return c.currentChild }

// CurrentInstitution returns the currently selected institution.
func (c *Client) CurrentInstitution() *InstitutionProfile { return c.currentInstitution }

// MyProfile gets the profile using the user's display name (partial match). Returns nil
// when no profile matches.
func (c *Client) MyProfile(displayNamePart string) *Profile {
	if c.allProfiles == nil {
		return nil
	}
	return c.allProfiles.ProfileByDisplayName(displayNamePart)
}

// SetMyCurrentProfile sets the current profile by ID. Returns false when no profile
// with the ID exists.
func (c *Client) SetMyCurrentProfile(profileID int64) bool {
	if c.allProfiles == nil {
		return false
	}
	for i := range c.allProfiles.Profiles {
		if c.allProfiles.Profiles[i].ProfileID == profileID {
			c.currentProfile = &c.allProfiles.Profiles[i]
			return true
		}
	}
	return false
}

// MyChild gets a specific child by partial name. Returns nil when no child matches.
func (c *Client) MyChild(childNamePart string) *Child {
	if c.currentProfile == nil {
		return nil
	}
	return c.currentProfile.ChildByName(childNamePart)
}

// MyChildren returns all children of the current profile.
func (c *Client) MyChildren() []Child {
	if c.currentProfile == nil {
		return nil
	}
	return c.currentProfile.Children
}

// SetMyCurrentChild sets the current child by ID. Returns false when no child with the
// ID exists.
func (c *Client) SetMyCurrentChild(childID int64) bool {
	for i := range c.allChildren {
		if c.allChildren[i].ID == childID {
			c.currentChild = &c.allChildren[i]
			return true
		}
	}
	return false
}

// MyInstitution gets the institution by partial name ("" matches the first).
func (c *Client) MyInstitution(institutionNamePart string) *InstitutionProfile {
	if c.currentProfile == nil {
		return nil
	}
	return c.currentProfile.InstitutionByName(institutionNamePart)
}

// SetMyCurrentInstitution sets the current institution by ID. Returns false when no
// institution with the ID exists.
func (c *Client) SetMyCurrentInstitution(institutionID int64) bool {
	for i := range c.allInstitutions {
		if c.allInstitutions[i].ID == institutionID {
			c.currentInstitution = &c.allInstitutions[i]
			return true
		}
	}
	return false
}

// DailyOverview gets the daily overview for a child.
func (c *Client) DailyOverview(childID int64) ([]DailyOverview, error) {
	method := "presence.getDailyOverview&childIds[]=" + strconv.FormatInt(childID, 10)
	data, err := c.callJSONAPI(method, "get", nil)
	if err != nil {
		return nil, err
	}
	var overviews []DailyOverview
	if err := unmarshalJSON(data, &overviews); err != nil {
		return nil, fmt.Errorf("failed to parse daily overview: %w", err)
	}
	return overviews, nil
}

// ==================== People search ====================

// FindPeople finds people by name. Useful to get IDs, emails, names, etc.
func (c *Client) FindPeople(namePart string) ([]Recipient, error) {
	return c.findMailboxRecipients(namePart)
}

// FindParents finds parents by name.
func (c *Client) FindParents(namePart string) ([]Recipient, error) {
	return c.filterRecipients(namePart, func(r *Recipient) bool {
		return r.PortalRole == string(PortalRoleParent)
	})
}

// FindChildren finds children by name.
func (c *Client) FindChildren(namePart string) ([]Recipient, error) {
	return c.filterRecipients(namePart, func(r *Recipient) bool {
		return r.PortalRole == string(PortalRoleChild)
	})
}

// FindTeachers finds teachers by name.
func (c *Client) FindTeachers(namePart string) ([]Recipient, error) {
	return c.filterRecipients(namePart, func(r *Recipient) bool {
		return r.PortalRole == string(PortalRoleEmployee) && r.InstitutionRole == string(InstitutionRoleTeacher)
	})
}

// FindPreschoolTeachers finds preschool teachers (SFO) by name.
func (c *Client) FindPreschoolTeachers(namePart string) ([]Recipient, error) {
	return c.filterRecipients(namePart, func(r *Recipient) bool {
		return r.PortalRole == string(PortalRoleEmployee) && r.InstitutionRole == string(InstitutionRolePreschoolTeacher)
	})
}

// FindLeaders finds leaders (SFO leader, principal, etc.) by name.
func (c *Client) FindLeaders(namePart string) ([]Recipient, error) {
	return c.filterRecipients(namePart, func(r *Recipient) bool {
		return r.PortalRole == string(PortalRoleEmployee) && r.InstitutionRole == string(InstitutionRoleLeader)
	})
}

// FindEmployees finds employees by name.
func (c *Client) FindEmployees(namePart string) ([]Recipient, error) {
	return c.filterRecipients(namePart, func(r *Recipient) bool {
		return r.PortalRole == string(PortalRoleEmployee)
	})
}

func (c *Client) filterRecipients(namePart string, keep func(*Recipient) bool) ([]Recipient, error) {
	recipients, err := c.FindPeople(namePart)
	if err != nil {
		return nil, err
	}
	var result []Recipient
	for i := range recipients {
		if keep(&recipients[i]) {
			result = append(result, recipients[i])
		}
	}
	return result, nil
}

// ==================== Contacts (Kontakter) ====================

// ContactGroups lists the groups (classes, SFO, etc.) each of the user's children
// belongs to. These group IDs are what the contacts list (profiles.getContactlist)
// needs to look up class contact lists with visible phone numbers and emails. Empty
// childIDs means all children.
func (c *Client) ContactGroups(childIDs []int64) ([]ChildGroups, error) {
	institutionProfileIDs := c.institutionProfileIDsForChildren(childIDs)
	var method strings.Builder
	method.WriteString("groups.getGroupsByContext")
	for _, id := range institutionProfileIDs {
		method.WriteString("&childInstitutionProfileIds[]=")
		method.WriteString(strconv.FormatInt(id, 10))
	}
	data, err := c.callJSONAPI(method.String(), "get", nil)
	if err != nil {
		return nil, err
	}
	var groups []ChildGroups
	if err := unmarshalJSON(data, &groups); err != nil {
		return nil, fmt.Errorf("failed to parse groups by context: %w", err)
	}
	return groups, nil
}

// ContactList gets the contact list of a group: the children of the group and their
// relations (guardians), including email and phone numbers where the person has
// consented to show contact information. filter is e.g. "child" to list the children
// with their relations, or "parent" to list the guardians directly ("" means "child").
//
// The list is paginated (20 entries per page, 1-based); this method transparently
// fetches all pages.
func (c *Client) ContactList(groupID int64, filter string) ([]ContactEntry, error) {
	if filter == "" {
		filter = "child"
	}
	var all []ContactEntry
	const maxPages = 100
	for page := 1; page <= maxPages; page++ {
		method := "profiles.getContactlist&groupId=" + strconv.FormatInt(groupID, 10) +
			"&filter=" + filter + "&field=name&page=" + strconv.Itoa(page) + "&order=asc"
		data, err := c.callJSONAPI(method, "get", nil)
		if err != nil {
			return nil, err
		}
		var entries []ContactEntry
		if err := unmarshalJSON(data, &entries); err != nil {
			return nil, fmt.Errorf("failed to parse contact list (page %d): %w", page, err)
		}
		all = append(all, entries...)
		if len(entries) < 20 {
			return all, nil
		}
	}
	slog.Warn("Contact list exceeded max pages, truncating", "groupId", groupID, "maxPages", maxPages)
	return all, nil
}

// ==================== Messages & Threads ====================

// FindMessages finds messages within a date range, with optional child filtering and
// pagination.
//
//	fromDate:       start date (nil = 3 days ago)
//	toDate:         end date (nil = now)
//	participantIDs: participant IDs to filter by (nil for all)
//	childIDs:       child IDs to filter by (nil or empty for all children)
//	page:           page index (0-based)
//	pageSize:       number of results per page
func (c *Client) FindMessages(fromDate, toDate *time.Time, participantIDs, childIDs []int64, page, pageSize int) (*PagedResult[FoundMessage], error) {
	from := time.Now().AddDate(0, 0, -3)
	if fromDate != nil {
		from = *fromDate
	}
	to := time.Now()
	if toDate != nil {
		to = *toDate
	}

	// Determine which children to use for filtering
	childrenToUse := c.filterChildren(childIDs)

	var institutionProfileIDs []int64
	var institutionCodes []string
	for i := range childrenToUse {
		if childrenToUse[i].InstitutionProfile != nil {
			institutionProfileIDs = append(institutionProfileIDs, childrenToUse[i].InstitutionProfile.ID)
		}
		institutionCodes = append(institutionCodes, childrenToUse[i].InstitutionCode)
	}

	request := NewFindMessageRequest(institutionProfileIDs, institutionCodes)
	request.SetDateRange(from, to)
	request.Limit = pageSize
	request.Offset = page * pageSize

	for _, id := range participantIDs {
		request.Participants = append(request.Participants, MailboxParticipant{
			ID:               id,
			MailBoxOwnerType: "institutionProfile",
		})
	}

	data, err := c.callJSONAPI("search.findMessage", "post", request)
	if err != nil {
		return nil, err
	}

	var parsed struct {
		Results   []FindMessageResult `json:"results"`
		TotalSize *int                `json:"totalSize"`
	}
	if err := unmarshalJSON(data, &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse find messages result: %w", err)
	}

	// Transpose threadId and threadSubject to the search message
	messages := make([]FoundMessage, 0, len(parsed.Results))
	for i := range parsed.Results {
		result := &parsed.Results[i]
		if result.Thread != nil && result.SearchMessage != nil {
			result.SearchMessage.ThreadID = result.Thread.ID
			result.SearchMessage.ThreadSubject = result.Thread.Subject
		}
		if result.SearchMessage != nil {
			messages = append(messages, *result.SearchMessage)
		}
	}

	hasMore := false
	if parsed.TotalSize != nil {
		hasMore = (page+1)*pageSize < *parsed.TotalSize
	} else {
		hasMore = len(messages) == pageSize
	}

	return NewPagedResult(messages, page, pageSize, parsed.TotalSize, hasMore), nil
}

// SetDateRange sets the from/to dates on the request (as ISO instants).
func (r *FindMessageRequest) SetDateRange(fromDate, toDate time.Time) {
	r.FromDate = formatInstant(fromDate)
	r.ToDate = formatInstant(toDate)
}

// AllMessages gets all messages within the past N days, automatically fetching all
// pages. pageSize defaults to 20 when <= 0.
func (c *Client) AllMessages(retrievePastDays int, childIDs []int64, pageSize int) ([]FoundMessage, error) {
	toDate := time.Now()
	fromDate := toDate.AddDate(0, 0, -retrievePastDays)

	var allMessages []FoundMessage
	for page := 0; ; page++ {
		result, err := c.FindMessages(&fromDate, &toDate, nil, childIDs, page, pageSize)
		if err != nil {
			return nil, err
		}
		allMessages = append(allMessages, result.Items...)
		if !result.HasMore {
			break
		}
	}
	return allMessages, nil
}

// Messages gets messages within the past N days (first page only), optionally filtered
// by children.
func (c *Client) Messages(retrievePastDays int, childIDs []int64, resultsLimit int) ([]FoundMessage, error) {
	toDate := time.Now()
	fromDate := toDate.AddDate(0, 0, -retrievePastDays)
	result, err := c.FindMessages(&fromDate, &toDate, nil, childIDs, 0, resultsLimit)
	if err != nil {
		return nil, err
	}
	return result.Items, nil
}

// Threads gets all threads updated within the past N days, optionally filtered by
// children. Checks for messages within the last N days, and retrieves the full thread
// containing each message (including all pages of messages).
func (c *Client) Threads(retrievePastDays int, childIDs []int64) ([]MessagesForThread, error) {
	messages, err := c.AllMessages(retrievePastDays, childIDs, 50)
	if err != nil {
		return nil, err
	}

	var threadIDs []string
	seen := make(map[string]bool)
	for _, m := range messages {
		if !seen[m.ThreadID] {
			seen[m.ThreadID] = true
			threadIDs = append(threadIDs, m.ThreadID)
		}
	}

	threads := make([]MessagesForThread, 0, len(threadIDs))
	for _, threadID := range threadIDs {
		thread, err := c.ThreadAllPages(threadID)
		if err != nil {
			return nil, err
		}
		threads = append(threads, *thread)
	}
	return threads, nil
}

// Thread gets a specific thread by ID (first page of messages only).
func (c *Client) Thread(threadID string) (*MessagesForThread, error) {
	return c.ThreadPage(threadID, 0)
}

// ThreadPage gets a specific page of messages for a thread.
func (c *Client) ThreadPage(threadID string, page int) (*MessagesForThread, error) {
	method := "messaging.getMessagesForThread&threadId=" + threadID + "&page=" + strconv.Itoa(page)
	data, err := c.callJSONAPI(method, "get", nil)
	if err != nil {
		return nil, err
	}
	var thread MessagesForThread
	if err := unmarshalJSON(data, &thread); err != nil {
		return nil, fmt.Errorf("failed to parse thread: %w", err)
	}

	// Add thread context to each message
	for i := range thread.Messages {
		thread.Messages[i].ThreadID = thread.ID
		thread.Messages[i].ThreadSubject = thread.Subject
		thread.Messages[i].ThreadCreator = thread.ThreadCreator
		thread.Messages[i].ThreadStartedDateTime = thread.ThreadStartedDateTime
	}

	return &thread, nil
}

// ThreadAllPages gets a specific thread by ID, fetching all pages of messages.
func (c *Client) ThreadAllPages(threadID string) (*MessagesForThread, error) {
	firstPage, err := c.ThreadPage(threadID, 0)
	if err != nil {
		return nil, err
	}

	if !firstPage.MoreMessagesExist || firstPage.Messages == nil {
		return firstPage, nil
	}

	// Fetch remaining pages
	allMessages := append([]Message{}, firstPage.Messages...)
	for page := 1; ; page++ {
		nextPage, err := c.ThreadPage(threadID, page)
		if err != nil {
			return nil, err
		}
		if len(nextPage.Messages) == 0 {
			break
		}
		allMessages = append(allMessages, nextPage.Messages...)
		if !nextPage.MoreMessagesExist {
			break
		}
	}

	firstPage.Messages = allMessages
	firstPage.MoreMessagesExist = false
	return firstPage, nil
}

// ListThreads lists threads directly using the messaging.getThreads endpoint, with
// page-based pagination. Threads are scoped to the current profile/child context. To
// filter by a specific child, call SetMyCurrentChild first.
//
// Note: This endpoint uses "page" (not "index") for pagination, and does NOT accept
// institutionProfileIds[] — childIDs is ignored for this endpoint.
func (c *Client) ListThreads(page, pageSize int, childIDs []int64) (*PagedResult[Thread], error) {
	method := "messaging.getThreads&page=" + strconv.Itoa(page) + "&limit=" + strconv.Itoa(pageSize) +
		"&sortOn=date&orderDirection=desc"
	slog.Debug("Fetching threads", "page", page, "pageSize", pageSize)
	data, err := c.callJSONAPI(method, "get", nil)
	if err != nil {
		return nil, err
	}
	var wrapper ThreadsResponse
	if err := unmarshalJSON(data, &wrapper); err != nil {
		return nil, fmt.Errorf("failed to parse threads: %w", err)
	}

	threads := wrapper.Threads
	if threads == nil {
		threads = []Thread{}
	}
	return NewPagedResult(threads, page, pageSize, nil, wrapper.MoreMessagesExist), nil
}

// AllThreads lists all threads, automatically fetching all pages.
func (c *Client) AllThreads(childIDs []int64, pageSize int) ([]Thread, error) {
	var allThreads []Thread
	for page := 0; ; page++ {
		result, err := c.ListThreads(page, pageSize, childIDs)
		if err != nil {
			return nil, err
		}
		allThreads = append(allThreads, result.Items...)
		if !result.HasMore {
			break
		}
	}
	return allThreads, nil
}

// ==================== Posts ====================

// Posts gets a page of posts using offset-based pagination (matches the Aula UI
// behavior). Posts are returned newest-first. To get the next page, increase the offset
// by the limit (e.g. offset=0, limit=10, then offset=10, limit=10, etc.).
//
// Example API call: posts.getAllPosts&parent=profile&index=10&institutionProfileIds[]=...&limit=10
func (c *Client) Posts(offset, limit int, childIDs []int64) (*PagedResult[Post], error) {
	institutionProfileIDs := c.institutionProfileIDsForChildren(childIDs)
	parts := make([]string, 0, len(institutionProfileIDs))
	for _, id := range institutionProfileIDs {
		parts = append(parts, "institutionProfileIds[]="+strconv.FormatInt(id, 10))
	}
	institutionQueryString := strings.Join(parts, "&")

	method := "posts.getAllPosts&parent=profile&index=" + strconv.Itoa(offset) + "&" + institutionQueryString +
		"&limit=" + strconv.Itoa(limit)
	slog.Debug("Fetching posts", "offset", offset, "limit", limit, "institutions", institutionProfileIDs)
	data, err := c.callJSONAPI(method, "get", nil)
	if err != nil {
		return nil, err
	}

	var postsWrapper PostsResponse
	if err := unmarshalJSON(data, &postsWrapper); err != nil {
		return nil, fmt.Errorf("failed to parse posts: %w", err)
	}

	posts := postsWrapper.Posts
	if posts == nil {
		posts = []Post{}
	}
	slog.Debug("API returned posts", "count", len(posts), "hasMorePosts", postsWrapper.HasMorePosts)

	// hasMore: if we got a full page, there might be more; if we got less, we're at the end
	hasMore := len(posts) == limit && limit > 0

	page := 0
	if limit > 0 {
		page = offset / limit
	}
	return NewPagedResult(posts, page, limit, nil, hasMore), nil
}

// RecentPosts gets the latest N posts (from offset 0), optionally filtered by date and
// child. retrievePastDays filters results client-side; use 0 or negative for no
// filtering.
func (c *Client) RecentPosts(retrievePastDays, postLimit int, childIDs []int64) ([]Post, error) {
	result, err := c.Posts(0, postLimit, childIDs)
	if err != nil {
		return nil, err
	}
	if retrievePastDays <= 0 {
		return result.Items, nil
	}
	return filterPostsByDate(result.Items, retrievePastDays), nil
}

// AllPosts gets all posts within the past N days by paginating backwards (increasing
// offset) until posts are older than the date range. Since the API returns posts
// newest-first, it stops as soon as it encounters a post that's too old.
func (c *Client) AllPosts(retrievePastDays int, childIDs []int64, pageSize int) ([]Post, error) {
	var allPosts []Post
	offset := 0
	cutoff := time.Now().AddDate(0, 0, -retrievePastDays)

	slog.Info("Fetching all posts", "days", retrievePastDays, "pageSize", pageSize)

	for {
		page, err := c.Posts(offset, pageSize, childIDs)
		if err != nil {
			return nil, err
		}
		posts := page.Items

		if len(posts) == 0 {
			slog.Debug("Empty page, stopping", "offset", offset)
			break
		}

		// Check each post - since posts are newest-first, once we hit one that's too old,
		// we can stop
		foundOldPost := false
		for i := range posts {
			postTime, err := parsePostTimestamp(posts[i].Timestamp)
			if err == nil && postTime.Before(cutoff) {
				// This post and all subsequent ones are too old
				foundOldPost = true
				slog.Debug("Found post older than cutoff, stopping pagination", "postTime", postTime, "cutoff", cutoff)
				break
			}
			allPosts = append(allPosts, posts[i])
		}

		slog.Debug("Fetched page", "offset", offset, "posts", len(posts), "total", len(allPosts))

		// Stop if we found a post that's too old, or if there are no more pages
		if foundOldPost || !page.HasMore {
			break
		}

		offset += pageSize

		// Safety limit
		if offset > 10000 {
			slog.Warn("Reached maximum offset (10000) for posts, stopping pagination")
			break
		}
	}

	slog.Info("Fetched posts", "count", len(allPosts), "days", retrievePastDays)
	return allPosts, nil
}

// filterPostsByDate filters a list of posts by date range (client-side). Posts newer
// than retrievePastDays days ago are kept.
func filterPostsByDate(posts []Post, retrievePastDays int) []Post {
	cutoff := time.Now().AddDate(0, 0, -retrievePastDays)
	var result []Post
	for i := range posts {
		postTime, err := parsePostTimestamp(posts[i].Timestamp)
		if err != nil {
			result = append(result, posts[i]) // Keep if we can't parse
			continue
		}
		if !postTime.Before(cutoff) {
			result = append(result, posts[i])
		}
	}
	return result
}

// parsePostTimestamp parses a post timestamp string.
func parsePostTimestamp(timestamp string) (time.Time, error) {
	return time.Parse(time.RFC3339Nano, timestamp)
}

// ==================== Gallery ====================

// GalleryAlbumMedia gets gallery album media for the current profile, with optional
// child filtering.
//
//	albumLimit:       max number of albums (default 12)
//	mediaLimit:       max media per album (default 30)
//	mostRecentAlbums: if set, get the N most recent albums
//	retrievePastDays: if set, get albums from the past N days
//	childIDs:         child IDs to filter by (nil or empty for all children)
func (c *Client) GalleryAlbumMedia(albumLimit, mediaLimit int, mostRecentAlbums, retrievePastDays *int, childIDs []int64) ([]GalleryAlbum, error) {
	institutionProfileIDs := c.institutionProfileIDsForChildren(childIDs)
	parts := make([]string, 0, len(institutionProfileIDs))
	for _, id := range institutionProfileIDs {
		parts = append(parts, "institutionProfileIds[]="+strconv.FormatInt(id, 10))
	}
	institutionQueryString := strings.Join(parts, "&")

	method := "gallery.getAlbums&index=0&limit=" + strconv.Itoa(albumLimit) +
		"&sortOn=mediaCreatedAt&orderDirection=desc&filterBy=all&" + institutionQueryString
	data, err := c.callJSONAPI(method, "get", nil)
	if err != nil {
		return nil, err
	}

	var albums []GalleryAlbum
	if err := unmarshalJSON(data, &albums); err != nil {
		return nil, fmt.Errorf("failed to parse gallery albums: %w", err)
	}

	// Set IsDefaultMyChildAlbum flag
	for i := range albums {
		if albums[i].ID == 0 || albums[i].ID == -1 {
			albums[i].IsDefaultMyChildAlbum = true
		}
	}

	// Sort by creation date descending
	slices.SortStableFunc(albums, func(a, b GalleryAlbum) int {
		if a.CreationDate == "" || b.CreationDate == "" {
			return 0
		}
		return strings.Compare(b.CreationDate, a.CreationDate)
	})

	var returnAlbums []GalleryAlbum

	if retrievePastDays != nil {
		end := time.Now()
		start := end.AddDate(0, 0, -*retrievePastDays)

		for _, album := range albums {
			if album.CreationDate == "" {
				continue
			}
			albumDate, err := time.Parse(time.RFC3339Nano, album.CreationDate)
			if err != nil {
				returnAlbums = append(returnAlbums, album)
				continue
			}
			if !albumDate.Before(start) && !albumDate.After(end) {
				returnAlbums = append(returnAlbums, album)
			}
		}
	} else {
		// Not the my child album
		var nonDefaultAlbums []GalleryAlbum
		for _, album := range albums {
			if !album.IsDefaultMyChildAlbum {
				nonDefaultAlbums = append(nonDefaultAlbums, album)
			}
		}
		// Get the most recent
		if mostRecentAlbums != nil && len(nonDefaultAlbums) > *mostRecentAlbums {
			nonDefaultAlbums = nonDefaultAlbums[:*mostRecentAlbums]
		}
		returnAlbums = append(returnAlbums, nonDefaultAlbums...)
		// Add the default my child album back
		for _, album := range albums {
			if album.IsDefaultMyChildAlbum {
				returnAlbums = append(returnAlbums, album)
			}
		}
	}

	// Fetch media for each album
	for i := range returnAlbums {
		var albumScopeSignifier string
		if returnAlbums[i].IsDefaultMyChildAlbum {
			albumScopeSignifier = "userSpecificAlbum=true"
		} else {
			albumScopeSignifier = "albumId=" + strconv.FormatInt(returnAlbums[i].ID, 10)
		}

		mediaMethod := "gallery.getMedia&" + albumScopeSignifier + "&index=0&limit=" + strconv.Itoa(mediaLimit) +
			"&sortOn=uploadedAt&orderDirection=desc&filterBy=all&" + institutionQueryString
		mediaData, err := c.callJSONAPI(mediaMethod, "get", nil)
		if err != nil {
			return nil, err
		}

		var mediaWrapper AlbumMediaResponse
		if err := unmarshalJSON(mediaData, &mediaWrapper); err != nil {
			slog.Warn("Failed to parse album media", "albumId", returnAlbums[i].ID, "error", err)
			continue
		}
		if mediaWrapper.Results != nil {
			returnAlbums[i].Media = mediaWrapper.Results
		}
	}

	return returnAlbums, nil
}

// ==================== MeeBook ====================

// MeeBookInformation gets MeeBook weekly book and plan information.
func (c *Client) MeeBookInformation() (*MeeBookConsolidatedInformation, error) {
	if c.currentChild == nil || c.currentInstitution == nil {
		return nil, NewAPIError("No current child or institution selected")
	}
	childFilters := []string{c.currentChild.UserID}
	institutionFilters := []string{c.currentInstitution.InstitutionCode}

	weeklyBookToken, err := c.meebookAulaAuthorizationToken(WeeklyBookWidgetID)
	if err != nil {
		return nil, err
	}
	weeklyPlanToken, err := c.meebookAulaAuthorizationToken(WeeklyPlanWidgetID)
	if err != nil {
		return nil, err
	}

	meeBookClient := NewMeeBookClient(childFilters, institutionFilters, weeklyBookToken, weeklyPlanToken)
	defer meeBookClient.Close()

	return &MeeBookConsolidatedInformation{
		WorkPlan: meeBookClient.WorkPlanForWeeks(0),
		BookList: meeBookClient.BookListForWeeks(0),
	}, nil
}

func (c *Client) meebookAulaAuthorizationToken(widgetID string) (string, error) {
	data, err := c.callJSONAPI("aulaToken.getAulaToken&widgetId="+widgetID, "get", nil)
	if err != nil {
		return "", err
	}
	return jsonString(data)
}

// ==================== Calendar Events ====================

// CalendarEventsOnlyExcursions gets calendar events for the current profile and current
// child, only excursions.
func (c *Client) CalendarEventsOnlyExcursions(daysInPast, daysInFuture int) ([]CalendarEvent, error) {
	excursion := EventTypeExcursion
	return c.CalendarEvents(daysInPast, daysInFuture, nil, &excursion)
}

// CalendarEventsSpecialEvents gets calendar events: special events (holidays,
// birthdays, excursions, etc.).
func (c *Client) CalendarEventsSpecialEvents(daysInPast, daysInFuture int) ([]CalendarEvent, error) {
	events, err := c.CalendarEvents(daysInPast, daysInFuture, nil, nil)
	if err != nil {
		return nil, err
	}
	var result []CalendarEvent
	for _, e := range events {
		switch e.EventType() {
		case EventTypeHoliday, EventTypeBirthday, EventTypeExcursion, EventTypePresenceHoliday, EventTypeOther:
			result = append(result, e)
		}
	}
	return result, nil
}

// CalendarEventsOnlyLessons gets calendar events, only lessons.
func (c *Client) CalendarEventsOnlyLessons(daysInPast, daysInFuture int) ([]CalendarEvent, error) {
	lesson := EventTypeLesson
	return c.CalendarEvents(daysInPast, daysInFuture, nil, &lesson)
}

// CalendarEventsExceptLessons gets calendar events, except lessons.
func (c *Client) CalendarEventsExceptLessons(daysInPast, daysInFuture int) ([]CalendarEvent, error) {
	events, err := c.CalendarEvents(daysInPast, daysInFuture, nil, nil)
	if err != nil {
		return nil, err
	}
	var result []CalendarEvent
	for _, e := range events {
		if e.EventType() != EventTypeLesson {
			result = append(result, e)
		}
	}
	return result, nil
}

// CalendarEvents gets calendar events with optional child and event type filtering.
//
//	daysInPast:   number of days in the past (default 3)
//	daysInFuture: number of days in the future (default 7)
//	childIDs:     child IDs to filter by (nil or empty for all children/current context)
//	eventType:    optional event type filter
func (c *Client) CalendarEvents(daysInPast, daysInFuture int, childIDs []int64, eventType *CalendarEventType) ([]CalendarEvent, error) {
	now := time.Now()
	end := now.AddDate(0, 0, daysInFuture)
	start := now.AddDate(0, 0, -daysInPast)

	institutionProfileIDs := c.institutionProfileIDsForChildren(childIDs)

	postData := map[string]any{
		"end":            end.Format(aulaDateFormat),
		"start":          start.Format(aulaDateFormat),
		"instProfileIds": institutionProfileIDs,
		"resourceIds":    []int64{},
	}

	data, err := c.callJSONAPI("calendar.getEventsByProfileIdsAndResourceIds", "post", postData)
	if err != nil {
		return nil, err
	}

	var events []CalendarEvent
	if err := unmarshalJSON(data, &events); err != nil {
		return nil, fmt.Errorf("failed to parse calendar events: %w", err)
	}

	if eventType != nil {
		var filtered []CalendarEvent
		for _, e := range events {
			if e.EventType() == *eventType {
				filtered = append(filtered, e)
			}
		}
		events = filtered
	}

	return events, nil
}

// ==================== Private helpers ====================

// filterChildren filters the children list based on the provided child IDs. If childIDs
// is nil or empty, returns all children.
func (c *Client) filterChildren(childIDs []int64) []Child {
	if len(childIDs) == 0 {
		return c.allChildren
	}
	var result []Child
	for i := range c.allChildren {
		if slices.Contains(childIDs, c.allChildren[i].ID) {
			result = append(result, c.allChildren[i])
		}
	}
	return result
}

// institutionProfileIDsForChildren gets institution profile IDs for the specified
// children, or all contextual IDs if childIDs is nil/empty. Always includes the current
// profile's (guardian's) institution profiles, plus the specified children's
// institution profiles.
func (c *Client) institutionProfileIDsForChildren(childIDs []int64) []int64 {
	ids := []int64{}

	// Always include the guardian's (current profile's) institution profiles
	if c.currentProfile != nil {
		for _, i := range c.currentProfile.InstitutionProfiles {
			if i.ID != 0 && !slices.Contains(ids, i.ID) {
				ids = append(ids, i.ID)
			}
		}
	}

	// Add institution profiles for the specified children (or all children if nil/empty)
	for _, child := range c.filterChildren(childIDs) {
		if child.InstitutionProfile != nil && child.InstitutionProfile.ID != 0 &&
			!slices.Contains(ids, child.InstitutionProfile.ID) {
			ids = append(ids, child.InstitutionProfile.ID)
		}
	}

	slog.Debug("Institution profile IDs for children", "childIds", childIDs, "ids", ids)
	return ids
}

// callJSONAPI calls an Aula API method and returns the raw "data" element of the
// response envelope.
func (c *Client) callJSONAPI(aulaAPIMethod, httpMethod string, postData any) (json.RawMessage, error) {
	body, err := c.conn.callAulaAPI(aulaAPIMethod, httpMethod, postData)
	if err != nil {
		return nil, err
	}

	var envelope apiResponse
	if err := unmarshalJSON(body, &envelope); err != nil {
		return nil, NewAPIError("Failed to parse API response: " + err.Error())
	}

	if envelope.Version != c.conn.activeAPIVersion {
		slog.Warn("API version mismatch. This may cause issues or signify the API has changed.",
			"expected", c.conn.activeAPIVersion, "got", envelope.Version)
	}

	return envelope.Data, nil
}

func (c *Client) setupInitialAulaData() error {
	root, err := c.profilesByLogin()
	if err != nil {
		return err
	}
	c.allProfiles = root

	// Find the first guardian profile (keeping a pointer into allProfiles.Profiles)
	currentIdx := -1
	for i := range root.Profiles {
		if root.Profiles[i].PortalRole == "guardian" {
			currentIdx = i
			break
		}
	}
	if currentIdx < 0 && len(root.Profiles) > 0 {
		slog.Warn("No guardian profiles, just using the first profile")
		currentIdx = 0
	}
	if currentIdx >= 0 {
		c.currentProfile = &root.Profiles[currentIdx]
	}

	if c.currentProfile != nil {
		c.allChildren = c.currentProfile.Children
		if len(c.currentProfile.Children) > 0 {
			c.currentChild = &c.currentProfile.Children[0]
		}
		if c.currentChild == nil {
			slog.Warn("No children in the profile.")
		}

		c.allInstitutions = c.currentProfile.InstitutionProfiles
		if len(c.currentProfile.InstitutionProfiles) > 0 {
			c.currentInstitution = &c.currentProfile.InstitutionProfiles[0]
		}
		if c.currentInstitution == nil {
			slog.Warn("No institutions in the profile.")
		}
	}
	return nil
}

func (c *Client) profilesByLogin() (*Profiles, error) {
	data, err := c.callJSONAPI("profiles.getProfilesByLogin", "get", nil)
	if err != nil {
		return nil, err
	}
	var profiles Profiles
	if err := unmarshalJSON(data, &profiles); err != nil {
		return nil, fmt.Errorf("failed to parse profiles: %w", err)
	}
	return &profiles, nil
}

func (c *Client) findMailboxRecipients(namePart string) ([]Recipient, error) {
	var institutionID int64
	if inst := c.MyInstitution(""); inst != nil {
		institutionID = inst.ID
	}

	encodedName := url.QueryEscape(namePart)
	method := "search.findRecipients&text=" + encodedName + "&query=" + encodedName +
		"&id=" + strconv.FormatInt(institutionID, 10) +
		"&typeahead=true&limit=100&scopeEmployeesToInstitution=false" +
		"&fromModule=messages" +
		"&portalRoles[]=employee&portalRoles[]=child&portalRoles[]=guardian" +
		"&docTypes[]=Profile&docTypes[]=CommonInbox"

	data, err := c.callJSONAPI(method, "get", nil)
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Results []Recipient `json:"results"`
	}
	if err := unmarshalJSON(data, &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse recipients: %w", err)
	}
	return parsed.Results, nil
}

// formatInstant formats a time as a Java Instant.toString() compatible string
// (UTC, millisecond precision).
func formatInstant(t time.Time) string {
	return t.UTC().Truncate(time.Millisecond).Format("2006-01-02T15:04:05.999Z07:00")
}

// Close closes the client and releases resources.
func (c *Client) Close() error {
	c.conn.close()
	return nil
}
