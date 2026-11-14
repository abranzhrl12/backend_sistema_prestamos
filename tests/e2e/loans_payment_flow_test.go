//go:build e2e

package e2e

import (
	"testing"
	"time"
)

func TestLoans_PaymentPersistsOnceAndPreservesCash(t *testing.T) {
	f := newFixture(t)
	tok := f.ownerToken(t)
	customer := do(t, "POST", "/customers", newCustomer("Cliente préstamo"), bearer(tok))
	expectStatus(t, customer, 201)
	var c struct {
		ID string `json:"id"`
	}
	customer.decode(t, &c)
	body := map[string]any{"customer_id": c.ID, "principal": "500.00", "interest_type": "fixed", "interest_value": "25", "term_value": 2, "term_unit": "months", "frequency": "monthly", "disbursement_date": time.Now().Format("2006-01-02")}
	expectStatus(t, do(t, "POST", "/loans", body, bearer(tok)), 409)
	capPost(t, tok, "contribution", "1000.00")
	created := do(t, "POST", "/loans", body, bearer(tok))
	expectStatus(t, created, 201)
	var loan struct {
		ID string `json:"id"`
	}
	created.decode(t, &loan)
	expectStatus(t, do(t, "POST", "/loans/"+loan.ID+"/disburse", nil, bearer(tok)), 200)
	payment := map[string]any{"amount": "50.00", "parts": []map[string]string{{"method": "cash", "amount": "50.00"}}}
	path := "/loans/" + loan.ID + "/payments"
	first := do(t, "POST", path, payment, bearer(tok), header("Idempotency-Key", "payment-once"))
	if first.Status != 200 && first.Status != 201 {
		t.Fatalf("primer pago: %d %s", first.Status, first.Body)
	}
	var paid struct {
		ID string `json:"id"`
	}
	first.decode(t, &paid)
	if paid.ID == "" {
		t.Fatal("el pago debe tener id")
	}
	before := capGetBalance(t, tok)
	again := do(t, "POST", path, payment, bearer(tok), header("Idempotency-Key", "payment-once"))
	if again.Status != 200 && again.Status != 201 {
		t.Fatalf("reintento: %d %s", again.Status, again.Body)
	}
	var replay struct {
		ID string `json:"id"`
	}
	again.decode(t, &replay)
	after := capGetBalance(t, tok)
	if replay.ID != paid.ID || before.TotalCash != after.TotalCash || before.CapitalBalance != after.CapitalBalance || before.ProfitBalance != after.ProfitBalance || before.MovementsCount != after.MovementsCount {
		t.Fatalf("reintento duplicó pago o cambió caja: %s, %s", first.Body, again.Body)
	}
	if after.TotalCash != "550.00" {
		t.Fatalf("caja esperada 550.00: %+v", after)
	}
	payment["amount"] = "51.00"
	payment["parts"] = []map[string]string{{"method": "cash", "amount": "51.00"}}
	expectStatus(t, do(t, "POST", path, payment, bearer(tok), header("Idempotency-Key", "payment-once")), 409)
}
