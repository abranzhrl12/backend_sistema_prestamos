// Command seed crea (idempotente) la empresa de la plataforma (is_system=true) y su superadmin.
// No hay contraseña por defecto: SEED_ADMIN_PASSWORD es obligatoria. Con --demo crea además
// datos de desarrollo (nunca en APP_ENV=production). Detalle en docs/ADMIN_API.md.
//
//	go run ./cmd/seed [--demo]
//
// Lee del entorno: DATABASE_URL, SEED_ADMIN_EMAIL, SEED_ADMIN_PASSWORD (obligatorias),
// SEED_ADMIN_NAME (def. "Platform Admin"), APP_ENV. Si existe .env en el directorio actual,
// sus valores se usan solo cuando la variable no está definida en el entorno.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"prestamos/authkit"
	"prestamos/internal/platform/db"
	"prestamos/internal/platform/ids"
	tenantspg "prestamos/internal/tenants/adapters/postgres"
	tenantsapp "prestamos/internal/tenants/app"
	tenantsdomain "prestamos/internal/tenants/domain"
)

// deps son las dependencias externas de run (inyectables en pruebas).
type deps struct {
	getenv   func(string) string
	out      io.Writer
	openPool func(ctx context.Context, url string) (*pgxpool.Pool, error)
}

type config struct {
	DatabaseURL, AdminEmail, AdminPassword, AdminName, AppEnv string
}

func main() {
	raw, _ := os.ReadFile(".env") // sin .env no es un error: se usa solo el entorno
	d := deps{
		getenv: withDotEnv(os.Getenv, parseDotEnv(string(raw))),
		out:    os.Stdout,
		openPool: func(ctx context.Context, url string) (*pgxpool.Pool, error) {
			return db.NewPool(ctx, url, db.PoolOptions{MaxConns: 4, MinConns: 1})
		},
	}
	if err := run(context.Background(), d, os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", describe(err))
		os.Exit(1)
	}
}

func run(ctx context.Context, d deps, args []string) error {
	demo, err := parseArgs(args)
	if err != nil {
		return err
	}
	cfg, err := loadConfig(d.getenv)
	if err != nil {
		return err
	}
	if demo && cfg.AppEnv == "production" {
		return errors.New("--demo está prohibido con APP_ENV=production")
	}
	pool, err := d.openPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("no se pudo conectar a la base de datos: %w", err)
	}
	defer pool.Close()

	runner := db.NewPgRunner(pool)
	svc := tenantsapp.NewService(tenantspg.NewRepo(runner),
		authkit.NewPasswordHasher(authkit.DefaultPasswordParams), ids.UUIDv7{})
	created, err := svc.EnsurePlatformAdmin(ctx, cfg.AdminEmail, cfg.AdminName, cfg.AdminPassword)
	if err != nil {
		return err
	}
	printAdmin(d.out, created, cfg.AdminEmail)
	if !demo {
		return nil
	}
	return seedDemo(ctx, d.out, runner, svc)
}

func parseArgs(args []string) (bool, error) {
	demo := false
	for _, a := range args {
		if a != "--demo" {
			return false, fmt.Errorf("argumento desconocido %q (uso: go run ./cmd/seed [--demo])", a)
		}
		demo = true
	}
	return demo, nil
}

func loadConfig(getenv func(string) string) (config, error) {
	cfg := config{
		DatabaseURL:   getenv("DATABASE_URL"),
		AdminEmail:    getenv("SEED_ADMIN_EMAIL"),
		AdminPassword: getenv("SEED_ADMIN_PASSWORD"),
		AdminName:     getenv("SEED_ADMIN_NAME"),
		AppEnv:        getenv("APP_ENV"),
	}
	if cfg.AdminName == "" {
		cfg.AdminName = "Platform Admin"
	}
	var missing []string
	for _, req := range []struct{ name, val string }{
		{"DATABASE_URL", cfg.DatabaseURL}, {"SEED_ADMIN_EMAIL", cfg.AdminEmail}, {"SEED_ADMIN_PASSWORD", cfg.AdminPassword},
	} {
		if req.val == "" {
			missing = append(missing, req.name)
		}
	}
	if len(missing) > 0 {
		return cfg, fmt.Errorf("faltan variables obligatorias: %s", strings.Join(missing, ", "))
	}
	return cfg, nil
}

func printAdmin(out io.Writer, created bool, email string) {
	if created {
		fmt.Fprintf(out, "Superadmin creado: %s (empresa 'platform').\n", email)
		return
	}
	fmt.Fprintf(out, "El superadmin %s ya existía; no se modificó nada.\n", email)
}

// describe muestra todos los problemas de validación (no solo el primero).
func describe(err error) string {
	var ve *tenantsdomain.ValidationError
	if !errors.As(err, &ve) {
		return err.Error()
	}
	msgs := make([]string, 0, len(ve.Issues))
	for _, is := range ve.Issues {
		msgs = append(msgs, is.Field+": "+is.Msg)
	}
	return strings.Join(msgs, "; ")
}

// parseDotEnv lee líneas KEY=VALUE; ignora vacías y comentarios, y quita comillas envolventes.
func parseDotEnv(content string) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		out[strings.TrimSpace(k)] = strings.Trim(strings.TrimSpace(v), `"'`)
	}
	return out
}

// withDotEnv prioriza el entorno real y usa el .env solo como respaldo.
func withDotEnv(getenv func(string) string, file map[string]string) func(string) string {
	return func(k string) string {
		if v := getenv(k); v != "" {
			return v
		}
		return file[k]
	}
}
