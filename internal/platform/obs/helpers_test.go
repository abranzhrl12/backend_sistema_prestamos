package obs_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/requestid"

	"prestamos/authkit"
	"prestamos/internal/platform/httpauth"
	"prestamos/internal/platform/httpx"
	"prestamos/internal/platform/obs"
)

const testSecret = "obs-test-secret-obs-test-secret-1234"

// logLines decodifica cada línea JSON escrita en el buffer.
func logLines(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if line == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("la línea de log no es JSON: %v: %s", err, line)
		}
		out = append(out, m)
	}
	return out
}

// findLog devuelve el primer log con ese msg; falla si no existe.
func findLog(t *testing.T, buf *bytes.Buffer, msg string) map[string]any {
	t.Helper()
	for _, m := range logLines(t, buf) {
		if m["msg"] == msg {
			return m
		}
	}
	t.Fatalf("no hay log %q en: %s", msg, buf.String())
	return nil
}

// newTestApp replica la cadena de middlewares de bootstrap con rutas de prueba
// bajo el módulo "customers".
func newTestApp(t *testing.T, log *slog.Logger) *fiber.App {
	t.Helper()
	issuer, err := authkit.NewTokenIssuer(testSecret, "test", time.Hour, authkit.SystemClock{})
	if err != nil {
		t.Fatal(err)
	}
	app := fiber.New(fiber.Config{ErrorHandler: httpx.ErrorHandler})
	app.Use(requestid.New(), obs.Context(), obs.AccessLog(log), obs.Recover(log))
	app.Get("/healthz", func(c fiber.Ctx) error { return c.SendString("ok") })

	customers := app.Group("/api/v1/customers", obs.Module("customers"))
	customers.Get("/ok", func(c fiber.Ctx) error { return c.SendString("ok") })
	// Solo se prueba con Bearer, que nunca renueva: el renovador no llega a usar su refresher.
	renewer := httpauth.NewRenewer(nil, issuer, authkit.SystemClock{})
	customers.Get("/secure", httpauth.Authenticate(issuer, renewer, httpauth.CookieSettings{}), func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusNoContent)
	})
	customers.Get("/missing/:id", func(c fiber.Ctx) error {
		return httpx.NotFound("El cliente no existe")
	})
	customers.Get("/duplicado", func(c fiber.Ctx) error {
		return httpx.Conflict("DUPLICATE_DOCUMENT", "Ya existe un cliente con ese documento")
	})
	customers.Get("/fallo", func(c fiber.Ctx) error { return errors.New("base de datos caída") })
	customers.Get("/panic", func(c fiber.Ctx) error { panic("explotó") })
	return app
}

func get(t *testing.T, app *fiber.App, path string, headers map[string]string) *http.Response {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	res, err := app.Test(req, fiber.TestConfig{Timeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	return res
}

// bearer emite un access token real firmado con el secreto de prueba.
func bearer(t *testing.T, userID, tenantID string) string {
	t.Helper()
	issuer, err := authkit.NewTokenIssuer(testSecret, "test", time.Hour, authkit.SystemClock{})
	if err != nil {
		t.Fatal(err)
	}
	tok, _, err := issuer.Issue(&authkit.Account{UserID: userID, TenantID: tenantID})
	if err != nil {
		t.Fatal(err)
	}
	return tok
}
