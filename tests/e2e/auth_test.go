//go:build e2e

package e2e

import (
	"bytes"
	"testing"
)

func TestLogin_Bearer_ReturnsTokens(t *testing.T) {
	r := do(t, "POST", "/auth/login", map[string]string{"tenant": "acme", "email": "owner@acme.com", "password": pass})
	expectStatus(t, r, 200)
	var out struct {
		Access  string `json:"access_token"`
		Refresh string `json:"refresh_token"`
		Type    string `json:"token_type"`
	}
	r.decode(t, &out)
	if out.Access == "" || out.Refresh == "" || out.Type != "Bearer" {
		t.Fatalf("respuesta inesperada: %s", r.Body)
	}
	if len(r.Cookies) != 0 {
		t.Fatal("transporte bearer no debe setear cookies")
	}
}

func TestLogin_WrongPassword_ReturnsSpanishError(t *testing.T) {
	r := do(t, "POST", "/auth/login", map[string]string{"tenant": "acme", "email": "owner@acme.com", "password": "mala"})
	expectStatus(t, r, 401)
	e := r.apiErr(t)
	if e.Error.Code != "INVALID_CREDENTIALS" || e.Error.Message != "Correo o contraseña incorrectos" {
		t.Fatalf("error inesperado: %+v", e)
	}
}

func TestLogin_UnknownUserAndTenant_SameError(t *testing.T) {
	a := do(t, "POST", "/auth/login", map[string]string{"tenant": "acme", "email": "nadie@acme.com", "password": pass})
	b := do(t, "POST", "/auth/login", map[string]string{"tenant": "noexiste", "email": "owner@acme.com", "password": pass})
	expectStatus(t, a, 401)
	expectStatus(t, b, 401)
	if a.apiErr(t).Error.Code != b.apiErr(t).Error.Code {
		t.Fatal("no debe revelar si falló el usuario o la empresa")
	}
}

func TestLogin_MissingFields_ListsEachField(t *testing.T) {
	r := do(t, "POST", "/auth/login", map[string]string{})
	expectStatus(t, r, 400)
	e := r.apiErr(t)
	if e.Error.Code != "VALIDATION_ERROR" || len(e.Error.Fields) != 2 {
		t.Fatalf("esperaba 2 campos con error: %s", r.Body)
	}
}

func TestLogin_Cookie_SetsHttpOnlyAndCSRF(t *testing.T) {
	r := do(t, "POST", "/auth/login", map[string]string{
		"tenant": "acme", "email": "owner@acme.com", "password": pass, "transport": "cookie"})
	expectStatus(t, r, 200)
	for _, name := range []string{"access_token", "refresh_token", "csrf_token"} {
		if r.cookieValue(name) == "" {
			t.Fatalf("falta la cookie %s", name)
		}
	}
	for _, c := range r.Cookies {
		if (c.Name == "access_token" || c.Name == "refresh_token") && !c.HttpOnly {
			t.Fatalf("%s debe ser HttpOnly", c.Name)
		}
		if c.Name == "csrf_token" && c.HttpOnly {
			t.Fatal("csrf_token debe ser legible por JS")
		}
	}
	if bytes.Contains(r.Body, []byte(`"access_token":"`)) {
		t.Fatal("con cookies el token no debe ir en el JSON")
	}
}

func TestCookieAuth_RequiresCSRFOnWrites(t *testing.T) {
	login := do(t, "POST", "/auth/login", map[string]string{
		"tenant": "acme", "email": "owner@acme.com", "password": pass, "transport": "cookie"})
	access, csrf := login.cookieValue("access_token"), login.cookieValue("csrf_token")

	get := do(t, "GET", "/auth/me", nil, cookie("access_token", access))
	expectStatus(t, get, 200) // lectura sin CSRF

	body := newCustomer("Sin CSRF")
	noCSRF := do(t, "POST", "/customers", body, cookie("access_token", access))
	expectStatus(t, noCSRF, 403)
	if noCSRF.apiErr(t).Error.Code != "CSRF_INVALID" {
		t.Fatalf("esperaba CSRF_INVALID: %s", noCSRF.Body)
	}

	ok := do(t, "POST", "/customers", body, cookie("access_token", access), cookie("csrf_token", csrf), header("X-CSRF-Token", csrf))
	expectStatus(t, ok, 201)
}

func TestRefresh_RotatesAndRejectsReuse(t *testing.T) {
	login := do(t, "POST", "/auth/login", map[string]string{"tenant": "acme", "email": "owner@acme.com", "password": pass})
	var tok struct {
		Refresh string `json:"refresh_token"`
	}
	login.decode(t, &tok)

	first := do(t, "POST", "/auth/refresh", map[string]string{"refresh_token": tok.Refresh})
	expectStatus(t, first, 200)

	reuse := do(t, "POST", "/auth/refresh", map[string]string{"refresh_token": tok.Refresh})
	expectStatus(t, reuse, 401)
	if reuse.apiErr(t).Error.Code != "SESSION_EXPIRED" {
		t.Fatalf("esperaba SESSION_EXPIRED: %s", reuse.Body)
	}
}

func TestMe_WithoutToken_Returns401Spanish(t *testing.T) {
	r := do(t, "GET", "/auth/me", nil)
	expectStatus(t, r, 401)
	if e := r.apiErr(t); e.Error.Code != "UNAUTHENTICATED" || e.Error.Message == "" {
		t.Fatalf("error inesperado: %s", r.Body)
	}
}

func TestMe_GarbageToken_ReturnsTokenInvalid(t *testing.T) {
	r := do(t, "GET", "/auth/me", nil, bearer("no.es.un.jwt"))
	expectStatus(t, r, 401)
	if r.apiErr(t).Error.Code != "TOKEN_INVALID" {
		t.Fatalf("esperaba TOKEN_INVALID: %s", r.Body)
	}
}
