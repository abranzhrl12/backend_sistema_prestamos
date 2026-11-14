package db

import (
	"context"
	"runtime"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TxRunner es el puerto que usan los repositorios para ejecutar SQL en transacción.
// Los repos dependen de esta interfaz, no de pgxpool.
type TxRunner interface {
	// InTenant fija app.tenant_id (RLS) y ejecuta fn en una transacción.
	InTenant(ctx context.Context, tenantID string, fn func(tx pgx.Tx) error) error
	// InTx ejecuta fn en una transacción sin tenant (solo para tablas globales como tenants).
	InTx(ctx context.Context, fn func(tx pgx.Tx) error) error
}

// PgRunner implementa TxRunner sobre pgxpool.
type PgRunner struct{ pool *pgxpool.Pool }

func NewPgRunner(pool *pgxpool.Pool) *PgRunner { return &PgRunner{pool: pool} }

// PoolOptions ajusta el pool de conexiones (0 = valor por defecto razonable).
// Tracer es opcional (p. ej. obs.SlowQueryTracer para registrar consultas lentas o fallidas).
type PoolOptions struct {
	MaxConns, MinConns int32
	Tracer             pgx.QueryTracer
}

func NewPool(ctx context.Context, url string, o PoolOptions) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, err
	}
	cfg.MaxConns = cmpOr(o.MaxConns, int32(max(8, runtime.NumCPU()*4)))
	cfg.MinConns = cmpOr(o.MinConns, 2)
	cfg.ConnConfig.Tracer = o.Tracer
	cfg.MaxConnLifetime = time.Hour
	cfg.MaxConnIdleTime = 15 * time.Minute
	cfg.HealthCheckPeriod = time.Minute
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}

func (r *PgRunner) InTenant(ctx context.Context, tenantID string, fn func(tx pgx.Tx) error) error {
	return r.InTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", tenantID); err != nil {
			return err
		}
		return fn(tx)
	})
}

func (r *PgRunner) InTx(ctx context.Context, fn func(tx pgx.Tx) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func cmpOr(v, def int32) int32 {
	if v > 0 {
		return v
	}
	return def
}
