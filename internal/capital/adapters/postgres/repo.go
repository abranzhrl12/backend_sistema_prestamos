// Package postgres implementa domain.Repository sobre pgx con aislamiento por tenant (RLS).
package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"

	"prestamos/internal/capital/domain"
	"prestamos/internal/platform/db"
)

// lockSQL serializa las escrituras por empresa: el chequeo de fondos y el INSERT forman un bloque atómico.
// El bloqueo es transaccional (se libera al hacer COMMIT o ROLLBACK).
const lockSQL = `SELECT pg_advisory_xact_lock(hashtextextended($1::text, 0))`

// balanceSQL suma en la BD (no trae filas). El mapeo tipo -> bucket/dirección repite el del dominio:
// SQL no puede llamar a Go, así que cualquier tipo nuevo debe añadirse en ambos lugares.
const balanceSQL = `SELECT
	coalesce(sum(CASE WHEN type IN ('contribution','adjustment_in','principal_collection') THEN amount
	                  WHEN type IN ('withdrawal','adjustment_out','disbursement') THEN -amount END), 0)::text,
	coalesce(sum(CASE WHEN type IN ('interest_collection','late_fee_collection') THEN amount
	                  WHEN type = 'expense' THEN -amount END), 0)::text,
	coalesce(sum(CASE WHEN type = 'disbursement' THEN amount
	                  WHEN type = 'principal_collection' THEN -amount END), 0)::text,
	count(*)::int, now()
	FROM capital_movements`

const insertSQL = `INSERT INTO capital_movements
	(id, tenant_id, type, amount, note, reference, loan_id, payment_id, created_by)
	VALUES ($1::uuid,$2::uuid,$3,$4::numeric,nullif($5,''),nullif($6,''),$7::uuid,$8::uuid,$9::uuid)
	RETURNING created_at`

type Repo struct{ tx db.TxRunner }

func NewRepo(tx db.TxRunner) *Repo { return &Repo{tx: tx} }

func (r *Repo) Append(ctx context.Context, tenantID string, m *domain.Movement, requireFunds bool) error {
	return opErr("Append", r.tx.InTenant(ctx, tenantID, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, lockSQL, tenantID); err != nil {
			return err
		}
		if requireFunds {
			if err := checkFunds(ctx, tx, m.Amount); err != nil {
				return err
			}
		}
		return tx.QueryRow(ctx, insertSQL, m.ID, m.TenantID, string(m.Type), m.Amount.String(),
			m.Note, m.Reference, m.LoanID, m.PaymentID, m.CreatedBy).Scan(&m.CreatedAt)
	}))
}

// checkFunds corre dentro del bloqueo: el saldo que ve es el ya confirmado por las escrituras previas.
func checkFunds(ctx context.Context, tx pgx.Tx, amount decimal.Decimal) error {
	b, err := readBalance(ctx, tx)
	if err != nil {
		return err
	}
	if b.TotalCash.Sub(amount).IsNegative() {
		return domain.ErrInsufficientCapital
	}
	return nil
}

func (r *Repo) Balance(ctx context.Context, tenantID string) (b domain.Balance, err error) {
	err = r.tx.InTenant(ctx, tenantID, func(tx pgx.Tx) error {
		var e error
		b, e = readBalance(ctx, tx)
		return e
	})
	return b, opErr("Balance", err)
}

func readBalance(ctx context.Context, tx pgx.Tx) (domain.Balance, error) {
	var capital, profit, lent string
	var b domain.Balance
	var asOf time.Time
	err := tx.QueryRow(ctx, balanceSQL).Scan(&capital, &profit, &lent, &b.MovementsCount, &asOf)
	if err != nil {
		return b, err
	}
	if b.CapitalBalance, err = decimal.NewFromString(capital); err != nil {
		return b, fmt.Errorf("saldo de capital: %w", err)
	}
	if b.ProfitBalance, err = decimal.NewFromString(profit); err != nil {
		return b, fmt.Errorf("saldo de utilidad: %w", err)
	}
	if b.LentOutstanding, err = decimal.NewFromString(lent); err != nil {
		return b, fmt.Errorf("capital prestado: %w", err)
	}
	b.TotalCash = b.CapitalBalance.Add(b.ProfitBalance)
	b.AsOf = asOf
	return b, nil
}

// opErr añade módulo y capa al error (p. ej. "capital.repo.Append: ..."), conservando los centinelas.
func opErr(op string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("capital.repo.%s: %w", op, err)
}
