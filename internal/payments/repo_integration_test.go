//go:build payment_integration

package payments_test

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"os"
	"prestamos/internal/payments"
	"prestamos/internal/platform/db"
	"prestamos/internal/platform/ids"
	"prestamos/tests/testenv"
	"sync"
	"testing"
)

func TestPaymentTransaction(t *testing.T) {
	if os.Getenv("TEST_DB_PORT") != "55433" {
		t.Fatal("TEST_DB_PORT debe ser 55433 para esta prueba")
	}
	ctx := context.Background()
	env, e := testenv.Start(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer env.Close()
	tenant, e := env.Tenant(ctx, "payments-test")
	if e != nil {
		t.Fatal(e)
	}
	actor, e := env.User(ctx, tenant, "owner@payments.test", "password", "owner")
	if e != nil {
		t.Fatal(e)
	}
	customer, loan, first, second := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	statements := []struct {
		q string
		a []any
	}{
		{`INSERT INTO customers(id,tenant_id,full_name,document_id) VALUES($1,$2,'Cliente','12345678')`, []any{customer, tenant}},
		{`INSERT INTO loans(id,tenant_id,customer_id,created_by,principal,annual_rate,interest_type,term_count,frequency,status) VALUES($1,$2,$3,$4,100,10,'flat',2,'monthly','active')`, []any{loan, tenant, customer, actor}},
		{`INSERT INTO loan_schedule(id,tenant_id,loan_id,installment_no,due_date,principal_due,interest_due,late_fee_accrued) VALUES($1,$2,$3,1,current_date-10,50,10,5),($4,$2,$3,2,current_date+10,50,10,0)`, []any{first, tenant, loan, second}},
	}
	for _, s := range statements {
		if _, e = env.Admin.Exec(ctx, s.q, s.a...); e != nil {
			t.Fatal(e)
		}
	}
	repo := payments.NewRepo(db.NewPgRunner(env.App), ids.UUIDv7{})
	d := decimal.RequireFromString
	input := payments.Input{LoanID: loan, UserID: actor, IdempotencyKey: "one", Amount: d("20"), Parts: []payments.Part{{Method: "yape", Reference: "REF-1", Amount: d("20")}}}
	var wg sync.WaitGroup
	results := make([]payments.Payment, 2)
	errs := make([]error, 2)
	for n := range 2 {
		wg.Add(1)
		go func(n int) { defer wg.Done(); results[n], errs[n] = repo.Record(ctx, tenant, input) }(n)
	}
	wg.Wait()
	for _, e := range errs {
		if e != nil {
			t.Fatalf("reintento concurrente: %v", e)
		}
	}
	if results[0].ID != results[1].ID {
		t.Fatal("reintento creó pagos distintos")
	}
	if len(results[0].Allocations) != 1 || !results[0].Allocations[0].LateFee.Equal(d("5")) || !results[0].Allocations[0].Interest.Equal(d("10")) || !results[0].Allocations[0].Principal.Equal(d("5")) {
		t.Fatalf("reparto incorrecto: %+v", results[0].Allocations)
	}
	var count int
	var total string
	if e = env.Admin.QueryRow(ctx, `SELECT count(*),coalesce(sum(amount),0)::text FROM capital_movements WHERE payment_id=$1`, results[0].ID).Scan(&count, &total); e != nil {
		t.Fatal(e)
	}
	if count != 3 || !d(total).Equal(d("20")) {
		t.Fatalf("movimientos: %d %s", count, total)
	}
	changed := input
	changed.Amount = d("21")
	changed.Parts = []payments.Part{{Method: "cash", Amount: d("21")}}
	if _, e = repo.Record(ctx, tenant, changed); !errors.Is(e, payments.ErrIdempotencyConflict) {
		t.Fatalf("conflicto clave: %v", e)
	}
	duplicate := input
	duplicate.IdempotencyKey = "two"
	if _, e = repo.Record(ctx, tenant, duplicate); !errors.Is(e, payments.ErrDuplicateReference) {
		t.Fatalf("referencia repetida: %v", e)
	}
	invalid := input
	invalid.IdempotencyKey = "three"
	invalid.Parts = []payments.Part{{Method: "cash", Amount: d("19")}}
	if _, e = repo.Record(ctx, tenant, invalid); !errors.Is(e, payments.ErrInvalidInput) {
		t.Fatalf("partes: %v", e)
	}
	over := input
	over.IdempotencyKey = "four"
	over.Amount = d("1000")
	over.Parts = []payments.Part{{Method: "cash", Amount: d("1000")}}
	if _, e = repo.Record(ctx, tenant, over); !errors.Is(e, payments.ErrOverpayment) {
		t.Fatalf("sobrepago: %v", e)
	}
	var paymentsCount, allocCount, auditCount int
	if e = env.Admin.QueryRow(ctx, `SELECT (SELECT count(*) FROM payments),(SELECT count(*) FROM payment_allocations),(SELECT count(*) FROM audit_log WHERE action='payment.create')`).Scan(&paymentsCount, &allocCount, &auditCount); e != nil {
		t.Fatal(e)
	}
	if paymentsCount != 1 || allocCount != 1 || auditCount != 1 {
		t.Fatalf("rollback incompleto: %d %d %d", paymentsCount, allocCount, auditCount)
	}
	// A failure after allocations and cash movements must roll back every write.
	if _, e = env.Admin.Exec(ctx, `CREATE FUNCTION test_reject_payment_audit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'forced audit failure'; END $$`); e != nil {
		t.Fatal(e)
	}
	if _, e = env.Admin.Exec(ctx, `CREATE TRIGGER test_reject_payment_audit BEFORE INSERT ON audit_log FOR EACH ROW EXECUTE FUNCTION test_reject_payment_audit()`); e != nil {
		t.Fatal(e)
	}
	fail := input
	fail.IdempotencyKey = "forced"
	fail.Parts = []payments.Part{{Method: "cash", Amount: d("20")}}
	if _, e = repo.Record(ctx, tenant, fail); e == nil {
		t.Fatal("se esperaba falla de auditoría")
	}
	if _, e = env.Admin.Exec(ctx, `DROP TRIGGER test_reject_payment_audit ON audit_log`); e != nil {
		t.Fatal(e)
	}
	var paid string
	var moveCount int
	if e = env.Admin.QueryRow(ctx, `SELECT principal_paid::text,(SELECT count(*) FROM capital_movements) FROM loan_schedule WHERE id=$1`, first).Scan(&paid, &moveCount); e != nil {
		t.Fatal(e)
	}
	if !d(paid).Equal(d("5")) || moveCount != 3 {
		t.Fatalf("rollback tardío incompleto: principal %s, movimientos %d", paid, moveCount)
	}
	if e = env.Admin.QueryRow(ctx, `SELECT count(*) FROM payments`).Scan(&paymentsCount); e != nil {
		t.Fatal(e)
	}
	if paymentsCount != 1 {
		t.Fatalf("pago no revertido: %d", paymentsCount)
	}
	// A tenant may choose interest, principal, then late fee.
	if _, e = env.Admin.Exec(ctx, `INSERT INTO tenant_settings(tenant_id,allocation_order) VALUES($1,'interest_principal_late_fee')`, tenant); e != nil {
		t.Fatal(e)
	}
	if _, e = env.Admin.Exec(ctx, `UPDATE loan_schedule SET interest_due=interest_due+5,late_fee_accrued=late_fee_accrued+5 WHERE id=$1`, first); e != nil {
		t.Fatal(e)
	}
	configured := input
	configured.IdempotencyKey = "configured"
	configured.Amount = d("10")
	configured.Parts = []payments.Part{{Method: "cash", Amount: d("10")}}
	got, e := repo.Record(ctx, tenant, configured)
	if e != nil {
		t.Fatal(e)
	}
	if len(got.Allocations) != 1 || !got.Allocations[0].Interest.Equal(d("5")) || !got.Allocations[0].Principal.Equal(d("5")) || !got.Allocations[0].LateFee.IsZero() {
		t.Fatalf("orden configurado: %+v", got.Allocations)
	}
	other, e := env.Tenant(ctx, "payments-other")
	if e != nil {
		t.Fatal(e)
	}
	if _, _, _, e = repo.List(ctx, other, loan, 20, 0, false); !errors.Is(e, payments.ErrNotFound) {
		t.Fatalf("aislamiento: %v", e)
	}
	page, more, pageTotal, e := repo.List(ctx, tenant, loan, 1, 0, true)
	if e != nil {
		t.Fatal(e)
	}
	if len(page) != 1 || !more || pageTotal == nil || *pageTotal != 2 {
		t.Fatalf("página 1: %d %v %v", len(page), more, pageTotal)
	}
	next, more, pageTotal, e := repo.List(ctx, tenant, loan, 1, 1, false)
	if e != nil {
		t.Fatal(e)
	}
	if len(next) != 1 || more || pageTotal != nil || next[0].ID == page[0].ID {
		t.Fatal("página 2 incorrecta")
	}
}
