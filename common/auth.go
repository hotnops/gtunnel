package common

import (
	"context"
	"crypto/rand"
	"log"
	"math/big"
)

const (
	// BearerString is the string used to designate an auth token
	BearerString = "Bearer "
	// MaxTokenSize is the maximum number of characters for a bearer token
	MaxTokenSize = 48
	// MinTokenSize is the minimum number of characters for a bearer token
	MinTokenSize = 32
)

// TokenAuth is a structure repesenting a bearer token for
// client auth
type TokenAuth struct {
	token string
}

// GenerateToken will generate a random string to be
// used as a client authorization.
func GenerateToken() (string, error) {
	// Minimum token size is 24, max is 36
	reader := rand.Reader
	max := big.NewInt(int64(MaxTokenSize - MinTokenSize))
	min := big.NewInt(int64(MinTokenSize))

	tokenSize, err := rand.Int(reader, max)

	if err != nil {
		log.Print("[!] Failed to generate a password size\n")
	}

	tokenSize.Add(tokenSize, min)

	var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789@#$%^&*()=_+[]{};,.<>/?")

	b := make([]rune, tokenSize.Int64())
	for i := range b {
		newRand, _ := rand.Int(reader, big.NewInt(int64(len(letters))))
		b[i] = letters[newRand.Int64()]
	}

	return string(b), nil
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
