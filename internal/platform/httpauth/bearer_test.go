package httpauth_test

import (
	"testing"
	"time"

	"prestamos/internal/platform/httpauth"
)

func TestAuthenticate_BearerValidTokenPasses(t *testing.T) {
	h := newHarness(t)
	access, _ := h.session(t)

	r := h.call(t, "GET", "/api/v1/me", withBearer(access))
	expectStatus(t, r, 200)
}

func TestAuthenticate_BearerExpiredTokenNeverRenews(t *testing.T) {
	h := newHarness(t)
	access, refresh := h.session(t)
	h.clock.Advance(16 * time.Minute)

	// El refresh_token enviado como cookie no debe activar renovación con Bearer.
	r := h.call(t, "GET", "/api/v1/me", withBearer(access), withCookie(httpauth.RefreshCookie, refresh))
	expectStatus(t, r, 401)
	if code := r.errorCode(t); code != "TOKEN_INVALID" {
		t.Fatalf("código %s, esperaba TOKEN_INVALID", code)
	}
	if h.sessions.callCount() != 0 || len(r.cookies) != 0 {
		t.Fatal("bearer no debe renovar ni emitir cookies")
	}
}

func TestAuthenticate_BearerWithoutSchemeReturns401(t *testing.T) {
	h := newHarness(t)
	access, _ := h.session(t)

	r := h.call(t, "GET", "/api/v1/me", withHeader("Authorization", "Basic "+access))
	expectStatus(t, r, 401)
	if code := r.errorCode(t); code != "UNAUTHENTICATED" {
		t.Fatalf("código %s, esperaba UNAUTHENTICATED", code)
	}
}
