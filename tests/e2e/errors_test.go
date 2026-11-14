//go:build e2e

package e2e

import (
	"net/http"
	"time"

	"github.com/gofiber/fiber/v3"
	"strings"
	"testing"
)

func TestUnknownRoute_Returns404InStandardFormat(t *testing.T) {
	r := do(t, "GET", "/no-existe", nil)
	expectStatus(t, r, 404)
	e := r.apiErr(t)
	if e.Error.Code != "NOT_FOUND" || e.Error.Message != "El recurso solicitado no existe" {
		t.Fatalf("error inesperado: %s", r.Body)
	}
}

func TestInvalidJSON_Returns400WithCode(t *testing.T) {
	tok := loginBearer(t, "acme", "owner@acme.com")
	r := do(t, "POST", "/customers", "{esto no es json", bearer(tok))
	expectStatus(t, r, 400)
	if code := r.apiErr(t).Error.Code; code != "INVALID_JSON" {
		t.Fatalf("code %q, esperaba INVALID_JSON: %s", code, r.Body)
	}
}

func TestEveryError_IncludesRequestID(t *testing.T) {
	r := do(t, "GET", "/auth/me", nil)
	expectStatus(t, r, 401)
	if r.apiErr(t).Error.RequestID == "" {
		t.Fatalf("error sin request_id: %s", r.Body)
	}
	if r.Header.Get("X-Request-Id") == "" {
		t.Fatal("falta el header X-Request-Id")
	}
}

// fasthttp corta el cuerpo grande antes de llegar a los handlers; con app.Test (en proceso)
// eso se ve como error de transporte. En un servidor real el cliente recibe 413.
func TestOversizedBody_IsRejectedBeforeReachingHandlers(t *testing.T) {
	tok := loginBearer(t, "acme", "owner@acme.com")
	body := `{"notes":"` + strings.Repeat("a", 2<<20) + `"}`
	req, _ := http.NewRequest("POST", apiBase+"/customers", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tok)
	res, err := app.Test(req, fiber.TestConfig{Timeout: 20 * time.Second})
	switch {
	case err != nil && strings.Contains(err.Error(), "body size exceeds"):
		return // rechazado por el límite de cuerpo
	case err == nil && res.StatusCode == http.StatusRequestEntityTooLarge:
		return
	default:
		t.Fatalf("el cuerpo de 2 MiB debió rechazarse: status=%v err=%v", res, err)
	}
}
