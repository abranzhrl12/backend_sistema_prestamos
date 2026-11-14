package obs

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	sqlMaxLen        = 300
	uniqueViolation  = "23505"
	queryDescription = "sql"
)

type queryInfo struct {
	start time.Time
	sql   string
}

type queryKey struct{}

// SlowQueryTracer implementa pgx.QueryTracer: registra consultas lentas (warn) y
// fallidas (error). NUNCA registra los argumentos de la consulta, pueden tener datos sensibles.
type SlowQueryTracer struct {
	Log       *slog.Logger
	Threshold time.Duration
}

// TraceQueryStart guarda el instante y el SQL (sin argumentos) en el context.
func (t *SlowQueryTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	return context.WithValue(ctx, queryKey{}, queryInfo{start: time.Now(), sql: data.SQL})
}

// TraceQueryEnd emite slow_query si tardó ≥ Threshold y db_error si la consulta falló.
func (t *SlowQueryTracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	info, ok := ctx.Value(queryKey{}).(queryInfo)
	if !ok {
		return
	}
	elapsed := time.Since(info.start)
	meta := MetaFrom(ctx)
	if elapsed >= t.Threshold {
		t.logger().Warn("slow_query", queryAttrs(info.sql, elapsed, meta)...)
	}
	if data.Err != nil && !expectedErr(data.Err) {
		attrs := append(queryAttrs(info.sql, elapsed, meta), "err", data.Err.Error())
		t.logger().Error("db_error", attrs...)
	}
}

func (t *SlowQueryTracer) logger() *slog.Logger {
	if t.Log == nil {
		return slog.Default()
	}
	return t.Log
}

// expectedErr marca los errores de negocio que no son fallos de BD: sin filas o duplicado.
func expectedErr(err error) bool {
	var pg *pgconn.PgError
	return errors.Is(err, pgx.ErrNoRows) || (errors.As(err, &pg) && pg.Code == uniqueViolation)
}

func queryAttrs(sql string, elapsed time.Duration, meta Meta) []any {
	return []any{
		queryDescription, truncate(sql, sqlMaxLen),
		"duration_ms", float64(elapsed) / msPerDuration,
		"request_id", meta.RequestID,
		"module", meta.Module,
	}
}

func truncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}
