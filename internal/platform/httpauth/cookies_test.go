package httpauth_test

import (
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"

	"prestamos/authkit"
	"prestamos/internal/platform/httpauth"
	"prestamos/internal/platform/httpx"
)

// Las cookies de sesión deben viajar por toda la API, y el path antiguo se limpia al emitir.
func TestCookieSettings_SetUsesAPIRefreshPathAndExpiresLegacy(t *testing.T) {
	r := runCookieHandler(t, func(c fiber.Ctx, s httpauth.CookieSettings, p *authkit.TokenPair) error {
		return s.Set(c, p)
	})

	refresh := r.cookie(httpauth.RefreshCookie, httpauth.RefreshPath)
	if refresh == nil || refresh.Value != "refresh-nuevo" || !refresh.HttpOnly {
		t.Fatalf("refresh_token debe ir HttpOnly en %s", httpauth.RefreshPath)
	}
	if legacy := r.cookie(httpauth.RefreshCookie, "/api/v1/auth"); legacy == nil || legacy.Value != "" {
		t.Fatal("debe expirar la cookie refresh_token del path legado /api/v1/auth")
	}
	if csrf := r.cookie(httpauth.CSRFCookie, "/"); csrf == nil || csrf.HttpOnly || csrf.Value == "" {
		t.Fatal("csrf_token debe ser legible por JS y no vacío")
	}
}

func TestCookieSettings_ClearExpiresBothRefreshPaths(t *testing.T) {
	r := runCookieHandler(t, func(c fiber.Ctx, s httpauth.CookieSettings, _ *authkit.TokenPair) error {
		s.Clear(c)
		return c.SendStatus(fiber.StatusNoContent)
	})

	for _, path := range []string{httpauth.RefreshPath, "/api/v1/auth"} {
		got := r.cookie(httpauth.RefreshCookie, path)
		if got == nil || got.Value != "" || !got.Expires.Before(time.Now()) {
			t.Fatalf("refresh_token debe quedar caducada en %s", path)
		}
	}
}

func runCookieHandler(t *testing.T, fn func(fiber.Ctx, httpauth.CookieSettings, *authkit.TokenPair) error) result {
	t.Helper()
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	pair := &authkit.TokenPair{
		AccessToken: "access-nuevo", AccessExpiresAt: now.Add(15 * time.Minute),
		RefreshToken: "refresh-nuevo", RefreshExpiresAt: now.Add(time.Hour),
	}
	settings := httpauth.CookieSettings{RefreshPath: httpauth.RefreshPath}
	app := fiber.New(fiber.Config{ErrorHandler: httpx.ErrorHandler})
	app.Get("/x", func(c fiber.Ctx) error { return fn(c, settings, pair) })
	h := &harness{app: app}
	return h.call(t, "GET", "/x")
}
