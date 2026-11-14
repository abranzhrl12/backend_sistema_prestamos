//go:build e2e

package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
)

type response struct {
	Status  int
	Body    []byte
	Header  http.Header
	Cookies []*http.Cookie
}

type reqOpt func(*http.Request)

func bearer(token string) reqOpt {
	return func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+token) }
}

func header(k, v string) reqOpt { return func(r *http.Request) { r.Header.Set(k, v) } }

func cookie(name, value string) reqOpt {
	return func(r *http.Request) { r.AddCookie(&http.Cookie{Name: name, Value: value}) }
}

// do ejecuta la petición en proceso (sin red) contra la app real.
func do(t *testing.T, method, path string, body any, opts ...reqOpt) response {
	t.Helper()
	var rd io.Reader
	switch b := body.(type) {
	case nil:
	case string:
		rd = bytes.NewBufferString(b)
	default:
		raw, err := json.Marshal(b)
		if err != nil {
			t.Fatal(err)
		}
		rd = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, apiBase+path, rd)
	if err != nil {
		t.Fatal(err)
	}
	if rd != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for _, o := range opts {
		o(req)
	}
	res, err := app.Test(req, fiber.TestConfig{Timeout: 20 * time.Second})
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	return response{Status: res.StatusCode, Body: raw, Header: res.Header, Cookies: res.Cookies()}
}

func (r response) decode(t *testing.T, v any) {
	t.Helper()
	if err := json.Unmarshal(r.Body, v); err != nil {
		t.Fatalf("JSON inválido %q: %v", r.Body, err)
	}
}

// apiError es el formato estándar de error de la API.
type apiError struct {
	Error struct {
		Code      string `json:"code"`
		Message   string `json:"message"`
		RequestID string `json:"request_id"`
		Fields    []struct {
			Field   string `json:"field"`
			Message string `json:"message"`
		} `json:"fields"`
	} `json:"error"`
}

func (r response) apiErr(t *testing.T) apiError {
	t.Helper()
	var e apiError
	r.decode(t, &e)
	return e
}

func (r response) cookieValue(name string) string {
	for _, c := range r.Cookies {
		if c.Name == name {
			return c.Value
		}
	}
	return ""
}

func expectStatus(t *testing.T, r response, want int) {
	t.Helper()
	if r.Status != want {
		t.Fatalf("status %d, esperaba %d. body: %s", r.Status, want, r.Body)
	}
}

// loginBearer inicia sesión y devuelve el access token.
func loginBearer(t *testing.T, tenant, email string) string {
	t.Helper()
	r := do(t, "POST", "/auth/login", map[string]string{"tenant": tenant, "email": email, "password": pass})
	expectStatus(t, r, 200)
	var out struct {
		AccessToken string `json:"access_token"`
	}
	r.decode(t, &out)
	return out.AccessToken
}

var docSeq atomic.Int64

// nextDoc devuelve un documento único para evitar choques entre tests.
func nextDoc() string { return fmt.Sprintf("D%09d", docSeq.Add(1)) }

func newCustomer(name string) map[string]string {
	return map[string]string{"full_name": name, "document_id": nextDoc(), "phone": "987654321",
		"address": "Av. Lima 123", "address_reference": "frente al mercado"}
}
