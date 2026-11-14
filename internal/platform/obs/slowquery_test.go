package obs_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"prestamos/internal/platform/obs"
)

const secretArg = "secreto-del-argumento-123"

// trace ejecuta un ciclo start/end del tracer con la metadata de un request.
func trace(t *testing.T, tr *obs.SlowQueryTracer, sql string, work time.Duration, endErr error) {
	t.Helper()
	ctx := obs.WithMeta(context.Background(), obs.Meta{RequestID: "req-1", Module: "customers"})
	ctx = tr.TraceQueryStart(ctx, nil, pgx.TraceQueryStartData{SQL: sql, Args: []any{secretArg}})
	time.Sleep(work)
	tr.TraceQueryEnd(ctx, nil, pgx.TraceQueryEndData{Err: endErr})
}

func newTracer(buf *bytes.Buffer, threshold time.Duration) *obs.SlowQueryTracer {
	return &obs.SlowQueryTracer{Log: obs.NewLogger(buf, "debug", "json"), Threshold: threshold}
}

func TestSlowQuery_SobreUmbralLogueaWarn(t *testing.T) {
	var buf bytes.Buffer
	trace(t, newTracer(&buf, time.Millisecond), "SELECT id FROM customers WHERE id=$1", 5*time.Millisecond, nil)

	m := findLog(t, &buf, "slow_query")
	if m["level"] != "WARN" || m["request_id"] != "req-1" || m["module"] != "customers" {
		t.Fatalf("slow_query mal registrado: %v", m)
	}
	if ms, _ := m["duration_ms"].(float64); ms < 1 {
		t.Fatalf("duration_ms = %v, debe ser ≥ umbral", m["duration_ms"])
	}
	if m["sql"] != "SELECT id FROM customers WHERE id=$1" {
		t.Fatalf("sql = %v", m["sql"])
	}
}

func TestSlowQuery_BajoUmbralNoLoguea(t *testing.T) {
	var buf bytes.Buffer
	trace(t, newTracer(&buf, time.Hour), "SELECT 1", 0, nil)
	if buf.Len() != 0 {
		t.Fatalf("consulta rápida no debe loguearse: %s", buf.String())
	}
}

func TestSlowQuery_NuncaRegistraArgumentos(t *testing.T) {
	var buf bytes.Buffer
	trace(t, newTracer(&buf, time.Millisecond), "SELECT 1 WHERE x=$1", 5*time.Millisecond, errors.New("fallo"))
	if strings.Contains(buf.String(), secretArg) {
		t.Fatalf("los argumentos de la consulta no deben registrarse: %s", buf.String())
	}
}

func TestSlowQuery_SQLTruncadoA300Caracteres(t *testing.T) {
	var buf bytes.Buffer
	long := "SELECT " + strings.Repeat("a", 400)
	trace(t, newTracer(&buf, time.Millisecond), long, 5*time.Millisecond, nil)
	sql, _ := findLog(t, &buf, "slow_query")["sql"].(string)
	if len([]rune(sql)) != 300 {
		t.Fatalf("longitud del sql = %d, quiero 300", len([]rune(sql)))
	}
}

func TestSlowQuery_ErrorLogueDBError(t *testing.T) {
	var buf bytes.Buffer
	trace(t, newTracer(&buf, time.Hour), "UPDATE customers SET x=1", 0, errors.New("conexión perdida"))

	m := findLog(t, &buf, "db_error")
	if m["level"] != "ERROR" || m["err"] != "conexión perdida" || m["module"] != "customers" {
		t.Fatalf("db_error mal registrado: %v", m)
	}
}

func TestSlowQuery_ErroresEsperadosNoSonDBError(t *testing.T) {
	cases := map[string]error{
		"sin filas":          pgx.ErrNoRows,
		"duplicado (23505)":  &pgconn.PgError{Code: "23505"},
		"duplicado envuelto": wrap(&pgconn.PgError{Code: "23505"}),
	}
	for name, err := range cases {
		var buf bytes.Buffer
		trace(t, newTracer(&buf, time.Hour), "INSERT INTO customers VALUES ($1)", 0, err)
		if buf.Len() != 0 {
			t.Errorf("%s: no debe loguearse como db_error: %s", name, buf.String())
		}
	}
}

func wrap(err error) error { return errors.Join(errors.New("contexto"), err) }
