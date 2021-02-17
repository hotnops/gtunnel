package gserverlib

import (
	"context"
	"encoding/json"
	"log"
	"sync"

	"github.com/go-redis/redis/v8"
)

// ConfigStore is a structure that represents all of the configurations of
// the gServer and will keep state to a json file
type ConfigStore struct {
	// This map keeps all of the configured clients in a store that
	// uses their bearer token as a key for easy auth lookup
	configuredClients map[string]*ConfiguredClient

	// The filename where the configuration will save changes and load
	// on start
	redisClient *redis.Client
	context     context.Context
	mutex       sync.Mutex
}

func NewConfigStore() *ConfigStore {
	configStore := new(ConfigStore)
	configStore.context = context.Background()
	configStore.redisClient = redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})

	configStore.configuredClients = make(map[string]*ConfiguredClient)

	return configStore
}

// AddConfiguredClient will take a ConfiguredClient structure and
// add it to the redis datastore
func (c *ConfigStore) AddConfiguredClient(client *ConfiguredClient) error {

	clientJSON, err := json.Marshal(client)

	if err != nil {
		log.Printf("[!] Failed to convert configured client into json")
		return err
	}

	// Make sure that our operations are atmoic
	c.mutex.Lock()
	defer c.mutex.Unlock()

	err = c.redisClient.Set(c.context, client.Token, clientJSON, 0).Err()
	if err != nil {
		log.Printf("[!] Failed to insert configured client into redis database")
		return err
	}

	c.configuredClients[client.Token] = client

	return nil
}

func (c *ConfigStore) GetConfiguredClient(key string) *ConfiguredClient {
	client, ok := c.configuredClients[key]
	if !ok {
		return nil
	}
	return client
}

func (c *ConfigStore) DeleteConfiguredClient(key string) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	err := c.redisClient.Del(c.context, key).Err()
	if err != nil {
		log.Printf("[!] Failed to delete configured client")
		return err
	}

	delete(c.configuredClients, key)

	return nil
}

func (c *ConfigStore) Initialize() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	keys, err := c.redisClient.Keys(c.context, "*").Result()

	if err != nil {
		log.Printf("[!] Failed to initialize configuration store")
		return err
	}

	for _, key := range keys {
		clientConfig := new(ConfiguredClient)

		value, err := c.redisClient.Get(c.context, key).Result()

		err = json.Unmarshal([]byte(value), clientConfig)
		if err != nil {
			log.Printf("[!] Failed to load configured client")
			continue
		}

		c.configuredClients[key] = clientConfig
	}

	return nil
}
