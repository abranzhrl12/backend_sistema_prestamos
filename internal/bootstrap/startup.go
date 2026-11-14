package bootstrap

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"prestamos/authkit"
	"prestamos/internal/platform/config"
	"prestamos/internal/platform/db"
	"prestamos/internal/platform/ids"
	"prestamos/internal/platform/migrate"
	tenantspg "prestamos/internal/tenants/adapters/postgres"
	tenantsapp "prestamos/internal/tenants/app"
)

const (
	migrationsDir   = "migrations"
	startupMaxConns = 2
	startupMinConns = 1
)

// PlatformAdmin son los datos del superadmin de plataforma que el arranque asegura.
type PlatformAdmin struct{ Email, Name, Password string }

// StartupDeps son los efectos externos del arranque (inyectables en pruebas).
type StartupDeps struct {
	// Migrate abre una conexión a url, aplica las migraciones pendientes y otorga DML a role.
	Migrate func(ctx context.Context, url, role string) ([]string, error)
	// Seed abre una conexión a url y asegura el superadmin; created indica si se creó ahora.
	Seed func(ctx context.Context, url string, admin PlatformAdmin) (created bool, err error)
}

// DefaultStartupDeps devuelve las implementaciones reales sobre Postgres.
func DefaultStartupDeps() StartupDeps {
	return StartupDeps{Migrate: migrateWithPool, Seed: seedWithPool}
}

// RunStartup aplica, si están activados, las migraciones (AUTO_MIGRATE) y el seed del superadmin
// (AUTO_SEED) antes de que la API abra su pool y escuche. Cualquier fallo aborta el arranque.
func RunStartup(ctx context.Context, cfg config.Config, log *slog.Logger, deps StartupDeps) error {
	if cfg.AutoMigrate {
		if err := runMigrations(ctx, cfg, log, deps); err != nil {
			return err
		}
	}
	if cfg.AutoSeed {
		return runSeed(ctx, cfg, log, deps)
	}
	return nil
}

// runMigrations usa MIGRATE_DATABASE_URL (rol dueño) y, si falta, DATABASE_URL, igual que cmd/migrate.
func runMigrations(ctx context.Context, cfg config.Config, log *slog.Logger, deps StartupDeps) error {
	url := cfg.MigrateDatabaseURL
	if url == "" {
		url = cfg.DatabaseURL
	}
	applied, err := deps.Migrate(ctx, url, cfg.AppDBUser)
	if err != nil {
		return fmt.Errorf("arranque: migraciones: %w", err)
	}
	if len(applied) == 0 {
		log.Info("migraciones: no hay pendientes")
	}
	for _, v := range applied {
		log.Info("migración aplicada", "version", v)
	}
	log.Info("permisos DML otorgados", "rol_app", cfg.AppDBUser)
	return nil
}

// runSeed usa DATABASE_URL, igual que cmd/seed. El log nunca incluye email ni contraseña.
func runSeed(ctx context.Context, cfg config.Config, log *slog.Logger, deps StartupDeps) error {
	admin := PlatformAdmin{Email: cfg.SeedAdminEmail, Name: cfg.SeedAdminName, Password: cfg.SeedAdminPassword}
	created, err := deps.Seed(ctx, cfg.DatabaseURL, admin)
	if err != nil {
		return fmt.Errorf("arranque: seed del superadmin: %w", err)
	}
	if created {
		log.Info("superadmin de plataforma creado")
		return nil
	}
	log.Info("superadmin de plataforma ya existía; no se modificó")
	return nil
}

func migrateWithPool(ctx context.Context, url, role string) ([]string, error) {
	pool, err := openStartupPool(ctx, url)
	if err != nil {
		return nil, err
	}
	defer pool.Close()
	return migrate.UpAndGrant(ctx, pool, migrationsDir, role)
}

func seedWithPool(ctx context.Context, url string, admin PlatformAdmin) (bool, error) {
	pool, err := openStartupPool(ctx, url)
	if err != nil {
		return false, err
	}
	defer pool.Close()
	svc := tenantsapp.NewService(tenantspg.NewRepo(db.NewPgRunner(pool)),
		authkit.NewPasswordHasher(authkit.DefaultPasswordParams), ids.UUIDv7{})
	return svc.EnsurePlatformAdmin(ctx, admin.Email, admin.Name, admin.Password)
}

func openStartupPool(ctx context.Context, url string) (*pgxpool.Pool, error) {
	pool, err := db.NewPool(ctx, url, db.PoolOptions{MaxConns: startupMaxConns, MinConns: startupMinConns})
	if err != nil {
		return nil, fmt.Errorf("no se pudo conectar a la base de datos: %w", err)
	}
	return pool, nil
}
