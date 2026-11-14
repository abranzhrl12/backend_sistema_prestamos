//go:build e2e

package e2e

import (
	"context"
	"testing"
	"time"
)

func TestLoansPayments_ValidationIsolationAndPagination(t *testing.T) {
	f := newFixture(t)
	owner := f.ownerToken(t)
	customer := do(t, "POST", "/customers", newCustomer("Cliente seguridad préstamo"), bearer(owner))
	expectStatus(t, customer, 201)
	var c struct {
		ID string `json:"id"`
	}
	customer.decode(t, &c)
	loanBody := map[string]any{"customer_id": c.ID, "principal": "500.00", "interest_type": "fixed", "interest_value": "25", "term_value": 2, "term_unit": "months", "frequency": "monthly", "disbursement_date": time.Now().Format("2006-01-02")}
	expectStatus(t, do(t, "POST", "/loans", loanBody), 401)
	viewerEmail := "viewer@" + f.Slug + ".com"
	if _, err := env.User(context.Background(), f.TenantID, viewerEmail, pass, "viewer"); err != nil {
		t.Fatal(err)
	}
	viewer := loginBearer(t, f.Slug, viewerEmail)
	expectStatus(t, do(t, "POST", "/loans", loanBody, bearer(viewer)), 403)
	capPost(t, owner, "contribution", "1000")
	created := do(t, "POST", "/loans", loanBody, bearer(owner))
	expectStatus(t, created, 201)
	var loan struct {
		ID string `json:"id"`
	}
	created.decode(t, &loan)
	expectStatus(t, do(t, "POST", "/loans/"+loan.ID+"/disburse", nil, bearer(owner)), 200)
	path := "/loans/" + loan.ID + "/payments"
	good := map[string]any{"amount": "10", "parts": []map[string]string{{"method": "cash", "amount": "10"}}}
	expectStatus(t, do(t, "POST", path, good), 401)
	expectStatus(t, do(t, "POST", path, good, bearer(viewer), header("Idempotency-Key", "viewer")), 403)
	other := newFixture(t).ownerToken(t)
	expectStatus(t, do(t, "GET", "/loans/"+loan.ID+"/schedule", nil, bearer(other)), 404)
	expectStatus(t, do(t, "GET", path, nil, bearer(other)), 404)
	expectStatus(t, do(t, "POST", path, good, bearer(other), header("Idempotency-Key", "other")), 404)
	mismatch := map[string]any{"amount": "10", "parts": []map[string]string{{"method": "cash", "amount": "9"}}}
	expectStatus(t, do(t, "POST", path, mismatch, bearer(owner), header("Idempotency-Key", "bad-parts")), 400)
	excess := map[string]any{"amount": "9999", "parts": []map[string]string{{"method": "cash", "amount": "9999"}}}
	over := do(t, "POST", path, excess, bearer(owner), header("Idempotency-Key", "excess"))
	expectStatus(t, over, 409)
	if got := over.apiErr(t).Error.Code; got != "PAYMENT_EXCEEDS_BALANCE" {
		t.Fatalf("código sobrepago: %s", got)
	}
	for _, key := range []string{"page-one", "page-two"} {
		expectStatus(t, do(t, "POST", path, good, bearer(owner), header("Idempotency-Key", key)), 201)
	}
	first := do(t, "GET", path+"?limit=1&include_total=true", nil, bearer(owner))
	expectStatus(t, first, 200)
	var page struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
		Meta struct {
			HasMore bool `json:"has_more"`
			Total   int  `json:"total"`
		} `json:"meta"`
	}
	first.decode(t, &page)
	if len(page.Items) != 1 || !page.Meta.HasMore || page.Meta.Total != 2 {
		t.Fatalf("primera página: %+v", page)
	}
	id := page.Items[0].ID
	second := do(t, "GET", path+"?limit=1&offset=1", nil, bearer(owner))
	expectStatus(t, second, 200)
	page.Items = nil
	second.decode(t, &page)
	if len(page.Items) != 1 || page.Meta.HasMore || page.Items[0].ID == id {
		t.Fatalf("segunda página: %+v", page)
	}
	expectStatus(t, do(t, "GET", path+"?limit=101", nil, bearer(owner)), 400)
}
