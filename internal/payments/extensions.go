package payments

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
	"strings"
	"time"
)

var ErrExtensionDate = errors.New("la fecha debe ser posterior a la próxima cuota pendiente")

type ExtensionInput struct {
	LoanID, UserID, Key, FirstPaymentDate, Note string
	AdditionalInterest                          decimal.Decimal
}
type Extension struct {
	ID                 string    `json:"id"`
	FirstPaymentDate   string    `json:"first_payment_date"`
	AdditionalInterest string    `json:"additional_interest"`
	Note               string    `json:"note"`
	CreatedAt          time.Time `json:"created_at"`
}

func (r *Repo) Extend(ctx context.Context, tenant string, in ExtensionInput) (out Extension, err error) {
	date, e := time.Parse("2006-01-02", in.FirstPaymentDate)
	if e != nil || strings.TrimSpace(in.Note) == "" || len(in.Note) > 500 || in.Key == "" || len(in.Key) > 200 || in.AdditionalInterest.IsNegative() || !in.AdditionalInterest.Equal(in.AdditionalInterest.Round(4)) || in.AdditionalInterest.GreaterThanOrEqual(decimal.RequireFromString("1000000000000000")) {
		return out, ErrInvalidInput
	}
	hash := extensionHash(in)
	err = r.tx.InTenant(ctx, tenant, func(tx pgx.Tx) error {
		if _, e := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1::text,0))`, tenant); e != nil {
			return e
		}
		var previousHash string
		e := tx.QueryRow(ctx, `SELECT id::text,first_payment_date::text,additional_interest::text,note,created_at,request_hash FROM loan_extensions WHERE idempotency_key=$1`, in.Key).Scan(&out.ID, &out.FirstPaymentDate, &out.AdditionalInterest, &out.Note, &out.CreatedAt, &previousHash)
		if e == nil {
			if hash != previousHash {
				return ErrIdempotencyConflict
			}
			return nil
		}
		if !errors.Is(e, pgx.ErrNoRows) {
			return e
		}
		var status string
		e = tx.QueryRow(ctx, `SELECT status FROM loans WHERE id=$1::uuid FOR UPDATE`, in.LoanID).Scan(&status)
		if errors.Is(e, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if e != nil {
			return e
		}
		if status != "active" {
			return ErrLoanNotActive
		}
		var first time.Time
		var lastID string
		var snapshot []byte
		e = tx.QueryRow(ctx, `SELECT min(due_date), (array_agg(id::text ORDER BY due_date DESC,installment_no DESC))[1],jsonb_agg(to_jsonb(s) ORDER BY due_date,installment_no) FROM loan_schedule s WHERE loan_id=$1::uuid AND (principal_due>principal_paid OR interest_due>interest_paid OR late_fee_accrued>late_fee_paid)`, in.LoanID).Scan(&first, &lastID, &snapshot)
		if e != nil {
			return e
		}
		if !date.After(first) {
			return ErrExtensionDate
		}
		days := int(date.Sub(first).Hours() / 24)
		if _, e = tx.Exec(ctx, `UPDATE loan_schedule SET due_date=due_date+$2::int WHERE loan_id=$1::uuid AND (principal_due>principal_paid OR interest_due>interest_paid OR late_fee_accrued>late_fee_paid)`, in.LoanID, days); e != nil {
			return e
		}
		if _, e = tx.Exec(ctx, `UPDATE loan_schedule SET interest_due=interest_due+$2::numeric WHERE id=$1::uuid`, lastID, in.AdditionalInterest.String()); e != nil {
			return e
		}
		id, e := r.ids.New()
		if e != nil {
			return e
		}
		out = Extension{ID: id, FirstPaymentDate: in.FirstPaymentDate, AdditionalInterest: in.AdditionalInterest.StringFixed(4), Note: in.Note}
		e = tx.QueryRow(ctx, `INSERT INTO loan_extensions(id,tenant_id,loan_id,created_by,previous_schedule,first_payment_date,additional_interest,note,idempotency_key,request_hash) VALUES($1::uuid,$2::uuid,$3::uuid,$4::uuid,$5::jsonb,$6::date,$7::numeric,$8,$9,$10) RETURNING created_at`, id, tenant, in.LoanID, in.UserID, snapshot, in.FirstPaymentDate, in.AdditionalInterest.String(), in.Note, in.Key, hash).Scan(&out.CreatedAt)
		if e != nil {
			return e
		}
		audit, e := r.ids.New()
		if e != nil {
			return e
		}
		_, e = tx.Exec(ctx, `INSERT INTO audit_log(id,tenant_id,user_id,action,entity,entity_id) VALUES($1::uuid,$2::uuid,$3::uuid,'loan.extend','loan',$4::uuid)`, audit, tenant, in.UserID, in.LoanID)
		return e
	})
	return
}

func (r *Repo) Extensions(ctx context.Context, tenant, loanID string, limit, offset int) (out []Extension, more bool, err error) {
	out = []Extension{}
	err = r.tx.InTenant(ctx, tenant, func(tx pgx.Tx) error {
		var exists bool
		if e := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM loans WHERE id=$1::uuid)`, loanID).Scan(&exists); e != nil {
			return e
		}
		if !exists {
			return ErrNotFound
		}
		rows, e := tx.Query(ctx, `SELECT id::text,first_payment_date::text,additional_interest::text,note,created_at FROM loan_extensions WHERE loan_id=$1::uuid ORDER BY id DESC LIMIT $2 OFFSET $3`, loanID, limit+1, offset)
		if e != nil {
			return e
		}
		defer rows.Close()
		for rows.Next() {
			var v Extension
			if e = rows.Scan(&v.ID, &v.FirstPaymentDate, &v.AdditionalInterest, &v.Note, &v.CreatedAt); e != nil {
				return e
			}
			out = append(out, v)
		}
		return rows.Err()
	})
	if len(out) > limit {
		more = true
		out = out[:limit]
	}
	return
}
