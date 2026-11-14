package authkit_test

import (
	"errors"
	"sync"
	"testing"
	"time"

	"prestamos/authkit"
)

type stepClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *stepClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *stepClock) add(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

func TestTokenParse_ExpiredReturnsErrTokenExpired(t *testing.T) {
	clock := &stepClock{now: time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)}
	i, err := authkit.NewTokenIssuer("test-secret-test-secret-test-secret-1234", "test", time.Minute, clock)
	if err != nil {
		t.Fatal(err)
	}
	tok, _, err := i.Issue(&authkit.Account{UserID: "u1", TenantID: "t1"})
	if err != nil {
		t.Fatal(err)
	}
	clock.add(2 * time.Minute)

	if _, err := i.Parse(tok); !errors.Is(err, authkit.ErrTokenExpired) {
		t.Fatalf("esperaba ErrTokenExpired, obtuve %v", err)
	}
}

func TestTokenParse_TamperedReturnsErrInvalidToken(t *testing.T) {
	clock := &stepClock{now: time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)}
	i, err := authkit.NewTokenIssuer("test-secret-test-secret-test-secret-1234", "test", time.Minute, clock)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := i.Parse("no.es.un.jwt"); !errors.Is(err, authkit.ErrInvalidToken) {
		t.Fatalf("esperaba ErrInvalidToken, obtuve %v", err)
	}
}
