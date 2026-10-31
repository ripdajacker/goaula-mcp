package main

import (
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"

	"github.com/ripdajacker/aula-api-client-go"
)

// aulaSession holds the aula.Client for the MCP server and serializes access to it
// (the client and its cookie state are not thread-safe).
//
// The session token is assumed valid, exactly like the underlying client: login() reads
// it via the SessionProvider. If login fails, the MCP server still starts — tools then
// return an actionable error instead of the whole server refusing to boot.
type aulaSession struct {
	client *aula.Client
	mu     sync.Mutex
	ready  atomic.Bool
}

func newAulaSession(client *aula.Client) *aulaSession {
	return &aulaSession{client: client}
}

// login attempts to log in. On failure the server keeps running, but every tool call
// returns an actionable session error until the process is restarted with a valid
// session file.
func (s *aulaSession) login() {
	if err := s.client.Login(); err != nil {
		var invalidSession *aula.InvalidSessionError
		var apiErr *aula.APIError
		if errors.As(err, &invalidSession) || errors.As(err, &apiErr) {
			slog.Error(fmt.Sprintf("Could not log in to Aula. Is ~/.aula/session.txt present and valid? (%v)", err))
			return
		}
		panic(err)
	}
	s.ready.Store(true)
	slog.Info("Logged in to Aula", "apiVersion", s.client.ActiveAPIVersion())
}

// isReady reports whether login has succeeded and tools may talk to the API.
func (s *aulaSession) isReady() bool {
	return s.ready.Load()
}

// call runs a unit of work against the client, serialized through a lock. Returns an
// aula.InvalidSessionError when the server never managed to log in.
func (s *aulaSession) call(fn func(client *aula.Client) (any, error)) (any, error) {
	if !s.ready.Load() {
		return nil, aula.NewInvalidSessionError("The Aula session is invalid or expired. Refresh the session in " +
			"~/.aula/session.txt (for example by running the AulaSessionCapture login flow) and " +
			"restart the MCP server.")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return fn(s.client)
}

// close closes the underlying client, releasing HTTP resources.
func (s *aulaSession) close() {
	s.client.Close()
}
