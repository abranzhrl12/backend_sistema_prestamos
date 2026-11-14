//go:build e2e

package e2e

import (
	"context"
	"strings"
	"testing"
)

func TestCapital_CursorPagination_WalksEveryMovementOnce(t *testing.T) {
	f := newFixture(t)
	tok := f.ownerToken(t)
	for range 25 {
		capPost(t, tok, "contribution", "1.00")
	}

	var ids []string
	cursor := ""
	pages := 0
	for {
		q := "limit=10"
		if cursor != "" {
			q += "&after=" + cursor
		}
		p := capList(t, tok, q)
		pages++
		for _, m := range p.Items {
			ids = append(ids, m.ID)
		}
		if !p.Meta.HasMore {
			if p.Meta.NextCursor != nil {
				t.Fatalf("última página: next_cursor debe ser null")
			}
			break
		}
		if p.Meta.NextCursor == nil {
			t.Fatal("página intermedia sin next_cursor")
		}
		cursor = *p.Meta.NextCursor
		if pages > 10 {
			t.Fatal("demasiadas páginas: el cursor no avanza")
		}
	}

	if pages != 3 || len(ids) != 25 {
		t.Fatalf("páginas %d, elementos %d; esperaba 3 y 25", pages, len(ids))
	}
	seen := map[string]bool{}
	for i, id := range ids {
		if seen[id] {
			t.Fatalf("id repetido %s", id)
		}
		seen[id] = true
		if i > 0 && ids[i-1] <= id {
			t.Fatalf("orden no descendente en la posición %d", i)
		}
	}
}

func TestCapital_InvalidAfter_Returns400WithAfterField(t *testing.T) {
	f := newFixture(t)
	r := do(t, "GET", "/capital/movements?after=no-es-uuid", nil, bearer(f.ownerToken(t)))
	expectErrCode(t, r, 400, "VALIDATION_ERROR")
	if fieldError(t, r, "after") == "" {
		t.Fatalf("esperaba campo after: %s", r.Body)
	}
}

func TestCapital_ListFiltersByType(t *testing.T) {
	f := newFixture(t)
	tok := f.ownerToken(t)
	capPost(t, tok, "contribution", "1000")
	capPost(t, tok, "withdrawal", "100")
	capPost(t, tok, "withdrawal", "200")

	p := capList(t, tok, "type=withdrawal")
	if len(p.Items) != 2 {
		t.Fatalf("esperaba 2 retiros, got %d", len(p.Items))
	}
	for _, m := range p.Items {
		if m.Type != "withdrawal" {
			t.Fatalf("filtro type=withdrawal devolvió %s", m.Type)
		}
	}
	expectErrCode(t, do(t, "GET", "/capital/movements?type=prestamo", nil, bearer(tok)), 400, "VALIDATION_ERROR")
}

func TestCapital_ListFiltersByCreatedAt(t *testing.T) {
	f := newFixture(t)
	tok := f.ownerToken(t)
	capPost(t, tok, "contribution", "10")
	capPost(t, tok, "contribution", "20")

	if n := len(capList(t, tok, "from=2099-01-01T00:00:00Z").Items); n != 0 {
		t.Fatalf("from futuro debe devolver vacío, got %d", n)
	}
	if n := len(capList(t, tok, "from=2000-01-01T00:00:00Z&to=2099-01-01T00:00:00Z").Items); n != 2 {
		t.Fatalf("rango amplio debe devolver 2, got %d", n)
	}
	if n := len(capList(t, tok, "to=2000-01-01T00:00:00Z").Items); n != 0 {
		t.Fatalf("to pasado debe devolver vacío, got %d", n)
	}
	r := do(t, "GET", "/capital/movements?from=ayer", nil, bearer(tok))
	expectErrCode(t, r, 400, "VALIDATION_ERROR")
	if fieldError(t, r, "from") == "" {
		t.Fatalf("esperaba campo from: %s", r.Body)
	}
}

func TestCapital_LedgerIsImmutableInDatabase(t *testing.T) {
	f := newFixture(t)
	m := capPost(t, f.ownerToken(t), "contribution", "42.00")
	ctx := context.Background()

	_, err := env.Admin.Exec(ctx, `UPDATE capital_movements SET amount = 1 WHERE id = $1::uuid`, m.ID)
	if err == nil || !strings.Contains(err.Error(), "append-only") {
		t.Fatalf("UPDATE debe fallar por el trigger, got %v", err)
	}
	// Verificación negativa: el borrado debe ser rechazado por el trigger.
	_, err = env.Admin.Exec(ctx, `DELETE FROM capital_movements WHERE id = $1::uuid`, m.ID)
	if err == nil || !strings.Contains(err.Error(), "append-only") {
		t.Fatalf("DELETE debe fallar por el trigger, got %v", err)
	}
	var amount string
	if err := env.Admin.QueryRow(ctx, `SELECT amount::text FROM capital_movements WHERE id = $1::uuid`, m.ID).Scan(&amount); err != nil {
		t.Fatal(err)
	}
	if amount != "42.0000" {
		t.Fatalf("el monto no debe cambiar, got %s", amount)
	}
}

func TestCapital_IsolationBetweenTenants(t *testing.T) {
	a := newFixture(t)
	b := newFixture(t)
	tokA, tokB := a.ownerToken(t), b.ownerToken(t)

	mA := capPost(t, tokA, "contribution", "500")

	if bal := capGetBalance(t, tokB); bal.TotalCash != "0.00" || bal.MovementsCount != 0 {
		t.Fatalf("la otra empresa no debe ver saldo ajeno: %+v", bal)
	}
	for _, m := range capList(t, tokB, "limit=100").Items {
		if m.ID == mA.ID {
			t.Fatalf("la empresa B ve el movimiento %s de la empresa A", m.ID)
		}
	}
	found := false
	for _, m := range capList(t, tokA, "limit=100").Items {
		if m.ID == mA.ID {
			found = true
		}
	}
	if !found {
		t.Fatalf("la empresa A no ve su propio movimiento %s", mA.ID)
	}
}
