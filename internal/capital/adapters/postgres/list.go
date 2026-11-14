package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"

	"prestamos/internal/capital/domain"
)

const listCols = `capital_movements.id::text, tenant_id::text, type, amount::text, coalesce(note,''),
	coalesce(reference,''), loan_id::text, payment_id::text, created_by::text, created_at`

// List pagina por keyset sobre id DESC (UUIDv7: orden temporal). Pide limit+1 filas para has_more,
// sin COUNT: el libro crece sin límite y OFFSET se degradaría en páginas profundas.
func (r *Repo) List(ctx context.Context, tenantID string, f domain.ListFilter) (res domain.ListResult, err error) {
	where, args := buildListWhere(f)
	query := fmt.Sprintf(`SELECT %s FROM capital_movements%s ORDER BY capital_movements.id DESC LIMIT $%d`,
		listCols, where, len(args)+1)
	err = r.tx.InTenant(ctx, tenantID, func(tx pgx.Tx) error {
		rows, e := tx.Query(ctx, query, append(args, f.Limit+1)...)
		if e != nil {
			return e
		}
		defer rows.Close()
		for rows.Next() {
			m, e := scanMovement(rows)
			if e != nil {
				return e
			}
			res.Items = append(res.Items, m)
		}
		return rows.Err()
	})
	if res.HasMore = len(res.Items) > f.Limit; res.HasMore {
		res.Items = res.Items[:f.Limit]
	}
	return res, opErr("List", err)
}

// buildListWhere agrega solo los filtros presentes.
func buildListWhere(f domain.ListFilter) (string, []any) {
	var conds []string
	var args []any
	add := func(cond string, v any) {
		args = append(args, v)
		conds = append(conds, fmt.Sprintf(cond, len(args)))
	}
	if f.AfterID != "" {
		add("capital_movements.id < $%d::uuid", f.AfterID)
	}
	if f.Type != "" {
		add("type = $%d", string(f.Type))
	}
	if f.From != nil {
		add("created_at >= $%d", *f.From)
	}
	if f.To != nil {
		add("created_at <= $%d", *f.To)
	}
	if len(conds) == 0 {
		return "", nil
	}
	return " WHERE " + strings.Join(conds, " AND "), args
}

type scanner interface{ Scan(dest ...any) error }

func scanMovement(row scanner) (domain.Movement, error) {
	var m domain.Movement
	var typ, amount string
	err := row.Scan(&m.ID, &m.TenantID, &typ, &amount, &m.Note, &m.Reference,
		&m.LoanID, &m.PaymentID, &m.CreatedBy, &m.CreatedAt)
	if err != nil {
		return m, err
	}
	m.Type = domain.Type(typ)
	m.Amount, err = decimal.NewFromString(amount)
	return m, err
}
