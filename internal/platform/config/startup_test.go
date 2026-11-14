package config_test

import "testing"

const seedPassword = "Sup3r-Secreta-Seed-2026"

func seedEnv(extra map[string]string) map[string]string {
	m := map[string]string{"AUTO_SEED": "true", "SEED_ADMIN_EMAIL": "admin@plataforma.test", "SEED_ADMIN_PASSWORD": seedPassword}
	for k, v := range extra {
		m[k] = v
	}
	return m
}

func TestLoadStartup_DefectosDesactivados(t *testing.T) {
	c, err := load(base())
	if err != nil {
		t.Fatalf("mínimo válido falló: %v", err)
	}
	if c.AutoMigrate || c.AutoSeed {
		t.Fatalf("AUTO_MIGRATE/AUTO_SEED deben ser false por defecto: %+v", c)
	}
	if c.AppDBUser != "app_user" || c.SeedAdminName != "Platform Admin" || c.MigrateDatabaseURL != "" {
		t.Fatalf("defaults de arranque incorrectos: %+v", c)
	}
}

func TestLoadStartup_FlagsYCredencialesSePropagan(t *testing.T) {
	env := with(seedEnv(map[string]string{
		"AUTO_MIGRATE": "true", "MIGRATE_DATABASE_URL": "postgres://owner:o@localhost:5432/prestamos?sslmode=disable",
		"APP_DB_USER": "mi_app", "SEED_ADMIN_NAME": "Root",
	}))
	c, err := load(env)
	if err != nil {
		t.Fatalf("válido falló: %v", err)
	}
	if !c.AutoMigrate || !c.AutoSeed || c.MigrateDatabaseURL != env["MIGRATE_DATABASE_URL"] ||
		c.AppDBUser != "mi_app" || c.SeedAdminEmail != "admin@plataforma.test" ||
		c.SeedAdminPassword != seedPassword || c.SeedAdminName != "Root" {
		t.Fatalf("valores no propagados: %+v", c)
	}
}

func TestLoadStartup_AutoSeedSinCredencialesFalla(t *testing.T) {
	cases := []struct {
		name string
		drop string
		key  string
	}{
		{"sin email", "SEED_ADMIN_EMAIL", "SEED_ADMIN_EMAIL"},
		{"sin password", "SEED_ADMIN_PASSWORD", "SEED_ADMIN_PASSWORD"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := seedEnv(map[string]string{tc.drop: ""})
			_, err := load(with(env))
			if !hasProblem(err, tc.key) {
				t.Fatalf("se esperaba problema en %s, err = %v", tc.key, err)
			}
		})
	}
}

func TestLoadStartup_AutoSeedFalseNoExigeCredenciales(t *testing.T) {
	_, err := load(with(map[string]string{"AUTO_SEED": "false"}))
	if err != nil {
		t.Fatalf("sin AUTO_SEED no deben exigirse credenciales: %v", err)
	}
}

func TestLoadStartup_AutoSeedEnProductionSeRechaza(t *testing.T) {
	over := seedEnv(map[string]string{"APP_ENV": "production", "DATABASE_URL": prodDB, "COOKIE_SECURE": "true"})
	_, err := load(with(over))
	if !hasProblem(err, "AUTO_SEED") {
		t.Fatalf("production debe rechazar AUTO_SEED=true, err = %v", err)
	}
}

func TestLoadStartup_AutoMigrateEnProductionPermitido(t *testing.T) {
	over := map[string]string{"APP_ENV": "production", "DATABASE_URL": prodDB, "COOKIE_SECURE": "true", "AUTO_MIGRATE": "true"}
	c, err := load(with(over))
	if err != nil {
		t.Fatalf("production debe permitir AUTO_MIGRATE=true: %v", err)
	}
	if !c.AutoMigrate || c.AutoSeed {
		t.Fatalf("flags incorrectos en production: %+v", c)
	}
}

func TestLoadStartup_VariablesInvalidasFallan(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
		key  string
	}{
		{"AUTO_MIGRATE no bool", map[string]string{"AUTO_MIGRATE": "quizá"}, "AUTO_MIGRATE"},
		{"AUTO_SEED no bool", map[string]string{"AUTO_SEED": "si"}, "AUTO_SEED"},
		{"MIGRATE_DATABASE_URL de otro motor", map[string]string{"MIGRATE_DATABASE_URL": "mysql://u:p@h/db"}, "MIGRATE_DATABASE_URL"},
		{"MIGRATE_DATABASE_URL no parsea", map[string]string{"MIGRATE_DATABASE_URL": "://roto"}, "MIGRATE_DATABASE_URL"},
		{"APP_DB_USER con mayúsculas", map[string]string{"APP_DB_USER": "App_User"}, "APP_DB_USER"},
		{"APP_DB_USER con inyección", map[string]string{"APP_DB_USER": "app; DROP TABLE x"}, "APP_DB_USER"},
		{"APP_DB_USER demasiado largo", map[string]string{"APP_DB_USER": longName()}, "APP_DB_USER"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := load(with(tc.env))
			if !hasProblem(err, tc.key) {
				t.Fatalf("se esperaba problema en %s, err = %v", tc.key, err)
			}
		})
	}
}

func TestLoadStartup_MigrateURLEnProductionNoPuedeSerInseguro(t *testing.T) {
	over := map[string]string{
		"APP_ENV": "production", "DATABASE_URL": "postgres://u:p@db:5432/x?sslmode=require",
		"COOKIE_SECURE": "true", "MIGRATE_DATABASE_URL": "postgres://o:p@db:5432/x?sslmode=disable",
	}
	_, err := load(with(over))
	if !hasProblem(err, "MIGRATE_DATABASE_URL") {
		t.Fatalf("production debe rechazar sslmode=disable en MIGRATE_DATABASE_URL, err = %v", err)
	}
}

// longName devuelve una cadena que hace APP_DB_USER superar 63 caracteres.
func longName() string {
	b := make([]byte, 64)
	for i := range b {
		b[i] = 'a'
	}
	return string(b)
}
