package common

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"math/big"
	"os"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	bearerString = "Bearer "
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

// ClientConfig represents a structure containing all client
// configurations, such as the endpoint ID and system hash: TODO
type ClientConfig struct {
	ID     string
	IDHash string
}

// The AuthStore class is responsible for managing bearer token
// to client configuration relationships as well as persisting
// the datastore to disk.
type AuthStore struct {
	tokenToClient  map[string]*ClientConfig
	configFilename string
}

var authStore *AuthStore

// InitializeAuthStore will load in a configuration from the provided
// filename and return an AuthStore pointer. If the filename is empty,
// InitializeAuthStore will return a new, empty AuthStore.
func InitializeAuthStore(configFile string) (*AuthStore, error) {
	a := new(AuthStore)
	a.tokenToClient = make(map[string]*ClientConfig)
	// Setting a global variable so we can use it in the
	// interceptor function
	authStore = a

	if len(configFile) == 0 {
		return a, nil
	}

	a.configFilename = configFile

	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		log.Printf(
			"[*] No configuration file detected. Creating new file...\n")
		if _, err := os.Create(configFile); err != nil {
			log.Printf("[!] Failed to create configuration file\n")
			return nil, err
		}
		return a, nil
	}

	fileBytes, err := ioutil.ReadFile(configFile)
	if err != nil {
		log.Printf("[*] Failed to open %s\n", configFile)
		return nil, err
	}
	if len(fileBytes) == 0 {
		log.Printf("[*] Nothing in configuration file\n")
		return a, nil
	}

	err = json.Unmarshal(fileBytes, &a.tokenToClient)

	if err != nil {
		log.Printf("[!] Failed to load configuration file\n")
		return nil, err
	}

	log.Printf("[*] Successfully loaded configuration file")

	return a, nil
}

// generateToken will generate a random string to be
// used as a client authorization.
func (a *AuthStore) generateToken() (string, error) {
	// Minimum token size is 24, max is 36
	reader := rand.Reader
	max := big.NewInt(int64(MaxTokenSize - MinTokenSize))
	min := big.NewInt(int64(MinTokenSize))

	tokenSize, err := rand.Int(reader, max)

	if err != nil {
		log.Print("[!] Failed to generate a password size\n")
	}

	tokenSize.Add(tokenSize, min)

	var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*()-=_+[]{};,.<>/?")

	b := make([]rune, tokenSize.Int64())
	for i := range b {
		newRand, _ := rand.Int(reader, big.NewInt(int64(len(letters))))
		b[i] = letters[newRand.Int64()]
	}

	return string(b), nil
}

func (a *AuthStore) writeToFile() error {
	// Write dictionary to file
	jsonString, err := json.Marshal(a.tokenToClient)
	if err != nil {
		log.Printf("[!] Failed to convert auth dictionary to JSON")
		return err
	}

	err = ioutil.WriteFile(a.configFilename, jsonString, 600)
	if err != nil {
		log.Printf(
			"[!] Failed to write auth dictionary to configuration file")
		return err
	}
	return nil
}

// GenerateNewClientConfig will generate a new random token
// and insert it into the auth datastore. It returns the
// generated token on success and an error if one exists.
func (a *AuthStore) GenerateNewClientConfig(id string) (string, error) {
	token, err := a.generateToken()
	if err != nil {
		log.Printf("[!] Failed to generate new client config: %s", err)
		return "", err
	}
	clientConfig := new(ClientConfig)
	clientConfig.ID = id
	clientConfig.IDHash = ""
	err = a.AddClientConfig(token, clientConfig)
	if err != nil {
		log.Printf("[!] Failed to add client: %s", err)
		return "", err
	}

	return token, nil
}

// AddClientConfig will add a client configuration structure
// to the authToClient map with the provided token as the key
// Returns error
func (a *AuthStore) AddClientConfig(token string, client *ClientConfig) error {
	if _, ok := a.tokenToClient[token]; ok {
		log.Printf("[!] Client already exists.")
		return errors.New("Client already exists")
	}
	a.tokenToClient[token] = client
	err := a.writeToFile()
	if err != nil {
		log.Printf("[!] Failed to write configuration to file\n")
	}

	return nil
}

// DeleteClientConfig will delete a ClientConfig structure
// from the authToClient map with the provided key. Returns an
// error if the key doesn't exist or the delete failed.
func (a *AuthStore) DeleteClientConfig(token string) error {
	if _, ok := a.tokenToClient[token]; ok {
		delete(a.tokenToClient, token)

		err := a.writeToFile()
		if err != nil {
			log.Printf("[!] Failed to write configuration to file\n")
			return err
		}

		return nil
	}

	return errors.New("Client doesn't exist")
}

// GetClientConfig will return a pointer to a Clientconfig structure
// or an error if the lookup fails.
func (a *AuthStore) GetClientConfig(token string) (*ClientConfig, error) {
	if client, ok := a.tokenToClient[token]; ok {
		return client, nil
	}
	return nil, errors.New("Client does not exist")
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
func (t TokenAuth) GetRequestMetadata(ctx context.Context, in ...string) (
	map[string]string, error) {
	return map[string]string{
		"authorization": bearerString + t.token,
	}, nil
}

// RequireTransportSecurity always returns true
func (TokenAuth) RequireTransportSecurity() bool {
	return true
}

func (a *AuthStore) validateToken(token string) error {
	_, err := a.GetClientConfig(token)
	if err != nil {
		return err
	}
	return nil
}

// GetBearerTokenFromCtx will return the bearer token string
// from the auth header within the ctx structure.
func GetBearerTokenFromCtx(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)

	if !ok {
		return "", status.Error(codes.InvalidArgument, "Failed to retrieve metadata")
	}

	authHeader, ok := md["authorization"]
	if !ok {
		return "", status.Errorf(codes.Unauthenticated, "Authorization token is not provided")
	}

	bearerToken := authHeader[0]
	if !strings.HasPrefix(bearerToken, bearerString) {
		return "", status.Errorf(codes.InvalidArgument, "Invalid authorization header")
	}

	token := strings.Split(bearerToken, bearerString)[1]
	return token, nil
}

// UnaryAuthInterceptor is called for all unary gRPC functions
// to validate that the caller is authorized.
func UnaryAuthInterceptor(ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler) (interface{}, error) {

	token, err := GetBearerTokenFromCtx(ctx)

	if err != nil {
		return nil, err
	}

	err = authStore.validateToken(token)

	if err != nil {
		log.Printf("[!] Invalid bearer token\n")
		return nil, status.Errorf(codes.Unauthenticated, err.Error())
	}

	return handler(ctx, req)
}

// StreamAuthInterceptor will check for proper authorization for all
// stream based gRPC calls.
func StreamAuthInterceptor(srv interface{},
	ss grpc.ServerStream,
	info *grpc.StreamServerInfo,
	handler grpc.StreamHandler) error {

	ctx := ss.Context()

	token, err := GetBearerTokenFromCtx(ctx)

	if err != nil {
		return err
	}

	err = authStore.validateToken(token)

	if err != nil {
		log.Printf("[!] Invalid bearer token\n")
		return status.Errorf(codes.Unauthenticated, err.Error())
	}

	return handler(srv, ss)
}
