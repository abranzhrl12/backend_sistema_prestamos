//go:build e2e

package e2e

import "testing"

func TestPersonalLoan_DirectCreationAndPayment(t *testing.T) {
	f := newFixture(t)
	token := f.ownerToken(t)
	cr := do(t, "POST", "/customers", newCustomer("Cliente préstamo personal"), bearer(token))
	expectStatus(t, cr, 201)
	var customer struct {
		ID string `json:"id"`
	}
	cr.decode(t, &customer)
	body := map[string]any{"customer_id": customer.ID, "principal": "1000", "interest_type": "percentage", "interest_value": "10", "term_value": 3, "term_unit": "months", "frequency": "monthly", "disbursement_date": "2026-01-31"}
	capPost(t, token, "contribution", "1000")
	created := do(t, "POST", "/loans", body, bearer(token))
	expectStatus(t, created, 201)
	var loan struct {
		ID string `json:"id"`
	}
	created.decode(t, &loan)
	var productIsNull bool
	if e := env.Admin.QueryRow(t.Context(), `SELECT product_id IS NULL FROM loans WHERE id=$1`, loan.ID).Scan(&productIsNull); e != nil {
		t.Fatal(e)
	}
	if !productIsNull {
		t.Fatal("préstamo directo requiere producto nulo")
	}
	expectStatus(t, do(t, "POST", "/loans/"+loan.ID+"/disburse", nil, bearer(token)), 200)
	payment := map[string]any{"amount": "50", "parts": []map[string]string{{"method": "cash", "amount": "50"}}}
	path := "/loans/" + loan.ID + "/payments"
	first := do(t, "POST", path, payment, bearer(token), header("Idempotency-Key", "direct-payment"))
	expectStatus(t, first, 201)
	again := do(t, "POST", path, payment, bearer(token), header("Idempotency-Key", "direct-payment"))
	expectStatus(t, again, 201)
	var a, b struct {
		ID string `json:"id"`
	}
	first.decode(t, &a)
	again.decode(t, &b)
	if a.ID == "" || a.ID != b.ID {
		t.Fatalf("reintento duplicado: %s %s", first.Body, again.Body)
	}
	balance := capGetBalance(t, token)
	if balance.TotalCash != "50.00" {
		t.Fatalf("caja después del pago: %+v", balance)
	}
	history := do(t, "GET", path, nil, bearer(token))
	expectStatus(t, history, 200)
	var h struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	history.decode(t, &h)
	if len(h.Items) != 1 || h.Items[0].ID != a.ID {
		t.Fatalf("historial: %s", history.Body)
	}
}
