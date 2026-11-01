// Package login implements the interactive browser-based Aula login via
// MitID: it opens a visible Chrome/Chromium window, polls for the PHPSESSID
// cookie, handles Aula's "secure login" step-up through the UNI-Login SAML
// broker, and delivers QR code screenshots while the user has not completed
// the login.
//
// Two login methods are supported — the flow is identical, the user simply
// picks whichever matches their MitID setup:
//
//   - MitID app: scan the QR code shown in the browser window (or in the
//     screenshot delivered via Options.OnQR / Snapshot.QR) with the app on
//     the phone.
//   - MitID code reader: ignore the QR code and log in with username,
//     password and the physical code reader.
//
// The package backs both the goaula-login CLI and the goaula-mcp login tools.
package login

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"os"
	"sync"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

const (
	pollInterval   = 3 * time.Second
	pageSettleTime = 3 * time.Second // time for the MitID QR to render
	qrRefreshEvery = 4 * time.Minute // Aula/MitID QR codes expire after a few minutes

	defaultStartURL = "https://www.aula.dk/"
	defaultSettle   = 10 * time.Second
	defaultTimeout  = 10 * time.Minute

	stepUpURL    = "https://www.aula.dk/auth/stepup.php?stepupDone=true"
	stepUpWindow = 3 * time.Minute // how long to wait for the SAML step-up to complete

	// apiV25 is the API base URL used by the step-up assertion. Like the
	// client library's initial version detection, it starts at v25.
	apiV25 = "https://www.aula.dk/api/v25/"
)

// Status describes the state of a login flow.
type Status int

const (
	// StatusWaiting means login is still in progress. QR is non-nil when a
	// fresh QR code screenshot just became available.
	StatusWaiting Status = iota
	// StatusAuthenticated means the session was captured. The SteppedUp
	// field reports whether it carries the secure-login level; a basic
	// level token is also reported as authenticated (the step-up is
	// best-effort, since whether it can succeed depends on the user's
	// MitID specifics).
	StatusAuthenticated
	// StatusBrowserClosed means the user closed the browser before
	// completing the login.
	StatusBrowserClosed
)

// Snapshot is a state change of a login flow, delivered by Session.Poll.
type Snapshot struct {
	Status    Status
	SessionID string // set when Status == StatusAuthenticated
	SteppedUp bool   // StatusAuthenticated: whether the session has the secure-login level
	QR        []byte // PNG screenshot of a fresh QR code, if any
	Err       error  // set when the flow failed unrecoverably
}

// Options configures a login flow. Zero values select the defaults.
type Options struct {
	// StartURL is the Aula URL to open (default https://www.aula.dk/).
	StartURL string
	// Headless runs the browser without a visible window; the QR code is
	// then only available via the screenshots.
	Headless bool
	// BrowserPath overrides browser auto-detection.
	BrowserPath string
	// Settle is how long the session token must stay unchanged before it is
	// considered final (default 10s). Aula rotates the token once during
	// the secure-login step-up.
	Settle time.Duration
	// Timeout is the overall deadline of the login flow (default 10m).
	Timeout time.Duration
	// SkipStepUp skips the secure-login step-up (session will have the
	// basic authentication level).
	SkipStepUp bool
	// QRPath is an optional file to write each QR screenshot to.
	QRPath string
	// OnQR, when set, is called with each fresh QR screenshot (in addition
	// to the snapshot delivered via Poll).
	OnQR func(png []byte)
}

// sessionSteppedUpEnvelope mirrors the Aula API response envelope plus the
// profile fields read by the portal SPA: profiles.getProfileContext answers
// 448 ("secure login required") while the session still needs the step-up,
// and once stepped up it returns the profile with isSteppedUp=true.
type sessionSteppedUpEnvelope struct {
	Status struct {
		Code int `json:"code"`
	} `json:"status"`
	Data struct {
		IsSteppedUp bool `json:"isSteppedUp"`
	} `json:"data"`
}

// Session manages one interactive login flow. Create one with NewSession,
// start it with Start, and receive state changes with Poll. A Session can
// only be started once.
type Session struct {
	opts Options

	startOnce sync.Once
	cancel    context.CancelFunc // cancels the whole flow (and the browser)
	done      chan struct{}      // closed when the flow goroutine ends

	mu      sync.Mutex
	last    Snapshot // most recent snapshot (delivered or not)
	updates chan Snapshot
}

// NewSession creates a login flow with the given options.
func NewSession(opts Options) *Session {
	if opts.StartURL == "" {
		opts.StartURL = defaultStartURL
	}
	if opts.Settle <= 0 {
		opts.Settle = defaultSettle
	}
	if opts.Timeout <= 0 {
		opts.Timeout = defaultTimeout
	}
	return &Session{
		opts:    opts,
		updates: make(chan Snapshot, 32),
		done:    make(chan struct{}),
	}
}

// Start launches the browser and begins the login flow in a background
// goroutine. It is idempotent; use Poll to receive the results.
func (s *Session) Start() {
	s.startOnce.Do(func() {
		var ctx context.Context
		ctx, s.cancel = context.WithCancel(context.Background())
		go func() {
			defer close(s.done)
			s.run(ctx)
		}()
	})
}

// Poll waits up to timeout for the next state change of the login flow and
// returns it. When nothing changes within the timeout it returns
// StatusWaiting (the caller can simply call Poll again).
func (s *Session) Poll(timeout time.Duration) Snapshot {
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case snap := <-s.updates:
		return snap
	case <-s.done:
		// The flow ended; its final snapshot is the last one.
		s.mu.Lock()
		defer s.mu.Unlock()
		return s.last
	case <-timer.C:
		s.mu.Lock()
		defer s.mu.Unlock()
		if terminal(s.last) {
			return s.last
		}
		return Snapshot{Status: StatusWaiting}
	}
}

// Close cancels the login flow (if still running) and closes the browser.
func (s *Session) Close() {
	if s.cancel != nil {
		s.cancel()
	}
	<-s.done
}

// send records a snapshot and delivers it to a Poll caller (dropped when
// nobody is polling and the buffer is full; Poll falls back to last).
func (s *Session) send(snap Snapshot) {
	s.mu.Lock()
	s.last = snap
	s.mu.Unlock()

	select {
	case s.updates <- snap:
	default:
	}
}

func terminal(snap Snapshot) bool {
	return snap.Status == StatusAuthenticated ||
		snap.Status == StatusBrowserClosed ||
		snap.Err != nil
}

// run executes the login flow. It mirrors the Java/TS reference
// implementations, with the step-up handled explicitly.
func (s *Session) run(ctx context.Context) {
	opts := s.opts

	// DefaultExecAllocatorOptions includes Headless; add VisibleWindow for
	// a headed, maximized window.
	allocOpts := chromedp.DefaultExecAllocatorOptions[:]
	if !opts.Headless {
		allocOpts = append(allocOpts, chromedp.VisibleWindow)
	}
	exe, err := findBrowser(opts.BrowserPath)
	if err != nil {
		s.send(Snapshot{Err: err})
		return
	}
	slog.Debug("Using browser", "executable", exe)
	allocOpts = append(allocOpts, chromedp.ExecPath(exe))

	// The overall flow timeout and cancellation layer below the browser
	// context, so Session.Close kills the browser while the timeout guards
	// the flow.
	allocCtx, allocCancel := chromedp.NewExecAllocator(ctx, allocOpts...)
	defer allocCancel()
	browserCtx, browserCancel := chromedp.NewContext(allocCtx)
	defer browserCancel()
	flowCtx, flowCancel := context.WithTimeout(browserCtx, opts.Timeout)
	defer flowCancel()

	if err := chromedp.Do(flowCtx,
		chromedp.Navigate(opts.StartURL),
		chromedp.WaitVisible(chromedp.CSS("body")),
	); err != nil {
		s.send(Snapshot{Err: fmt.Errorf("navigating to %s: %w", opts.StartURL, err)})
		return
	}
	lastRefresh := time.Now()
	time.Sleep(pageSettleTime)
	s.screenshot(flowCtx)

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	// Only save a token that has remained unchanged for settle: Aula
	// rotates the PHPSESSID once during the secure-login step-up.
	var (
		lastToken      string
		settleStart    time.Time
		steppedUp      bool
		stepUpDeadline time.Time // non-zero while a step-up is in flight
	)

	// complete finishes the flow with the captured token, reporting whether
	// it ended up carrying the secure-login level. A basic token is a valid
	// outcome too — whether the step-up can succeed depends on the user's
	// MitID specifics, so the flow must not hang waiting for it.
	complete := func(token string) {
		secure := sessionSteppedUp(token)
		slog.Info("Session captured", "secureLogin", secure)
		s.send(Snapshot{
			Status:    StatusAuthenticated,
			SessionID: token,
			SteppedUp: secure,
		})
	}

	for {
		select {
		case <-browserCtx.Done():
			s.send(Snapshot{Status: StatusBrowserClosed})
			return
		case <-flowCtx.Done():
			if flowCtx.Err() == context.DeadlineExceeded {
				s.send(Snapshot{Err: fmt.Errorf("timed out after %s waiting for login", opts.Timeout)})
				return
			}
			s.send(Snapshot{Status: StatusBrowserClosed})
			return
		case <-ticker.C:
		}

		sess, ok, err := checkForSession(flowCtx)
		if err != nil {
			// The page may be mid-navigation; retry on the next tick.
			if browserCtx.Err() != nil {
				s.send(Snapshot{Status: StatusBrowserClosed})
				return
			}
			slog.Debug("Poll failed, retrying", "error", err)
			continue
		}
		if ok {
			if sess != lastToken {
				if lastToken == "" {
					slog.Info("Session token captured, watching briefly for the secure-login rotation",
						"window", opts.Settle)
				} else {
					slog.Info("Session token rotated, continuing to watch", "window", opts.Settle)
				}
				lastToken = sess
				settleStart = time.Now()
				continue
			}
			if time.Since(settleStart) < opts.Settle {
				continue
			}

			// Try the SAML step-up once if the session isn't already
			// stepped up — whether a plain login yields a privileged
			// token depends on the user's MitID specifics.
			if !opts.SkipStepUp && !steppedUp {
				steppedUp = true
				if sessionSteppedUp(sess) {
					slog.Info("Session already has the secure-login (step-up) level")
					complete(sess)
					return
				}
				slog.Info("Initiating secure-login step-up — an extra MitID approval may be required in the browser.")
				if err := chromedp.Do(flowCtx, chromedp.Navigate(stepUpURL)); err != nil {
					s.send(Snapshot{Err: fmt.Errorf("initiating step-up: %w", err)})
					return
				}
				stepUpDeadline = time.Now().Add(stepUpWindow)
				lastToken = "" // restart the settle logic for the upgraded token
				continue
			}

			// The token settled. If a step-up is in flight, give it time to
			// rotate the token — but complete with the basic token instead
			// of hanging when the window lapses without a privileged token.
			if !stepUpDeadline.IsZero() &&
				time.Now().Before(stepUpDeadline) &&
				!sessionSteppedUp(sess) {
				slog.Debug("Waiting for the secure-login step-up to complete")
				continue
			}

			complete(sess)
			return
		}

		// Re-load the login page if it has been open too long (stale QR).
		// Only while the user is still on the login page — never mid-MitID.
		if time.Since(lastRefresh) >= qrRefreshEvery {
			if loc, err := chromedp.Run(flowCtx, chromedp.Location()); err == nil && isLoginPage(loc) {
				slog.Info("Login page open for a while, reloading to refresh the QR code")
				if err := chromedp.Do(flowCtx, chromedp.Reload()); err == nil {
					lastRefresh = time.Now()
					time.Sleep(pageSettleTime)
					s.screenshot(flowCtx)
				}
			}
		}
	}
}

// checkForSession polls the browser for the Aula PHPSESSID cookie combined with
// a logged-in URL. It returns (sessionID, true) once the user has completed the
// login, and ("", false) while still waiting.
func checkForSession(ctx context.Context) (string, bool, error) {
	loc, err := chromedp.Run(ctx, chromedp.Location())
	if err != nil {
		return "", false, err
	}
	result, err := chromedp.Call(ctx, network.GetCookies, network.GetCookiesParams{})
	if err != nil {
		return "", false, err
	}

	var sess string
	for _, c := range result.Cookies {
		if c.Name == "PHPSESSID" {
			sess = c.Value
			break
		}
	}
	if sess == "" || !isLoggedIn(loc) {
		return "", false, nil
	}
	return sess, true, nil
}

// screenshot takes a full-page screenshot of the current browser window (the
// QR code area), delivers it via OnQR and as a QR snapshot. Quality 100 = PNG.
func (s *Session) screenshot(ctx context.Context) {
	if s.opts.QRPath == "" && s.opts.OnQR == nil {
		return
	}
	buf, err := chromedp.Run(ctx, chromedp.FullScreenshot(100))
	if err != nil {
		slog.Warn("Failed to capture QR screenshot", "error", err)
		return
	}
	if s.opts.QRPath != "" {
		if err := os.WriteFile(s.opts.QRPath, buf, 0o600); err != nil {
			slog.Warn("Failed to write QR screenshot", "path", s.opts.QRPath, "error", err)
		} else {
			fmt.Printf("QR screenshot written to %s — scan it with the MitID app\n", s.opts.QRPath)
		}
	}
	if s.opts.OnQR != nil {
		s.opts.OnQR(buf)
	}
	s.send(Snapshot{Status: StatusWaiting, QR: buf})
}

// sessionSteppedUp asserts whether the session already has the "secure login"
// (step-up) level — without touching the browser. Whether a plain login
// yields a stepped-up token depends on the user's MitID specifics, so this is
// checked before initiating the SAML step-up. Any failure to determine the
// state conservatively reports "not stepped up".
func sessionSteppedUp(sess string) bool {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return false
	}
	client := &http.Client{Timeout: 30 * time.Second, Jar: jar}

	// The first API response sets the Csrfp-Token cookie needed for
	// authenticated calls.
	bootstrap, err := http.NewRequest(http.MethodGet,
		apiV25+"?method=profiles.getProfilesByLogin", nil)
	if err != nil {
		return false
	}
	bootstrap.AddCookie(&http.Cookie{Name: "PHPSESSID", Value: sess})
	resp, err := client.Do(bootstrap)
	if err != nil {
		return false
	}
	resp.Body.Close()

	var csrf string
	for _, c := range resp.Cookies() {
		if c.Name == "Csrfp-Token" {
			csrf = c.Value
		}
	}

	req, err := http.NewRequest(http.MethodGet,
		apiV25+"?method=profiles.getProfileContext", nil)
	if err != nil {
		return false
	}
	req.AddCookie(&http.Cookie{Name: "PHPSESSID", Value: sess})
	req.Header.Set("Accept", "application/json, text/plain, */*")
	if csrf != "" {
		req.AddCookie(&http.Cookie{Name: "Csrfp-Token", Value: csrf})
		req.Header.Set("Csrfp-Token", csrf)
	}
	resp, err = client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	var env sessionSteppedUpEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return false
	}
	return env.Status.Code == 0 && env.Data.IsSteppedUp
}
