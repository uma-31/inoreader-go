package inoreader

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
)

var (
	// ErrBadRequest reports that mandatory parameters are missing.
	ErrBadRequest = errors.New("inoreader: bad request")

	// ErrUnauthorized reports that the user is not authorized, typically
	// because the session expired. Sign the user in again.
	ErrUnauthorized = errors.New("inoreader: unauthorized")

	// ErrForbidden reports wrong app credentials, or a method that is not
	// implemented.
	ErrForbidden = errors.New("inoreader: forbidden")

	// ErrRateLimited reports that the daily limit of the method's zone is
	// reached. It resets after [RateLimit.ResetAfter], so retrying sooner
	// does not help.
	ErrRateLimited = errors.New("inoreader: daily rate limit reached")

	// ErrServiceUnavailable reports that the service is unavailable.
	ErrServiceUnavailable = errors.New("inoreader: service unavailable")

	// ErrRedirect reports that a response redirected the request, which the
	// client refuses so that the credentials are not sent elsewhere. It
	// happens, for instance, with a base URL of http:// instead of https://.
	ErrRedirect = errors.New("inoreader: redirect refused")
)

// APIError is an error response from the API. Match it against the Err
// variables of this package with [errors.Is].
type APIError struct {
	// StatusCode is the HTTP status code.
	StatusCode int

	// Message is the message from the response body, without the `Error=`
	// prefix. It can be shown to the user.
	Message string

	// RateLimit is the rate limit reported by the response, if any.
	RateLimit *RateLimit
}

func newAPIError(status int, body []byte, rl *RateLimit) *APIError {
	msg := strings.TrimSpace(string(body))
	msg = strings.TrimPrefix(msg, "Error=")
	return &APIError{StatusCode: status, Message: msg, RateLimit: rl}
}

func (e *APIError) Error() string {
	if e.Message == "" {
		return fmt.Sprintf("inoreader: %d %s", e.StatusCode, http.StatusText(e.StatusCode))
	}
	return fmt.Sprintf("inoreader: %d %s: %s", e.StatusCode, http.StatusText(e.StatusCode), e.Message)
}

// Is reports whether target is the Err variable matching the status code.
func (e *APIError) Is(target error) bool {
	switch e.StatusCode {
	case http.StatusBadRequest:
		return target == ErrBadRequest
	case http.StatusUnauthorized:
		return target == ErrUnauthorized
	case http.StatusForbidden:
		return target == ErrForbidden
	case http.StatusTooManyRequests:
		return target == ErrRateLimited
	case http.StatusServiceUnavailable:
		return target == ErrServiceUnavailable
	default:
		return false
	}
}
