//go:build e2e

package e2e

import (
	"context"
	"testing"
)

func TestCollections_InterestPrincipalAndExtension(t *testing.T) {
	f := newFixture(t)
	tok := f.ownerToken(t)
	capPost(t, tok, "contribution", "100")
	customer := do(t, "POST", "/customers", newCustomer("Cobros prueba"), bearer(tok))
	expectStatus(t, customer, 201)
	var c struct {
		ID string `json:"id"`
	}
	customer.decode(t, &c)
	body := map[string]any{"customer_id": c.ID, "principal": "100", "interest_type": "percentage", "interest_value": "20", "installments_count": 1, "first_payment_date": "2026-12-15", "frequency": "monthly", "disbursement_date": "2026-11-14"}
	created := do(t, "POST", "/loans", body, bearer(tok))
	expectStatus(t, created, 201)
	var loan struct {
		ID string `json:"id"`
	}
	created.decode(t, &loan)
	viewer := loginBearer(t, f.Slug, f.Cobrador)
	expectStatus(t, do(t, "GET", "/collections", nil, bearer(viewer)), 403)
	expectStatus(t, do(t, "POST", "/loans/"+loan.ID+"/extensions", map[string]string{}, bearer(viewer)), 403)
	expectStatus(t, do(t, "GET", "/collections", nil), 401)
	expectStatus(t, do(t, "GET", "/collections?status=wrong", nil, bearer(tok)), 400)
	list := do(t, "GET", "/collections", nil, bearer(tok))
	expectStatus(t, list, 200)
	var page struct {
		Items []struct {
			LoanID  string `json:"loan_id"`
			Balance string `json:"balance"`
		} `json:"items"`
	}
	list.decode(t, &page)
	if len(page.Items) != 0 {
		t.Fatal("pending loans must not be collectible")
	}
	expectStatus(t, do(t, "POST", "/loans/"+loan.ID+"/disburse", nil, bearer(tok)), 200)
	list = do(t, "GET", "/collections?q=Cobros", nil, bearer(tok))
	expectStatus(t, list, 200)
	list.decode(t, &page)
	if len(page.Items) != 1 || page.Items[0].Balance != "120.0000" {
		t.Fatalf("collections: %s", list.Body)
	}
	bad := map[string]any{"amount": "20", "distribution": map[string]string{"principal": "0", "interest": "19", "late_fee": "0"}, "parts": []map[string]string{{"method": "cash", "amount": "20"}}}
	expectStatus(t, do(t, "POST", "/loans/"+loan.ID+"/payments", bad, bearer(tok), header("Idempotency-Key", "bad-sum")), 400)
	pay := map[string]any{"amount": "70", "distribution": map[string]string{"principal": "50", "interest": "20", "late_fee": "0"}, "parts": []map[string]string{{"method": "cash", "amount": "20"}, {"method": "yape", "amount": "50"}}}
	path := "/loans/" + loan.ID + "/payments"
	paid := do(t, "POST", path, pay, bearer(tok), header("Idempotency-Key", "manual-payment"))
	expectStatus(t, paid, 201)
	expectStatus(t, do(t, "POST", path, pay, bearer(tok), header("Idempotency-Key", "manual-payment")), 201)
	list = do(t, "GET", "/collections", nil, bearer(tok))
	list.decode(t, &page)
	if len(page.Items) != 1 || page.Items[0].Balance != "50.0000" {
		t.Fatalf("balance: %s", list.Body)
	}
	pay["distribution"] = map[string]string{"principal": "49", "interest": "21", "late_fee": "0"}
	expectStatus(t, do(t, "POST", path, pay, bearer(tok), header("Idempotency-Key", "invalid-distribution")), 409)
	ext := map[string]string{"first_payment_date": "2027-01-15", "additional_interest": "10", "note": "Nuevo plazo acordado"}
	ep := "/loans/" + loan.ID + "/extensions"
	login := do(t, "POST", "/auth/login", map[string]string{"email": f.Owner, "password": pass, "transport": "cookie"})
	expectStatus(t, login, 200)
	access, csrf := cookieAt(login, "access_token", "/"), cookieAt(login, "csrf_token", "/")
	expectStatus(t, do(t, "POST", ep, ext, cookie("access_token", access)), 403)
	expectStatus(t, do(t, "GET", "/collections", nil, cookie("access_token", access)), 200)
	expectStatus(t, do(t, "POST", ep, ext, bearer(tok), header("Idempotency-Key", "extend-once")), 201)
	expectStatus(t, do(t, "POST", ep, ext, cookie("access_token", access), cookie("csrf_token", csrf), header("X-CSRF-Token", csrf), header("Idempotency-Key", "extend-once")), 201)
	list = do(t, "GET", "/collections", nil, bearer(tok))
	list.decode(t, &page)
	if len(page.Items) != 1 || page.Items[0].Balance != "60.0000" {
		t.Fatalf("extension duplicated: %s", list.Body)
	}
	history := do(t, "GET", ep, nil, bearer(tok))
	expectStatus(t, history, 200)
	var hist struct {
		Items []map[string]any `json:"items"`
	}
	history.decode(t, &hist)
	if len(hist.Items) != 1 {
		t.Fatalf("history: %s", history.Body)
	}
	if _, err := env.Admin.Exec(context.Background(), `UPDATE loan_extensions SET note='edited' WHERE loan_id=$1::uuid`, loan.ID); err == nil {
		t.Fatal("agreements must be immutable")
	}
	ext["note"] = ""
	expectStatus(t, do(t, "POST", ep, ext, bearer(tok), header("Idempotency-Key", "empty-note")), 400)
	ext["note"] = "Nuevo plazo acordado"
	ext["first_payment_date"] = "2026-12-01"
	expectStatus(t, do(t, "POST", ep, ext, bearer(tok), header("Idempotency-Key", "invalid-date")), 409)
	other := newFixture(t).ownerToken(t)
	expectStatus(t, do(t, "GET", ep, nil, bearer(other)), 404)
	otherList := do(t, "GET", "/collections", nil, bearer(other))
	otherList.decode(t, &page)
	if len(page.Items) != 0 {
		t.Fatal("cross tenant collections")
	}
}
