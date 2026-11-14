// Package postgres implementa domain.Repository sobre pgx con aislamiento por tenant (RLS).
// Toda consulta excluye las filas eliminadas (deleted_at IS NULL); nunca se borra físicamente.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"prestamos/internal/loanproducts/domain"
	"prestamos/internal/platform/db"
	"prestamos/internal/platform/money"
)

const uniqueViolation = "23505"

// Los montos se leen como texto (::text) y se normalizan en Go: nada de float.
const cols = `loan_products.id::text, tenant_id::text, name, frequency, interest_type, rate_period,
	rate_default::text, rate_min::text, rate_max::text, amount_min::text, amount_max::text,
	terms_min, terms_max, grace_days, late_fee_mode, late_fee_amount::text, late_fee_per_day,
	fee_type, fee_value::text, active, created_at, updated_at`

// fieldSQL y fieldParams son las columnas de negocio y sus parámetros ($3..$20) en el INSERT.
const fieldSQL = `name, frequency, interest_type, rate_period, rate_default, rate_min, rate_max,
	amount_min, amount_max, terms_min, terms_max, grace_days, late_fee_mode, late_fee_amount,
	late_fee_per_day, fee_type, fee_value, active`

const fieldParams = `$3,$4,$5,$6,$7::text::numeric,$8::text::numeric,$9::text::numeric,$10::text::numeric,
	$11::text::numeric,$12,$13,$14,$15,$16::text::numeric,$17,$18,$19::text::numeric,$20`

const insertSQL = `INSERT INTO loan_products (id, tenant_id, ` + fieldSQL + `, created_by)
	VALUES ($1::uuid,$2::uuid,` + fieldParams + `,$21::uuid)`

const updateSQL = `UPDATE loan_products SET name=$3, frequency=$4, interest_type=$5, rate_period=$6,
	rate_default=$7::text::numeric, rate_min=$8::text::numeric, rate_max=$9::text::numeric,
	amount_min=$10::text::numeric, amount_max=$11::text::numeric, terms_min=$12, terms_max=$13,
	grace_days=$14, late_fee_mode=$15, late_fee_amount=$16::text::numeric, late_fee_per_day=$17,
	fee_type=$18, fee_value=$19::text::numeric, active=$20, updated_at=now()
	WHERE id=$1::uuid AND tenant_id=$2::uuid AND deleted_at IS NULL`

const deleteSQL = `UPDATE loan_products SET deleted_at=now(), deleted_by=$3::uuid, updated_at=now()
	WHERE id=$1::uuid AND tenant_id=$2::uuid AND deleted_at IS NULL`

const auditSQL = `INSERT INTO audit_log (id, tenant_id, user_id, action, entity, entity_id, data)
	VALUES ($1::uuid,$2::uuid,$3::uuid,$4,'loan_product',$5::uuid,$6::jsonb)`

type Repo struct{ tx db.TxRunner }

func NewRepo(tx db.TxRunner) *Repo { return &Repo{tx: tx} }

func scan(row pgx.Row) (*domain.Product, error) {
	p := &domain.Product{}
	var rd, rmin, rmax, amin, amax, fee string
	var lateAmount *string
	err := row.Scan(&p.ID, &p.TenantID, &p.Name, &p.Frequency, &p.InterestType, &p.RatePeriod,
		&rd, &rmin, &rmax, &amin, &amax, &p.TermsMin, &p.TermsMax, &p.GraceDays, &p.LateFeeMode,
		&lateAmount, &p.LateFeePerDay, &p.FeeType, &fee, &p.Active, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	p.RateDefault, p.RateMin, p.RateMax = canon(rd), canon(rmin), canon(rmax)
	p.AmountMin, p.AmountMax, p.FeeValue = canon(amin), canon(amax), canon(fee)
	if lateAmount != nil {
		v := canon(*lateAmount)
		p.LateFeeAmount = &v
	}
	return p, nil
}

// fieldArgs son los valores de fieldSQL en el mismo orden.
func fieldArgs(p *domain.Product) []any {
	return []any{p.Name, p.Frequency, p.InterestType, p.RatePeriod, p.RateDefault, p.RateMin, p.RateMax,
		p.AmountMin, p.AmountMax, p.TermsMin, p.TermsMax, p.GraceDays, p.LateFeeMode, p.LateFeeAmount,
		p.LateFeePerDay, p.FeeType, p.FeeValue, p.Active}
}

func (r *Repo) Create(ctx context.Context, p *domain.Product, a domain.Audit) error {
	args := append([]any{p.ID, p.TenantID}, fieldArgs(p)...)
	args = append(args, a.UserID)
	return opErr("Create", mapErr(r.tx.InTenant(ctx, p.TenantID, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, insertSQL, args...); err != nil {
			return err
		}
		return insertAudit(ctx, tx, p.TenantID, p.ID, a)
	})))
}

func (r *Repo) Update(ctx context.Context, p *domain.Product, a domain.Audit) error {
	args := append([]any{p.ID, p.TenantID}, fieldArgs(p)...)
	return opErr("Update", mapErr(r.tx.InTenant(ctx, p.TenantID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, updateSQL, args...)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return domain.ErrNotFound
		}
		return insertAudit(ctx, tx, p.TenantID, p.ID, a)
	})))
}

func (r *Repo) Delete(ctx context.Context, tenantID, id string, a domain.Audit) error {
	return opErr("Delete", mapErr(r.tx.InTenant(ctx, tenantID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, deleteSQL, id, tenantID, a.UserID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return domain.ErrNotFound
		}
		return insertAudit(ctx, tx, tenantID, id, a)
	})))
}

func (r *Repo) Get(ctx context.Context, tenantID, id string) (p *domain.Product, err error) {
	err = r.tx.InTenant(ctx, tenantID, func(tx pgx.Tx) error {
		var e error
		p, e = scan(tx.QueryRow(ctx, `SELECT `+cols+` FROM loan_products
			WHERE loan_products.id=$1::uuid AND deleted_at IS NULL`, id))
		return e
	})
	if err != nil {
		return nil, opErr("Get", mapErr(err))
	}
	return p, nil
}

func (r *Repo) List(ctx context.Context, tenantID string, f domain.ListFilter) (res domain.ListResult, err error) {
	where, args := buildWhere(f)
	err = r.tx.InTenant(ctx, tenantID, func(tx pgx.Tx) error {
		// Pide limit+1 para saber si hay otra página sin contar la tabla.
		n := len(args)
		page := fmt.Sprintf(`SELECT %s FROM loan_products%s ORDER BY lower(name), loan_products.id LIMIT $%d OFFSET $%d`,
			cols, where, n+1, n+2)
		rows, e := tx.Query(ctx, page, append(args[:n:n], f.Limit+1, f.Offset)...)
		if e != nil {
			return e
		}
		defer rows.Close()
		for rows.Next() {
			p, e := scan(rows)
			if e != nil {
				return e
			}
			res.Items = append(res.Items, *p)
		}
		if e := rows.Err(); e != nil {
			return e
		}
		if res.HasMore = len(res.Items) > f.Limit; res.HasMore {
			res.Items = res.Items[:f.Limit]
		}
		if !f.WithTotal {
			return nil
		}
		total, e := r.total(ctx, tx, f, where, args, len(res.Items), res.HasMore)
		res.Total = &total
		return e
	})
	return res, opErr("List", err)
}

// total evita el COUNT cuando la página incompleta ya lo determina.
func (r *Repo) total(ctx context.Context, tx pgx.Tx, f domain.ListFilter, where string, args []any, got int, hasMore bool) (int, error) {
	if !hasMore && (got > 0 || f.Offset == 0) {
		return f.Offset + got, nil
	}
	var total int
	err := tx.QueryRow(ctx, `SELECT count(*) FROM loan_products`+where, args...).Scan(&total)
	return total, err
}

// buildWhere agrega solo los filtros presentes; siempre excluye las filas eliminadas.
func buildWhere(f domain.ListFilter) (string, []any) {
	conds := []string{"deleted_at IS NULL"}
	var args []any
	if f.Active != nil {
		args = append(args, *f.Active)
		conds = append(conds, fmt.Sprintf("active = $%d", len(args)))
	}
	if f.Frequency != "" {
		args = append(args, f.Frequency)
		conds = append(conds, fmt.Sprintf("frequency = $%d", len(args)))
	}
	if q := strings.TrimSpace(f.Query); q != "" {
		args = append(args, "%"+escapeLike(q)+"%")
		conds = append(conds, fmt.Sprintf("name ILIKE $%d", len(args)))
	}
	return " WHERE " + strings.Join(conds, " AND "), args
}

func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(strings.TrimSpace(s))
}

// canon devuelve el monto con 2 o 4 decimales ("5.0000" -> "5.00"), el formato de la API.
func canon(s string) string {
	d, err := money.Parse(s)
	if err != nil {
		return s
	}
	return money.Format(d)
}

func insertAudit(ctx context.Context, tx pgx.Tx, tenantID, entityID string, a domain.Audit) error {
	_, err := tx.Exec(ctx, auditSQL, a.ID, tenantID, a.UserID, a.Action, entityID, a.Data)
	return err
}

func mapErr(err error) error {
	var pg *pgconn.PgError
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return domain.ErrNotFound
	case errors.As(err, &pg) && pg.Code == uniqueViolation:
		return domain.ErrNameTaken
	}
	return err
}

// opErr añade módulo, capa y operación; %w conserva los centinelas del dominio.
func opErr(op string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("loanproducts.repo.%s: %w", op, err)
}
