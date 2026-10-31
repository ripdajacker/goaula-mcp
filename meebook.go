package aula

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"
)

const (
	meebookBaseURL   = "https://app.meebook.com/aulaapi/"
	meebookUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:109.0) Gecko/20100101 Firefox/112.0"

	// WeeklyBookWidgetID is the widget ID for the weekly book.
	WeeklyBookWidgetID = "0003"
	// WeeklyPlanWidgetID is the widget ID for the weekly plan.
	WeeklyPlanWidgetID = "0004"
)

// MeeBookClient is a client for the MeeBook API (separate from Aula). Used to get
// weekly book and plan information.
type MeeBookClient struct {
	httpClient *http.Client

	childFilterCodes             []string
	institutionFilterIDs         []string
	weeklyBookAuthorizationToken string
	weeklyPlanAuthorizationToken string
}

// NewMeeBookClient creates a MeeBook client. The authorization tokens are obtained via
// the Aula client's aulaToken.getAulaToken endpoint (see MeeBookInformation).
func NewMeeBookClient(childFilterCodes, institutionFilterIDs []string, weeklyBookAuthorizationToken, weeklyPlanAuthorizationToken string) *MeeBookClient {
	return &MeeBookClient{
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		childFilterCodes:             childFilterCodes,
		institutionFilterIDs:         institutionFilterIDs,
		weeklyBookAuthorizationToken: weeklyBookAuthorizationToken,
		weeklyPlanAuthorizationToken: weeklyPlanAuthorizationToken,
	}
}

// WorkPlanForWeeks gets work plans for the current week plus additional future weeks.
func (c *MeeBookClient) WorkPlanForWeeks(numberOfAdditionalFutureWeeks int) []WeekPlanOverview {
	var result []WeekPlanOverview
	now := time.Now()
	year, currentWeekNumber := now.ISOWeek()

	for i := 0; i <= numberOfAdditionalFutureWeeks; i++ {
		weekNumber := currentWeekNumber + i
		// Handle year rollover
		if weekNumber > 52 {
			year++
			weekNumber -= 52
		}
		plan := c.RelatedWorkPlan(year, weekNumber)
		if plan != nil {
			result = append(result, *plan)
		}
	}
	return result
}

// RelatedWorkPlan gets a specific week's work plan. Returns nil on error or when empty.
func (c *MeeBookClient) RelatedWorkPlan(year, weekNumber int) *WeekPlanOverview {
	url := meebookBaseURL + "relatedweekplan/all?currentWeekNumber=" + weekParam(year, weekNumber) +
		"&userProfile=guardian&" + c.childFilter() + "&" + c.institutionFilter()

	response, err := c.callMeeBookAPI(url, c.weeklyPlanAuthorizationToken)
	if err != nil {
		slog.Error("Error getting work plan", "year", year, "week", weekNumber, "error", err)
		return nil
	}
	var list []WeekPlanOverview
	if err := unmarshalJSON([]byte(response), &list); err != nil {
		slog.Error("Error parsing work plan", "year", year, "week", weekNumber, "error", err)
		return nil
	}
	if len(list) == 0 {
		return nil
	}
	return &list[0]
}

// BookListForWeeks gets book lists for the current week plus additional future weeks.
func (c *MeeBookClient) BookListForWeeks(numberOfAdditionalFutureWeeks int) []MeeWeeklyBooks {
	var result []MeeWeeklyBooks
	now := time.Now()
	year, currentWeekNumber := now.ISOWeek()

	for i := 0; i <= numberOfAdditionalFutureWeeks; i++ {
		weekNumber := currentWeekNumber + i
		if weekNumber > 52 {
			year++
			weekNumber -= 52
		}
		books := c.RelatedWeeklyBook(year, weekNumber)
		if books != nil {
			result = append(result, *books)
		}
	}
	return result
}

// RelatedWeeklyBook gets a specific week's book list. Returns nil on error or when empty.
func (c *MeeBookClient) RelatedWeeklyBook(year, weekNumber int) *MeeWeeklyBooks {
	url := meebookBaseURL + "relatedweeklybook/all?currentWeekNumber=" + weekParam(year, weekNumber) +
		"&userProfile=guardian&" + c.childFilter() + "&" + c.institutionFilter()

	response, err := c.callMeeBookAPI(url, c.weeklyBookAuthorizationToken)
	if err != nil {
		slog.Error("Error getting weekly book", "year", year, "week", weekNumber, "error", err)
		return nil
	}
	var list []MeeWeeklyBooks
	if err := unmarshalJSON([]byte(response), &list); err != nil {
		slog.Error("Error parsing weekly book", "year", year, "week", weekNumber, "error", err)
		return nil
	}
	if len(list) == 0 {
		return nil
	}
	return &list[0]
}

func weekParam(year, weekNumber int) string {
	return strconv.Itoa(year) + "-W" + strconv.Itoa(weekNumber)
}

func (c *MeeBookClient) childFilter() string {
	result := ""
	for i, code := range c.childFilterCodes {
		if i > 0 {
			result += "&"
		}
		result += "childFilter[]=" + code
	}
	return result
}

func (c *MeeBookClient) institutionFilter() string {
	result := ""
	for i, id := range c.institutionFilterIDs {
		if i > 0 {
			result += "&"
		}
		result += "institutionFilter[]=" + id
	}
	return result
}

// callMeeBookAPI makes an HTTP GET request to the MeeBook API. A 403 or 410 status
// yields an empty list ("[]").
func (c *MeeBookClient) callMeeBookAPI(reqURL, authorizationToken string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return "", fmt.Errorf("MeeBook API call failed: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Authorization", "Bearer "+authorizationToken)
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Origin", "https://www.aula.dk")
	req.Header.Set("Referer", "https://www.aula.dk/")
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	req.Header.Set("User-Agent", meebookUserAgent)
	req.Header.Set("X-Version", "1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("MeeBook API call failed: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf("MeeBook API call failed: %w", err)
		}
		return string(body), nil
	case http.StatusForbidden, http.StatusGone:
		slog.Warn("MeeBook API returned status", "status", resp.StatusCode, "url", reqURL)
		return "[]", nil
	default:
		return "", fmt.Errorf("MeeBook API connection failed with status %d", resp.StatusCode)
	}
}

// Close closes the HTTP client.
func (c *MeeBookClient) Close() {
	c.httpClient.CloseIdleConnections()
}
