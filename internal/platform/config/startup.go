package config

import "regexp"

const (
	defaultAppDBUser     = "app_user"
	defaultSeedAdminName = "Platform Admin"
)

// appRoleRe es la misma regla que migrate.ValidRole. Config no importa platform/migrate
// para no arrastrar pgx a la capa de configuración.
var appRoleRe = regexp.MustCompile(`^[a-z_][a-z0-9_]{0,62}$`)

// loadStartup lee las variables del arranque automático (migraciones y seed).
func loadStartup(r *reader, c *Config, prod bool) {
	c.AutoMigrate = r.boolean("AUTO_MIGRATE", false)
	c.AutoSeed = r.boolean("AUTO_SEED", false)
	c.MigrateDatabaseURL = r.optionalPostgresURL("MIGRATE_DATABASE_URL")
	checkTLS(r, "MIGRATE_DATABASE_URL", c.MigrateDatabaseURL, prod)
	c.AppDBUser = r.str("APP_DB_USER", defaultAppDBUser)
	if !appRoleRe.MatchString(c.AppDBUser) {
		r.fail("APP_DB_USER", "solo minúsculas, dígitos y guion bajo; debe empezar por letra o _")
	}
	c.SeedAdminEmail = r.raw("SEED_ADMIN_EMAIL")
	c.SeedAdminPassword = r.raw("SEED_ADMIN_PASSWORD")
	c.SeedAdminName = r.str("SEED_ADMIN_NAME", defaultSeedAdminName)
	loadAutoSeed(r, c, prod)
}

// loadAutoSeed valida el seed automático: nunca en production y con credenciales completas.
func loadAutoSeed(r *reader, c *Config, prod bool) {
	if !c.AutoSeed {
		return
	}
	if prod {
		r.fail("AUTO_SEED", "en production no se permite; el seed de producción se ejecuta a mano con go run ./cmd/seed")
		return
	}
	if c.SeedAdminEmail == "" {
		r.fail("SEED_ADMIN_EMAIL", "es obligatoria si AUTO_SEED=true")
	}
	if c.SeedAdminPassword == "" {
		r.fail("SEED_ADMIN_PASSWORD", "es obligatoria si AUTO_SEED=true")
	}
}
