// Package ghapi provides GitHub API clients for nota sync operations.
package ghapi

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/cli/go-gh/v2/pkg/api"
)

// AuthError indicates missing or invalid GitHub credentials.
type AuthError struct {
	Host    string
	Message string
}

func (e *AuthError) Error() string {
	return fmt.Sprintf("GitHub authentication failed for %s: %s\nRun `gh auth login` or set GH_TOKEN", e.Host, e.Message)
}

// RateLimitError indicates the API rate limit was exceeded.
type RateLimitError struct {
	Limit     int
	Remaining int
	Reset     time.Time
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("GitHub API rate limit exceeded (limit=%d, remaining=%d, resets at %s)",
		e.Limit, e.Remaining, e.Reset.Format(time.RFC3339))
}

// NotFoundError indicates the requested resource was not found.
type NotFoundError struct {
	Resource string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("GitHub resource not found: %s", e.Resource)
}

// GraphQLError wraps errors from the GraphQL API, which can return errors even on HTTP 200.
type GraphQLError struct {
	Errors []struct {
		Message string   `json:"message"`
		Type    string   `json:"type"`
		Path    []string `json:"path"`
	}
}

func (e *GraphQLError) Error() string {
	if len(e.Errors) == 0 {
		return "unknown GraphQL error"
	}
	return fmt.Sprintf("GraphQL error: %s", e.Errors[0].Message)
}

// IsAuthError returns true if err is an authentication error.
func IsAuthError(err error) bool {
	var authErr *AuthError
	return errors.As(err, &authErr)
}

// IsRateLimit returns true if err is a rate limit error.
func IsRateLimit(err error) bool {
	var rlErr *RateLimitError
	return errors.As(err, &rlErr)
}

// IsNotFound returns true if err is a not found error.
func IsNotFound(err error) bool {
	var nfErr *NotFoundError
	return errors.As(err, &nfErr)
}

// ClassifyHTTPError converts an api.HTTPError into a typed error.
func ClassifyHTTPError(err error, resource string) error {
	var httpErr *api.HTTPError
	if !errors.As(err, &httpErr) {
		return err
	}

	switch httpErr.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		if isRateLimitResponse(httpErr) {
			return extractRateLimitError(httpErr)
		}
		return &AuthError{Message: httpErr.Message}
	case http.StatusNotFound:
		return &NotFoundError{Resource: resource}
	case http.StatusTooManyRequests:
		return extractRateLimitError(httpErr)
	default:
		return err
	}
}

// ClassifyGraphQLError checks for errors in a GraphQL response.
// GraphQL can return HTTP 200 with errors in the response body.
func ClassifyGraphQLError(err error) error {
	var gqlErr *api.GraphQLError
	if errors.As(err, &gqlErr) {
		wrapped := &GraphQLError{}
		for _, e := range gqlErr.Errors {
			var path []string
			for _, p := range e.Path {
				if s, ok := p.(string); ok {
					path = append(path, s)
				}
			}
			wrapped.Errors = append(wrapped.Errors, struct {
				Message string   `json:"message"`
				Type    string   `json:"type"`
				Path    []string `json:"path"`
			}{
				Message: e.Message,
				Type:    e.Type,
				Path:    path,
			})
		}
		return wrapped
	}
	return err
}

func isRateLimitResponse(err *api.HTTPError) bool {
	if err.Headers == nil {
		return false
	}
	remaining := err.Headers.Get("X-RateLimit-Remaining")
	return remaining == "0"
}

func extractRateLimitError(err *api.HTTPError) *RateLimitError {
	rle := &RateLimitError{}
	if err.Headers == nil {
		return rle
	}

	if v := err.Headers.Get("X-RateLimit-Limit"); v != "" {
		rle.Limit, _ = strconv.Atoi(v)
	}
	if v := err.Headers.Get("X-RateLimit-Remaining"); v != "" {
		rle.Remaining, _ = strconv.Atoi(v)
	}
	if v := err.Headers.Get("X-RateLimit-Reset"); v != "" {
		if ts, err := strconv.ParseInt(v, 10, 64); err == nil {
			rle.Reset = time.Unix(ts, 0)
		}
	}
	return rle
}
