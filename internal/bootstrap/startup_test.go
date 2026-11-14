package bootstrap_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"prestamos/internal/bootstrap"
	"prestamos/internal/platform/config"
)

const (
	ownerURL   = "postgres://owner:owner-pw@db:5432/prestamos?sslmode=disable"
	appURL     = "postgres://app_user:app-pw@db:5432/prestamos?sslmode=disable"
	adminPass  = "Clave-Del-Seed-2026"
	adminEmail = "admin@plataforma.test"
	errBoom    = "boom"
)

// fakeDeps registra las llamadas a Migrate y Seed sin tocar ninguna BD.
type fakeDeps struct {
	calls      []string
	migrateURL string
	migrateRol string
	seedURL    string
	seedAdmin  bootstrap.PlatformAdmin
	applied    []string
	created    bool
	migrateErr error
	seedErr    error
}

func (f *fakeDeps) deps() bootstrap.StartupDeps {
	return bootstrap.StartupDeps{
		Migrate: func(_ context.Context, url, role string) ([]string, error) {
			f.calls = append(f.calls, "migrate")
			f.migrateURL, f.migrateRol = url, role
			return f.applied, f.migrateErr
		},
		Seed: func(_ context.Context, url string, admin bootstrap.PlatformAdmin) (bool, error) {
			f.calls = append(f.calls, "seed")
			f.seedURL, f.seedAdmin = url, admin
			return f.created, f.seedErr
		},
	}
}

func startupCfg() config.Config {
	return config.Config{
		DatabaseURL: appURL, AppDBUser: "app_user",
		SeedAdminEmail: adminEmail, SeedAdminPassword: adminPass, SeedAdminName: "Platform Admin",
	}
}

func logger(buf *bytes.Buffer) *slog.Logger {
	return slog.New(slog.NewTextHandler(buf, nil))
}

func TestRunStartup_SinFlagsNoHaceNada(t *testing.T) {
	f := &fakeDeps{}
	if err := bootstrap.RunStartup(context.Background(), startupCfg(), logger(&bytes.Buffer{}), f.deps()); err != nil {
		t.Fatalf("sin flags no debe fallar: %v", err)
	}
	if len(f.calls) != 0 {
		t.Fatalf("sin flags no debe llamar a nada, llamó %v", f.calls)
	}
}

func TestRunStartup_AutoMigrateUsaURLDeMigracionYRolApp(t *testing.T) {
	cfg := startupCfg()
	cfg.AutoMigrate = true
	cfg.MigrateDatabaseURL = ownerURL
	f := &fakeDeps{applied: []string{"0001_init.sql"}}
	if err := bootstrap.RunStartup(context.Background(), cfg, logger(&bytes.Buffer{}), f.deps()); err != nil {
		t.Fatalf("auto migrate falló: %v", err)
	}
	if f.migrateURL != ownerURL || f.migrateRol != "app_user" {
		t.Fatalf("migrate con url/rol incorrectos: %q %q", f.migrateURL, f.migrateRol)
	}
}

func TestRunStartup_AutoMigrateCaeAlUrlDeLaApp(t *testing.T) {
	cfg := startupCfg()
	cfg.AutoMigrate = true
	f := &fakeDeps{}
	if err := bootstrap.RunStartup(context.Background(), cfg, logger(&bytes.Buffer{}), f.deps()); err != nil {
		t.Fatalf("auto migrate falló: %v", err)
	}
	if f.migrateURL != appURL {
		t.Fatalf("sin MIGRATE_DATABASE_URL debe usar DATABASE_URL, usó otra")
	}
}

func TestRunStartup_MigrarAntesQueSembrar(t *testing.T) {
	cfg := startupCfg()
	cfg.AutoMigrate, cfg.AutoSeed = true, true
	f := &fakeDeps{}
	if err := bootstrap.RunStartup(context.Background(), cfg, logger(&bytes.Buffer{}), f.deps()); err != nil {
		t.Fatalf("arranque falló: %v", err)
	}
	if strings.Join(f.calls, ",") != "migrate,seed" {
		t.Fatalf("orden incorrecto: %v", f.calls)
	}
	if f.seedURL != appURL || f.seedAdmin.Email != adminEmail || f.seedAdmin.Password != adminPass {
		t.Fatalf("seed con datos incorrectos: url=%q admin=%+v", f.seedURL, f.seedAdmin)
	}
}

func TestRunStartup_ErrorDeMigracionAbortaSinSembrar(t *testing.T) {
	cfg := startupCfg()
	cfg.AutoMigrate, cfg.AutoSeed = true, true
	cause := errors.New(errBoom)
	f := &fakeDeps{migrateErr: cause}
	err := bootstrap.RunStartup(context.Background(), cfg, logger(&bytes.Buffer{}), f.deps())
	if !errors.Is(err, cause) {
		t.Fatalf("el error debe envolver la causa con %%w, err = %v", err)
	}
	if strings.Join(f.calls, ",") != "migrate" {
		t.Fatalf("tras fallar la migración no debe sembrar: %v", f.calls)
	}
}

func TestRunStartup_ErrorDeSeedAborta(t *testing.T) {
	cfg := startupCfg()
	cfg.AutoSeed = true
	cause := errors.New(errBoom)
	f := &fakeDeps{seedErr: cause}
	err := bootstrap.RunStartup(context.Background(), cfg, logger(&bytes.Buffer{}), f.deps())
	if !errors.Is(err, cause) {
		t.Fatalf("el error del seed debe envolverse con %%w, err = %v", err)
	}
}

func TestRunStartup_LogNoRevelaContrasenia(t *testing.T) {
	cfg := startupCfg()
	cfg.AutoMigrate, cfg.AutoSeed = true, true
	var buf bytes.Buffer
	f := &fakeDeps{applied: []string{"0002_x.sql"}, created: true}
	if err := bootstrap.RunStartup(context.Background(), cfg, logger(&buf), f.deps()); err != nil {
		t.Fatalf("arranque falló: %v", err)
	}
	out := buf.String()
	for _, secreto := range []string{adminPass, "owner-pw", "app-pw", adminEmail} {
		if strings.Contains(out, secreto) {
			t.Fatalf("el log revela %q:\n%s", secreto, out)
		}
	}
	if !strings.Contains(out, "0002_x.sql") || !strings.Contains(out, "creado") {
		t.Fatalf("el log debe indicar migración y creación del superadmin:\n%s", out)
	}
}

func TestRunStartup_LogIndicaSuperadminExistente(t *testing.T) {
	cfg := startupCfg()
	cfg.AutoSeed = true
	var buf bytes.Buffer
	f := &fakeDeps{created: false}
	if err := bootstrap.RunStartup(context.Background(), cfg, logger(&buf), f.deps()); err != nil {
		t.Fatalf("arranque falló: %v", err)
	}
	if !strings.Contains(buf.String(), "ya existía") {
		t.Fatalf("el log debe indicar que el superadmin ya existía:\n%s", buf.String())
	}
}
