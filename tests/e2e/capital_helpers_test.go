//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
)

// capBalance es la respuesta de GET /capital.
type capBalance struct {
	TotalCash       string `json:"total_cash"`
	CapitalBalance  string `json:"capital_balance"`
	ProfitBalance   string `json:"profit_balance"`
	LentOutstanding string `json:"lent_outstanding"`
	MovementsCount  int    `json:"movements_count"`
	AsOf            string `json:"as_of"`
}

// capMovement es un movimiento del libro.
type capMovement struct {
	ID        string  `json:"id"`
	Type      string  `json:"type"`
	Direction string  `json:"direction"`
	Bucket    string  `json:"bucket"`
	Amount    string  `json:"amount"`
	Note      string  `json:"note"`
	Reference string  `json:"reference"`
	CreatedBy *string `json:"created_by"`
	CreatedAt string  `json:"created_at"`
}

// capPageDTO es la respuesta paginada por cursor de GET /capital/movements.
type capPageDTO struct {
	Items []capMovement `json:"items"`
	Meta  struct {
		Limit      int     `json:"limit"`
		NextCursor *string `json:"next_cursor"`
		HasMore    bool    `json:"has_more"`
	} `json:"meta"`
}

func capGetBalance(t *testing.T, tok string) capBalance {
	t.Helper()
	r := do(t, "GET", "/capital", nil, bearer(tok))
	expectStatus(t, r, 200)
	var b capBalance
	r.decode(t, &b)
	return b
}

// capPost registra un movimiento manual con nota y espera 201.
func capPost(t *testing.T, tok, typ, amount string) capMovement {
	t.Helper()
	body := map[string]string{"type": typ, "amount": amount, "note": "Movimiento de prueba"}
	r := do(t, "POST", "/capital/movements", body, bearer(tok))
	expectStatus(t, r, 201)
	var m capMovement
	r.decode(t, &m)
	return m
}

// capList pide GET /capital/movements?query y espera 200.
func capList(t *testing.T, tok, query string) capPageDTO {
	t.Helper()
	r := do(t, "GET", "/capital/movements?"+query, nil, bearer(tok))
	expectStatus(t, r, 200)
	var p capPageDTO
	r.decode(t, &p)
	return p
}

// doRaw ejecuta una petición sin usar t: es seguro llamarla desde goroutines.
// Devuelve el status HTTP; un fallo de transporte se devuelve como error.
func doRaw(method, path string, body any, opts ...reqOpt) (int, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return 0, err
	}
	req, err := http.NewRequest(method, apiBase+path, bytes.NewReader(raw))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	for _, o := range opts {
		o(req)
	}
	res, err := app.Test(req, fiber.TestConfig{Timeout: 20 * time.Second})
	if err != nil {
		return 0, err
	}
	defer res.Body.Close()
	_, _ = io.Copy(io.Discard, res.Body)
	return res.StatusCode, nil
}

// capAddUser crea un usuario con el rol indicado en la empresa y devuelve su token.
func capAddUser(t *testing.T, f fixture, role string) string {
	t.Helper()
	email := role + "@" + f.Slug + ".com"
	if _, err := env.User(context.Background(), f.TenantID, email, pass, role); err != nil {
		t.Fatal(err)
	}
	return loginBearer(t, f.Slug, email)
}

// expectErrCode espera un status y un code de error estable.
func expectErrCode(t *testing.T, r response, status int, code string) {
	t.Helper()
	expectStatus(t, r, status)
	if got := r.apiErr(t).Error.Code; got != code {
		t.Fatalf("code %q, esperaba %s: %s", got, code, r.Body)
	}
}

// fieldError devuelve el mensaje del problema del campo indicado ("" si no existe).
func fieldError(t *testing.T, r response, field string) string {
	t.Helper()
	for _, f := range r.apiErr(t).Error.Fields {
		if f.Field == field {
			return f.Message
		}
	}
	return ""
}
