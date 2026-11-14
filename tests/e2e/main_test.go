//go:build e2e

// Package e2e prueba la API completa contra un Postgres real y temporal (Docker).
// Ejecutar: go test -tags=e2e ./tests/e2e -count=1
package e2e

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"

	"prestamos/internal/bootstrap"
	"prestamos/internal/platform/config"
	"prestamos/tests/testenv"
)

const (
	pass    = "Secreto123"
	apiBase = "/api/v1"

	// Mismo secreto que la app de pruebas: los tests firman access tokens vencidos con él.
	testJWTSecret = "test-secret-test-secret-test-secret-1234"
	testJWTIssuer = "test"
)

var (
	app     *fiber.App
	env     *testenv.Env // entorno compartido (Admin = superusuario para sembrar/inspeccionar)
	tenantA string       // slug "acme"
	tenantB string       // slug "beta"
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
	defer env.Close() // docker compose down -v: borra contenedor y datos

	go func() { // Ctrl+C: limpiar igual
		<-ctx.Done()
		env.Close()
		os.Exit(130)
	}()

	cfg := config.Config{
		JWTSecret: testJWTSecret, JWTIssuer: testJWTIssuer,
		AccessTTL: 15 * time.Minute, RefreshTTL: 24 * time.Hour, CookieSecure: false,
	}
	if app, err = bootstrap.NewApp(cfg, env.App); err != nil {
		fmt.Fprintln(os.Stderr, "bootstrap:", err)
		return 1
	}
	if err := seed(ctx, env); err != nil {
		fmt.Fprintln(os.Stderr, "seed:", err)
		return 1
	}
	return m.Run()
}

func seed(ctx context.Context, env *testenv.Env) (err error) {
	if tenantA, err = env.Tenant(ctx, "acme"); err != nil {
		return err
	}
	if tenantB, err = env.Tenant(ctx, "beta"); err != nil {
		return err
	}
	platformID, err := env.SystemTenant(ctx, "platform")
	if err != nil {
		return err
	}
	users := []struct {
		tenant, email string
		roles         []string
	}{
		{tenantA, "owner@acme.com", []string{"owner"}},
		{tenantA, "cobrador@acme.com", []string{"cobrador"}},
		{tenantB, "owner@beta.com", []string{"owner"}},
		{platformID, rootEmail, []string{"superadmin"}},
	}
	for _, u := range users {
		if _, err = env.User(ctx, u.tenant, u.email, pass, u.roles...); err != nil {
			return err
		}
	}
	return nil
}
