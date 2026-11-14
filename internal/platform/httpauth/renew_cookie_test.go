package httpauth_test

import (
	"testing"
	"time"

	"prestamos/internal/platform/httpauth"
)

func TestAuthenticate_ExpiredAccessRenewsSession(t *testing.T) {
	h := newHarness(t)
	access, refresh := h.session(t)
	h.clock.Advance(16 * time.Minute)

	r := h.call(t, "GET", "/api/v1/me", withCookie(httpauth.AccessCookie, access), withCookie(httpauth.RefreshCookie, refresh))
	expectStatus(t, r, 200)
	if r.body != testAccount.UserID {
		t.Fatalf("cuerpo %q, esperaba el usuario de la sesión renovada", r.body)
	}

	newAccess := r.cookie(httpauth.AccessCookie, "/")
	newRefresh := r.cookie(httpauth.RefreshCookie, httpauth.RefreshPath)
	csrf := r.cookie(httpauth.CSRFCookie, "/")
	if newAccess == nil || newAccess.Value == access {
		t.Fatal("debe emitir un access_token nuevo")
	}
	if newRefresh == nil || newRefresh.Value == refresh {
		t.Fatal("debe rotar el refresh_token en el path de la API")
	}
	if csrf == nil || csrf.Value == "" {
		t.Fatal("debe emitir un csrf_token nuevo")
	}
	if h.sessions.callCount() != 1 {
		t.Fatalf("esperaba 1 rotación, hubo %d", h.sessions.callCount())
	}
}

func TestAuthenticate_MissingAccessWithRefreshRenewsSession(t *testing.T) {
	h := newHarness(t)
	_, refresh := h.session(t)

	r := h.call(t, "GET", "/api/v1/me", withCookie(httpauth.RefreshCookie, refresh))
	expectStatus(t, r, 200)
	if r.cookie(httpauth.AccessCookie, "/") == nil {
		t.Fatal("sin access_token debe renovar y emitir uno nuevo")
	}
}

func TestAuthenticate_InvalidRefreshReturns401AndClearsCookies(t *testing.T) {
	h := newHarness(t)
	access, _ := h.session(t)
	h.clock.Advance(16 * time.Minute)

	r := h.call(t, "GET", "/api/v1/me", withCookie(httpauth.AccessCookie, access), withCookie(httpauth.RefreshCookie, "tenant.inventado"))
	expectStatus(t, r, 401)
	if code := r.errorCode(t); code != "SESSION_EXPIRED" {
		t.Fatalf("código %s, esperaba SESSION_EXPIRED", code)
	}
	for _, c := range []struct{ name, path string }{
		{httpauth.AccessCookie, "/"}, {httpauth.RefreshCookie, httpauth.RefreshPath}, {httpauth.CSRFCookie, "/"},
	} {
		got := r.cookie(c.name, c.path)
		if got == nil || got.Value != "" {
			t.Fatalf("%s debe quedar limpiada en %s", c.name, c.path)
		}
	}
}

func TestAuthenticate_RevokedSessionReturns401(t *testing.T) {
	h := newHarness(t)
	access, refresh := h.session(t)
	h.sessions.revoke(refresh) // p. ej. usuario eliminado: su refresh deja de ser válido
	h.clock.Advance(16 * time.Minute)

	r := h.call(t, "GET", "/api/v1/me", withCookie(httpauth.AccessCookie, access), withCookie(httpauth.RefreshCookie, refresh))
	expectStatus(t, r, 401)
	if code := r.errorCode(t); code != "SESSION_EXPIRED" {
		t.Fatalf("código %s, esperaba SESSION_EXPIRED", code)
	}
}

func TestAuthenticate_ExpiredAccessWithoutRefreshReturnsTokenInvalid(t *testing.T) {
	h := newHarness(t)
	access, _ := h.session(t)
	h.clock.Advance(16 * time.Minute)

	r := h.call(t, "GET", "/api/v1/me", withCookie(httpauth.AccessCookie, access))
	expectStatus(t, r, 401)
	if code := r.errorCode(t); code != "TOKEN_INVALID" {
		t.Fatalf("código %s, esperaba TOKEN_INVALID", code)
	}
	if h.sessions.callCount() != 0 {
		t.Fatal("sin refresh_token no debe llamar al servicio de sesión")
	}
}

func TestAuthenticate_ForgedAccessDoesNotRenew(t *testing.T) {
	h := newHarness(t)
	_, refresh := h.session(t)
	forged, _, err := newIssuer(t, "otro-secreto-otro-secreto-otro-secreto-99", h.clock).Issue(testAccount)
	if err != nil {
		t.Fatal(err)
	}

	r := h.call(t, "GET", "/api/v1/me", withCookie(httpauth.AccessCookie, forged), withCookie(httpauth.RefreshCookie, refresh))
	expectStatus(t, r, 401)
	if code := r.errorCode(t); code != "TOKEN_INVALID" {
		t.Fatalf("código %s, esperaba TOKEN_INVALID", code)
	}
	if h.sessions.callCount() != 0 {
		t.Fatal("un access firmado con otra clave no debe disparar renovación")
	}
}

func TestAuthenticate_ValidAccessDoesNotRenew(t *testing.T) {
	h := newHarness(t)
	access, refresh := h.session(t)

	r := h.call(t, "GET", "/api/v1/me", withCookie(httpauth.AccessCookie, access), withCookie(httpauth.RefreshCookie, refresh))
	expectStatus(t, r, 200)
	if h.sessions.callCount() != 0 || r.cookie(httpauth.RefreshCookie, httpauth.RefreshPath) != nil {
		t.Fatal("con access vigente no debe renovar ni tocar cookies")
	}
}

func TestAuthenticate_NoCookiesReturns401(t *testing.T) {
	h := newHarness(t)
	r := h.call(t, "GET", "/api/v1/me")
	expectStatus(t, r, 401)
	if code := r.errorCode(t); code != "UNAUTHENTICATED" {
		t.Fatalf("código %s, esperaba UNAUTHENTICATED", code)
	}
}
