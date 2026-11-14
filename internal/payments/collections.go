package payments

import (
	"context"
	"github.com/jackc/pgx/v5"
)

type Collection struct {
	ID           string `json:"id"`
	LoanID       string `json:"loan_id"`
	CustomerName string `json:"customer_name"`
	Number       int    `json:"installment_no"`
	DueDate      string `json:"due_date"`
	Principal    string `json:"principal_balance"`
	Interest     string `json:"interest_balance"`
	LateFee      string `json:"late_fee_balance"`
	Balance      string `json:"balance"`
	Status       string `json:"status"`
}
type CollectionQuery struct {
	Status, Search string
	Limit, Offset  int
}

func (r *Repo) Collections(ctx context.Context, tenant string, q CollectionQuery) (out []Collection, more bool, err error) {
	out = []Collection{}
	err = r.tx.InTenant(ctx, tenant, func(tx pgx.Tx) error {
		rows, e := tx.Query(ctx, `SELECT s.id::text,l.id::text,c.full_name,s.installment_no,s.due_date::text,
   (s.principal_due-s.principal_paid)::text,(s.interest_due-s.interest_paid)::text,(s.late_fee_accrued-s.late_fee_paid)::text,
   (s.principal_due-s.principal_paid+s.interest_due-s.interest_paid+s.late_fee_accrued-s.late_fee_paid)::text,
   CASE WHEN s.due_date < (now() AT TIME ZONE coalesce((SELECT timezone FROM tenant_settings),'America/Lima'))::date THEN 'overdue'
    WHEN s.due_date = (now() AT TIME ZONE coalesce((SELECT timezone FROM tenant_settings),'America/Lima'))::date THEN 'today' ELSE 'upcoming' END
   FROM loan_schedule s JOIN loans l ON l.id=s.loan_id JOIN customers c ON c.id=l.customer_id
   WHERE l.status='active' AND c.deleted_at IS NULL
   AND (s.principal_due>s.principal_paid OR s.interest_due>s.interest_paid OR s.late_fee_accrued>s.late_fee_paid)
   AND ($1='' OR c.full_name ILIKE '%'||$1||'%' OR c.document_id ILIKE '%'||$1||'%')
   AND ($2='all' OR ($2='overdue' AND s.due_date < (now() AT TIME ZONE coalesce((SELECT timezone FROM tenant_settings),'America/Lima'))::date)
    OR ($2='today' AND s.due_date = (now() AT TIME ZONE coalesce((SELECT timezone FROM tenant_settings),'America/Lima'))::date)
    OR ($2='upcoming' AND s.due_date > (now() AT TIME ZONE coalesce((SELECT timezone FROM tenant_settings),'America/Lima'))::date))
   ORDER BY s.due_date,s.id LIMIT $3 OFFSET $4`, q.Search, q.Status, q.Limit+1, q.Offset)
		if e != nil {
			return e
		}
		defer rows.Close()
		for rows.Next() {
			var row Collection
			if e = rows.Scan(&row.ID, &row.LoanID, &row.CustomerName, &row.Number, &row.DueDate, &row.Principal, &row.Interest, &row.LateFee, &row.Balance, &row.Status); e != nil {
				return e
			}
			out = append(out, row)
		}
		return rows.Err()
	})
	if len(out) > q.Limit {
		more = true
		out = out[:q.Limit]
	}
	return
}
