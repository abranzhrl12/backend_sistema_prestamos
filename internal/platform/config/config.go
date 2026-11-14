// Package config carga la configuración desde variables de entorno de forma ESTRICTA:
// si alguna variable está definida pero es inválida, Load falla y lista TODOS los problemas.
package config

import (
	"os"
	"strings"
	"time"
)

const (
	envKey        = "APP_ENV"
	envProduction = "production"
	minSecretLen  = 32
	msPerSlowUnit = time.Millisecond
)

// weakSecretMarkers son fragmentos típicos de los valores de ejemplo; en production se rechazan.
var weakSecretMarkers = []string{"cambia", "changeme", "secret-secret"}

// Config es la configuración tipada de la aplicación.
type Config struct {
	Env                string
	HTTPAddr           string
	DatabaseURL        string
	JWTSecret          string
	JWTIssuer          string
	AccessTTL          time.Duration
	RefreshTTL         time.Duration
	CookieSecure       bool
	CookieDomain       string
	DBMaxConns         int32
	DBMinConns         int32
	LogLevel           string
	LogFormat          string
	SlowQueryThreshold time.Duration
	AutoMigrate        bool
	AutoSeed           bool
	MigrateDatabaseURL string
	AppDBUser          string
	SeedAdminEmail     string
	SeedAdminPassword  string
	SeedAdminName      string
}

// ConfigError lista todos los problemas de configuración encontrados al arrancar.
type ConfigError struct{ Problems []string }

func (e *ConfigError) Error() string {
	var b strings.Builder
	b.WriteString("configuración inválida:")
	for _, p := range e.Problems {
		b.WriteString("\n- " + p)
	}
	return b.String()
}

// LoadFromOS lee la configuración del entorno del proceso.
func LoadFromOS() (Config, error) {
	return Load(os.Getenv)
}

// Load lee y valida la configuración usando getenv (inyectable en tests).
func Load(getenv func(string) string) (Config, error) {
	r := &reader{get: getenv}
	var c Config
	c.Env = r.enum(envKey, "development", "development", "test", envProduction)
	prod := c.Env == envProduction

	c.HTTPAddr = r.addr("HTTP_ADDR", defaultAddr(r))
	loadDatabase(r, &c, prod)
	loadAuth(r, &c, prod)
	loadLogging(r, &c, prod)
	loadStartup(r, &c, prod)

	if len(r.problems) > 0 {
		return c, &ConfigError{Problems: r.problems}
	}
	return c, nil
}

// defaultAddr arma host:puerto desde SERVER_HOST/SERVER_PORT (convención del .env del proyecto)
// cuando HTTP_ADDR no está definida; HTTP_ADDR, si existe, tiene prioridad.
func defaultAddr(r *reader) string {
	port := r.raw("SERVER_PORT")
	if port == "" {
		return ":8080"
	}
	return r.raw("SERVER_HOST") + ":" + port
}

func loadDatabase(r *reader, c *Config, prod bool) {
	c.DatabaseURL = r.postgresURL("DATABASE_URL")
	checkTLS(r, "DATABASE_URL", c.DatabaseURL, prod)
	c.DBMaxConns = int32(r.integer("DB_MAX_CONNS", 0, 0, 500))
	c.DBMinConns = int32(r.integer("DB_MIN_CONNS", 0, 0, 500))
	if c.DBMaxConns > 0 && c.DBMinConns > c.DBMaxConns {
		r.fail("DB_MIN_CONNS", "no puede ser mayor que DB_MAX_CONNS")
	}
}

// checkTLS rechaza en production una URL de BD sin TLS (sslmode=disable). Vacía no se comprueba.
func checkTLS(r *reader, key, dbURL string, prod bool) {
	if prod && strings.Contains(strings.ToLower(dbURL), "sslmode=disable") {
		r.fail(key, "en production no puede usar sslmode=disable")
	}
}

func loadAuth(r *reader, c *Config, prod bool) {
	c.JWTSecret = r.raw("JWT_SECRET")
	checkSecret(r, c.JWTSecret, prod)
	c.JWTIssuer = r.str("JWT_ISSUER", "prestamos")
	c.AccessTTL = r.duration("ACCESS_TTL", 15*time.Minute, time.Minute, 24*time.Hour)
	c.RefreshTTL = r.duration("REFRESH_TTL", 720*time.Hour, time.Hour, 2160*time.Hour)
	c.CookieSecure = r.boolean("COOKIE_SECURE", true)
	if prod && !c.CookieSecure {
		r.fail("COOKIE_SECURE", "en production debe ser true")
	}
	c.CookieDomain = r.raw("COOKIE_DOMAIN")
}

func checkSecret(r *reader, secret string, prod bool) {
	switch {
	case secret == "":
		r.fail("JWT_SECRET", "es obligatoria")
	case len(secret) < minSecretLen:
		r.fail("JWT_SECRET", "debe tener al menos 32 bytes")
	case prod && hasWeakMarker(secret):
		r.fail("JWT_SECRET", "en production no puede ser un valor de ejemplo")
	}
}

func hasWeakMarker(secret string) bool {
	lower := strings.ToLower(secret)
	for _, m := range weakSecretMarkers {
		if strings.Contains(lower, m) {
			return true
		}
	}
	return false
}

func loadLogging(r *reader, c *Config, prod bool) {
	c.LogLevel = r.enum("LOG_LEVEL", "info", "debug", "info", "warn", "error")
	defFormat := "text"
	if prod {
		defFormat = "json"
	}
	c.LogFormat = r.enum("LOG_FORMAT", defFormat, "json", "text")
	c.SlowQueryThreshold = time.Duration(r.integer("SLOW_QUERY_MS", 200, 10, 60000)) * msPerSlowUnit
}
