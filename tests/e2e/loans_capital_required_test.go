//go:build e2e

package e2e

import "testing"

func TestLoans_RequireOwnAvailableCapitalAtCreationAndDelivery(t *testing.T) {
	f := newFixture(t)
	token := f.ownerToken(t)
	cr := do(t, "POST", "/customers", newCustomer("Cliente requiere capital"), bearer(token))
	expectStatus(t, cr, 201)
	var customer struct {
		ID string `json:"id"`
	}
	cr.decode(t, &customer)
	body := map[string]any{"customer_id": customer.ID, "principal": "200", "interest_type": "none", "interest_value": "0", "installments_count": 1, "frequency": "single", "first_payment_date": "2026-12-15", "disbursement_date": "2026-11-14"}
	expectStatus(t, do(t, "POST", "/loans/simulate", body, bearer(token)), 200)
	other := newFixture(t)
	capPost(t, other.ownerToken(t), "contribution", "1000")
	rejected := do(t, "POST", "/loans", body, bearer(token))
	expectStatus(t, rejected, 409)
	if rejected.apiErr(t).Error.Code != "INSUFFICIENT_CAPITAL" {
		t.Fatalf("unexpected error: %s", rejected.Body)
	}
	var count int
	if err := env.Admin.QueryRow(t.Context(), `SELECT count(*) FROM loans WHERE tenant_id=$1`, f.TenantID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("rejected loan must not persist")
	}
	if _, err := env.Admin.Exec(t.Context(), `UPDATE tenant_settings SET allow_negative_capital=true WHERE tenant_id=$1`, f.TenantID); err != nil {
		t.Fatal(err)
	}
	expectStatus(t, do(t, "POST", "/loans", body, bearer(token)), 409)
	capPost(t, token, "contribution", "100")
	expectStatus(t, do(t, "POST", "/loans", body, bearer(token)), 409)
	capPost(t, token, "contribution", "100")
	created := do(t, "POST", "/loans", body, bearer(token))
	expectStatus(t, created, 201)
	var loan struct {
		ID string `json:"id"`
	}
	created.decode(t, &loan)
	if balance := capGetBalance(t, token); balance.TotalCash != "200.00" {
		t.Fatalf("creation must not deduct cash: %+v", balance)
	}
	capPost(t, token, "withdrawal", "1")
	expectStatus(t, do(t, "POST", "/loans/"+loan.ID+"/disburse", nil, bearer(token)), 409)
	capPost(t, token, "contribution", "1")
	expectStatus(t, do(t, "POST", "/loans/"+loan.ID+"/disburse", nil, bearer(token)), 200)
	if balance := capGetBalance(t, token); balance.TotalCash != "0.00" {
		t.Fatalf("delivery must deduct once: %+v", balance)
	}
}
