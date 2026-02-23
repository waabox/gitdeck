// internal/domain/errors.go
package domain

import "errors"

// ErrUnauthorized is returned by providers when the API responds with HTTP 401.
// Callers can check for it using errors.Is to trigger token refresh or re-auth.
var ErrUnauthorized = errors.New("unauthorized")
