package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
)

type Cache struct {
	mu     sync.Mutex
	values map[string]interface{}
	client *redis.Client
}

func NewCache() (*Cache, error) {
	cache := &Cache{
		values: make(map[string]interface{}),
	}

	redisHost := os.Getenv("REDIS_HOST")
	redisPort := os.Getenv("REDIS_PORT")
	redisPassword := os.Getenv("REDIS_PASSWORD")

	if redisHost != "" && redisPort != "" {
		redisAddr := fmt.Sprintf("%s:%s", redisHost, redisPort)
		cache.client = redis.NewClient(&redis.Options{
			Addr:      redisAddr,
			Password:  redisPassword,
			DB:        0,
			TLSConfig: &tls.Config{},
		})

		if err := cache.client.Ping(cache.client.Context()).Err(); err != nil {
			return nil, err
		}
	}

	return cache, nil
}

func (c *Cache) Get(key string, value interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.client != nil {
		data, err := c.client.Get(c.client.Context(), key).Bytes()
		if err != nil {
			return err
		}
		if err := json.Unmarshal(data, value); err != nil {
			return err
		}
		return nil
	}

	if v, ok := c.values[key]; ok {
		data, err := json.Marshal(v)
		if err != nil {
			return err
		}
		if err := json.Unmarshal(data, value); err != nil {
			return err
		}
		return nil
	}

	return fmt.Errorf("key not found: %s", key)
}

func (c *Cache) Set(key string, value interface{}, expiration time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.client != nil {
		data, err := json.Marshal(value)
		if err != nil {
			return err
		}
		if err := c.client.Set(c.client.Context(), key, data, expiration).Err(); err != nil {
			return err
		}
		return nil
	}

	c.values[key] = value
	return nil
}

func (c *Cache) Close() error {
	if c.client != nil {
		return c.client.Close()
	}
	return nil
}
