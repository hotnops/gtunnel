package common

import (
	"context"
)

// TokenAuth is a structure repesenting a bearer token for
// client auth
type TokenAuth struct {
	token string
}

// NewToken will generate a new TokenAuth structure
// with the provided string set to the token member.
func NewToken(token string) *TokenAuth {
	t := new(TokenAuth)
	if token == "" {
		return nil
	}
	t.token = token
	return t
}

// GetRequestMetadata will return the bearer token as an
// authorization header
func (t *TokenAuth) GetRequestMetadata(ctx context.Context, in ...string) (
	map[string]string, error) {
	return map[string]string{
		"authorization": "Bearer " + t.token,
	}, nil
}

// RequireTransportSecurity always returns true
func (t *TokenAuth) RequireTransportSecurity() bool {
	return true
}
