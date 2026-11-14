//go:build e2e

package e2e

import "testing"

func TestLoanPaymentCount_ScheduleAndValidation(t *testing.T) {
	f := newFixture(t)
	token := f.ownerToken(t)
	body := map[string]any{"principal": "300", "interest_type": "percentage", "interest_value": "20", "installments_count": 5, "frequency": "daily", "first_payment_date": "2026-11-15", "disbursement_date": "2026-11-14"}
	r := do(t, "POST", "/loans/simulate", body, bearer(token))
	expectStatus(t, r, 200)
	var result struct {
		Schedule []struct {
			DueDate string `json:"due_date"`
			Total   string `json:"total_due"`
		} `json:"schedule"`
		Total string `json:"total_due"`
	}
	r.decode(t, &result)
	if len(result.Schedule) != 5 || result.Total != "360.0000" || result.Schedule[0].DueDate != "2026-11-15" || result.Schedule[4].DueDate != "2026-11-19" {
		t.Fatalf("wrong schedule: %s", r.Body)
	}
	for _, row := range result.Schedule {
		if row.Total != "72.0000" {
			t.Fatalf("wrong installment: %s", r.Body)
		}
	}
	expectStatus(t, do(t, "POST", "/loans/simulate", body), 401)
	for _, tc := range []struct {
		key   string
		value any
	}{{"installments_count", 0}, {"installments_count", 1001}, {"first_payment_date", "2026-11-13"}, {"first_payment_date", "bad"}, {"frequency", "single"}, {"term_value", 5}} {
		old, exists := body[tc.key]
		body[tc.key] = tc.value
		expectStatus(t, do(t, "POST", "/loans/simulate", body, bearer(token)), 400)
		if exists {
			body[tc.key] = old
		} else {
			delete(body, tc.key)
		}
	}
	cr := do(t, "POST", "/customers", newCustomer("Cliente cinco pagos"), bearer(token))
	expectStatus(t, cr, 201)
	var customer struct {
		ID string `json:"id"`
	}
	cr.decode(t, &customer)
	body["customer_id"] = customer.ID
	capPost(t, token, "contribution", "300")
	created := do(t, "POST", "/loans", body, bearer(token))
	expectStatus(t, created, 201)
	var loan struct {
		ID    string `json:"id"`
		Count int    `json:"term_count"`
	}
	created.decode(t, &loan)
	if loan.Count != 5 {
		t.Fatal("loan must retain five installments")
	}
	stored := do(t, "GET", "/loans/"+loan.ID+"/schedule", nil, bearer(token))
	expectStatus(t, stored, 200)
	var schedule struct {
		Items []struct {
			DueDate string `json:"due_date"`
		} `json:"items"`
	}
	stored.decode(t, &schedule)
	if len(schedule.Items) != 5 || schedule.Items[4].DueDate != "2026-11-19" {
		t.Fatalf("persisted schedule differs: %s", stored.Body)
	}
	body["frequency"] = "single"
	body["installments_count"] = 1
	body["first_payment_date"] = "2026-11-20"
	expectStatus(t, do(t, "POST", "/loans/simulate", body, bearer(token)), 200)
}
