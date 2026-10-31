package aula

import (
	"strconv"
	"strings"
	"time"
)

// PagedResult wraps a page of paginated API results.
type PagedResult[T any] struct {
	// Items in this page.
	Items []T `json:"items"`
	// Page is the current page index (0-based).
	Page int `json:"page"`
	// PageSize is the number of items requested per page.
	PageSize int `json:"pageSize"`
	// TotalSize is the total number of items available, when known.
	TotalSize *int `json:"totalSize,omitempty"`
	// HasMore indicates whether more pages are available.
	HasMore bool `json:"hasMore"`
}

// NewPagedResult creates a PagedResult.
func NewPagedResult[T any](items []T, page, pageSize int, totalSize *int, hasMore bool) *PagedResult[T] {
	return &PagedResult[T]{
		Items:     items,
		Page:      page,
		PageSize:  pageSize,
		TotalSize: totalSize,
		HasMore:   hasMore,
	}
}

// NextPage returns the next page index, or -1 when there are no more pages.
func (p *PagedResult[T]) NextPage() int {
	if p.HasMore {
		return p.Page + 1
	}
	return -1
}

// IsFirstPage reports whether this is the first page.
func (p *PagedResult[T]) IsFirstPage() bool {
	return p.Page == 0
}

// IsLastPage reports whether this is the last page.
func (p *PagedResult[T]) IsLastPage() bool {
	return !p.HasMore
}

// ==================== MeeBook ====================

// MeeBook is a book in MeeBook.
type MeeBook struct {
	ID       int64  `json:"id,omitempty"`
	Title    string `json:"title,omitempty"`
	Category string `json:"category,omitempty"`
}

// MeeWeeklyBooks is the weekly books list from MeeBook.
type MeeWeeklyBooks struct {
	ID       int64     `json:"id,omitempty"`
	Name     string    `json:"name,omitempty"`
	Unilogin string    `json:"unilogin,omitempty"`
	Books    []MeeBook `json:"books,omitempty"`
}

// MeeBookConsolidatedInformation is consolidated MeeBook information
// (work plans + book lists).
type MeeBookConsolidatedInformation struct {
	WorkPlan []WeekPlanOverview `json:"workPlan,omitempty"`
	BookList []MeeWeeklyBooks   `json:"bookList,omitempty"`
}

// ==================== Week plans ====================

// WeekPlanDay is a day in a week plan. Date is a Danish date string like "tir 15. okt".
type WeekPlanDay struct {
	Date  string         `json:"date,omitempty"`
	Tasks []WeekPlanTask `json:"tasks,omitempty"`
}

var danishMonths = []string{"jan", "feb", "mar", "apr", "maj", "jun", "jul", "aug", "sep", "okt", "nov", "dec"}

// HasTasks reports whether the day has any tasks.
func (d *WeekPlanDay) HasTasks() bool {
	return len(d.Tasks) > 0
}

// FullDate resolves the partial Danish date (e.g. "tir 15. okt") to a full date in the
// current year.
func (d *WeekPlanDay) FullDate() time.Time {
	now := time.Now()
	monthIndex := -1
	for i, m := range danishMonths {
		if m == d.monthAbbreviation() {
			monthIndex = i
			break
		}
	}
	return time.Date(now.Year(), time.Month(monthIndex+1), d.dayNumber(), 0, 0, 0, 0, time.Local)
}

func (d *WeekPlanDay) dayNumber() int {
	parts := strings.Split(d.Date, " ")
	if len(parts) >= 2 {
		if n, err := strconv.Atoi(strings.ReplaceAll(parts[1], ".", "")); err == nil {
			return n
		}
	}
	return 1
}

func (d *WeekPlanDay) monthAbbreviation() string {
	parts := strings.Split(d.Date, " ")
	if len(parts) >= 3 {
		return strings.ReplaceAll(parts[2], ".", "")
	}
	return ""
}

// WeekPlanOverview is a week plan overview.
type WeekPlanOverview struct {
	ID       int64         `json:"id,omitempty"`
	Name     string        `json:"name,omitempty"`
	Unilogin string        `json:"unilogin,omitempty"`
	WeekPlan []WeekPlanDay `json:"weekPlan,omitempty"`
}

// IsEmptyOfTasks reports whether this week is completely empty of tasks.
func (w *WeekPlanOverview) IsEmptyOfTasks() bool {
	for i := range w.WeekPlan {
		if w.WeekPlan[i].HasTasks() {
			return false
		}
	}
	return true
}

// WeekStartDate returns the date of the first day in the week plan.
func (w *WeekPlanOverview) WeekStartDate() time.Time {
	if len(w.WeekPlan) == 0 {
		return time.Time{}
	}
	return w.WeekPlan[0].FullDate()
}

// WeekEndDate returns the date of the last day in the week plan.
func (w *WeekPlanOverview) WeekEndDate() time.Time {
	if len(w.WeekPlan) == 0 {
		return time.Time{}
	}
	return w.WeekPlan[len(w.WeekPlan)-1].FullDate()
}

// WeekPlanTask is a task in a week plan.
type WeekPlanTask struct {
	ID      int64  `json:"id,omitempty"`
	Type    string `json:"type,omitempty"`
	Group   string `json:"group,omitempty"`
	Pill    string `json:"pill,omitempty"`
	Content string `json:"content,omitempty"`
}

// PillTags returns the pill split into tags.
func (t *WeekPlanTask) PillTags() []string {
	if t.Pill == "" {
		return nil
	}
	return strings.Split(t.Pill, ", ")
}

// ContentStringFiltered returns the content lines joined with newlines.
func (t *WeekPlanTask) ContentStringFiltered() string {
	return strings.Join(t.ContentLines(), "\n")
}

// ContentLines returns the trimmed, non-empty content lines (lines that only contain
// dashes are considered empty).
func (t *WeekPlanTask) ContentLines() []string {
	if t.Content == "" {
		return nil
	}
	var lines []string
	for _, line := range strings.Split(t.Content, "\n") {
		line = strings.TrimSpace(line)
		if strings.ReplaceAll(line, "-", "") != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

// RelatedToATrip reports whether any of the content explicitly mentions a "tur"
// (trip in Danish).
func (t *WeekPlanTask) RelatedToATrip() bool {
	for _, line := range t.ContentLines() {
		if strings.Contains(strings.ToLower(line), " tur ") {
			return true
		}
	}
	return false
}
