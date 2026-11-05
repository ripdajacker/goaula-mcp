package aula

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	// DefaultAPIURL is the default base URL of the Aula API.
	DefaultAPIURL = "https://www.aula.dk/api/"

	defaultUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:109.0) Gecko/20100101 Firefox/112.0"

	// expectedAPIVersion is the API version the client starts probing from.
	expectedAPIVersion = 22

	maxAPIVersionTries = 5
)

// connector is the HTTP connector for the Aula API. It handles cookies, session
// management, API version detection, and PHPSESSID rotation.
type connector struct {
	httpClient *http.Client
	cookies    *cookieManager
	session    SessionProvider

	activeAPIVersion int
	baseAPIURL       string
	versionedAPIURL  string

	lastUsedSessionID string
	providerSessionID string // last session ID seen from the provider
}

func newConnector(session SessionProvider) *connector {
	return &connector{
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		cookies:          newCookieManager(),
		session:          session,
		activeAPIVersion: expectedAPIVersion,
		baseAPIURL:       DefaultAPIURL,
		versionedAPIURL:  "https://www.aula.dk/api/v25",
	}
}

// initializeAPIUse initializes the API connection by finding a working API version,
// probing baseURL + "v<N>/" starting at the expected version.
func (c *connector) initializeAPIUse(baseAPIURL string) (string, error) {
	c.baseAPIURL = baseAPIURL

	tryAPIVer := c.activeAPIVersion
	var workingVersionedAPIURL string
	success := false

	for versionTries := 1; !success; versionTries++ {
		if versionTries > maxAPIVersionTries {
			return "", NewAPIError("Failed to find a working API version")
		}

		workingVersionedAPIURL = baseAPIURL + "v" + strconv.Itoa(tryAPIVer) + "/"

		resp, err := c.makeRequest(workingVersionedAPIURL+"?method=profiles.getProfilesByLogin", http.MethodGet, nil)
		if err != nil {
			if apiErr, ok := err.(*APIError); ok {
				return "", apiErr
			}
			return "", invalidSessionf("Unexpected error in API Client calling the Aula API: %v", err)
		}

		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()

		status := resp.StatusCode
		switch {
		case status == http.StatusGone || status == http.StatusForbidden:
			// Wrong version, try next
			tryAPIVer++
			continue
		case status == http.StatusOK:
			// Process cookies from response
			c.processResponseCookies(resp)

			var js any
			if readErr != nil {
				return "", invalidSessionf("Unexpected error in API Client calling the Aula API: %v", readErr)
			}
			if jsonErr := json.Unmarshal(body, &js); jsonErr != nil {
				return "", invalidSessionf("Unexpected error in API Client calling the Aula API: %v", jsonErr)
			}
			success = true
		default:
			var parsed struct {
				Status struct {
					Code    *int   `json:"code"`
					SubCode *int   `json:"subCode"`
					Message string `json:"message"`
				} `json:"status"`
			}
			_ = json.Unmarshal(body, &parsed)
			return "", &InvalidSessionError{APIError{
				Message:        "Unexpected error in API Client calling the Aula API, on subsequent initial success",
				HTTPStatus:     status,
				AulaStatusCode: parsed.Status.Code,
				AulaSubCode:    parsed.Status.SubCode,
				AulaMessage:    parsed.Status.Message,
			}}
		}
	}

	c.versionedAPIURL = workingVersionedAPIURL
	c.activeAPIVersion = tryAPIVer
	slog.Info("API usage verified", "endpoint", c.versionedAPIURL)

	return workingVersionedAPIURL, nil
}

// callAulaAPI calls an Aula API method (e.g. "profiles.getProfilesByLogin") and returns
// the raw JSON response body.
func (c *connector) callAulaAPI(aulaMethod, httpMethod string, postData any) (json.RawMessage, error) {
	reqURL := c.versionedAPIURL + "?method=" + aulaMethod

	var jsonBody []byte
	if strings.EqualFold(httpMethod, "post") && postData != nil {
		var err error
		jsonBody, err = MarshalJSON(postData)
		if err != nil {
			return nil, NewAPIError("Failed to serialize request body: " + err.Error())
		}
	}

	attempt := func() (*http.Response, error) {
		if strings.EqualFold(httpMethod, "post") {
			return c.makeRequest(reqURL, http.MethodPost, jsonBody)
		}
		return c.makeRequest(reqURL, http.MethodGet, nil)
	}

	resp, err := attempt()
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Handle PHPSESSID rotation (must run before the cookie merge below, since it compares
	// against the current cookie value)
	c.handlePHPSESSIDRotation(resp)

	// Refresh the cookie store from Set-Cookie headers. Aula rotates cookies like
	// Csrfp-Token over time ("Do not cache this token for reuse") — only merging them on
	// actual PHPSESSID rotation would leave every subsequent request sending a stale CSRF
	// token, which Aula rejects with a 403 + HTML error page.
	c.processResponseCookies(resp)

	respBody, readErr := io.ReadAll(resp.Body)
	resp.Body.Close()

	// A denied call may be a stale CSRF state rather than a real refusal: Aula
	// answers those with HTTP 403 ("Invalid CSRF Token" HTML page or the bare
	// JSON status code 403). A cheap authenticated GET re-establishes a fresh
	// CSRF token cookie, and the exact request is retried once. Callers keep
	// seeing the denial error when the retry is rejected as well.
	if resp.StatusCode == http.StatusForbidden && deniedResponseBody(respBody) {
		slog.Debug("403 from Aula — refreshing the CSRF state and retrying once", "method", aulaMethod)
		if refreshErr := c.refreshCSRFToken(); refreshErr != nil {
			slog.Debug("CSRF refresh failed, reporting the denial", "error", refreshErr)
		} else {
			resp2, err2 := attempt()
			if err2 == nil {
				b2, bErr := io.ReadAll(resp2.Body)
				resp2.Body.Close()
				// Process the retry's cookies even on failure, so the state
				// stays fresh for the calls after this one.
				c.handlePHPSESSIDRotation(resp2)
				c.processResponseCookies(resp2)
				if bErr == nil {
					resp, respBody, readErr = resp2, b2, nil
				}
			} else {
				slog.Debug("CSRF retry failed", "error", err2)
			}
		}
	}

	status := resp.StatusCode

	switch {
	case status == http.StatusForbidden:
		var parsed struct {
			Status struct {
				Code    *int   `json:"code"`
				SubCode *int   `json:"subCode"`
				Message string `json:"message"`
			} `json:"status"`
		}
		if jsonErr := json.Unmarshal(respBody, &parsed); jsonErr != nil {
			// Aula can answer 403 with an HTML error page (e.g. "<h2>Invalid CSRF Token</h2>")
			// instead of JSON. Report that verbatim instead of a cryptic JSON parse error.
			snippet := strings.TrimSpace(string(respBody))
			if len(snippet) > 200 {
				snippet = snippet[:200] + "..."
			}
			return nil, invalidSessionf("Access denied while calling API method '%s', Aula returned a non-JSON error response: %s", aulaMethod, snippet)
		}

		if parsed.Status.Code != nil {
			switch *parsed.Status.Code {
			case aulaCodeAccessDenied:
				// The token may still be valid (unlike the invalid-session
				// code 448): a retry often suffices.
				return nil, &AccessDeniedError{APIError{
					Message:        fmt.Sprintf("Access denied while calling API method '%s' (Aula code 403, empty message) even after a CSRF refresh and one automatic retry: retry later, and re-login (secure login step-up) if it keeps failing.", aulaMethod),
					HTTPStatus:     status,
					AulaStatusCode: parsed.Status.Code,
					AulaSubCode:    parsed.Status.SubCode,
					AulaMessage:    parsed.Status.Message,
				}}
			}
			sessionID := c.lastUsedSessionID
			if sessionID == "" {
				sessionID = "unknown"
			}
			message := strings.TrimSpace(fmt.Sprintf("Invalid or expired session while calling API method '%s'. %s. [SessionId: %s]",
				aulaMethod, parsed.Status.Message, sessionID))
			return nil, &InvalidSessionError{APIError{
				Message:        message,
				HTTPStatus:     status,
				AulaStatusCode: parsed.Status.Code,
				AulaSubCode:    parsed.Status.SubCode,
				AulaMessage:    parsed.Status.Message,
			}}
		}
		return nil, invalidSessionf("Access denied while calling API method '%s'", aulaMethod)
	case status == http.StatusNotFound:
		return nil, NewAPIError("Method not found: " + aulaMethod)
	case status != http.StatusOK:
		return nil, NewAPIError(fmt.Sprintf("API connection failed with status %d", status))
	}

	if readErr != nil {
		return nil, NewAPIError("API connection failed: " + readErr.Error())
	}
	return json.RawMessage(respBody), nil
}

// deniedResponseBody reports whether a 403 response body looks like a
// recoverable CSRF/denial state: the "Invalid CSRF Token" HTML error page or
// the bare JSON status envelope with Aula code 403 and an empty message.
func deniedResponseBody(body []byte) bool {
	if bytes.Contains(bytes.ToLower(body), []byte("invalid csrf token")) {
		return true
	}
	var status struct {
		Status struct {
			Code *int `json:"code"`
		} `json:"status"`
	}
	if err := json.Unmarshal(body, &status); err != nil {
		return false
	}
	return status.Status.Code != nil && *status.Status.Code == aulaCodeAccessDenied
}

// refreshCSRFToken re-establishes a fresh CSRF token cookie with a cheap
// authenticated GET. Aula's Csrfp-Token can go stale ("Do not cache this
// token for reuse"), which answers calls with HTTP 403 / "Invalid CSRF Token".
func (c *connector) refreshCSRFToken() error {
	resp, err := c.makeRequest(c.versionedAPIURL+"?method=profiles.getProfilesByLogin", http.MethodGet, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		return err
	}
	// Handle PHPSESSID rotation (must run before the cookie merge below, since
	// it compares against the current cookie value)
	c.handlePHPSESSIDRotation(resp)
	c.processResponseCookies(resp)
	return nil
}

// makeRequest makes an HTTP request with cookie injection.
func (c *connector) makeRequest(reqURL, method string, jsonBody []byte) (*http.Response, error) {
	u, err := url.Parse(reqURL)
	if err != nil {
		return nil, NewAPIError("Failed to make HTTP request: " + err.Error())
	}
	host := u.Hostname()

	var body io.Reader
	if jsonBody != nil {
		body = bytes.NewReader(jsonBody)
	}
	req, err := http.NewRequest(method, reqURL, body)
	if err != nil {
		return nil, NewAPIError("Failed to make HTTP request: " + err.Error())
	}

	// CSRF token
	const csrfpTokenName = "Csrfp-Token"
	if c.cookies.hasCookie(host, csrfpTokenName) {
		req.Header.Set(csrfpTokenName, c.cookies.cookieValue(host, csrfpTokenName))
	}

	// Session ID: consult the provider on every request — the read is cheap
	// (a small file or an in-memory provider). A provider value that differs
	// from the previous one is adopted immediately, so an externally
	// refreshed session file (e.g. another goaula-login run) is picked up on
	// the next request without a restart. An unchanged value keeps the
	// current cookie, which may be a fresher, Aula-rotated ID that the
	// rotation handler stored and the provider failed to persist.
	sessionID, serr := c.session.SessionID()
	if serr != nil {
		slog.Warn("Failed to read session ID", "error", serr)
	} else if sessionID != "" {
		if sessionID != c.providerSessionID {
			slog.Debug("Adopting session ID from the session provider",
				"id", truncate(sessionID, 10), "first", c.providerSessionID == "")
			c.cookies.addCookieValue(host, "PHPSESSID", sessionID)
		}
		c.providerSessionID = sessionID
		c.lastUsedSessionID = c.cookies.cookieValue(host, "PHPSESSID")
	}

	// Profile change counter for www.aula.dk
	if c.cookies.hasEntries(host) {
		if host == "www.aula.dk" {
			if v := c.cookies.cookieValue(host, "profile_change"); v != "" {
				if n, perr := strconv.Atoi(strings.TrimSpace(v)); perr == nil {
					c.cookies.addCookieValue(host, "profile_change", strconv.Itoa(n+1))
				} else {
					c.cookies.addCookieValue(host, "profile_change", "10")
				}
			} else {
				c.cookies.addCookieValue(host, "profile_change", "10")
			}
		}

		req.Header.Set("Cookie", c.cookies.allCookiesString(host))
	}

	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("User-Agent", defaultUserAgent)

	if method == http.MethodPost && jsonBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, NewAPIError("Failed to make HTTP request: " + err.Error())
	}
	return resp, nil
}

// processResponseCookies processes Set-Cookie headers from a response.
func (c *connector) processResponseCookies(resp *http.Response) {
	setCookies := resp.Header.Values("Set-Cookie")
	if len(setCookies) == 0 {
		return
	}
	c.cookies.addCookieHeaderArray(hostForResponse(resp), setCookies)
}

// hostForResponse determines the host to store response cookies under: the request's
// host, falling back to the default Aula host.
func hostForResponse(resp *http.Response) string {
	if resp.Request != nil && resp.Request.URL != nil && resp.Request.URL.Hostname() != "" {
		return resp.Request.URL.Hostname()
	}
	return "www.aula.dk"
}

// handlePHPSESSIDRotation detects when Aula returns a new PHPSESSID and persists it.
func (c *connector) handlePHPSESSIDRotation(resp *http.Response) {
	setCookieHeaders := resp.Header.Values("Set-Cookie")
	if len(setCookieHeaders) == 0 {
		return
	}

	newPHPSESSID := extractPHPSESSID(setCookieHeaders)
	if newPHPSESSID == "" {
		return
	}

	host := hostForResponse(resp)
	currentPHPSESSID := c.cookies.cookieValue(host, "PHPSESSID")

	if currentPHPSESSID != "" && newPHPSESSID != currentPHPSESSID {
		slog.Info("PHPSESSID rotation detected",
			"from", truncate(currentPHPSESSID, 10), "to", truncate(newPHPSESSID, 10))
		if err := c.session.SaveSessionID(newPHPSESSID); err != nil {
			slog.Error("Failed to persist rotated session ID", "error", err)
		}
		c.cookies.addCookieHeaderArray(host, setCookieHeaders)
	} else if currentPHPSESSID == "" {
		c.cookies.addCookieHeaderArray(host, setCookieHeaders)
	}
}

// extractPHPSESSID extracts the PHPSESSID value from Set-Cookie headers.
func extractPHPSESSID(setCookieHeaders []string) string {
	for _, cookieHeader := range setCookieHeaders {
		if strings.HasPrefix(cookieHeader, "PHPSESSID=") {
			value := cookieHeader[len("PHPSESSID="):]
			if semicolonIndex := strings.Index(value, ";"); semicolonIndex != -1 {
				value = value[:semicolonIndex]
			}
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// close releases idle HTTP connections.
func (c *connector) close() {
	c.httpClient.CloseIdleConnections()
}
