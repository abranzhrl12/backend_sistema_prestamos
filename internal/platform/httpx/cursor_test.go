package httpx_test

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"

	"prestamos/internal/platform/httpx"
)

const validUUID = "0190f3a2-7c1e-7d4b-9a3e-5f6a7b8c9d0e"

// parseVia ejecuta ParseCursor dentro de una ruta Fiber real con la query indicada.
func parseVia(t *testing.T, query string) (httpx.CursorParams, error) {
	t.Helper()
	app := fiber.New()
	var got httpx.CursorParams
	var gotErr error
	app.Get("/", func(c fiber.Ctx) error {
		got, gotErr = httpx.ParseCursor(c)
		return nil
	})
	resp, err := app.Test(httptest.NewRequest("GET", "/?"+query, nil))
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.ReadAll(resp.Body)
	return got, gotErr
}

func fieldNames(t *testing.T, err error) map[string]bool {
	t.Helper()
	ae, ok := httpx.As(err)
	if !ok {
		t.Fatalf("esperaba AppError, got %v", err)
	}
	out := map[string]bool{}
	for _, f := range ae.Fields {
		out[f.Field] = true
	}
	return out
}

func TestParseCursor_DefaultsToLimit20AndEmptyAfter(t *testing.T) {
	p, err := parseVia(t, "")
	if err != nil {
		t.Fatalf("sin parámetros no debe fallar: %v", err)
	}
	if p.Limit != 20 || p.After != "" {
		t.Fatalf("defaults inesperados: %+v", p)
	}
}

func TestParseCursor_AcceptsLimitBoundsAndValidAfter(t *testing.T) {
	for _, q := range []string{"limit=1", "limit=100", "after=" + validUUID + "&limit=5"} {
		if _, err := parseVia(t, q); err != nil {
			t.Errorf("%q: no debería fallar: %v", q, err)
		}
	}
}

func TestParseCursor_RejectsLimitOutOfRange(t *testing.T) {
	for _, q := range []string{"limit=0", "limit=101", "limit=-3"} {
		_, err := parseVia(t, q)
		if !fieldNames(t, err)["limit"] {
			t.Errorf("%q: esperaba error en limit, got %v", q, err)
		}
	}
}

func TestParseCursor_RejectsInvalidAfterWithFieldName(t *testing.T) {
	_, err := parseVia(t, "after=no-es-uuid")
	ae, _ := httpx.As(err)
	if ae == nil || ae.Status != 400 || ae.Code != httpx.CodeValidation {
		t.Fatalf("esperaba 400 VALIDATION_ERROR, got %v", err)
	}
	if !fieldNames(t, err)["after"] {
		t.Fatalf("esperaba el campo after, got %+v", ae.Fields)
	}
}

func TestParseCursor_ReportsLimitAndAfterTogether(t *testing.T) {
	_, err := parseVia(t, "limit=500&after=xyz")
	f := fieldNames(t, err)
	if !f["limit"] || !f["after"] {
		t.Fatalf("esperaba limit y after juntos, got %v", f)
	}
}

func TestNewCursorPage_NextCursorIsLastIDWhenMore(t *testing.T) {
	items := []string{"c", "b"}
	page := httpx.NewCursorPage(items, httpx.CursorParams{Limit: 2}, true, func(s string) string { return s })
	if page.Meta.NextCursor == nil || *page.Meta.NextCursor != "b" {
		t.Fatalf("next_cursor debe ser el id del último item, got %v", page.Meta.NextCursor)
	}
	if !page.Meta.HasMore || page.Meta.Limit != 2 {
		t.Fatalf("meta inesperada: %+v", page.Meta)
	}
}

func TestNewCursorPage_NoMoreMeansNullCursor(t *testing.T) {
	page := httpx.NewCursorPage([]string{"a"}, httpx.CursorParams{Limit: 10}, false, func(s string) string { return s })
	if page.Meta.NextCursor != nil || page.Meta.HasMore {
		t.Fatalf("sin más páginas next_cursor debe ser nil: %+v", page.Meta)
	}
}

func TestNewCursorPage_EmptyItemsIsArrayNotNull(t *testing.T) {
	page := httpx.NewCursorPage[string](nil, httpx.CursorParams{Limit: 20}, false, func(s string) string { return s })
	raw, err := json.Marshal(page)
	if err != nil {
		t.Fatal(err)
	}
	var shape struct {
		Items any `json:"items"`
		Meta  struct {
			NextCursor any `json:"next_cursor"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(raw, &shape); err != nil {
		t.Fatal(err)
	}
	if arr, ok := shape.Items.([]any); !ok || arr == nil {
		t.Fatalf("items debe ser [] y no null: %s", raw)
	}
	if shape.Meta.NextCursor != nil {
		t.Fatalf("next_cursor debe ser null: %s", raw)
	}
}
