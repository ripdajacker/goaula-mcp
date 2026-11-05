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

// aulaCodeAccessDenied is the Aula status code observed when Aula answers
// HTTP 403 with this code and an empty message while rejecting an API call
// for a session token that may still be valid — e.g. transient CSRF-token
// or cookie state after a re-login/token rotation, or methods the session
// genuinely lacks permission for. Not to be confused with the HTTP status,
// which is also 403 in that reply.
const aulaCodeAccessDenied = 403

// AccessDeniedError is returned for that case: the call was rejected but the
// session is not necessarily invalid (that is InvalidSessionError, Aula code
// 448). Retrying may work; re-login (with the secure-login step-up) if the
// method keeps failing.
type AccessDeniedError struct {
	APIError
}

func (e *AccessDeniedError) Error() string { return e.Message }
