package gserverlib

import (
	"context"
	"crypto/rand"
	"log"
	"math/big"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

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

// GetClientInfoFromCtx will return the bearer token string
// from the auth header within the ctx structure.
func GetClientInfoFromCtx(ctx context.Context) (string, string, error) {
	md, ok := metadata.FromIncomingContext(ctx)

	if !ok {
		return "", "", status.Error(codes.InvalidArgument, "Failed to retrieve metadata")
	}

	authHeader, ok := md["authorization"]
	if !ok {
		return "", "", status.Errorf(codes.Unauthenticated, "Authorization token is not provided")
	}

	bearerToken := authHeader[0]
	if !strings.HasPrefix(bearerToken, bearerString) {
		return "", "", status.Errorf(codes.InvalidArgument, "Invalid authorization header")
	}

	auth := strings.Split(bearerToken, bearerString)[1]
	token := strings.Split(auth, "-")[0]
	uuid := strings.Split(auth, "-")[1]
	return token, uuid, nil
}
