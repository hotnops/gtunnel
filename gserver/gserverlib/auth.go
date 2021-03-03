package gserverlib

import (
	"context"
	"strings"

	"github.com/hotnops/gTunnel/common"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

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
	if !strings.HasPrefix(bearerToken, common.BearerString) {
		return "", "", status.Errorf(codes.InvalidArgument, "Invalid authorization header")
	}

	auth := strings.Split(bearerToken, common.BearerString)[1]
	token := strings.Split(auth, "-")[0]
	uuid := strings.Split(auth, "-")[1]
	return token, uuid, nil
}
