package aula

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// SessionProvider provides the Aula session ID (PHPSESSID) and persists a new one
// when Aula rotates it.
type SessionProvider interface {
	// SessionID returns the current Aula session ID, or "" when none is known.
	SessionID() (string, error)
	// SaveSessionID persists a new session ID (called when Aula rotates it).
	SaveSessionID(id string) error
}

// DefaultSessionFile returns the default session file path: ~/.aula/session.txt.
func DefaultSessionFile() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".aula", "session.txt")
	}
	return filepath.Join(home, ".aula", "session.txt")
}

// FileSessionProvider reads and writes the session ID from a file
// (by default ~/.aula/session.txt), with owner-only permissions.
type FileSessionProvider struct {
	// Path of the session file. Defaults to ~/.aula/session.txt.
	Path string
}

// NewFileSessionProvider creates a FileSessionProvider using ~/.aula/session.txt.
func NewFileSessionProvider() *FileSessionProvider {
	return &FileSessionProvider{Path: DefaultSessionFile()}
}

// SessionID reads the session ID from the session file. Returns "" when the file
// does not exist or is empty.
func (p *FileSessionProvider) SessionID() (string, error) {
	path := p.path()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		slog.Warn("Failed to read session file", "path", path, "error", err)
		return "", nil
	}
	return strings.TrimSpace(string(data)), nil
}

// SaveSessionID writes the session ID to the session file with owner-only permissions.
func (p *FileSessionProvider) SaveSessionID(id string) error {
	path := p.path()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		slog.Error("Failed to create session directory", "path", path, "error", err)
		return err
	}
	if err := os.WriteFile(path, []byte(id+"\n"), 0o600); err != nil {
		slog.Error("Failed to write session file", "path", path, "error", err)
		return err
	}
	slog.Info("Session ID saved", "path", path)
	return nil
}

func (p *FileSessionProvider) path() string {
	if p.Path != "" {
		return p.Path
	}
	return DefaultSessionFile()
}

// StaticSessionProvider returns a fixed session ID, e.g. from an environment variable:
//
//	aula.StaticSessionProvider{ID: os.Getenv("AULA_SESSION_ID")}
type StaticSessionProvider struct {
	// ID is the session ID to use.
	ID string
	// OnRotate is called (if set) when Aula rotates the session ID.
	OnRotate func(newID string)
}

// SessionID returns the static session ID.
func (p StaticSessionProvider) SessionID() (string, error) {
	return p.ID, nil
}

// SaveSessionID invokes OnRotate when set.
func (p StaticSessionProvider) SaveSessionID(id string) error {
	if p.OnRotate != nil {
		p.OnRotate(id)
	}
	return nil
}
