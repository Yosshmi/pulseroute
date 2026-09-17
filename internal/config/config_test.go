package config

import (
	"strings"
	"testing"
)

func TestConfiguration(t *testing.T) {
	t.Setenv("ALLOW_PRIVATE_ENDPOINTS", "")
	t.Setenv("COOKIE_SECURE", "")
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("ENCRYPTION_KEY", strings.Repeat("ab", 32))
	t.Setenv("WORKERS", "5")
	t.Setenv("HTTP_TIMEOUT", "10s")
	t.Setenv("LEASE_DURATION", "60s")
	c, e := Load()
	if e != nil || c.Workers != 5 || !c.CookieSecure || c.AllowPrivate {
		t.Fatal(c, e)
	}
	t.Setenv("WORKERS", "0")
	if _, e = Load(); e == nil {
		t.Fatal("unbounded worker setting")
	}
	t.Setenv("WORKERS", "5")
	t.Setenv("LEASE_DURATION", "2s")
	if _, e = Load(); e == nil {
		t.Fatal("unsafe lease accepted")
	}
}
