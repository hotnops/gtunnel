package common

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"testing"

	"google.golang.org/grpc/metadata"
)

func TestNewToken(t *testing.T) {
	want := "UNITTEST"
	bad := ""
	got := NewToken(want)
	if got.token != want {
		t.Errorf("NewToken = %s; want %s", got.token, want)
	}

	got = NewToken(bad)
	if got != nil {
		t.Errorf("NewToken = %s; Needs to be nil", got)
	}

}

func TestGetRequestMetadata(t *testing.T) {
	ctx := context.Background()
	newToken := NewToken("UNITTEST")

	metadata, err := newToken.GetRequestMetadata(ctx, "")
	if err != nil {
		t.Errorf("GetRequestMetadata: GetRequestMetadata failed")
	}

	got := metadata["authorization"]
	want := "Bearer UNITTEST"
	if got != want {
		t.Errorf("GetRequestMetadata: Got: %s Want: %s", got, want)
	}

}

func handler(ctx context.Context, req interface{}) (interface{}, error) {
	return nil, nil
}

func TestAuthInterceptorValid(t *testing.T) {
	// Test if valid creds succeed
	ctx := context.Background()

	clientConfig := new(ClientConfig)
	clientConfig.ID = "UNITTESTID"
	clientConfig.IDHash = "UNITTESTHAST"

	authStore, err := InitializeAuthStore("")

	if err != nil {
		t.Errorf("[!] Failed to initialize auth store.")
	}

	token, _ := authStore.GenerateNewClientConfig("unittestid")
	t.Logf("[*] Generated token: %s\n", token)

	t.Run("ValidCreds", func(t *testing.T) {
		md := metadata.New(map[string]string{"authorization": bearerString + token})
		ctx = metadata.NewIncomingContext(ctx, md)
		_, err := UnaryAuthInterceptor(ctx, nil, nil, handler)

		if err != nil {
			t.Errorf("AuthInterceptor error: %s", err)
		}
	})
	t.Run("InvalidCreds", func(t *testing.T) {
		md := metadata.New(map[string]string{"authorization": bearerString + "BADTOKEN"})
		ctx = metadata.NewIncomingContext(ctx, md)
		_, err := UnaryAuthInterceptor(ctx, nil, nil, handler)

		if err == nil {
			t.Errorf("AuthInterceptor validated non-existent client")
		}
	})
}

func LoadFileToMap(filename string) (map[string]ClientConfig, error) {
	fileBytes, err := ioutil.ReadFile(filename)
	data := make(map[string]ClientConfig)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(fileBytes, &data)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func TestConfigurationFile(t *testing.T) {
	// This test will load a configuration file, ensure it has
	// the correct entries, add a new entry, and confirm that the entry
	// gets saved to file. It will then delete the entry and ensure
	// the entry no longer exists

	TestConfigFile := ".gtunnel.test.conf"
	authStore, err := InitializeAuthStore(TestConfigFile)
	if err != nil {
		t.Fatalf("[!] Failed to load test configuration")
	}
	config1, err := authStore.GetClientConfig("%YwnP94_kMDLT?#mj?CLYl=C7zu4E7A7")

	if err != nil {
		t.Errorf("Failed to lookup bearer token")
	}

	got := config1.ID
	want := "test1"

	if got != want {
		t.Errorf("GetClientConfig: Got: %s Want: %s\n", got, want)
	}

	newToken, err := authStore.GenerateNewClientConfig("newClient")

	if err != nil {
		t.Errorf("Failed to add new user")
	}

	data, err := LoadFileToMap(TestConfigFile)

	if err != nil {
		t.Errorf("Failed to load file: %s", err)
	}

	newConfig, ok := data[newToken]
	if !ok {
		t.Errorf("Failed to find new client in file")
	}

	if newConfig.ID != "newClient" {
		t.Errorf("ClientID lookup failed: Got: %s Want: newClient",
			newConfig.ID)
	}

	authStore.DeleteClientConfig(newToken)
	data, err = LoadFileToMap(TestConfigFile)

	if err != nil {
		t.Errorf("Failed to load file: %s", err)
	}

	newConfig, ok = data[newToken]
	if ok {
		t.Errorf("Found client that should have been deleted")
	}

}
