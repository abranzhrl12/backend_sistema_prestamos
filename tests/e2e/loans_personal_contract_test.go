//go:build e2e

package e2e

import (
	"context"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"testing"
)

func TestPersonalLoan_SimulationTotalsAndDates(t *testing.T) {
	f := newFixture(t)
	token := f.ownerToken(t)
	cases := []struct {
		name, kind, value, unit, frequency string
		term                               int
		dates                              []string
		interest                           string
	}{
		{"porcentaje mensual", "percentage", "10", "months", "monthly", 3, []string{"2026-02-28", "2026-03-31", "2026-04-30"}, "100"},
		{"porcentaje semanal", "percentage", "10", "weeks", "weekly", 3, []string{"2026-02-07", "2026-02-14", "2026-02-21"}, "100"},
		{"monto fijo quincenal", "fixed", "100", "days", "biweekly", 45, []string{"2026-02-15", "2026-03-02", "2026-03-17"}, "100"},
		{"sin interés", "none", "0", "months", "monthly", 1, []string{"2026-02-28"}, "0"},
		{"plazo parcial", "fixed", "100", "days", "weekly", 20, []string{"2026-02-07", "2026-02-14", "2026-02-20"}, "100"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := map[string]any{"principal": "1000", "interest_type": tc.kind, "interest_value": tc.value, "term_value": tc.term, "term_unit": tc.unit, "frequency": tc.frequency, "disbursement_date": "2026-01-31"}
			r := do(t, "POST", "/loans/simulate", body, bearer(token))
			expectStatus(t, r, 200)
			var got struct {
				Schedule []struct {
					DueDate   string `json:"due_date"`
					Principal string `json:"principal_due"`
					Interest  string `json:"interest_due"`
					Balance   string `json:"balance"`
				} `json:"schedule"`
				TotalInterest string `json:"total_interest"`
			}
			r.decode(t, &got)
			if len(got.Schedule) != len(tc.dates) {
				t.Fatalf("cuotas %d esperadas %d: %s", len(got.Schedule), len(tc.dates), r.Body)
			}
			principal, interest := decimal.Zero, decimal.Zero
			for i, row := range got.Schedule {
				if row.DueDate != tc.dates[i] {
					t.Fatalf("fecha %d: %s esperada %s", i, row.DueDate, tc.dates[i])
				}
				p, e := decimal.NewFromString(row.Principal)
				if e != nil {
					t.Fatal(e)
				}
				v, e := decimal.NewFromString(row.Interest)
				if e != nil {
					t.Fatal(e)
				}
				principal = principal.Add(p)
				interest = interest.Add(v)
			}
			if !principal.Equal(decimal.RequireFromString("1000")) || !interest.Equal(decimal.RequireFromString(tc.interest)) || !decimal.RequireFromString(got.TotalInterest).Equal(interest) {
				t.Fatalf("totales principal=%s interés=%s: %s", principal, interest, r.Body)
			}
			if !decimal.RequireFromString(got.Schedule[len(got.Schedule)-1].Balance).IsZero() {
				t.Fatalf("saldo final: %s", r.Body)
			}
		})
	}
}

func TestPersonalLoan_LegacyProductRemainsPayable(t *testing.T) {
	f := newFixture(t)
	token := f.ownerToken(t)
	cr := do(t, "POST", "/customers", newCustomer("Cliente legado"), bearer(token))
	expectStatus(t, cr, 201)
	var customer struct {
		ID string `json:"id"`
	}
	cr.decode(t, &customer)
	pb := productBody("Producto legado")
	pb["fee_type"], pb["fee_value"] = "none", "0"
	product := createProduct(t, token, pb)
	ctx := context.Background()
	var actor string
	if e := env.Admin.QueryRow(ctx, `SELECT id::text FROM users WHERE tenant_id=$1 AND email=$2`, f.TenantID, f.Owner).Scan(&actor); e != nil {
		t.Fatal(e)
	}
	loan, installment := uuid.NewString(), uuid.NewString()
	_, e := env.Admin.Exec(ctx, `INSERT INTO loans(id,tenant_id,customer_id,created_by,principal,annual_rate,interest_type,term_count,frequency,status,product_id,rate,rate_period,disbursement_date) VALUES($1,$2,$3,$4,100,10,'flat',1,'monthly','active',$5,10,'monthly','2026-01-01')`, loan, f.TenantID, customer.ID, actor, product.ID)
	if e != nil {
		t.Fatal(e)
	}
	_, e = env.Admin.Exec(ctx, `INSERT INTO loan_schedule(id,tenant_id,loan_id,installment_no,due_date,principal_due,interest_due) VALUES($1,$2,$3,1,'2026-02-01',100,10)`, installment, f.TenantID, loan)
	if e != nil {
		t.Fatal(e)
	}
	list := do(t, "GET", "/loans", nil, bearer(token))
	expectStatus(t, list, 200)
	var result struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	list.decode(t, &result)
	found := false
	for _, item := range result.Items {
		if item.ID == loan {
			found = true
		}
	}
	if !found {
		t.Fatalf("préstamo legado no visible: %s", list.Body)
	}
	expectStatus(t, do(t, "GET", "/loans/"+loan+"/schedule", nil, bearer(token)), 200)
	payment := map[string]any{"amount": "10", "parts": []map[string]string{{"method": "cash", "amount": "10"}}}
	r := do(t, "POST", "/loans/"+loan+"/payments", payment, bearer(token), header("Idempotency-Key", "legacy-payment"))
	expectStatus(t, r, 201)
	history := do(t, "GET", "/loans/"+loan+"/payments", nil, bearer(token))
	expectStatus(t, history, 200)
	var h struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	history.decode(t, &h)
	if len(h.Items) != 1 {
		t.Fatalf("historial legado: %s", history.Body)
	}
}
