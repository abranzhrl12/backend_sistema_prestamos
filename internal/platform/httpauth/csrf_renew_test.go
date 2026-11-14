package httpauth_test

import (
	"testing"
	"time"

	"prestamos/internal/platform/httpauth"
)

func TestAuthenticate_MutationWithoutCSRFDoesNotRenew(t *testing.T) {
	h := newHarness(t)
	access, refresh := h.session(t)
	h.clock.Advance(16 * time.Minute)

	r := h.call(t, "POST", "/api/v1/items",
		withCookie(httpauth.AccessCookie, access), withCookie(httpauth.RefreshCookie, refresh))
	expectStatus(t, r, 403)
	if code := r.errorCode(t); code != "CSRF_INVALID" {
		t.Fatalf("código %s, esperaba CSRF_INVALID", code)
	}
	if h.sessions.callCount() != 0 || r.cookie(httpauth.AccessCookie, "/") != nil {
		t.Fatal("una mutación sin CSRF válido no debe renovar la sesión")
	}
}

func TestAuthenticate_MutationWithMismatchedCSRFDoesNotRenew(t *testing.T) {
	h := newHarness(t)
	access, refresh := h.session(t)
	h.clock.Advance(16 * time.Minute)

	r := h.call(t, "POST", "/api/v1/items",
		withCookie(httpauth.AccessCookie, access), withCookie(httpauth.RefreshCookie, refresh),
		withCookie(httpauth.CSRFCookie, "uno"), withHeader(httpauth.CSRFHeader, "otro"))
	expectStatus(t, r, 403)
	if h.sessions.callCount() != 0 {
		t.Fatal("CSRF que no coincide no debe renovar la sesión")
	}
}

func TestAuthenticate_MutationWithCSRFRenewsAndKeepsCSRFCheck(t *testing.T) {
	h := newHarness(t)
	access, refresh := h.session(t)
	h.clock.Advance(16 * time.Minute)
	const csrf = "token-csrf-de-prueba"

	r := h.call(t, "POST", "/api/v1/items",
		withCookie(httpauth.AccessCookie, access), withCookie(httpauth.RefreshCookie, refresh),
		withCookie(httpauth.CSRFCookie, csrf), withHeader(httpauth.CSRFHeader, csrf))
	expectStatus(t, r, 200)
	if h.sessions.callCount() != 1 {
		t.Fatalf("esperaba 1 rotación, hubo %d", h.sessions.callCount())
	}
	newCSRF := r.cookie(httpauth.CSRFCookie, "/")
	if newCSRF == nil || newCSRF.Value == "" || newCSRF.Value == csrf {
		t.Fatal("tras renovar el cliente debe recibir un csrf_token nuevo")
	}
}
