// Package middleware provides role-based access control for PocketBase routes.
package middleware

import (
	"net/http"
	"strings"

	"github.com/pocketbase/pocketbase/core"
)

// Role is the academy role stored on the auth record.
type Role string

const (
	RoleAdmin  Role = "admin"
	RoleManage Role = "manager"
	RoleCoach  Role = "coach"
	RoleScout  Role = "scout"
	RolePlayer Role = "player"
	RoleTrial  Role = "trial"
)

// RequireAuth returns a middleware that ensures the request has a valid PocketBase
// auth cookie and the user's role is in the allowed set.
func RequireAuth(allowed ...Role) func(*core.RequestEvent) error {
	return func(re *core.RequestEvent) error {
		authRecord := re.Auth
		if authRecord == nil {
			return re.NoContent(http.StatusUnauthorized)
		}
		role := Role(strings.ToLower(authRecord.GetString("role")))
		for _, r := range allowed {
			if role == r {
				return nil
			}
		}
		return re.NoContent(http.StatusForbidden)
	}
}

// CurrentRole extracts the caller's role from the auth record (or empty if anon).
func CurrentRole(re *core.RequestEvent) Role {
	authRecord := re.Auth
	if authRecord == nil {
		return ""
	}
	return Role(strings.ToLower(authRecord.GetString("role")))
}
