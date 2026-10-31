package aula

import "fmt"

// APIError is the base error for all Aula API errors.
type APIError struct {
	// Message is the human-readable error message.
	Message string
	// HTTPStatus is the HTTP status code, or 0 when not applicable.
	HTTPStatus int
	// AulaStatusCode is Aula's internal status code, when provided.
	AulaStatusCode *int
	// AulaSubCode is Aula's internal sub-code, when provided.
	AulaSubCode *int
	// AulaMessage is Aula's own error message, when provided.
	AulaMessage string
}

func (e *APIError) Error() string {
	return e.Message
}

// NewAPIError creates a new APIError with just a message.
func NewAPIError(message string) *APIError {
	return &APIError{Message: message}
}

// InvalidSessionError is returned when the session ID (PHPSESSID) is no longer valid.
//
// This occurs when Aula returns HTTP 403 with an internal status code of 448 (or similar).
type InvalidSessionError struct {
	APIError
}

const defaultInvalidSessionMessage = "Session ID is no longer valid. The PHPSESSID has expired or is invalid. Please re-authenticate."

// NewInvalidSessionError creates a new InvalidSessionError with the given message, or a
// default message when msg is empty.
func NewInvalidSessionError(msg string) *InvalidSessionError {
	if msg == "" {
		msg = defaultInvalidSessionMessage
	}
	return &InvalidSessionError{APIError{Message: msg}}
}

func invalidSessionf(format string, args ...any) *InvalidSessionError {
	return &InvalidSessionError{APIError{Message: fmt.Sprintf(format, args...)}}
}
