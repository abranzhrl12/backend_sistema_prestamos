package httpauth_test

import (
	"context"
	"fmt"
	"sync"
	"time"

	"prestamos/authkit"
)

// fakeClock es un reloj controlable para avanzar el tiempo sin dormir.
type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func newFakeClock() *fakeClock {
	return &fakeClock{now: time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)}
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// fakeSessions implementa httpauth.Refresher con la misma semántica de rotación que
// authkit.Service: cada refresh consume el token anterior y emite uno nuevo. Si gate no es
// nil, cada Refresh espera a que se cierre (para forzar peticiones concurrentes).
type fakeSessions struct {
	mu     sync.Mutex
	issuer *authkit.TokenIssuer
	clock  authkit.Clock
	live   map[string]*authkit.Account
	seq    int
	calls  int
	gate   chan struct{}
}

func newFakeSessions(issuer *authkit.TokenIssuer, clock authkit.Clock) *fakeSessions {
	return &fakeSessions{issuer: issuer, clock: clock, live: make(map[string]*authkit.Account)}
}

// seed registra un refresh token válido para la cuenta, como tras un login.
func (f *fakeSessions) seed(acc *authkit.Account) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.issueRefreshLocked(acc)
}

// revoke invalida un refresh token (equivale a usuario eliminado o sesión revocada).
func (f *fakeSessions) revoke(token string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.live, token)
}

func (f *fakeSessions) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

func (f *fakeSessions) setGate(g chan struct{}) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.gate = g
}

func (f *fakeSessions) issueRefreshLocked(acc *authkit.Account) string {
	f.seq++
	tok := fmt.Sprintf("%s.refresh-%d", acc.TenantID, f.seq)
	f.live[tok] = acc
	return tok
}

func (f *fakeSessions) Refresh(_ context.Context, refreshToken string) (*authkit.TokenPair, *authkit.Account, error) {
	f.mu.Lock()
	f.calls++
	gate := f.gate
	f.mu.Unlock()
	if gate != nil {
		<-gate
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	acc, ok := f.live[refreshToken]
	if !ok {
		return nil, nil, authkit.ErrInvalidRefresh
	}
	delete(f.live, refreshToken)
	access, exp, err := f.issuer.Issue(acc)
	if err != nil {
		return nil, nil, err
	}
	return &authkit.TokenPair{
		AccessToken: access, AccessExpiresAt: exp,
		RefreshToken: f.issueRefreshLocked(acc), RefreshExpiresAt: f.clock.Now().Add(time.Hour),
	}, acc, nil
}
