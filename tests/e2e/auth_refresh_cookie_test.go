//go:build e2e

package e2e

import (
	"context"
	"net/http"
	"testing"
	"time"

	"prestamos/authkit"
)

// webSession es una sesión web real: login por cookies de un usuario recién creado.
type webSession struct {
	userID  string
	access  string
	refresh string
	csrf    string
}

func newWebSession(t *testing.T) webSession {
	t.Helper()
	ctx := context.Background()
	email := "renew-" + nextDoc() + "@acme.com"
	id, err := env.User(ctx, tenantA, email, pass, "owner")
	if err != nil {
		t.Fatal(err)
	}
	login := do(t, "POST", "/auth/login", map[string]string{
		"tenant": "acme", "email": email, "password": pass, "transport": "cookie"})
	expectStatus(t, login, 200)
	return webSession{
		userID:  id,
		access:  cookieAt(login, "access_token", "/"),
		refresh: cookieAt(login, "refresh_token", apiPath),
		csrf:    cookieAt(login, "csrf_token", "/"),
	}
}

// apiPath es el path de la cookie refresh_token tras esta versión.
const apiPath = "/api/v1"

// expiredAccess firma un access token ya vencido para el usuario, con el secreto de la app.
// Así la expiración es determinista y no depende de esperar al reloj.
func expiredAccess(t *testing.T, s webSession) string {
	t.Helper()
	i, err := authkit.NewTokenIssuer(testJWTSecret, testJWTIssuer, time.Minute, pastClock{})
	if err != nil {
		t.Fatal(err)
	}
	tok, _, err := i.Issue(&authkit.Account{UserID: s.userID, TenantID: tenantA, Roles: []string{"owner"}})
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

// pastClock sitúa la emisión una hora atrás: con TTL de 1 min el token nace ya vencido.
type pastClock struct{}

func (pastClock) Now() time.Time { return time.Now().Add(-time.Hour) }

// cookieAt devuelve el valor de la cookie emitida con ese nombre y path (vacío si no hay).
func cookieAt(r response, name, path string) string {
	if c := setCookieAt(r, name, path); c != nil {
		return c.Value
	}
	return ""
}

// setCookieAt devuelve la cabecera Set-Cookie con ese nombre y path (nil si no vino).
func setCookieAt(r response, name, path string) *http.Cookie {
	for _, c := range r.Cookies {
		if c.Name == name && c.Path == path {
			return c
		}
	}
	return nil
}

func isCleared(c *http.Cookie) bool {
	return c != nil && c.Value == "" && !c.Expires.IsZero() && c.Expires.Before(time.Now())
}

func TestRefreshCookie_ExpiredAccessRenewsTransparently(t *testing.T) {
	s := newWebSession(t)
	r := do(t, "GET", "/auth/me", nil, cookie("access_token", expiredAccess(t, s)), cookie("refresh_token", s.refresh))
	expectStatus(t, r, 200)

	newAccess := cookieAt(r, "access_token", "/")
	newRefresh := cookieAt(r, "refresh_token", apiPath)
	if newAccess == "" || newRefresh == "" || newRefresh == s.refresh {
		t.Fatalf("debe emitir access y refresh nuevos en Set-Cookie: %v", r.Cookies)
	}
	if cookieAt(r, "csrf_token", "/") == "" {
		t.Fatal("debe emitir csrf_token nuevo")
	}
	if again := do(t, "GET", "/auth/me", nil, cookie("access_token", newAccess)); again.Status != 200 {
		t.Fatalf("el access renovado debe funcionar: %d %s", again.Status, again.Body)
	}
}

func TestRefreshCookie_MissingAccessWithRefreshRenews(t *testing.T) {
	s := newWebSession(t)
	r := do(t, "GET", "/auth/me", nil, cookie("refresh_token", s.refresh))
	expectStatus(t, r, 200)
	if cookieAt(r, "access_token", "/") == "" {
		t.Fatal("sin access_token debe renovar")
	}
}

func TestRefreshCookie_SameOldCookieWithinGraceGetsSamePair(t *testing.T) {
	s := newWebSession(t)
	expired := expiredAccess(t, s)
	first := do(t, "GET", "/auth/me", nil, cookie("access_token", expired), cookie("refresh_token", s.refresh))
	second := do(t, "GET", "/auth/me", nil, cookie("access_token", expired), cookie("refresh_token", s.refresh))
	expectStatus(t, first, 200)
	expectStatus(t, second, 200)
	if cookieAt(first, "refresh_token", apiPath) != cookieAt(second, "refresh_token", apiPath) {
		t.Fatal("dos peticiones con la misma cookie vieja dentro de la ventana deben recibir el mismo refresh")
	}
}

func TestRefreshCookie_MutationWithoutCSRFReturns403AndDoesNotRenew(t *testing.T) {
	s := newWebSession(t)
	r := do(t, "POST", "/customers", newCustomer("Renovación sin CSRF"),
		cookie("access_token", expiredAccess(t, s)), cookie("refresh_token", s.refresh))
	expectStatus(t, r, 403)
	if e := r.apiErr(t); e.Error.Code != "CSRF_INVALID" {
		t.Fatalf("esperaba CSRF_INVALID: %s", r.Body)
	}
	if cookieAt(r, "access_token", "/") != "" {
		t.Fatal("una mutación sin CSRF no debe renovar la sesión")
	}
}

func TestRefreshCookie_MutationWithCSRFRenewsAndCreates(t *testing.T) {
	s := newWebSession(t)
	r := do(t, "POST", "/customers", newCustomer("Renovación con CSRF"),
		cookie("access_token", expiredAccess(t, s)), cookie("refresh_token", s.refresh),
		cookie("csrf_token", s.csrf), header("X-CSRF-Token", s.csrf))
	expectStatus(t, r, 201)
	if newCSRF := cookieAt(r, "csrf_token", "/"); newCSRF == "" || newCSRF == s.csrf {
		t.Fatal("tras renovar debe llegar un csrf_token nuevo")
	}
}

func TestRefreshCookie_InvalidRefreshReturns401AndClears(t *testing.T) {
	s := newWebSession(t)
	r := do(t, "GET", "/auth/me", nil, cookie("access_token", expiredAccess(t, s)),
		cookie("refresh_token", "tenant-inventado.basura"))
	expectStatus(t, r, 401)
	if e := r.apiErr(t); e.Error.Code != "SESSION_EXPIRED" {
		t.Fatalf("esperaba SESSION_EXPIRED: %s", r.Body)
	}
	if !isCleared(setCookieAt(r, "refresh_token", apiPath)) {
		t.Fatal("debe limpiar refresh_token en el path de la API")
	}
}

func TestRefreshCookie_SoftDeletedUserCannotRenew(t *testing.T) {
	s := newWebSession(t)
	if _, err := env.Admin.Exec(context.Background(),
		`UPDATE users SET deleted_at = now() WHERE id = $1::uuid`, s.userID); err != nil {
		t.Fatal(err)
	}
	r := do(t, "GET", "/auth/me", nil, cookie("access_token", expiredAccess(t, s)), cookie("refresh_token", s.refresh))
	expectStatus(t, r, 401)
	if e := r.apiErr(t); e.Error.Code != "SESSION_EXPIRED" {
		t.Fatalf("usuario eliminado: esperaba SESSION_EXPIRED: %s", r.Body)
	}
}

func TestRefreshCookie_BearerExpiredNeverRenews(t *testing.T) {
	s := newWebSession(t)
	r := do(t, "GET", "/auth/me", nil, bearer(expiredAccess(t, s)), cookie("refresh_token", s.refresh))
	expectStatus(t, r, 401)
	if e := r.apiErr(t); e.Error.Code != "TOKEN_INVALID" {
		t.Fatalf("esperaba TOKEN_INVALID: %s", r.Body)
	}
	if len(r.Cookies) != 0 {
		t.Fatal("bearer no debe emitir cookies ni renovar")
	}
}

func TestRefreshCookie_LoginExpiresLegacyRefreshPath(t *testing.T) {
	login := do(t, "POST", "/auth/login", map[string]string{
		"tenant": "acme", "email": "owner@acme.com", "password": pass, "transport": "cookie"})
	expectStatus(t, login, 200)
	if !isCleared(setCookieAt(login, "refresh_token", "/api/v1/auth")) {
		t.Fatal("login debe expirar el refresh_token del path legado para no dejar dos cookies")
	}
}

func TestRefreshCookie_LogoutClearsBothRefreshPaths(t *testing.T) {
	s := newWebSession(t)
	r := do(t, "POST", "/auth/logout", nil,
		cookie("access_token", s.access), cookie("refresh_token", s.refresh),
		cookie("csrf_token", s.csrf), header("X-CSRF-Token", s.csrf))
	expectStatus(t, r, 204)
	for _, path := range []string{apiPath, "/api/v1/auth"} {
		if !isCleared(setCookieAt(r, "refresh_token", path)) {
			t.Fatalf("logout debe limpiar refresh_token en %s", path)
		}
	}
}
