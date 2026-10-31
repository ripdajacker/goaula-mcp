package aula

import (
	"encoding/json"
	"slices"
	"strings"
)

// This file contains the model types for the Aula API (v25).
// All types tolerate unknown JSON fields.

// ==================== Enums ====================

// PortalRole is the portal role of a profile.
type PortalRole string

const (
	PortalRoleParent   PortalRole = "guardian"
	PortalRoleChild    PortalRole = "child"
	PortalRoleEmployee PortalRole = "employee"
)

// ParsePortalRole converts a string to a PortalRole, or "" when unknown.
func ParsePortalRole(value string) PortalRole {
	switch PortalRole(value) {
	case PortalRoleParent, PortalRoleChild, PortalRoleEmployee:
		return PortalRole(value)
	}
	return ""
}

// InstitutionRole is the role of a profile at an institution.
type InstitutionRole string

const (
	InstitutionRoleTeacher          InstitutionRole = "teacher"
	InstitutionRoleLeader           InstitutionRole = "leader"
	InstitutionRolePreschoolTeacher InstitutionRole = "preschool-teacher"
)

// ParseInstitutionRole converts a string to an InstitutionRole, or "" when unknown.
func ParseInstitutionRole(value string) InstitutionRole {
	switch InstitutionRole(value) {
	case InstitutionRoleTeacher, InstitutionRoleLeader, InstitutionRolePreschoolTeacher:
		return InstitutionRole(value)
	}
	return ""
}

// RelationType is the type of relation between profiles.
type RelationType string

const (
	RelationTypeParent  RelationType = "guardian"
	RelationTypeTeacher RelationType = "teacher"
	RelationTypeChild   RelationType = "child"
)

// ParseRelationType converts a string to a RelationType, or "" when unknown.
func ParseRelationType(value string) RelationType {
	switch RelationType(value) {
	case RelationTypeParent, RelationTypeTeacher, RelationTypeChild:
		return RelationType(value)
	}
	return ""
}

// CalendarEventType is the type of a calendar event.
type CalendarEventType string

const (
	EventTypeEvent              CalendarEventType = "event"
	EventTypeHoliday            CalendarEventType = "holiday"
	EventTypePresenceHoliday    CalendarEventType = "presence_holiday"
	EventTypeBirthday           CalendarEventType = "birthday"
	EventTypeExcursion          CalendarEventType = "excursion"
	EventTypeOther              CalendarEventType = "other"
	EventTypeSchoolHomeMeeting  CalendarEventType = "school_home_meeting"
	EventTypeParentalMeeting    CalendarEventType = "parental_meeting"
	EventTypePerformanceMeeting CalendarEventType = "performance_meeting"
	EventTypeLesson             CalendarEventType = "lesson"
)

// ParseCalendarEventType converts a string to a CalendarEventType. The second return
// value reports whether the value is a known event type.
func ParseCalendarEventType(value string) (CalendarEventType, bool) {
	switch CalendarEventType(value) {
	case EventTypeEvent, EventTypeHoliday, EventTypePresenceHoliday, EventTypeBirthday,
		EventTypeExcursion, EventTypeOther, EventTypeSchoolHomeMeeting,
		EventTypeParentalMeeting, EventTypePerformanceMeeting, EventTypeLesson:
		return CalendarEventType(value), true
	}
	return "", false
}

// ==================== Basic shared types ====================

// Address for profile addresses.
type Address struct {
	ID             int64  `json:"id,omitempty"`
	Street         string `json:"street,omitempty"`
	PostalCode     int    `json:"postalCode,omitempty"`
	PostalDistrict string `json:"postalDistrict,omitempty"`
}

// ProfilePicture model.
type ProfilePicture struct {
	ID                    int64  `json:"id,omitempty"`
	Key                   string `json:"key,omitempty"`
	Bucket                string `json:"bucket,omitempty"`
	IsImageScalingPending bool   `json:"isImageScalingPending,omitempty"`
	URL                   string `json:"url,omitempty"`
}

// Tag on media.
type Tag struct {
	ProfileID       int64          `json:"profileId,omitempty"`
	ID              int64          `json:"id,omitempty"`
	InstitutionCode string         `json:"institutionCode,omitempty"`
	InstitutionName string         `json:"institutionName,omitempty"`
	Role            string         `json:"role,omitempty"`
	Name            string         `json:"name,omitempty"`
	ProfilePicture  ProfilePicture `json:"profilePicture,omitempty"`
	MainGroup       string         `json:"mainGroup,omitempty"`
	ShortName       string         `json:"shortName,omitempty"`
	InstitutionRole string         `json:"institutionRole,omitempty"`
	Metadata        string         `json:"metadata,omitempty"`
}

// MainGroup interface.
type MainGroup struct {
	ID                    int64  `json:"id,omitempty"`
	Name                  string `json:"name,omitempty"`
	ShortName             string `json:"shortName,omitempty"`
	InstitutionCode       string `json:"institutionCode,omitempty"`
	InstitutionName       string `json:"institutionName,omitempty"`
	MainGroup             bool   `json:"mainGroup,omitempty"`
	UniGroupType          string `json:"uniGroupType,omitempty"`
	IsDeactivated         bool   `json:"isDeactivated,omitempty"`
	AllowMembersToBeShown bool   `json:"allowMembersToBeShown,omitempty"`
}

// InstitutionProfile representing a profile at an institution.
type InstitutionProfile struct {
	ID                                          int64           `json:"id,omitempty"`
	ProfileID                                   int64           `json:"profileId,omitempty"`
	InstitutionCode                             string          `json:"institutionCode,omitempty"`
	InstitutionName                             string          `json:"institutionName,omitempty"`
	MunicipalityCode                            string          `json:"municipalityCode,omitempty"`
	MunicipalityName                            string          `json:"municipalityName,omitempty"`
	FirstName                                   string          `json:"firstName,omitempty"`
	LastName                                    string          `json:"lastName,omitempty"`
	FullName                                    string          `json:"fullName,omitempty"`
	Gender                                      string          `json:"gender,omitempty"`
	Role                                        string          `json:"role,omitempty"`
	InstitutionRole                             string          `json:"institutionRole,omitempty"`
	InstitutionType                             string          `json:"institutionType,omitempty"`
	AulaEmail                                   string          `json:"aulaEmail,omitempty"`
	Address                                     *Address        `json:"address,omitempty"`
	Email                                       string          `json:"email,omitempty"`
	HomePhoneNumber                             string          `json:"homePhoneNumber,omitempty"`
	MobilePhoneNumber                           string          `json:"mobilePhoneNumber,omitempty"`
	WorkPhoneNumber                             string          `json:"workPhoneNumber,omitempty"`
	MainGroup                                   string          `json:"mainGroup,omitempty"`
	ShortName                                   string          `json:"shortName,omitempty"`
	ProfilePictureURL                           string          `json:"profilePictureUrl,omitempty"`
	ProfilePicture                              *ProfilePicture `json:"profilePicture,omitempty"`
	NewInstitutionProfile                       bool            `json:"newInstitutionProfile,omitempty"`
	CommunicationBlocked                        bool            `json:"communicationBlocked,omitempty"`
	IsPrimary                                   bool            `json:"isPrimary,omitempty"`
	Birthday                                    string          `json:"birthday,omitempty"`
	InstitutionProfileDescriptions              any             `json:"institutionProfileDescriptions,omitempty"`
	LastActivity                                string          `json:"lastActivity,omitempty"`
	HasCustody                                  bool            `json:"hasCustody,omitempty"`
	Alias                                       bool            `json:"alias,omitempty"`
	Groups                                      any             `json:"groups,omitempty"`
	Relation                                    any             `json:"relation,omitempty"`
	IsInternalProfilePicture                    bool            `json:"isInternalProfilePicture,omitempty"`
	AccessLevel                                 any             `json:"accessLevel,omitempty"`
	CurrentUserCanViewContactInformation        bool            `json:"currentUserCanViewContactInformation,omitempty"`
	UserHasGivenConsentToShowContactInformation bool            `json:"userHasGivenConsentToShowContactInformation,omitempty"`
	Deactivated                                 bool            `json:"deactivated,omitempty"`
	ProfileStatus                               string          `json:"profileStatus,omitempty"`
	CurrentUserCanSeeProfileDescription         bool            `json:"currentUserCanSeeProfileDescription,omitempty"`
	CurrentUserCanEditProfileDescription        bool            `json:"currentUserCanEditProfileDescription,omitempty"`
	CurrentUserCanEditContactInformation        bool            `json:"currentUserCanEditContactInformation,omitempty"`
	CurrentUserCanEditProfilePicture            bool            `json:"currentUserCanEditProfilePicture,omitempty"`
	CurrentUserCanDeleteProfilePicture          bool            `json:"currentUserCanDeleteProfilePicture,omitempty"`
	ShouldShowDeclineConsentTwoWarning          bool            `json:"shouldShowDeclineConsentTwoWarning,omitempty"`
	ContactType                                 string          `json:"contactType,omitempty"`
	HasBlockedCommunicationChannels             bool            `json:"hasBlockedCommunicationChannels,omitempty"`
	Metadata                                    string          `json:"metadata,omitempty"`
}

// Child representing a child profile.
type Child struct {
	InstitutionProfile         *InstitutionProfile `json:"institutionProfile,omitempty"`
	ID                         int64               `json:"id,omitempty"`
	ProfileID                  int64               `json:"profileId,omitempty"`
	UserID                     string              `json:"userId,omitempty"`
	Name                       string              `json:"name,omitempty"`
	ProfilePicture             *ProfilePicture     `json:"profilePicture,omitempty"`
	ShortName                  string              `json:"shortName,omitempty"`
	InstitutionCode            string              `json:"institutionCode,omitempty"`
	HasCustodyOrExtendedAccess bool                `json:"hasCustodyOrExtendedAccess,omitempty"`
}

// Profile representing a user profile.
type Profile struct {
	InstitutionProfiles        []InstitutionProfile `json:"institutionProfiles,omitempty"`
	Children                   []Child              `json:"children,omitempty"`
	Age18AndOlder              bool                 `json:"age18AndOlder,omitempty"`
	OverConsentAge             bool                 `json:"overConsentAge,omitempty"`
	ContactInfoEditable        bool                 `json:"contactInfoEditable,omitempty"`
	PortalRole                 string               `json:"portalRole,omitempty"`
	IsLatestDataPolicyAccepted bool                 `json:"isLatestDataPolicyAccepted,omitempty"`
	SupportRole                bool                 `json:"supportRole,omitempty"`
	ProfileID                  int64                `json:"profileId,omitempty"`
	DisplayName                string               `json:"displayName,omitempty"`
	FullName                   string               `json:"fullName,omitempty"`
}

// ChildByName finds a child by partial name.
func (p *Profile) ChildByName(childNamePart string) *Child {
	for i := range p.Children {
		if strings.Contains(p.Children[i].Name, childNamePart) {
			return &p.Children[i]
		}
	}
	return nil
}

// InstitutionByName finds an institution by partial name.
func (p *Profile) InstitutionByName(institutionNamePart string) *InstitutionProfile {
	for i := range p.InstitutionProfiles {
		if strings.Contains(p.InstitutionProfiles[i].InstitutionName, institutionNamePart) {
			return &p.InstitutionProfiles[i]
		}
	}
	return nil
}

// Profiles is the top-level data structure for the profiles.getProfilesByLogin response.
type Profiles struct {
	Profiles []Profile `json:"profiles,omitempty"`
}

// ProfileByDisplayName finds a profile by partial display name.
func (p *Profiles) ProfileByDisplayName(displayNamePart string) *Profile {
	for i := range p.Profiles {
		if strings.Contains(p.Profiles[i].DisplayName, displayNamePart) {
			return &p.Profiles[i]
		}
	}
	return nil
}

// FirstProfile returns the first profile, or nil when there are none.
func (p *Profiles) FirstProfile() *Profile {
	if len(p.Profiles) == 0 {
		return nil
	}
	return &p.Profiles[0]
}

// ==================== Attachments ====================

// Attachment represents a post/message attachment.
type Attachment struct {
	ID                   int64              `json:"id,omitempty"`
	Name                 string             `json:"name,omitempty"`
	Status               string             `json:"status,omitempty"`
	AttachmentType       string             `json:"attachmentType,omitempty"`
	Creator              *AttachmentCreator `json:"creator,omitempty"`
	File                 *AttachmentFile    `json:"file,omitempty"`
	Link                 any                `json:"link,omitempty"`
	Media                *AttachmentMedia   `json:"media,omitempty"`
	Document             any                `json:"document,omitempty"`
	CurrentUserCanReport bool               `json:"currentUserCanReport,omitempty"`
}

// AttachmentCreator is the creator of an attachment.
type AttachmentCreator struct {
	ProfileID       int64           `json:"profileId,omitempty"`
	ID              int64           `json:"id,omitempty"`
	InstitutionCode string          `json:"institutionCode,omitempty"`
	InstitutionName string          `json:"institutionName,omitempty"`
	Role            string          `json:"role,omitempty"`
	Name            string          `json:"name,omitempty"`
	ProfilePicture  *ProfilePicture `json:"profilePicture,omitempty"`
	MainGroup       string          `json:"mainGroup,omitempty"`
	ShortName       string          `json:"shortName,omitempty"`
	InstitutionRole string          `json:"institutionRole,omitempty"`
	Metadata        string          `json:"metadata,omitempty"`
}

// AttachmentFile is the file information of an attachment.
type AttachmentFile struct {
	ID      int64  `json:"id,omitempty"`
	Bucket  string `json:"bucket,omitempty"`
	Key     string `json:"key,omitempty"`
	URL     string `json:"url,omitempty"`
	Created string `json:"created,omitempty"`
	Name    string `json:"name,omitempty"`
}

// AttachmentMedia is the media information of an attachment.
type AttachmentMedia struct {
	Title                      string             `json:"title,omitempty"`
	Description                string             `json:"description,omitempty"`
	Tags                       []Tag              `json:"tags,omitempty"`
	MediaType                  string             `json:"mediaType,omitempty"`
	Album                      any                `json:"album,omitempty"`
	Duration                   any                `json:"duration,omitempty"`
	CurrentUserCanReport       bool               `json:"currentUserCanReport,omitempty"`
	CurrentUserCanDelete       bool               `json:"currentUserCanDelete,omitempty"`
	CurrentUserCanEditMetaData bool               `json:"currentUserCanEditMetaData,omitempty"`
	IsUploadPending            bool               `json:"isUploadPending,omitempty"`
	CommentCount               int                `json:"commentCount,omitempty"`
	AllowsComments             bool               `json:"allowsComments,omitempty"`
	CanViewComments            bool               `json:"canViewComments,omitempty"`
	IsDeleted                  bool               `json:"isDeleted,omitempty"`
	CurrentUserCanEditTags     bool               `json:"currentUserCanEditTags,omitempty"`
	ConversionStatus           string             `json:"conversionStatus,omitempty"`
	ID                         int64              `json:"id,omitempty"`
	File                       *AttachmentFile    `json:"file,omitempty"`
	Creator                    *AttachmentCreator `json:"creator,omitempty"`
	ThumbnailURL               string             `json:"thumbnailUrl,omitempty"`
	LargeThumbnailURL          string             `json:"largeThumbnailUrl,omitempty"`
	MediumThumbnailURL         string             `json:"mediumThumbnailUrl,omitempty"`
	SmallThumbnailURL          string             `json:"smallThumbnailUrl,omitempty"`
	ExtraSmallThumbnailURL     string             `json:"extraSmallThumbnailUrl,omitempty"`
}

// IsImage reports whether the attachment is an image.
func (a *Attachment) IsImage() bool {
	return a.Media != nil && strings.Contains(strings.ToLower(a.Media.MediaType), "image")
}

// IsFile reports whether the attachment is a file.
func (a *Attachment) IsFile() bool {
	return a.File != nil
}

// ThumbnailURL returns the URL of the thumbnail image (smallest available). The URL
// expires (S3 signed URL), so it must be used quickly.
func (a *Attachment) ThumbnailURL() string {
	if a.Media != nil {
		return a.Media.ThumbnailURL
	}
	return ""
}

// LargeThumbnailURL returns the URL of the large thumbnail image. The URL expires (S3
// signed URL), so it must be used quickly.
func (a *Attachment) LargeThumbnailURL() string {
	if a.Media != nil {
		return a.Media.LargeThumbnailURL
	}
	return ""
}

// FullSizeURL returns the URL of the full size image (largest available). The URL
// expires (S3 signed URL), so it must be used quickly.
func (a *Attachment) FullSizeURL() string {
	if a.Media != nil && a.Media.File != nil {
		return a.Media.File.URL
	}
	return ""
}

// FileName returns the file name of a file attachment.
func (a *Attachment) FileName() string {
	if a.File != nil {
		return a.File.Name
	}
	return ""
}

// FileURL returns the URL of the file on S3. The URL expires (S3 signed URL), so it
// must be used quickly.
func (a *Attachment) FileURL() string {
	if a.File != nil {
		return a.File.URL
	}
	return ""
}

// imageAttachments returns the attachments that are images.
func imageAttachments(attachments []Attachment) []Attachment {
	var result []Attachment
	for _, a := range attachments {
		if a.IsImage() {
			result = append(result, a)
		}
	}
	return result
}

// fileAttachments returns the attachments that are files.
func fileAttachments(attachments []Attachment) []Attachment {
	var result []Attachment
	for _, a := range attachments {
		if a.IsFile() {
			result = append(result, a)
		}
	}
	return result
}

// imageAttachmentURLs returns the full-size URLs of all image attachments.
func imageAttachmentURLs(attachments []Attachment) []string {
	images := imageAttachments(attachments)
	urls := make([]string, 0, len(images))
	for i := range images {
		urls = append(urls, images[i].FullSizeURL())
	}
	return urls
}

// fileAttachmentURLs returns the URLs of all file attachments.
func fileAttachmentURLs(attachments []Attachment) []string {
	files := fileAttachments(attachments)
	urls := make([]string, 0, len(files))
	for i := range files {
		urls = append(urls, files[i].FileURL())
	}
	return urls
}

// ==================== Posts ====================

// PostContent is the content (HTML) of a post.
type PostContent struct {
	HTML string `json:"html,omitempty"`
}

// Group that a post can be shared with.
type Group struct {
	PortalRoles           []string `json:"portalRoles,omitempty"`
	ID                    int64    `json:"id,omitempty"`
	Name                  string   `json:"name,omitempty"`
	ShortName             string   `json:"shortName,omitempty"`
	InstitutionCode       string   `json:"institutionCode,omitempty"`
	InstitutionName       string   `json:"institutionName,omitempty"`
	MainGroup             bool     `json:"mainGroup,omitempty"`
	IsDeactivated         bool     `json:"isDeactivated,omitempty"`
	AllowMembersToBeShown bool     `json:"allowMembersToBeShown,omitempty"`
}

// OwnerProfile is the owner profile of a post.
type OwnerProfile struct {
	ID             int64            `json:"id,omitempty"`
	ProfileID      int64            `json:"profileId,omitempty"`
	UniPersonID    int64            `json:"uniPersonId,omitempty"`
	FirstName      string           `json:"firstName,omitempty"`
	LastName       string           `json:"lastName,omitempty"`
	FullName       string           `json:"fullName,omitempty"`
	ShortName      string           `json:"shortName,omitempty"`
	Role           string           `json:"role,omitempty"`
	MailBoxID      int64            `json:"mailBoxId,omitempty"`
	Institution    *InstitutionInfo `json:"institution,omitempty"`
	Gender         string           `json:"gender,omitempty"`
	ProfilePicture *ProfilePicture  `json:"profilePicture,omitempty"`
	MainGroupName  string           `json:"mainGroupName,omitempty"`
	Metadata       string           `json:"metadata,omitempty"`
}

// InstitutionInfo holds institution details of an owner profile.
type InstitutionInfo struct {
	InstitutionCode         string                   `json:"institutionCode,omitempty"`
	InstitutionName         string                   `json:"institutionName,omitempty"`
	MunicipalityCode        string                   `json:"municipalityCode,omitempty"`
	MunicipalityName        string                   `json:"municipalityName,omitempty"`
	Type                    string                   `json:"type,omitempty"`
	AdministrativeAuthority *AdministrativeAuthority `json:"administrativeAuthority,omitempty"`
}

// AdministrativeAuthority holds municipality authority details.
type AdministrativeAuthority struct {
	ID               int64    `json:"id,omitempty"`
	Name             string   `json:"name,omitempty"`
	InstitutionCodes []string `json:"institutionCodes,omitempty"`
}

// Post is a post from Aula.
type Post struct {
	ID                    int64         `json:"id,omitempty"`
	Title                 string        `json:"title,omitempty"`
	Content               *PostContent  `json:"content,omitempty"`
	Timestamp             string        `json:"timestamp,omitempty"`
	OwnerProfile          *OwnerProfile `json:"ownerProfile,omitempty"`
	AllowComments         bool          `json:"allowComments,omitempty"`
	SharedWithGroups      []Group       `json:"sharedWithGroups,omitempty"`
	PublishAt             string        `json:"publishAt,omitempty"`
	IsPublished           bool          `json:"isPublished,omitempty"`
	ExpireAt              string        `json:"expireAt,omitempty"`
	IsExpired             bool          `json:"isExpired,omitempty"`
	IsImportant           bool          `json:"isImportant,omitempty"`
	ImportantFrom         string        `json:"importantFrom,omitempty"`
	ImportantTo           string        `json:"importantTo,omitempty"`
	RelatedProfiles       []Profile     `json:"relatedProfiles,omitempty"`
	Attachments           []Attachment  `json:"attachments,omitempty"`
	PendingMedia          bool          `json:"pendingMedia,omitempty"`
	CommentCount          int           `json:"commentCount,omitempty"`
	CanCurrentUserDelete  bool          `json:"canCurrentUserDelete,omitempty"`
	CanCurrentUserReport  bool          `json:"canCurrentUserReport,omitempty"`
	CanCurrentUserComment bool          `json:"canCurrentUserComment,omitempty"`
	EditedAt              string        `json:"editedAt,omitempty"`
}

// HasImageAttachments reports whether the post has image attachments.
func (p *Post) HasImageAttachments() bool { return len(p.ImageAttachments()) > 0 }

// HasFileAttachments reports whether the post has file attachments.
func (p *Post) HasFileAttachments() bool { return len(p.FileAttachments()) > 0 }

// ImageAttachments returns the image attachments of the post.
func (p *Post) ImageAttachments() []Attachment { return imageAttachments(p.Attachments) }

// FileAttachments returns the file attachments of the post.
func (p *Post) FileAttachments() []Attachment { return fileAttachments(p.Attachments) }

// ImageAttachmentURLs returns the full-size URLs of the image attachments.
func (p *Post) ImageAttachmentURLs() []string { return imageAttachmentURLs(p.Attachments) }

// FileAttachmentURLs returns the URLs of the file attachments.
func (p *Post) FileAttachmentURLs() []string { return fileAttachmentURLs(p.Attachments) }

// PostsResponse is the wrapper for the posts.getAllPosts response.
type PostsResponse struct {
	Posts                   []Post `json:"posts,omitempty"`
	ProfileLastSeenPostDate string `json:"profileLastSeenPostDate,omitempty"`
	HasMorePosts            bool   `json:"hasMorePosts,omitempty"`
}

// ==================== Messages & threads ====================

// MessageText is the text content of a message.
type MessageText struct {
	HTML string `json:"html,omitempty"`
}

// MailBoxOwner is the mailbox owner of a message.
type MailBoxOwner struct {
	ProfileID        int64  `json:"profileId,omitempty"`
	PortalRole       string `json:"portalRole,omitempty"`
	IsDeactivated    bool   `json:"isDeactivated,omitempty"`
	MailBoxOwnerType string `json:"mailBoxOwnerType,omitempty"`
	ID               int64  `json:"id,omitempty"`
	IsDeleted        bool   `json:"isDeleted,omitempty"`
}

// Sender of a message.
type Sender struct {
	ShortName          string          `json:"shortName,omitempty"`
	ProfilePicture     *ProfilePicture `json:"profilePicture,omitempty"`
	InstitutionCode    string          `json:"institutionCode,omitempty"`
	MailBoxOwner       *MailBoxOwner   `json:"mailBoxOwner,omitempty"`
	FullName           string          `json:"fullName,omitempty"`
	Metadata           string          `json:"metadata,omitempty"`
	AnswerDirectlyName string          `json:"answerDirectlyName,omitempty"`
}

// Message is a message within a thread.
type Message struct {
	ID                 string       `json:"id,omitempty"`
	SendDateTime       string       `json:"sendDateTime,omitempty"`
	DeletedAt          string       `json:"deletedAt,omitempty"`
	Text               *MessageText `json:"text,omitempty"`
	HasAttachments     bool         `json:"hasAttachments,omitempty"`
	PendingMedia       bool         `json:"pendingMedia,omitempty"`
	MessageType        string       `json:"messageType,omitempty"`
	LeaverNames        []string     `json:"leaverNames,omitempty"`
	InviterName        string       `json:"inviterName,omitempty"`
	Sender             *Sender      `json:"sender,omitempty"`
	NewRecipients      any          `json:"newRecipients,omitempty"`
	OriginalRecipients any          `json:"originalRecipients,omitempty"`
	Attachments        []Attachment `json:"attachments,omitempty"`
	CanReplyToMessage  bool         `json:"canReplyToMessage,omitempty"`

	// Thread context (added by the client, not from Aula)
	ThreadID              int64   `json:"threadId,omitempty"`
	ThreadSubject         string  `json:"threadSubject,omitempty"`
	ThreadCreator         *Sender `json:"threadCreator,omitempty"`
	ThreadStartedDateTime string  `json:"threadStartedDateTime,omitempty"`
}

// TextHTML returns the message text as HTML string.
func (m *Message) TextHTML() string {
	if m.Text != nil {
		return m.Text.HTML
	}
	return ""
}

// HasImageAttachments reports whether the message has image attachments.
func (m *Message) HasImageAttachments() bool { return len(m.ImageAttachments()) > 0 }

// HasFileAttachments reports whether the message has file attachments.
func (m *Message) HasFileAttachments() bool { return len(m.FileAttachments()) > 0 }

// ImageAttachments returns the image attachments of the message.
func (m *Message) ImageAttachments() []Attachment { return imageAttachments(m.Attachments) }

// FileAttachments returns the file attachments of the message.
func (m *Message) FileAttachments() []Attachment { return fileAttachments(m.Attachments) }

// ImageAttachmentURLs returns the full-size URLs of the image attachments.
func (m *Message) ImageAttachmentURLs() []string { return imageAttachmentURLs(m.Attachments) }

// FileAttachmentURLs returns the URLs of the file attachments.
func (m *Message) FileAttachmentURLs() []string { return fileAttachmentURLs(m.Attachments) }

// ThreadEntityLink is a thread entity link DTO.
type ThreadEntityLink struct {
	EntityID   string `json:"entityId,omitempty"`
	ThreadType string `json:"threadType,omitempty"`
}

// MessagesForThread is a thread with all its messages.
type MessagesForThread struct {
	Messages              []Message         `json:"messages,omitempty"`
	FirstMessage          *Message          `json:"firstMessage,omitempty"`
	ThreadCreator         *Sender           `json:"threadCreator,omitempty"`
	ThreadStartedDateTime string            `json:"threadStartedDateTime,omitempty"`
	IsThreadForwarded     bool              `json:"isThreadForwarded,omitempty"`
	MoreMessagesExist     bool              `json:"moreMessagesExist,omitempty"`
	HasSecureDocuments    bool              `json:"hasSecureDocuments,omitempty"`
	Page                  int               `json:"page,omitempty"`
	TotalMessageCount     int               `json:"totalMessageCount,omitempty"`
	ThreadEntityLinkDTO   *ThreadEntityLink `json:"threadEntityLinkDto,omitempty"`
	ID                    int64             `json:"id,omitempty"`
	Subject               string            `json:"subject,omitempty"`
	Recipients            []Sender          `json:"recipients,omitempty"`
	ExtraRecipientsCount  int               `json:"extraRecipientsCount,omitempty"`
	Muted                 bool              `json:"muted,omitempty"`
	Marked                bool              `json:"marked,omitempty"`
	Sensitive             bool              `json:"sensitive,omitempty"`
	LastReadMessageID     string            `json:"lastReadMessageId,omitempty"`
	IsArchived            bool              `json:"isArchived,omitempty"`
	MailBoxOwner          *MailBoxOwner     `json:"mailBoxOwner,omitempty"`
	InstitutionCode       string            `json:"institutionCode,omitempty"`
}

// SentToFewPeople reports whether this thread is sent to few people (less than 10).
// Normal threads might be hundreds of recipients; a thread with only a few people is
// more relevant (not a "broadcast").
func (t *MessagesForThread) SentToFewPeople() bool {
	return t.SentToFewerPeopleThan(10)
}

// SentToFewerPeopleThan reports whether this thread has fewer than count recipients.
func (t *MessagesForThread) SentToFewerPeopleThan(count int) bool {
	return len(t.Recipients) < count
}

// ParentRecipients returns the recipients whose portal role is parent (guardian).
func (t *MessagesForThread) ParentRecipients() []Sender {
	return t.recipientsWithRole(PortalRoleParent)
}

// ChildRecipients returns the recipients whose portal role is child.
func (t *MessagesForThread) ChildRecipients() []Sender {
	return t.recipientsWithRole(PortalRoleChild)
}

// EmployeeRecipients returns the recipients whose portal role is employee.
func (t *MessagesForThread) EmployeeRecipients() []Sender {
	return t.recipientsWithRole(PortalRoleEmployee)
}

func (t *MessagesForThread) recipientsWithRole(role PortalRole) []Sender {
	var result []Sender
	for _, r := range t.Recipients {
		if r.MailBoxOwner != nil && r.MailBoxOwner.PortalRole == string(role) {
			result = append(result, r)
		}
	}
	return result
}

// AnyMessageHasAttachments reports whether any message has attachments.
func (t *MessagesForThread) AnyMessageHasAttachments() bool {
	for i := range t.Messages {
		if t.Messages[i].HasAttachments {
			return true
		}
	}
	return false
}

// AnyMessageHasImageAttachments reports whether any message has image attachments.
func (t *MessagesForThread) AnyMessageHasImageAttachments() bool {
	for i := range t.Messages {
		if t.Messages[i].HasImageAttachments() {
			return true
		}
	}
	return false
}

// AnyMessageHasFileAttachments reports whether any message has file attachments.
func (t *MessagesForThread) AnyMessageHasFileAttachments() bool {
	for i := range t.Messages {
		if t.Messages[i].HasFileAttachments() {
			return true
		}
	}
	return false
}

// AllImageAttachments returns all image attachments of all messages.
func (t *MessagesForThread) AllImageAttachments() []Attachment {
	var result []Attachment
	for i := range t.Messages {
		result = append(result, t.Messages[i].ImageAttachments()...)
	}
	return result
}

// AllFileAttachments returns all file attachments of all messages.
func (t *MessagesForThread) AllFileAttachments() []Attachment {
	var result []Attachment
	for i := range t.Messages {
		result = append(result, t.Messages[i].FileAttachments()...)
	}
	return result
}

// AllImageAttachmentURLs returns the full-size URLs of all image attachments.
func (t *MessagesForThread) AllImageAttachmentURLs() []string {
	attachments := t.AllImageAttachments()
	urls := make([]string, 0, len(attachments))
	for i := range attachments {
		urls = append(urls, attachments[i].FullSizeURL())
	}
	return urls
}

// AllFileAttachmentURLs returns the URLs of all file attachments.
func (t *MessagesForThread) AllFileAttachmentURLs() []string {
	attachments := t.AllFileAttachments()
	urls := make([]string, 0, len(attachments))
	for i := range attachments {
		urls = append(urls, attachments[i].FileURL())
	}
	return urls
}

// LatestMessage is the latest message in a thread.
type LatestMessage struct {
	ID           string       `json:"id,omitempty"`
	SendDateTime string       `json:"sendDateTime,omitempty"`
	Text         *MessageText `json:"text,omitempty"`
}

// RegardingChild is a child that a thread is regarding.
type RegardingChild struct {
	ProfilePicture *ProfilePicture `json:"profilePicture,omitempty"`
	ShortName      string          `json:"shortName,omitempty"`
	ProfileID      int64           `json:"profileId,omitempty"`
	DisplayName    string          `json:"displayName,omitempty"`
}

// Creator of a thread.
type Creator struct {
	MailBoxOwner       *MailBoxOwner `json:"mailBoxOwner,omitempty"`
	FullName           string        `json:"fullName,omitempty"`
	Metadata           string        `json:"metadata,omitempty"`
	AnswerDirectlyName string        `json:"answerDirectlyName,omitempty"`
}

// ThreadRecipient is a recipient of a thread.
type ThreadRecipient struct {
	LastReadMessageID  string          `json:"lastReadMessageId,omitempty"`
	LastReadTimestamp  string          `json:"lastReadTimestamp,omitempty"`
	LeaveTime          string          `json:"leaveTime,omitempty"`
	DeletedAt          string          `json:"deletedAt,omitempty"`
	ShortName          string          `json:"shortName,omitempty"`
	ProfilePicture     *ProfilePicture `json:"profilePicture,omitempty"`
	MailBoxOwner       *MailBoxOwner   `json:"mailBoxOwner,omitempty"`
	FullName           string          `json:"fullName,omitempty"`
	Metadata           string          `json:"metadata,omitempty"`
	AnswerDirectlyName string          `json:"answerDirectlyName,omitempty"`
}

// Thread is a conversation.
type Thread struct {
	LeaveTime                     string            `json:"leaveTime,omitempty"`
	LatestMessage                 *LatestMessage    `json:"latestMessage,omitempty"`
	RegardingChildren             []RegardingChild  `json:"regardingChildren,omitempty"`
	Creator                       *Creator          `json:"creator,omitempty"`
	StartedTime                   string            `json:"startedTime,omitempty"`
	Read                          bool              `json:"read,omitempty"`
	IsThreadOrSubscriptionDeleted bool              `json:"isThreadOrSubscriptionDeleted,omitempty"`
	SubscriptionID                int64             `json:"subscriptionId,omitempty"`
	SubscriptionType              string            `json:"subscriptionType,omitempty"`
	NumberOfBundleItems           int               `json:"numberOfBundleItems,omitempty"`
	PrimarySubscriptionID         int64             `json:"primarySubscriptionId,omitempty"`
	ThreadEntityLinkDTO           *ThreadEntityLink `json:"threadEntityLinkDto,omitempty"`
	ID                            int64             `json:"id,omitempty"`
	Subject                       string            `json:"subject,omitempty"`
	Recipients                    []ThreadRecipient `json:"recipients,omitempty"`
	ExtraRecipientsCount          int               `json:"extraRecipientsCount,omitempty"`
	Muted                         bool              `json:"muted,omitempty"`
	Marked                        bool              `json:"marked,omitempty"`
	Sensitive                     bool              `json:"sensitive,omitempty"`
	LastReadMessageID             string            `json:"lastReadMessageId,omitempty"`
	IsArchived                    bool              `json:"isArchived,omitempty"`
	MailBoxOwner                  *MailBoxOwner     `json:"mailBoxOwner,omitempty"`
	InstitutionCode               string            `json:"institutionCode,omitempty"`
	Messages                      []Message         `json:"messages,omitempty"`
}

// ThreadsResponse is the wrapper for the messaging.getThreads response.
type ThreadsResponse struct {
	MoreMessagesExist bool     `json:"moreMessagesExist,omitempty"`
	Page              int      `json:"page,omitempty"`
	Threads           []Thread `json:"threads,omitempty"`
}

// ==================== Message search ====================

// MailboxParticipant is a mailbox participant for message search.
type MailboxParticipant struct {
	ID               int64  `json:"id,omitempty"`
	MailBoxOwnerType string `json:"mailBoxOwnerType,omitempty"`
}

// FindMessageRequest is the request body for search.findMessage.
type FindMessageRequest struct {
	ActiveChildrenInstitutionProfileIDs []int64              `json:"activeChildrenInstitutionProfileIds"`
	CommonInboxID                       *string              `json:"commonInboxId"`
	ExactTerm                           bool                 `json:"exactTerm"`
	FilterBy                            string               `json:"filterBy"`
	FromDate                            string               `json:"fromDate"`
	ToDate                              string               `json:"toDate"`
	HasAttachments                      *bool                `json:"hasAttachments"`
	InstitutionCodes                    []string             `json:"institutionCodes"`
	Limit                               int                  `json:"limit"`
	MessageContent                      *string              `json:"messageContent"`
	Offset                              int                  `json:"offset"`
	Participants                        []MailboxParticipant `json:"participants"`
	SortBy                              string               `json:"sortBy"`
	SortDirection                       string               `json:"sortDirection"`
	Text                                string               `json:"text"`
	ThreadCreators                      []MailBoxOwner       `json:"threadCreators"`
	ThreadSubject                       *string              `json:"threadSubject"`
}

// NewFindMessageRequest creates a FindMessageRequest with Aula's default values.
func NewFindMessageRequest(activeChildrenInstitutionProfileIDs []int64, institutionCodes []string) *FindMessageRequest {
	if activeChildrenInstitutionProfileIDs == nil {
		activeChildrenInstitutionProfileIDs = []int64{}
	}
	if institutionCodes == nil {
		institutionCodes = []string{}
	}
	return &FindMessageRequest{
		ActiveChildrenInstitutionProfileIDs: activeChildrenInstitutionProfileIDs,
		ExactTerm:                           true,
		FilterBy:                            "all",
		InstitutionCodes:                    institutionCodes,
		Limit:                               20,
		Offset:                              0,
		Participants:                        []MailboxParticipant{},
		SortBy:                              "date",
		SortDirection:                       "desc",
		Text:                                "",
		ThreadCreators:                      []MailBoxOwner{},
	}
}

// FoundMessage is a message found via search.
type FoundMessage struct {
	ID            string       `json:"id,omitempty"`
	Text          *MessageText `json:"text,omitempty"`
	SendDateTime  string       `json:"sendDateTime,omitempty"`
	ThreadID      string       `json:"threadId,omitempty"`
	ThreadSubject string       `json:"threadSubject,omitempty"`
	Attachments   []Attachment `json:"attachments,omitempty"`
}

// TextHTML returns the message text as HTML string.
func (m *FoundMessage) TextHTML() string {
	if m.Text != nil {
		return m.Text.HTML
	}
	return ""
}

// HasImageAttachments reports whether the message has image attachments.
func (m *FoundMessage) HasImageAttachments() bool { return len(m.ImageAttachments()) > 0 }

// HasFileAttachments reports whether the message has file attachments.
func (m *FoundMessage) HasFileAttachments() bool { return len(m.FileAttachments()) > 0 }

// ImageAttachments returns the image attachments of the message.
func (m *FoundMessage) ImageAttachments() []Attachment { return imageAttachments(m.Attachments) }

// FileAttachments returns the file attachments of the message.
func (m *FoundMessage) FileAttachments() []Attachment { return fileAttachments(m.Attachments) }

// ImageAttachmentURLs returns the full-size URLs of the image attachments.
func (m *FoundMessage) ImageAttachmentURLs() []string { return imageAttachmentURLs(m.Attachments) }

// FileAttachmentURLs returns the URLs of the file attachments.
func (m *FoundMessage) FileAttachmentURLs() []string { return fileAttachmentURLs(m.Attachments) }

// FindMessageThread is the thread info for a found message.
type FindMessageThread struct {
	ID               string `json:"id,omitempty"`
	Subject          string `json:"subject,omitempty"`
	SensitivityLevel int    `json:"sensitivityLevel,omitempty"`
	IsForwarded      bool   `json:"isForwarded,omitempty"`
	ThreadType       string `json:"threadType,omitempty"`
}

// FindMessageResult is the result of a message search.
type FindMessageResult struct {
	SubscriptionID  int64              `json:"subscriptionId,omitempty"`
	InstitutionCode string             `json:"institutionCode,omitempty"`
	Deleted         bool               `json:"deleted,omitempty"`
	Marked          bool               `json:"marked,omitempty"`
	Muted           bool               `json:"muted,omitempty"`
	Thread          *FindMessageThread `json:"thread,omitempty"`
	SearchMessage   *FoundMessage      `json:"searchMessage,omitempty"`
}

// ==================== People search ====================

// RelatedProfile is a related profile (e.g. a parent, child, or teacher related to a recipient).
type RelatedProfile struct {
	FirstName    string       `json:"firstName,omitempty"`
	LastName     string       `json:"lastName,omitempty"`
	ProfileID    int64        `json:"profileId,omitempty"`
	ID           int64        `json:"id,omitempty"`
	RelationType RelationType `json:"relationType,omitempty"`
	AulaEmail    string       `json:"aulaEmail,omitempty"`
	MainGroup    *MainGroup   `json:"mainGroup,omitempty"`
	Metadata     string       `json:"metadata,omitempty"`
}

// Recipient is a recipient found via search (person, group, etc.).
type Recipient struct {
	ID               string           `json:"id,omitempty"`
	InstitutionName  string           `json:"institutionName,omitempty"`
	InstitutionCode  string           `json:"institutionCode,omitempty"`
	Name             string           `json:"name,omitempty"`
	Description      string           `json:"description,omitempty"`
	MunicipalityName string           `json:"municipalityName,omitempty"`
	MunicipalityCode string           `json:"municipalityCode,omitempty"`
	AulaEmail        string           `json:"aulaEmail,omitempty"`
	PortalRole       string           `json:"portalRole,omitempty"`
	ProfileID        string           `json:"profileId,omitempty"`
	InstitutionRole  string           `json:"institutionRole,omitempty"`
	ShortName        string           `json:"shortName,omitempty"`
	RelatedProfiles  []RelatedProfile `json:"relatedProfiles,omitempty"`
	IsGroupMember    bool             `json:"isGroupMember,omitempty"`
}

// PortalRoleEnum returns the portal role as an enum.
func (r *Recipient) PortalRoleEnum() PortalRole {
	return ParsePortalRole(r.PortalRole)
}

// InstitutionRoleEnum returns the institution role as an enum.
func (r *Recipient) InstitutionRoleEnum() InstitutionRole {
	return ParseInstitutionRole(r.InstitutionRole)
}

// Parents returns the related profiles that are parents.
func (r *Recipient) Parents() []RelatedProfile {
	return r.relatedByType(RelationTypeParent)
}

// Children returns the related profiles that are children.
func (r *Recipient) Children() []RelatedProfile {
	return r.relatedByType(RelationTypeChild)
}

// Teachers returns the related profiles that are teachers.
func (r *Recipient) Teachers() []RelatedProfile {
	return r.relatedByType(RelationTypeTeacher)
}

func (r *Recipient) relatedByType(t RelationType) []RelatedProfile {
	var result []RelatedProfile
	for _, rp := range r.RelatedProfiles {
		if rp.RelationType == t {
			result = append(result, rp)
		}
	}
	return result
}

// ==================== Contacts ====================

// ChildGroup is a single group (class, SFO, etc.) on one of the user's children, as
// returned by groups.getGroupsByContext.
type ChildGroup struct {
	ID              int64  `json:"id,omitempty"`
	Name            string `json:"name,omitempty"`
	InstitutionCode string `json:"institutionCode,omitempty"`
	ShowAsDefault   bool   `json:"showAsDefault,omitempty"`
}

// ChildGroups is the result of groups.getGroupsByContext for one of the user's
// children: the child and the groups it belongs to.
type ChildGroups struct {
	ProfileID   int64        `json:"profileId,omitempty"`
	DisplayName string       `json:"displayName,omitempty"`
	Groups      []ChildGroup `json:"groups,omitempty"`
}

// SortedGroups returns the groups sorted by name.
func (c *ChildGroups) SortedGroups() []ChildGroup {
	groups := make([]ChildGroup, len(c.Groups))
	copy(groups, c.Groups)
	slices.SortStableFunc(groups, func(a, b ChildGroup) int { return strings.Compare(a.Name, b.Name) })
	return groups
}

// ContactEntry is one entry of the contact list (profiles.getContactlist): a person
// (typically a child) plus its relations (guardians, etc.). Contact details are only
// visible when the person has consented to show contact information.
type ContactEntry struct {
	ID                                          int64             `json:"id,omitempty"`
	ProfileID                                   int64             `json:"profileId,omitempty"`
	InstitutionCode                             string            `json:"institutionCode,omitempty"`
	InstitutionName                             string            `json:"institutionName,omitempty"`
	FirstName                                   string            `json:"firstName,omitempty"`
	LastName                                    string            `json:"lastName,omitempty"`
	FullName                                    string            `json:"fullName,omitempty"`
	Role                                        string            `json:"role,omitempty"`
	AulaEmail                                   string            `json:"aulaEmail,omitempty"`
	Email                                       string            `json:"email,omitempty"`
	HomePhoneNumber                             string            `json:"homePhoneNumber,omitempty"`
	MobilePhoneNumber                           string            `json:"mobilePhoneNumber,omitempty"`
	WorkPhoneNumber                             string            `json:"workPhoneNumber,omitempty"`
	ShortName                                   string            `json:"shortName,omitempty"`
	Metadata                                    string            `json:"metadata,omitempty"`
	CurrentUserCanViewContactInformation        bool              `json:"currentUserCanViewContactInformation,omitempty"`
	UserHasGivenConsentToShowContactInformation bool              `json:"userHasGivenConsentToShowContactInformation,omitempty"`
	CommunicationBlocked                        bool              `json:"communicationBlocked,omitempty"`
	Relations                                   []ContactRelation `json:"relations,omitempty"`
}

// VisibleGuardians returns the relations (guardians) of this person's profile that have
// contact information visible.
func (e *ContactEntry) VisibleGuardians() []ContactRelation {
	var result []ContactRelation
	for _, r := range e.Relations {
		if r.ContactInformationVisible() {
			result = append(result, r)
		}
	}
	return result
}

// ContactRelation is a person related to a ContactEntry (guardian, grandparent, etc.)
// with its relation name and contact details. Contact details are masked server-side
// unless the person has consented to show them.
type ContactRelation struct {
	ID        int64  `json:"id,omitempty"`
	ProfileID int64  `json:"profileId,omitempty"`
	FullName  string `json:"fullName,omitempty"`
	Role      string `json:"role,omitempty"`
	// Relation name in Danish, e.g. "Mor", "Far", "Bedsteforælder".
	Relation                                    string `json:"relation,omitempty"`
	AulaEmail                                   string `json:"aulaEmail,omitempty"`
	Email                                       string `json:"email,omitempty"`
	HomePhoneNumber                             string `json:"homePhoneNumber,omitempty"`
	MobilePhoneNumber                           string `json:"mobilePhoneNumber,omitempty"`
	WorkPhoneNumber                             string `json:"workPhoneNumber,omitempty"`
	ShortName                                   string `json:"shortName,omitempty"`
	Metadata                                    string `json:"metadata,omitempty"`
	CurrentUserCanViewContactInformation        bool   `json:"currentUserCanViewContactInformation,omitempty"`
	UserHasGivenConsentToShowContactInformation bool   `json:"userHasGivenConsentToShowContactInformation,omitempty"`
	CommunicationBlocked                        bool   `json:"communicationBlocked,omitempty"`
}

// ContactInformationVisible reports whether the person has given consent to show
// contact information and it is visible to the current user.
func (r *ContactRelation) ContactInformationVisible() bool {
	return r.CurrentUserCanViewContactInformation && r.UserHasGivenConsentToShowContactInformation
}

// ==================== Presence ====================

// DailyOverview represents a single daily overview entry.
type DailyOverview struct {
	ID                                int64               `json:"id,omitempty"`
	InstitutionProfile                *InstitutionProfile `json:"institutionProfile,omitempty"`
	MainGroup                         *MainGroup          `json:"mainGroup,omitempty"`
	Status                            int                 `json:"status,omitempty"`
	Location                          string              `json:"location,omitempty"`
	SleepIntervals                    []any               `json:"sleepIntervals,omitempty"`
	CheckInTime                       string              `json:"checkInTime,omitempty"`
	CheckOutTime                      string              `json:"checkOutTime,omitempty"`
	EditablePresenceStates            []any               `json:"editablePresenceStates,omitempty"`
	IsDefaultEntryTime                bool                `json:"isDefaultEntryTime,omitempty"`
	IsDefaultExitTime                 bool                `json:"isDefaultExitTime,omitempty"`
	IsPlannedTimesOutsideOpeningHours bool                `json:"isPlannedTimesOutsideOpeningHours,omitempty"`
	ActivityType                      int                 `json:"activityType,omitempty"`
	EntryTime                         string              `json:"entryTime,omitempty"`
	ExitTime                          string              `json:"exitTime,omitempty"`
	ExitWith                          string              `json:"exitWith,omitempty"`
	Comment                           string              `json:"comment,omitempty"`
	SpareTimeActivity                 any                 `json:"spareTimeActivity,omitempty"`
	SelfDeciderStartTime              string              `json:"selfDeciderStartTime,omitempty"`
	SelfDeciderEndTime                string              `json:"selfDeciderEndTime,omitempty"`
}

// ==================== Gallery ====================

// AlbumCreator is the creator of a gallery album.
type AlbumCreator struct {
	ProfileID       int64           `json:"profileId,omitempty"`
	ID              int64           `json:"id,omitempty"`
	InstitutionCode string          `json:"institutionCode,omitempty"`
	InstitutionName string          `json:"institutionName,omitempty"`
	Role            string          `json:"role,omitempty"`
	Name            string          `json:"name,omitempty"`
	ProfilePicture  *ProfilePicture `json:"profilePicture,omitempty"`
	MainGroup       string          `json:"mainGroup,omitempty"`
	ShortName       string          `json:"shortName,omitempty"`
	InstitutionRole string          `json:"institutionRole,omitempty"`
	Metadata        string          `json:"metadata,omitempty"`
}

// MediaCreator is the creator of media.
type MediaCreator struct {
	ProfileID       int64           `json:"profileId,omitempty"`
	ID              int64           `json:"id,omitempty"`
	InstitutionCode string          `json:"institutionCode,omitempty"`
	InstitutionName string          `json:"institutionName,omitempty"`
	Role            string          `json:"role,omitempty"`
	Name            string          `json:"name,omitempty"`
	ProfilePicture  *ProfilePicture `json:"profilePicture,omitempty"`
	MainGroup       string          `json:"mainGroup,omitempty"`
	ShortName       string          `json:"shortName,omitempty"`
	InstitutionRole string          `json:"institutionRole,omitempty"`
	Metadata        string          `json:"metadata,omitempty"`
}

// MediaFile is media file info.
type MediaFile struct {
	ID      int64  `json:"id,omitempty"`
	Bucket  string `json:"bucket,omitempty"`
	Key     string `json:"key,omitempty"`
	URL     string `json:"url,omitempty"`
	Created string `json:"created,omitempty"`
	Name    string `json:"name,omitempty"`
}

// AlbumMedia is a media item in an album.
type AlbumMedia struct {
	Title                      string        `json:"title,omitempty"`
	Description                string        `json:"description,omitempty"`
	Tags                       []Tag         `json:"tags,omitempty"`
	MediaType                  string        `json:"mediaType,omitempty"`
	Album                      any           `json:"album,omitempty"`
	Duration                   int64         `json:"duration,omitempty"`
	CurrentUserCanReport       bool          `json:"currentUserCanReport,omitempty"`
	CurrentUserCanDelete       bool          `json:"currentUserCanDelete,omitempty"`
	CurrentUserCanEditMetaData bool          `json:"currentUserCanEditMetaData,omitempty"`
	IsUploadPending            bool          `json:"isUploadPending,omitempty"`
	CommentCount               int           `json:"commentCount,omitempty"`
	AllowsComments             bool          `json:"allowsComments,omitempty"`
	CanViewComments            bool          `json:"canViewComments,omitempty"`
	IsDeleted                  bool          `json:"isDeleted,omitempty"`
	CurrentUserCanEditTags     bool          `json:"currentUserCanEditTags,omitempty"`
	ConversionStatus           string        `json:"conversionStatus,omitempty"`
	ID                         int64         `json:"id,omitempty"`
	File                       *MediaFile    `json:"file,omitempty"`
	Creator                    *MediaCreator `json:"creator,omitempty"`
	ThumbnailURL               string        `json:"thumbnailUrl,omitempty"`
	LargeThumbnailURL          string        `json:"largeThumbnailUrl,omitempty"`
	MediumThumbnailURL         string        `json:"mediumThumbnailUrl,omitempty"`
	SmallThumbnailURL          string        `json:"smallThumbnailUrl,omitempty"`
	ExtraSmallThumbnailURL     string        `json:"extraSmallThumbnailUrl,omitempty"`
}

// HasChildNameTag checks if the media has a tag with the name part and role child.
func (m *AlbumMedia) HasChildNameTag(namePart string) bool {
	for _, tag := range m.Tags {
		if strings.Contains(strings.ToLower(tag.Name), strings.ToLower(namePart)) && tag.Role == "child" {
			return true
		}
	}
	return false
}

// FullSizeURL returns the URL of the full size image (largest available). The URL
// expires (S3 signed URL), so it must be used quickly.
func (m *AlbumMedia) FullSizeURL() string {
	if m.File != nil {
		return m.File.URL
	}
	return ""
}

// GalleryAlbum is a gallery album.
type GalleryAlbum struct {
	ID                            int64         `json:"id,omitempty"`
	Title                         string        `json:"title,omitempty"`
	Description                   string        `json:"description,omitempty"`
	Creator                       *AlbumCreator `json:"creator,omitempty"`
	CreationDate                  string        `json:"creationDate,omitempty"`
	SharedWithGroups              []Group       `json:"sharedWithGroups,omitempty"`
	ThumbnailsURLs                []string      `json:"thumbnailsUrls,omitempty"`
	RegardingInstitutionProfileID int64         `json:"regardingInstitutionProfileId,omitempty"`
	CurrentUserCanEdit            bool          `json:"currentUserCanEdit,omitempty"`
	CurrentUserCanDelete          bool          `json:"currentUserCanDelete,omitempty"`
	CurrentUserCanAddMedia        bool          `json:"currentUserCanAddMedia,omitempty"`

	// IsDefaultMyChildAlbum indicates whether this is the default "my child" album
	// (generated dynamically by Aula).
	IsDefaultMyChildAlbum bool `json:"isDefaultMyChildAlbum,omitempty"`

	// Media in this album (populated by the client).
	Media []AlbumMedia `json:"media,omitempty"`
}

// HasImages reports whether the album has images.
func (a *GalleryAlbum) HasImages() bool {
	return len(a.Media) > 0
}

// Images returns the media of the album.
func (a *GalleryAlbum) Images() []AlbumMedia {
	return a.Media
}

// ImageURLs returns the full-size URLs of all media in the album.
func (a *GalleryAlbum) ImageURLs() []string {
	urls := make([]string, 0, len(a.Media))
	for i := range a.Media {
		urls = append(urls, a.Media[i].FullSizeURL())
	}
	return urls
}

// AlbumMediaResponse is the wrapper for the gallery.getMedia response.
type AlbumMediaResponse struct {
	Album      *GalleryAlbum `json:"album,omitempty"`
	MediaCount int           `json:"mediaCount,omitempty"`
	Results    []AlbumMedia  `json:"results,omitempty"`
	TotalSize  int64         `json:"totalSize,omitempty"`
	Limit      int           `json:"limit,omitempty"`
	StartIndex int           `json:"startIndex,omitempty"`
}

// ==================== Calendar ====================

// InvitedGroup is an invited group for a calendar event.
type InvitedGroup struct {
	ID                    int64  `json:"id,omitempty"`
	Name                  string `json:"name,omitempty"`
	ShortName             string `json:"shortName,omitempty"`
	InstitutionCode       string `json:"institutionCode,omitempty"`
	InstitutionName       string `json:"institutionName,omitempty"`
	MainGroup             bool   `json:"mainGroup,omitempty"`
	UniGroupType          string `json:"uniGroupType,omitempty"`
	IsDeactivated         bool   `json:"isDeactivated,omitempty"`
	AllowMembersToBeShown bool   `json:"allowMembersToBeShown,omitempty"`
}

// LessonParticipant is a lesson participant.
type LessonParticipant struct {
	TeacherID       int64  `json:"teacherId,omitempty"`
	TeacherName     string `json:"teacherName,omitempty"`
	TeacherInitials string `json:"teacherInitials,omitempty"`
	ParticipantRole string `json:"participantRole,omitempty"`
}

// Lesson is a lesson for a calendar event.
type Lesson struct {
	LessonID        string              `json:"lessonId,omitempty"`
	LessonStatus    string              `json:"lessonStatus,omitempty"`
	Participants    []LessonParticipant `json:"participants,omitempty"`
	HasRelevantNote bool                `json:"hasRelevantNote,omitempty"`
}

// TimeSlotAnswer is a time slot answer for a calendar event.
type TimeSlotAnswer struct {
	ID                    int64 `json:"id,omitempty"`
	InstProfileID         int64 `json:"instProfileId,omitempty"`
	ConcerningProfileID   int64 `json:"concerningProfileId,omitempty"`
	SelectedTimeSlotIndex int   `json:"selectedTimeSlotIndex,omitempty"`
}

// TimeSlotIndex is a time slot index.
type TimeSlotIndex struct {
	StartTime string `json:"startTime,omitempty"`
	EndTime   string `json:"endTime,omitempty"`
}

// TimeSlotItem is a time slot item for a calendar event.
type TimeSlotItem struct {
	ID                int64            `json:"id,omitempty"`
	StartDate         string           `json:"startDate,omitempty"`
	EndDate           string           `json:"endDate,omitempty"`
	Answers           []TimeSlotAnswer `json:"answers,omitempty"`
	TimeSlotIndexes   []TimeSlotIndex  `json:"timeSlotIndexes,omitempty"`
	BelongsToResource any              `json:"belongsToResource,omitempty"`
}

// TimeSlot is a time slot for a calendar event.
type TimeSlot struct {
	TimeSlots     []TimeSlotItem `json:"timeSlots,omitempty"`
	ChildRequired bool           `json:"childRequired,omitempty"`
}

// CalendarEvent is a calendar event from Aula.
type CalendarEvent struct {
	CreatorInstProfileID         int64          `json:"creatorInstProfileId,omitempty"`
	CreatorProfileID             int64          `json:"creatorProfileId,omitempty"`
	CreatorName                  string         `json:"creatorName,omitempty"`
	InvitedGroups                []InvitedGroup `json:"invitedGroups,omitempty"`
	PrimaryResource              any            `json:"primaryResource,omitempty"`
	HasAttachments               bool           `json:"hasAttachments,omitempty"`
	CreatedDateTime              string         `json:"createdDateTime,omitempty"`
	Lesson                       *Lesson        `json:"lesson,omitempty"`
	TimeSlot                     *TimeSlot      `json:"timeSlot,omitempty"`
	VacationChildrenCountByDates any            `json:"vacationChildrenCountByDates,omitempty"`
	BelongsToProfiles            []int64        `json:"belongsToProfiles,omitempty"`
	BelongsToResources           []any          `json:"belongsToResources,omitempty"`
	RequiresNewAnswer            bool           `json:"requiresNewAnswer,omitempty"`
	DirectlyRelated              bool           `json:"directlyRelated,omitempty"`
	ResponseDeadline             string         `json:"responseDeadline,omitempty"`
	ResponseStatus               string         `json:"responseStatus,omitempty"`
	ID                           int64          `json:"id,omitempty"`
	Title                        string         `json:"title,omitempty"`
	AllDay                       bool           `json:"allDay,omitempty"`
	StartDateTime                string         `json:"startDateTime,omitempty"`
	EndDateTime                  string         `json:"endDateTime,omitempty"`
	OldEndDateTime               string         `json:"oldEndDateTime,omitempty"`
	OldStartDateTime             string         `json:"oldStartDateTime,omitempty"`
	OldAllDay                    bool           `json:"oldAllDay,omitempty"`
	ResponseRequired             bool           `json:"responseRequired,omitempty"`
	IsPrivate                    bool           `json:"isPrivate,omitempty"`
	Type                         string         `json:"type,omitempty"`
	PrimaryResourceText          string         `json:"primaryResourceText,omitempty"`
	AdditionalResources          []any          `json:"additionalResources,omitempty"`
	AdditionalResourceText       string         `json:"additionalResourceText,omitempty"`
	Repeating                    any            `json:"repeating,omitempty"`
	InstitutionCode              string         `json:"institutionCode,omitempty"`
	InstitutionName              string         `json:"institutionName,omitempty"`
	AddedToInstitutionCalendar   bool           `json:"addedToInstitutionCalendar,omitempty"`
}

// EventType returns the event type as an enum ("" when unknown).
func (e *CalendarEvent) EventType() CalendarEventType {
	t, _ := ParseCalendarEventType(e.Type)
	return t
}

// ==================== Response envelope ====================

// apiResponse is the envelope of Aula API responses: a data object with a status object.
type apiResponse struct {
	Status struct {
		Code    *int   `json:"code"`
		SubCode *int   `json:"subCode"`
		Message string `json:"message"`
	} `json:"status"`
	Data    json.RawMessage `json:"data"`
	Version int             `json:"version"`
	Module  string          `json:"module"`
	Method  string          `json:"method"`
}
