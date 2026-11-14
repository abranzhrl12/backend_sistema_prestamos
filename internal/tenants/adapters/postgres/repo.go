// Package postgres implementa domain.Repository sobre pgx. Las tablas de usuarios y auditoría
// usan RLS: cada operación fija app.tenant_id de la empresa objetivo (nunca la del superadmin).
package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"prestamos/internal/platform/db"
	"prestamos/internal/tenants/domain"
)

const uniqueViolation = "23505"

var sentinels = []error{domain.ErrTenantNotFound, domain.ErrUserNotFound, domain.ErrSlugTaken,
	domain.ErrEmailTaken, domain.ErrPlatformProtected, domain.ErrLastSuperadmin}

// Repo es el adaptador de persistencia. Fuera de Atomic cada método abre su propia transacción;
// dentro de Atomic (tx != nil) todas las operaciones comparten la transacción de Atomic.
type Repo struct {
	run db.TxRunner
	tx  pgx.Tx
}

func NewRepo(run db.TxRunner) *Repo { return &Repo{run: run} }

func (r *Repo) Atomic(ctx context.Context, fn func(domain.Repository) error) error {
	if r.tx != nil {
		return fn(r)
	}
	return r.run.InTx(ctx, func(tx pgx.Tx) error { return fn(&Repo{run: r.run, tx: tx}) })
}

// unscoped ejecuta fn sin fijar tenant (tabla tenants, sin RLS).
func (r *Repo) unscoped(ctx context.Context, fn func(pgx.Tx) error) error {
	if r.tx != nil {
		return fn(r.tx)
	}
	return r.run.InTx(ctx, fn)
}

// scoped ejecuta fn con app.tenant_id = tenantID (RLS). Dentro de Atomic usa set_config en la misma tx.
func (r *Repo) scoped(ctx context.Context, tenantID string, fn func(pgx.Tx) error) error {
	if r.tx == nil {
		return r.run.InTenant(ctx, tenantID, fn)
	}
	if _, err := r.tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", tenantID); err != nil {
		return err
	}
	return fn(r.tx)
}

// wrap envuelve errores inesperados; los sentinels de dominio se devuelven tal cual.
func wrap(op string, err error) error {
	if err == nil {
		return nil
	}
	for _, s := range sentinels {
		if errors.Is(err, s) {
			return err
		}
	}
	return fmt.Errorf("tenants.repo.%s: %w", op, err)
}

// orNotFound traduce pgx.ErrNoRows al sentinel de "no existe".
func orNotFound(err, notFound error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return notFound
	}
	return err
}

// uniqueTo traduce una violación de unicidad al sentinel de dominio correspondiente.
func uniqueTo(err, sentinel error) error {
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == uniqueViolation {
		return sentinel
	}
	return err
}

// totalOf calcula el total sin COUNT cuando la página incompleta ya lo determina.
func totalOf(ctx context.Context, tx pgx.Tx, countSQL string, args []any, offset, got int, hasMore bool) (int, error) {
	if !hasMore && (got > 0 || offset == 0) {
		return offset + got, nil
	}
	var total int
	err := tx.QueryRow(ctx, countSQL, args...).Scan(&total)
	return total, err
}

// whereClause une condiciones; vacío si no hay ninguna.
func whereClause(conds []string) string {
	if len(conds) == 0 {
		return ""
	}
	return " WHERE " + strings.Join(conds, " AND ")
}

// likeArg prepara un patrón ILIKE '%texto%' con los comodines escapados.
func likeArg(q string) string {
	return "%" + strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(strings.TrimSpace(q)) + "%"
}
