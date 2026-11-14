package httpauth_test

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"

	"prestamos/authkit"
	"prestamos/internal/platform/httpauth"
	"prestamos/internal/platform/httpx"
)

const testSecret = "test-secret-test-secret-test-secret-1234"

var testAccount = &authkit.Account{
	UserID: "0190a000-0000-7000-8000-000000000001", TenantID: "acme-tenant", Roles: []string{"owner"}, Active: true,
}

// harness monta una app Fiber con una ruta GET y una POST protegidas por el middleware real.
type harness struct {
	app      *fiber.App
	clock    *fakeClock
	issuer   *authkit.TokenIssuer
	sessions *fakeSessions
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	clock := newFakeClock()
	issuer := newIssuer(t, testSecret, clock)
	sessions := newFakeSessions(issuer, clock)
	renewer := httpauth.NewRenewer(sessions, issuer, clock)
	authn := httpauth.Authenticate(issuer, renewer, httpauth.CookieSettings{RefreshPath: httpauth.RefreshPath})

	app := fiber.New(fiber.Config{ErrorHandler: httpx.ErrorHandler})
	app.Get("/api/v1/me", authn, whoami)
	app.Post("/api/v1/items", authn, whoami)
	return &harness{app: app, clock: clock, issuer: issuer, sessions: sessions}
}

func newIssuer(t *testing.T, secret string, clock authkit.Clock) *authkit.TokenIssuer {
	t.Helper()
	i, err := authkit.NewTokenIssuer(secret, "test", 15*time.Minute, clock)
	if err != nil {
		t.Fatal(err)
	}
	return i
}

func whoami(c fiber.Ctx) error { return c.SendString(httpauth.Claims(c).UserID()) }

// session emite el access y el refresh de un login válido para testAccount.
func (h *harness) session(t *testing.T) (access, refresh string) {
	t.Helper()
	a, _, err := h.issuer.Issue(testAccount)
	if err != nil {
		t.Fatal(err)
	}
	return a, h.sessions.seed(testAccount)
}

type reqOpt func(*http.Request)

func withCookie(name, value string) reqOpt {
	return func(r *http.Request) { r.AddCookie(&http.Cookie{Name: name, Value: value}) }
}

func withHeader(k, v string) reqOpt {
	return func(r *http.Request) { r.Header.Set(k, v) }
}

func withBearer(token string) reqOpt { return withHeader("Authorization", "Bearer "+token) }

type result struct {
	status  int
	body    string
	cookies []*http.Cookie
}

// send ejecuta la petición en proceso. No usa t, así que es seguro desde goroutines.
func (h *harness) send(method, path string, opts ...reqOpt) (result, error) {
	req, err := http.NewRequest(method, path, nil)
	if err != nil {
		return result{}, err
	}
	for _, o := range opts {
		o(req)
	}
	res, err := h.app.Test(req, fiber.TestConfig{Timeout: 5 * time.Second})
	if err != nil {
		return result{}, err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		return result{}, err
	}
	return result{status: res.StatusCode, body: string(raw), cookies: res.Cookies()}, nil
}

func (h *harness) call(t *testing.T, method, path string, opts ...reqOpt) result {
	t.Helper()
	r, err := h.send(method, path, opts...)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	return r
}

// cookie devuelve la cookie emitida con ese nombre y path (nil si no vino en Set-Cookie).
func (r result) cookie(name, path string) *http.Cookie {
	for _, c := range r.cookies {
		if c.Name == name && c.Path == path {
			return c
		}
	}
	return nil
}

func (r result) errorCode(t *testing.T) string {
	t.Helper()
	var e struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(r.body), &e); err != nil {
		t.Fatalf("cuerpo no JSON: %q", r.body)
	}
	return e.Error.Code
}

func expectStatus(t *testing.T, r result, want int) {
	t.Helper()
	if r.status != want {
		t.Fatalf("status %d, esperaba %d; cuerpo: %s", r.status, want, r.body)
	}
}
