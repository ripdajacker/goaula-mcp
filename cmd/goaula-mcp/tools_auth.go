package main

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/ripdajacker/goaula-mcp"
	"github.com/ripdajacker/goaula-mcp/login"
)

// textResult builds a simple single-text tool result.
func textResult(text string) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}, nil, nil
}

// fail logs and returns a tool error.
func fail(text string) (*mcp.CallToolResult, any, error) {
	slog.Error(text)
	return nil, nil, errors.New(text)
}

// registerAuthTools registers the interactive login tools. They allow the MCP
// client to perform a full browser login (MitID QR or code reader) without
// the goaula-login binary, by opening a Chrome window on the machine the
// server runs on and polling for the captured session.
func registerAuthTools(server *mcp.Server, session *aulaSession) {
	var (
		mu   sync.Mutex
		flow *login.Session // login flow in progress, if any
	)

	imageResult := func(text string, png []byte) (*mcp.CallToolResult, any, error) {
		return &mcp.CallToolResult{Content: []mcp.Content{
			&mcp.TextContent{Text: text},
			&mcp.ImageContent{Data: png, MIMEType: "image/png"},
		}}, nil, nil
	}
	clearFlow := func() {
		mu.Lock()
		flow = nil
		mu.Unlock()
	}

	// ── aula_login ─────────────────────────────────────────────────────
	mcp.AddTool(server, &mcp.Tool{
		Name:        "aula_login",
		Title:       "Log in to Aula",
		Description: "Start Aula authentication. If a valid session exists, confirms it. Otherwise opens a Chrome window on the server machine with the Aula/MitID login page and returns a screenshot of the QR code for you to scan with the MitID app. A physical MitID code reader works too: the user then logs in with username, password and reader in the browser window.",
		Annotations: &mcp.ToolAnnotations{Title: "Log in to Aula"},
	}, func(ctx context.Context, req *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
		// First, try the existing session.
		if session.isReady() {
			if err := session.ping(); err == nil {
				return textResult("Already authenticated. You can now use any Aula data tools.")
			}
		}

		mu.Lock()
		if flow != nil {
			mu.Unlock()
			return textResult("A login flow is already in progress. Call aula_check_auth immediately — do NOT wait for the user to message you.")
		}
		f := login.NewSession(login.Options{})
		f.Start()
		flow = f
		mu.Unlock()

		// Give the browser a moment to open and the QR to render.
		snap := f.Poll(15 * time.Second)
		switch snap.Status {
		case login.StatusAuthenticated:
			clearFlow()
			defer f.Close()
			return finishLogin(session, snap.SessionID)
		case login.StatusBrowserClosed:
			clearFlow()
			return fail("The authentication browser window was closed. Call aula_login to start a new authentication flow.")
		case login.StatusWaiting:
			if snap.Err != nil {
				clearFlow()
				return fail("Failed to start the login flow: " + snap.Err.Error())
			}
			if snap.QR != nil {
				return imageResult("A Chrome window has opened with the Aula/MitID login page. "+
					"The user needs to complete a few manual steps (click login, enter username, then scan the QR code with MitID — or use a physical MitID code reader). "+
					"IMPORTANT: Immediately call aula_check_auth now WITHOUT waiting for the user to message you. "+
					"The user will be busy in the browser. Keep calling aula_check_auth until it reports that authentication is complete.",
					snap.QR)
			}
			return textResult("The login browser is opening. IMPORTANT: Call aula_check_auth immediately — do NOT wait for the user to message you.")
		}
		return fail("Unexpected login state.")
	})

	// ── aula_check_auth ────────────────────────────────────────────────
	mcp.AddTool(server, &mcp.Tool{
		Name:        "aula_check_auth",
		Title:       "Check Aula login status",
		Description: "Check if the MitID login was completed after aula_login. May return a refreshed QR screenshot if the previous one expired.",
		Annotations: &mcp.ToolAnnotations{Title: "Check Aula login status"},
	}, func(ctx context.Context, req *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
		mu.Lock()
		f := flow
		mu.Unlock()
		if f == nil {
			return fail("No authentication flow in progress. Call aula_login first.")
		}

		snap := f.Poll(90 * time.Second)
		switch snap.Status {
		case login.StatusAuthenticated:
			clearFlow()
			defer f.Close()
			return finishLogin(session, snap.SessionID)
		case login.StatusBrowserClosed:
			clearFlow()
			return fail("The authentication browser window was closed. Call aula_login to start a new authentication flow.")
		case login.StatusWaiting:
			if snap.Err != nil {
				clearFlow()
				return fail("Login flow failed: " + snap.Err.Error())
			}
			if snap.QR != nil {
				return imageResult("The QR code was refreshed (the previous one expired). "+
					"IMPORTANT: Call aula_check_auth again immediately — do NOT wait for a user message.",
					snap.QR)
			}
			return textResult("Still waiting for the user to complete authentication in the browser. " +
				"IMPORTANT: Call aula_check_auth again immediately — do NOT wait for a user message. " +
				"The user is busy with the MitID flow in the Chrome window.")
		}
		return fail("Unexpected login state.")
	})

	// ── aula_session_status ────────────────────────────────────────────
	mcp.AddTool(server, &mcp.Tool{
		Name:        "aula_session_status",
		Title:       "Aula session status",
		Description: "Check if the current Aula session is valid without starting an auth flow.",
		Annotations: &mcp.ToolAnnotations{Title: "Aula session status", ReadOnlyHint: true},
	}, func(ctx context.Context, req *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
		if !session.isReady() {
			return textResult(`{"authenticated": false, "reason": "No valid session. Call aula_login."}`)
		}
		if err := session.ping(); err != nil {
			return textResult(`{"authenticated": false, "reason": "Session expired (ping failed). Call aula_login."}`)
		}
		return textResult(`{"authenticated": true}`)
	})
}

// finishLogin saves a captured session ID and swaps the server's client over
// to it.
func finishLogin(session *aulaSession, sessionID string) (*mcp.CallToolResult, any, error) {
	provider, ok := session.sessionProvider()
	if !ok {
		return nil, nil, errors.New("cannot determine the session provider for saving the captured session")
	}
	if err := provider.SaveSessionID(sessionID); err != nil {
		return nil, nil, errors.New("authentication successful, but saving the session failed: " + err.Error())
	}
	if err := session.relogin(); err != nil {
		return textResult("Session captured and stored, but logging in with it failed: " +
			err.Error() + ". The session may still be valid — call aula_session_status to check.")
	}
	slog.Info("Interactive login completed and client re-initialized")
	return textResult("Authentication successful! Session captured, stored and verified. You can now use any Aula data tools.")
}

// sessionProvider exposes the session provider (used by the login tools to
// persist captured sessions).
func (s *aulaSession) sessionProvider() (aula.SessionProvider, bool) {
	if s.config == nil || s.config.Session == nil {
		return nil, false
	}
	return s.config.Session, true
}
