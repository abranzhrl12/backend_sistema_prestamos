package postgres

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"prestamos/internal/loans/app"
	"prestamos/internal/loans/schedule"
	"prestamos/internal/platform/db"
	"prestamos/internal/platform/ids"
)

type Repo struct {
	tx  db.TxRunner
	ids ids.Generator
}

func NewRepo(tx db.TxRunner, gen ids.Generator) *Repo { return &Repo{tx: tx, ids: gen} }
func (r *Repo) Create(ctx context.Context, actor app.Actor, loan app.Loan, rows []schedule.Installment) (app.Loan, error) {
	id, err := r.ids.New()
	if err != nil {
		return loan, err
	}
	loan.ID = id
	err = r.tx.InTenant(ctx, actor.TenantID, func(tx pgx.Tx) error {
		if _, e := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1::text,0))`, actor.TenantID); e != nil {
			return e
		}
		if e := requireCapital(ctx, tx, loan.Principal); e != nil {
			return e
		}
		var status string
		e := tx.QueryRow(ctx, `SELECT status, full_name FROM customers WHERE id=$1::uuid AND deleted_at IS NULL FOR UPDATE`, loan.CustomerID).Scan(&status, &loan.CustomerName)
		if errors.Is(e, pgx.ErrNoRows) || (e == nil && status != "active") {
			return app.ErrInvalid
		}
		if e != nil {
			return e
		}
		_, e = tx.Exec(ctx, `INSERT INTO loans(id,tenant_id,customer_id,created_by,principal,annual_rate,interest_type,term_count,frequency,status,disbursement_date,interest_value,term_value,term_unit) VALUES($1::uuid,$2::uuid,$3::uuid,$4::uuid,$5::numeric,0,$6,$7,$8,'pending',$9::date,$10::numeric,nullif($11,0),nullif($12,''))`, loan.ID, actor.TenantID, loan.CustomerID, actor.UserID, loan.Principal, loan.InterestType, loan.TermCount, loan.Frequency, loan.DisbursementDate, loan.InterestValue, loan.TermValue, loan.TermUnit)
		if e != nil {
			return e
		}
		for _, row := range rows {
			sid, e := r.ids.New()
			if e != nil {
				return e
			}
			_, e = tx.Exec(ctx, `INSERT INTO loan_schedule(id,tenant_id,loan_id,installment_no,due_date,principal_due,interest_due) VALUES($1::uuid,$2::uuid,$3::uuid,$4,$5,$6::numeric,$7::numeric)`, sid, actor.TenantID, loan.ID, row.Number, row.DueDate, row.Principal.String(), row.Interest.String())
			if e != nil {
				return e
			}
		}
		audit, e := r.ids.New()
		if e != nil {
			return e
		}
		_, e = tx.Exec(ctx, `INSERT INTO audit_log(id,tenant_id,user_id,action,entity,entity_id) VALUES($1::uuid,$2::uuid,$3::uuid,'loan.create','loan',$4::uuid)`, audit, actor.TenantID, actor.UserID, loan.ID)
		return e
	})
	loan.Status = "pending"
	return loan, err
}
