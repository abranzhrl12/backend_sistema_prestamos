//go:build e2e

package e2e

import (
	"context"
	"testing"
)

type settingsDTO struct {
	Currency                  string  `json:"currency"`
	Timezone                  string  `json:"timezone"`
	LateFeeMode               string  `json:"late_fee_mode"`
	LateFeeAmount             string  `json:"late_fee_amount"`
	LateFeePerDay             bool    `json:"late_fee_per_day"`
	GraceDays                 int     `json:"grace_days"`
	AllocationOrder           string  `json:"allocation_order"`
	MaxActiveLoansPerCustomer int     `json:"max_active_loans_per_customer"`
	AllowNegativeCapital      bool    `json:"allow_negative_capital"`
	IsDefault                 bool    `json:"is_default"`
	UpdatedAt                 *string `json:"updated_at"`
}

// fieldSet devuelve los campos con error de una respuesta 400.
func fieldSet(e apiError) map[string]string {
	m := map[string]string{}
	for _, f := range e.Error.Fields {
		m[f.Field] = f.Message
	}
	return m
}

func validSettingsBody() map[string]any {
	return map[string]any{
		"currency": "USD", "timezone": "America/Bogota", "late_fee_mode": "proportional",
		"late_fee_amount": "2.5", "late_fee_per_day": true, "grace_days": 3,
		"allocation_order": "interest_principal_late_fee", "max_active_loans_per_customer": 5,
		"allow_negative_capital": true,
	}
}

func TestSettings_NewCompanyGetsDefaults(t *testing.T) {
	f := newFixture(t)
	r := do(t, "GET", "/settings", nil, bearer(f.ownerToken(t)))
	expectStatus(t, r, 200)
	var s settingsDTO
	r.decode(t, &s)
	if !s.IsDefault || s.UpdatedAt != nil {
		t.Fatalf("esperaba configuración por defecto (is_default true, updated_at null): %s", r.Body)
	}
	if s.Currency != "PEN" || s.Timezone != "America/Lima" || s.LateFeeMode != "fixed" ||
		s.LateFeeAmount != "10.00" || s.GraceDays != 0 || s.MaxActiveLoansPerCustomer != 3 ||
		s.AllocationOrder != "late_fee_interest_principal" || s.AllowNegativeCapital {
		t.Fatalf("defaults inesperados: %s", r.Body)
	}
}

func TestSettings_CobradorCanReadButNotWrite(t *testing.T) {
	f := newFixture(t)
	cob := f.cobradorToken(t)
	expectStatus(t, do(t, "GET", "/settings", nil, bearer(cob)), 200)

	r := do(t, "PUT", "/settings", validSettingsBody(), bearer(cob))
	expectStatus(t, r, 403)
	if code := r.apiErr(t).Error.Code; code != "FORBIDDEN" {
		t.Fatalf("code %q, esperaba FORBIDDEN", code)
	}
}

func TestSettings_PutThenGetReturnsSavedValues(t *testing.T) {
	f := newFixture(t)
	tok := f.ownerToken(t)

	put := do(t, "PUT", "/settings", validSettingsBody(), bearer(tok))
	expectStatus(t, put, 200)
	var saved settingsDTO
	put.decode(t, &saved)
	if saved.IsDefault || saved.LateFeeAmount != "2.50" || saved.Currency != "USD" {
		t.Fatalf("respuesta del PUT inesperada: %s", put.Body)
	}

	got := do(t, "GET", "/settings", nil, bearer(tok))
	expectStatus(t, got, 200)
	var s settingsDTO
	got.decode(t, &s)
	if s.IsDefault || s.UpdatedAt == nil || s.Timezone != "America/Bogota" || s.LateFeeMode != "proportional" ||
		!s.LateFeePerDay || s.GraceDays != 3 || s.AllocationOrder != "interest_principal_late_fee" ||
		s.MaxActiveLoansPerCustomer != 5 || !s.AllowNegativeCapital || s.LateFeeAmount != "2.50" {
		t.Fatalf("GET no devuelve lo guardado: %s", got.Body)
	}
}

func TestSettings_InvalidValuesReturnFieldErrors(t *testing.T) {
	f := newFixture(t)
	body := validSettingsBody()
	body["currency"] = "EUR"
	body["timezone"] = "Mars/Olympus"
	body["grace_days"] = 31
	body["late_fee_amount"] = "abc"

	r := do(t, "PUT", "/settings", body, bearer(f.ownerToken(t)))
	expectStatus(t, r, 400)
	e := r.apiErr(t)
	if e.Error.Code != "VALIDATION_ERROR" {
		t.Fatalf("code %q, esperaba VALIDATION_ERROR", e.Error.Code)
	}
	got := fieldSet(e)
	for _, want := range []string{"currency", "timezone", "grace_days", "late_fee_amount"} {
		if _, ok := got[want]; !ok {
			t.Fatalf("falta el campo %q con error en %s", want, r.Body)
		}
	}
	if len(got) != 4 {
		t.Fatalf("esperaba exactamente 4 campos con error, got %v", got)
	}
}

func TestSettings_EmptyBodyListsEveryMissingField(t *testing.T) {
	f := newFixture(t)
	r := do(t, "PUT", "/settings", map[string]any{}, bearer(f.ownerToken(t)))
	expectStatus(t, r, 400)
	got := fieldSet(r.apiErr(t))
	want := []string{"currency", "timezone", "late_fee_mode", "late_fee_amount", "late_fee_per_day",
		"grace_days", "allocation_order", "max_active_loans_per_customer", "allow_negative_capital"}
	for _, field := range want {
		if msg, ok := got[field]; !ok || msg != "es obligatorio" {
			t.Fatalf("campo %q: mensaje %q (presente %v), esperaba \"es obligatorio\"", field, msg, ok)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("esperaba %d campos faltantes, got %d: %v", len(want), len(got), got)
	}
}

func TestSettings_TenantsAreIsolated(t *testing.T) {
	a := newFixture(t)
	b := newFixture(t)
	expectStatus(t, do(t, "PUT", "/settings", validSettingsBody(), bearer(a.ownerToken(t))), 200)

	r := do(t, "GET", "/settings", nil, bearer(b.ownerToken(t)))
	expectStatus(t, r, 200)
	var s settingsDTO
	r.decode(t, &s)
	if !s.IsDefault || s.Currency != "PEN" {
		t.Fatalf("la otra empresa no debe ver la configuración ajena: %s", r.Body)
	}
}

func TestSettings_UpdateWritesAuditRow(t *testing.T) {
	f := newFixture(t)
	expectStatus(t, do(t, "PUT", "/settings", validSettingsBody(), bearer(f.ownerToken(t))), 200)

	var n int
	err := env.Admin.QueryRow(context.Background(),
		`SELECT count(*) FROM audit_log WHERE tenant_id=$1::uuid AND action='settings.update'`, f.TenantID).Scan(&n)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("esperaba 1 fila de audit settings.update, hubo %d", n)
	}
}
