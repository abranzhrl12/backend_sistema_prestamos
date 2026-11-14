//go:build e2e

package e2e

import (
	"strings"
	"testing"
)

func TestCapital_InitialBalanceIsZeroEverywhere(t *testing.T) {
	f := newFixture(t)
	b := capGetBalance(t, f.ownerToken(t))
	want := capBalance{TotalCash: "0.00", CapitalBalance: "0.00", ProfitBalance: "0.00",
		LentOutstanding: "0.00", MovementsCount: 0}
	b.AsOf = ""
	if b != want {
		t.Fatalf("saldo inicial %+v, esperaba %+v", b, want)
	}
}

func TestCapital_ContributionAndWithdrawalUpdateBalance(t *testing.T) {
	f := newFixture(t)
	tok := f.ownerToken(t)

	capPost(t, tok, "contribution", "1000")
	if b := capGetBalance(t, tok); b.TotalCash != "1000.00" || b.CapitalBalance != "1000.00" {
		t.Fatalf("tras aporte de 1000: %+v", b)
	}
	m := capPost(t, tok, "withdrawal", "300")
	if m.Direction != "out" || m.Bucket != "capital" || m.Amount != "300.00" {
		t.Fatalf("respuesta del retiro inesperada: %+v", m)
	}
	if b := capGetBalance(t, tok); b.TotalCash != "700.00" || b.MovementsCount != 2 {
		t.Fatalf("tras retiro de 300: %+v", b)
	}
}

func TestCapital_WithdrawalAboveBalance_Returns409AndKeepsBalance(t *testing.T) {
	f := newFixture(t)
	tok := f.ownerToken(t)
	capPost(t, tok, "contribution", "700")

	r := do(t, "POST", "/capital/movements", map[string]string{"type": "withdrawal", "amount": "800"}, bearer(tok))
	expectErrCode(t, r, 409, "INSUFFICIENT_CAPITAL")
	if msg := r.apiErr(t).Error.Message; msg != "No hay capital suficiente para esta operación" {
		t.Fatalf("mensaje inesperado: %q", msg)
	}
	if b := capGetBalance(t, tok); b.TotalCash != "700.00" || b.MovementsCount != 1 {
		t.Fatalf("el saldo no debe cambiar tras un 409: %+v", b)
	}
}

func TestCapital_AdjustmentWithoutNote_Returns400WithNoteField(t *testing.T) {
	f := newFixture(t)
	tok := f.ownerToken(t)
	r := do(t, "POST", "/capital/movements", map[string]string{"type": "adjustment_in", "amount": "50"}, bearer(tok))
	expectErrCode(t, r, 400, "VALIDATION_ERROR")
	if fieldError(t, r, "note") == "" {
		t.Fatalf("esperaba problema en el campo note: %s", r.Body)
	}
}

func TestCapital_InvalidAmounts_Return400WithAmountField(t *testing.T) {
	f := newFixture(t)
	tok := f.ownerToken(t)
	for _, amount := range []string{"abc", "-5", "0", "1.23456"} {
		body := map[string]string{"type": "contribution", "amount": amount}
		r := do(t, "POST", "/capital/movements", body, bearer(tok))
		expectErrCode(t, r, 400, "VALIDATION_ERROR")
		if fieldError(t, r, "amount") == "" {
			t.Fatalf("monto %q: esperaba campo amount: %s", amount, r.Body)
		}
	}
}

func TestCapital_NumericAmountInJSON_Returns400(t *testing.T) {
	f := newFixture(t)
	tok := f.ownerToken(t)
	r := do(t, "POST", "/capital/movements", `{"type":"contribution","amount":100}`, bearer(tok))
	expectErrCode(t, r, 400, "INVALID_JSON")
}

func TestCapital_NonManualType_Returns400WithTypeField(t *testing.T) {
	f := newFixture(t)
	tok := f.ownerToken(t)
	r := do(t, "POST", "/capital/movements", map[string]string{"type": "disbursement", "amount": "10"}, bearer(tok))
	expectErrCode(t, r, 400, "VALIDATION_ERROR")
	if fieldError(t, r, "type") == "" {
		t.Fatalf("esperaba campo type: %s", r.Body)
	}
}

func TestCapital_Cobrador_IsForbiddenToWriteAndRead(t *testing.T) {
	f := newFixture(t)
	tok := f.cobradorToken(t)
	expectErrCode(t, do(t, "POST", "/capital/movements", map[string]string{"type": "contribution", "amount": "10"}, bearer(tok)), 403, "FORBIDDEN")
	expectErrCode(t, do(t, "GET", "/capital", nil, bearer(tok)), 403, "FORBIDDEN")
	expectErrCode(t, do(t, "GET", "/capital/movements", nil, bearer(tok)), 403, "FORBIDDEN")
}

func TestCapital_Admin_CanContributeButNotWithdraw(t *testing.T) {
	f := newFixture(t)
	admin := capAddUser(t, f, "admin")
	capPost(t, f.ownerToken(t), "contribution", "400")

	capPost(t, admin, "adjustment_in", "10") // el adjustment_in sí lo puede hacer
	r := do(t, "POST", "/capital/movements", map[string]string{"type": "withdrawal", "amount": "100"}, bearer(admin))
	expectErrCode(t, r, 403, "FORBIDDEN")
	if msg := r.apiErr(t).Error.Message; !strings.Contains(msg, "dueño") {
		t.Fatalf("mensaje en español inesperado: %q", msg)
	}
	if b := capGetBalance(t, admin); b.TotalCash != "410.00" {
		t.Fatalf("el admin puede leer el saldo: %+v", b)
	}
}

func TestCapital_Viewer_CanReadButNotWrite(t *testing.T) {
	f := newFixture(t)
	viewer := capAddUser(t, f, "viewer")
	expectStatus(t, do(t, "GET", "/capital", nil, bearer(viewer)), 200)
	expectStatus(t, do(t, "GET", "/capital/movements", nil, bearer(viewer)), 200)
	expectErrCode(t, do(t, "POST", "/capital/movements", map[string]string{"type": "contribution", "amount": "10"}, bearer(viewer)), 403, "FORBIDDEN")
}

func TestCapital_DecimalsAreExact(t *testing.T) {
	f := newFixture(t)
	tok := f.ownerToken(t)
	capPost(t, tok, "contribution", "0.10")
	capPost(t, tok, "contribution", "0.20")
	if b := capGetBalance(t, tok); b.TotalCash != "0.30" {
		t.Fatalf("0.10 + 0.20 debe ser 0.30 exacto, got %s", b.TotalCash)
	}
}

func TestCapital_NegativeCapitalAllowed_WhenSettingEnabled(t *testing.T) {
	f := newFixture(t)
	tok := f.ownerToken(t)

	settings := map[string]any{"currency": "PEN", "timezone": "America/Lima", "late_fee_mode": "fixed",
		"late_fee_amount": "10.00", "late_fee_per_day": false, "grace_days": 0,
		"allocation_order": "late_fee_interest_principal", "max_active_loans_per_customer": 3,
		"allow_negative_capital": true}
	expectStatus(t, do(t, "PUT", "/settings", settings, bearer(tok)), 200)

	expectStatus(t, do(t, "POST", "/capital/movements", map[string]string{"type": "withdrawal", "amount": "500"}, bearer(tok)), 201)
	if b := capGetBalance(t, tok); b.TotalCash != "-500.00" {
		t.Fatalf("con capital negativo permitido el saldo debe ser -500.00, got %s", b.TotalCash)
	}
}
