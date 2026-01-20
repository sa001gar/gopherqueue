// Package security provides authentication and authorization for GopherQueue.
package security

import (
	"context"
	"crypto/subtle"
	"net/http"
	"strings"
	"time"
)

// Principal represents an authenticated entity.
type Principal struct {
	ID        string            `json:"id"`
	Type      PrincipalType     `json:"type"`
	Name      string            `json:"name"`
	Roles     []string          `json:"roles"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	ExpiresAt time.Time         `json:"expires_at,omitempty"`
}

// PrincipalType represents the type of principal.
type PrincipalType string

const (
	PrincipalTypeUser    PrincipalType = "user"
	PrincipalTypeService PrincipalType = "service"
	PrincipalTypeSystem  PrincipalType = "system"
)

// Role defines a set of permissions.
type Role struct {
	Name        string       `json:"name"`
	Permissions []Permission `json:"permissions"`
}

// Permission represents an allowed action.
type Permission struct {
	Resource string `json:"resource"`
	Action   string `json:"action"`
}

// Common permissions.
var (
	PermissionJobSubmit = Permission{Resource: "jobs", Action: "submit"}
	PermissionJobRead   = Permission{Resource: "jobs", Action: "read"}
	PermissionJobCancel = Permission{Resource: "jobs", Action: "cancel"}
	PermissionJobRetry  = Permission{Resource: "jobs", Action: "retry"}
	PermissionJobDelete = Permission{Resource: "jobs", Action: "delete"}
	PermissionStatsRead = Permission{Resource: "stats", Action: "read"}
	PermissionAdminAll  = Permission{Resource: "*", Action: "*"}
)

// Authenticator verifies identity.
type Authenticator interface {
	// Authenticate verifies credentials and returns a principal.
	Authenticate(ctx context.Context, credentials Credentials) (*Principal, error)

	// AuthenticateRequest extracts and verifies credentials from an HTTP request.
	AuthenticateRequest(r *http.Request) (*Principal, error)
}

// Credentials represents authentication credentials.
type Credentials struct {
	Type     CredentialType `json:"type"`
	APIKey   string         `json:"api_key,omitempty"`
	Token    string         `json:"token,omitempty"`
	Username string         `json:"username,omitempty"`
	Password string         `json:"password,omitempty"`
}

// CredentialType represents the type of credentials.
type CredentialType string

const (
	CredentialTypeAPIKey CredentialType = "api_key"
	CredentialTypeToken  CredentialType = "token"
	CredentialTypeBasic  CredentialType = "basic"
)

// Authorizer checks permissions.
type Authorizer interface {
	// Authorize checks if a principal has the required permission.
	Authorize(ctx context.Context, principal *Principal, permission Permission) (bool, error)

	// AuthorizeRequest creates middleware that checks permissions.
	AuthorizeRequest(permission Permission) func(http.Handler) http.Handler
}

// APIKeyAuthenticator is a simple API key authenticator.
type APIKeyAuthenticator struct {
	keys map[string]*Principal
}

// NewAPIKeyAuthenticator creates a new API key authenticator.
func NewAPIKeyAuthenticator(keys map[string]*Principal) *APIKeyAuthenticator {
	return &APIKeyAuthenticator{keys: keys}
}

// Authenticate verifies an API key.
func (a *APIKeyAuthenticator) Authenticate(ctx context.Context, credentials Credentials) (*Principal, error) {
	if credentials.Type != CredentialTypeAPIKey {
		return nil, ErrInvalidCredentials
	}

	for key, principal := range a.keys {
		if subtle.ConstantTimeCompare([]byte(credentials.APIKey), []byte(key)) == 1 {
			return principal, nil
		}
	}

	return nil, ErrInvalidCredentials
}

// AuthenticateRequest extracts and verifies credentials from an HTTP request.
func (a *APIKeyAuthenticator) AuthenticateRequest(r *http.Request) (*Principal, error) {
	// Check header
	apiKey := r.Header.Get("X-API-Key")
	if apiKey == "" {
		// Check Authorization header
		auth := r.Header.Get("Authorization")
		if strings.HasPrefix(auth, "Bearer ") {
			apiKey = strings.TrimPrefix(auth, "Bearer ")
		}
	}

	if apiKey == "" {
		return nil, ErrMissingCredentials
	}

	return a.Authenticate(r.Context(), Credentials{
		Type:   CredentialTypeAPIKey,
		APIKey: apiKey,
	})
}

// SimpleAuthorizer is a basic role-based authorizer.
type SimpleAuthorizer struct {
	roles map[string]*Role
}

// NewSimpleAuthorizer creates a new authorizer.
func NewSimpleAuthorizer() *SimpleAuthorizer {
	return &SimpleAuthorizer{
		roles: map[string]*Role{
			"admin": {
				Name:        "admin",
				Permissions: []Permission{PermissionAdminAll},
			},
			"operator": {
				Name: "operator",
				Permissions: []Permission{
					PermissionJobSubmit,
					PermissionJobRead,
					PermissionJobCancel,
					PermissionJobRetry,
					PermissionStatsRead,
				},
			},
			"viewer": {
				Name: "viewer",
				Permissions: []Permission{
					PermissionJobRead,
					PermissionStatsRead,
				},
			},
		},
	}
}

// Authorize checks if a principal has the required permission.
func (a *SimpleAuthorizer) Authorize(ctx context.Context, principal *Principal, permission Permission) (bool, error) {
	if principal == nil {
		return false, nil
	}

	for _, roleName := range principal.Roles {
		role, exists := a.roles[roleName]
		if !exists {
			continue
		}

		for _, perm := range role.Permissions {
			if a.matchPermission(perm, permission) {
				return true, nil
			}
		}
	}

	return false, nil
}

// matchPermission checks if a permission matches the required permission.
func (a *SimpleAuthorizer) matchPermission(has, required Permission) bool {
	if has.Resource == "*" && has.Action == "*" {
		return true
	}
	if has.Resource != required.Resource && has.Resource != "*" {
		return false
	}
	if has.Action != required.Action && has.Action != "*" {
		return false
	}
	return true
}

// AuthorizeRequest creates middleware that checks permissions.
func (a *SimpleAuthorizer) AuthorizeRequest(permission Permission) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal := GetPrincipal(r.Context())
			if principal == nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			allowed, _ := a.Authorize(r.Context(), principal, permission)
			if !allowed {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// Context keys.
type contextKey string

const principalKey contextKey = "principal"

// WithPrincipal adds a principal to the context.
func WithPrincipal(ctx context.Context, principal *Principal) context.Context {
	return context.WithValue(ctx, principalKey, principal)
}

// GetPrincipal retrieves the principal from the context.
func GetPrincipal(ctx context.Context) *Principal {
	principal, _ := ctx.Value(principalKey).(*Principal)
	return principal
}

// Errors.
var (
	ErrInvalidCredentials = &AuthError{Message: "invalid credentials"}
	ErrMissingCredentials = &AuthError{Message: "missing credentials"}
	ErrUnauthorized       = &AuthError{Message: "unauthorized"}
	ErrForbidden          = &AuthError{Message: "forbidden"}
)

// AuthError represents an authentication/authorization error.
type AuthError struct {
	Message string
}

func (e *AuthError) Error() string {
	return e.Message
}

// Ensure implementations match interfaces.
var _ Authenticator = (*APIKeyAuthenticator)(nil)
var _ Authorizer = (*SimpleAuthorizer)(nil)
