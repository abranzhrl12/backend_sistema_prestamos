//go:build perf

// Package perf mide rendimiento real (app + Postgres temporal en Docker).
//
//	go test -tags=perf ./tests/perf -run TestLoad -v -count=1
//
// Variables: PERF_DURATION (def. 5s por escenario), PERF_WORKERS (def. 32),
// PERF_ROWS (def. 20000 clientes), PERF_P95_MS (umbral p95, def. 250).
package perf

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"

	"prestamos/internal/bootstrap"
	"prestamos/internal/platform/config"
	"prestamos/tests/testenv"
)

const pass = "Secreto123"

var (
	app      *fiber.App
	env      *testenv.Env
	token    string   // access token del owner del tenant "perf"
	tenantID string   // tenant "perf"
	ids      []string // IDs de clientes existentes (para GET por id)
	rows     = envInt("PERF_ROWS", 20000)
)

func TestMain(m *testing.M) { os.Exit(run(m)) }

func run(m *testing.M) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	var err error
	if env, err = testenv.Start(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "no se pudo levantar la BD de pruebas:", err)
		return 1
	}
	defer env.Close()
	go func() {
		<-ctx.Done()
		env.Close()
		os.Exit(130)
	}()

	cfg := config.Config{JWTSecret: "perf-secret-perf-secret-perf-secret-1234", JWTIssuer: "perf",
		AccessTTL: time.Hour, RefreshTTL: 24 * time.Hour}
	if app, err = bootstrap.NewApp(cfg, env.App); err != nil {
		fmt.Fprintln(os.Stderr, "bootstrap:", err)
		return 1
	}
	if err = seed(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "seed:", err)
		return 1
	}
	return m.Run()
}

func seed(ctx context.Context) (err error) {
	if tenantID, err = env.Tenant(ctx, "perf"); err != nil {
		return err
	}
	if _, err = env.User(ctx, tenantID, "owner@perf.com", pass, "owner"); err != nil {
		return err
	}
	start := time.Now()
	if err = env.BulkCustomers(ctx, tenantID, rows); err != nil {
		return err
	}
	if _, err = env.Admin.Exec(ctx, "ANALYZE customers"); err != nil {
		return err
	}
	fmt.Printf("sembrados %d clientes en %s\n", rows, time.Since(start).Round(time.Millisecond))

	rs, err := env.Admin.Query(ctx, `SELECT id::text FROM customers WHERE tenant_id=$1::uuid LIMIT 500`, tenantID)
	if err != nil {
		return err
	}
	defer rs.Close()
	for rs.Next() {
		var id string
		if err = rs.Scan(&id); err != nil {
			return err
		}
		ids = append(ids, id)
	}
	token, err = loginToken()
	return err
}

func envInt(k string, def int) int {
	if n, err := strconv.Atoi(os.Getenv(k)); err == nil && n > 0 {
		return n
	}
	return def
}
