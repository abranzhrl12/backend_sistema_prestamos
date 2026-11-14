// Package postgres implementa domain.Repository sobre pgx con aislamiento por tenant (RLS).
package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"prestamos/internal/platform/db"
	"prestamos/internal/platform/money"
	"prestamos/internal/settings/domain"
)

const cols = `currency, timezone, late_fee_mode, late_fee_amount::text, late_fee_per_day, grace_days,
	allocation_order, max_active_loans_per_customer, allow_negative_capital, updated_at`

// upsertSQL inserta o reemplaza la fila de la empresa. El monto viaja como texto y se castea a NUMERIC
// para no pasar por float.
const upsertSQL = `INSERT INTO tenant_settings (tenant_id, currency, timezone, late_fee_mode, late_fee_amount,
	late_fee_per_day, grace_days, allocation_order, max_active_loans_per_customer, allow_negative_capital,
	updated_by, updated_at)
	VALUES ($1::uuid,$2,$3,$4,$5::text::numeric,$6,$7,$8,$9,$10,$11::uuid,now())
	ON CONFLICT (tenant_id) DO UPDATE SET currency=EXCLUDED.currency, timezone=EXCLUDED.timezone,
	late_fee_mode=EXCLUDED.late_fee_mode, late_fee_amount=EXCLUDED.late_fee_amount,
	late_fee_per_day=EXCLUDED.late_fee_per_day, grace_days=EXCLUDED.grace_days,
	allocation_order=EXCLUDED.allocation_order,
	max_active_loans_per_customer=EXCLUDED.max_active_loans_per_customer,
	allow_negative_capital=EXCLUDED.allow_negative_capital, updated_by=EXCLUDED.updated_by, updated_at=now()
	RETURNING ` + cols

const auditSQL = `INSERT INTO audit_log (id, tenant_id, user_id, action, entity, entity_id, data)
	VALUES ($1::uuid,$2::uuid,$3::uuid,$4,'tenant_settings',$2::uuid,$5::jsonb)`

type Repo struct{ tx db.TxRunner }

func NewRepo(tx db.TxRunner) *Repo { return &Repo{tx: tx} }

func scan(row pgx.Row) (*domain.Settings, error) {
	s := &domain.Settings{}
	var amount string
	err := row.Scan(&s.Currency, &s.Timezone, &s.LateFeeMode, &amount, &s.LateFeePerDay, &s.GraceDays,
		&s.AllocationOrder, &s.MaxActiveLoansPerCustomer, &s.AllowNegativeCapital, &s.UpdatedAt)
	s.LateFeeAmount = canon(amount)
	return s, err
}

func (r *Repo) Get(ctx context.Context, tenantID string) (*domain.Settings, error) {
	var out *domain.Settings
	err := r.tx.InTenant(ctx, tenantID, func(tx pgx.Tx) error {
		var e error
		out, e = scan(tx.QueryRow(ctx, `SELECT `+cols+` FROM tenant_settings WHERE tenant_id=$1::uuid`, tenantID))
		return e
	})
	if err != nil {
		return nil, opErr("Get", mapErr(err))
	}
	return out, nil
}

func (r *Repo) Save(ctx context.Context, tenantID string, s domain.Settings, a domain.Audit) (*domain.Settings, error) {
	var out *domain.Settings
	err := r.tx.InTenant(ctx, tenantID, func(tx pgx.Tx) error {
		var e error
		out, e = scan(tx.QueryRow(ctx, upsertSQL, tenantID, s.Currency, s.Timezone, s.LateFeeMode,
			s.LateFeeAmount, s.LateFeePerDay, s.GraceDays, s.AllocationOrder, s.MaxActiveLoansPerCustomer,
			s.AllowNegativeCapital, a.UserID))
		if e != nil {
			return e
		}
		_, e = tx.Exec(ctx, auditSQL, a.ID, tenantID, a.UserID, a.Action, a.Data)
		return e
	})
	if err != nil {
		return nil, opErr("Save", mapErr(err))
	}
	out.IsDefault = false
	return out, nil
}

// canon devuelve el monto con 2 o 4 decimales ("10.0000" -> "10.00"), el formato de la API.
func canon(s string) string {
	d, err := money.Parse(s)
	if err != nil {
		return s
	}
	return money.Format(d)
}

func mapErr(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	return err
}

// opErr añade módulo, capa y operación al error; %w conserva los centinelas.
func opErr(op string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("settings.repo.%s: %w", op, err)
}
