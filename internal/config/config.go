package config

import (
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	DatabaseURL, RedisURL, Address, WorkerAddress, StaticDir string
	EncryptionKey                                            []byte
	Workers                                                  int
	PollInterval, HTTPTimeout, LeaseDuration                 time.Duration
	AllowPrivate, CookieSecure                               bool
}

func Load() (Config, error) {
	c := Config{DatabaseURL: os.Getenv("DATABASE_URL"), RedisURL: env("REDIS_URL", "redis://localhost:6379/0"), Address: ":" + env("PORT", "8080"), WorkerAddress: ":" + env("WORKER_PORT", "8081"), StaticDir: env("STATIC_DIR", "frontend/dist")}
	var err error
	c.EncryptionKey, err = hex.DecodeString(os.Getenv("ENCRYPTION_KEY"))
	if err != nil || len(c.EncryptionKey) != 32 {
		return c, fmt.Errorf("ENCRYPTION_KEY must be 64 hexadecimal characters")
	}
	if c.DatabaseURL == "" {
		return c, fmt.Errorf("DATABASE_URL is required")
	}
	c.Workers, err = strconv.Atoi(env("WORKERS", "5"))
	if err != nil || c.Workers < 1 || c.Workers > 128 {
		return c, fmt.Errorf("WORKERS must be 1..128")
	}
	for _, item := range []struct {
		name, fallback string
		target         *time.Duration
	}{{"POLL_INTERVAL", "500ms", &c.PollInterval}, {"HTTP_TIMEOUT", "10s", &c.HTTPTimeout}, {"LEASE_DURATION", "60s", &c.LeaseDuration}} {
		*item.target, err = time.ParseDuration(env(item.name, item.fallback))
		if err != nil || *item.target <= 0 {
			return c, fmt.Errorf("invalid %s", item.name)
		}
	}
	if c.LeaseDuration < 3*c.HTTPTimeout {
		return c, fmt.Errorf("LEASE_DURATION must be at least three times HTTP_TIMEOUT")
	}
	c.AllowPrivate, err = strconv.ParseBool(env("ALLOW_PRIVATE_ENDPOINTS", "false"))
	if err != nil {
		return c, fmt.Errorf("invalid ALLOW_PRIVATE_ENDPOINTS")
	}
	c.CookieSecure, err = strconv.ParseBool(env("COOKIE_SECURE", "true"))
	if err != nil {
		return c, fmt.Errorf("invalid COOKIE_SECURE")
	}
	return c, nil
}
func env(k, fallback string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return fallback
}
