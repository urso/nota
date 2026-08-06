package ghapi

import (
	"github.com/cli/go-gh/v2/pkg/auth"
)

const defaultHost = "github.com"

// CheckAuth verifies that credentials are available for the given host.
// Returns nil if authenticated, or an AuthError with actionable guidance.
func CheckAuth(host string) error {
	if host == "" {
		host = defaultHost
	}

	token, _ := auth.TokenForHost(host)
	if token == "" {
		return &AuthError{
			Host:    host,
			Message: "no token found",
		}
	}
	return nil
}

// TokenForHost returns the authentication token for the given host.
// Returns empty string if no token is available.
func TokenForHost(host string) string {
	if host == "" {
		host = defaultHost
	}
	token, _ := auth.TokenForHost(host)
	return token
}
