package postgres

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"prestamos/internal/loans/app"
)

const loanCols = `id::text, customer_id::text, coalesce(product_id::text,''), principal::text, coalesce(rate::text,''), coalesce(rate_period,''), interest_type, frequency, term_count, status, coalesce(disbursement_date::text,''),CASE WHEN term_value IS NULL THEN annual_rate::text ELSE '' END,coalesce((SELECT full_name FROM customers WHERE customers.id=loans.customer_id),''),coalesce(interest_value::text,''),coalesce(term_value,0),coalesce(term_unit,'')`

func scan(row pgx.Row) (l app.Loan, e error) {
	e = row.Scan(&l.ID, &l.CustomerID, &l.ProductID, &l.Principal, &l.Rate, &l.RatePeriod, &l.InterestType, &l.Frequency, &l.TermCount, &l.Status, &l.DisbursementDate, &l.AnnualRate, &l.CustomerName, &l.InterestValue, &l.TermValue, &l.TermUnit)
	return
}
func (r *Repo) Disburse(ctx context.Context, a app.Actor, id string) (l app.Loan, err error) {
	err = r.tx.InTenant(ctx, a.TenantID, func(tx pgx.Tx) error {
		if _, e := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1::text,0))`, a.TenantID); e != nil {
			return e
		}
		var e error
		l, e = scan(tx.QueryRow(ctx, `SELECT `+loanCols+` FROM loans WHERE id=$1::uuid FOR UPDATE`, id))
		if errors.Is(e, pgx.ErrNoRows) {
			return app.ErrNotFound
		}
		if e != nil {
			return e
		}
		if l.Status != "pending" {
			return app.ErrInvalidStatus
		}
		if e := requireCapital(ctx, tx, l.Principal); e != nil {
			return e
		}
		_, e = tx.Exec(ctx, `UPDATE loans SET status='active',disbursed_at=now(),updated_at=now() WHERE id=$1::uuid`, id)
		if e != nil {
			return e
		}
		mid, e := r.ids.New()
		if e != nil {
			return e
		}
		_, e = tx.Exec(ctx, `INSERT INTO capital_movements(id,tenant_id,type,amount,loan_id,created_by) VALUES($1::uuid,$2::uuid,'disbursement',$3::numeric,$4::uuid,$5::uuid)`, mid, a.TenantID, l.Principal, id, a.UserID)
		if e != nil {
			return e
		}
		aid, e := r.ids.New()
		if e != nil {
			return e
		}
		_, e = tx.Exec(ctx, `INSERT INTO audit_log(id,tenant_id,user_id,action,entity,entity_id) VALUES($1::uuid,$2::uuid,$3::uuid,'loan.disburse','loan',$4::uuid)`, aid, a.TenantID, a.UserID, id)
		l.Status = "active"
		return e
	})
	return
}
func (r *Repo) List(ctx context.Context, tenant string, limit, offset int, includeTotal bool) (out []app.Loan, more bool, total *int, err error) {
	out = []app.Loan{}
	err = r.tx.InTenant(ctx, tenant, func(tx pgx.Tx) error {
		rows, e := tx.Query(ctx, `SELECT `+loanCols+` FROM loans ORDER BY loans.id DESC LIMIT $1 OFFSET $2`, limit+1, offset)
		if e != nil {
			return e
		}
		defer rows.Close()
		for rows.Next() {
			l, e := scan(rows)
			if e != nil {
				return e
			}
			out = append(out, l)
		}
		if e = rows.Err(); e != nil {
			return e
		}
		if len(out) > limit {
			more = true
			out = out[:limit]
		}
		rows.Close()
		if includeTotal {
			var n int
			if e = tx.QueryRow(ctx, `SELECT count(*) FROM loans`).Scan(&n); e != nil {
				return e
			}
			total = &n
		}
		return nil
	})
	return
}
func (r *Repo) Schedule(ctx context.Context, tenant, id string) (out []app.ScheduleRow, err error) {
	out = []app.ScheduleRow{}
	err = r.tx.InTenant(ctx, tenant, func(tx pgx.Tx) error {
		var exists bool
		if e := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM loans WHERE id=$1::uuid)`, id).Scan(&exists); e != nil {
			return e
		}
		if !exists {
			return app.ErrNotFound
		}
		rows, e := tx.Query(ctx, `SELECT id::text,installment_no,due_date::text,principal_due::text,interest_due::text,late_fee_accrued::text,principal_paid::text,interest_paid::text,late_fee_paid::text FROM loan_schedule WHERE loan_id=$1::uuid ORDER BY installment_no`, id)
		if e != nil {
			return e
		}
		defer rows.Close()
		for rows.Next() {
			var v app.ScheduleRow
			if e = rows.Scan(&v.ID, &v.Number, &v.DueDate, &v.PrincipalDue, &v.InterestDue, &v.LateFeeAccrued, &v.PrincipalPaid, &v.InterestPaid, &v.LateFeePaid); e != nil {
				return e
			}
			out = append(out, v)
		}
		return rows.Err()
	})
	return
}
