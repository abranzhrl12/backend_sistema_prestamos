package config_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"prestamos/internal/platform/config"
)

const (
	devDB    = "postgres://app_user:pw@localhost:5432/prestamos?sslmode=disable"
	strongJW = "un-secreto-de-prueba-con-mas-de-32-bytes-xx"
	prodDB   = "postgres://u:p@db:5432/x"
)

// base es el mínimo válido en development.
func base() map[string]string {
	return map[string]string{"DATABASE_URL": devDB, "JWT_SECRET": strongJW}
}

// with devuelve el mínimo válido con overrides aplicados.
func with(over map[string]string) map[string]string {
	m := base()
	for k, v := range over {
		m[k] = v
	}
	return m
}

// load llama a Load con un entorno fijo (DI, sin tocar el proceso).
func load(env map[string]string) (config.Config, error) {
	return config.Load(func(k string) string { return env[k] })
}

// hasProblem indica si el error es un ConfigError con un problema para esa variable.
func hasProblem(err error, key string) bool {
	var ce *config.ConfigError
	if !errors.As(err, &ce) {
		return false
	}
	for _, p := range ce.Problems {
		if strings.HasPrefix(p, key+":") {
			return true
		}
	}
	return false
}

func TestLoad_MinimoValidoUsaDefaults(t *testing.T) {
	c, err := load(base())
	if err != nil {
		t.Fatalf("mínimo válido falló: %v", err)
	}
	checks := []struct {
		name      string
		got, want any
	}{
		{"Env", c.Env, "development"},
		{"HTTPAddr", c.HTTPAddr, ":8080"},
		{"JWTIssuer", c.JWTIssuer, "prestamos"},
		{"AccessTTL", c.AccessTTL, 15 * time.Minute},
		{"RefreshTTL", c.RefreshTTL, 720 * time.Hour},
		{"CookieSecure", c.CookieSecure, true},
		{"DBMaxConns", c.DBMaxConns, int32(0)},
		{"DBMinConns", c.DBMinConns, int32(0)},
		{"LogLevel", c.LogLevel, "info"},
		{"LogFormat", c.LogFormat, "text"},
		{"SlowQueryThreshold", c.SlowQueryThreshold, 200 * time.Millisecond},
	}
	for _, ck := range checks {
		if ck.got != ck.want {
			t.Errorf("%s = %v, quiero %v", ck.name, ck.got, ck.want)
		}
	}
}

func TestLoad_ProductionUsaLogJSONPorDefecto(t *testing.T) {
	c, err := load(with(map[string]string{"APP_ENV": "production", "DATABASE_URL": prodDB}))
	if err != nil {
		t.Fatalf("production válido falló: %v", err)
	}
	if c.LogFormat != "json" || c.Env != "production" || !c.CookieSecure {
		t.Fatalf("defaults de production incorrectos: %+v", c)
	}
}

func TestLoad_VariablesOpcionalesSePropagan(t *testing.T) {
	env := with(map[string]string{
		"HTTP_ADDR": "127.0.0.1:9000", "JWT_ISSUER": "otra", "ACCESS_TTL": "5m", "REFRESH_TTL": "48h",
		"COOKIE_SECURE": "false", "COOKIE_DOMAIN": "ejemplo.com", "DB_MAX_CONNS": "50", "DB_MIN_CONNS": "5",
		"LOG_LEVEL": "debug", "LOG_FORMAT": "json", "SLOW_QUERY_MS": "500",
	})
	c, err := load(env)
	if err != nil {
		t.Fatalf("válido falló: %v", err)
	}
	if c.HTTPAddr != "127.0.0.1:9000" || c.JWTIssuer != "otra" || c.AccessTTL != 5*time.Minute ||
		c.RefreshTTL != 48*time.Hour || c.CookieSecure || c.CookieDomain != "ejemplo.com" ||
		c.DBMaxConns != 50 || c.DBMinConns != 5 || c.LogLevel != "debug" || c.LogFormat != "json" ||
		c.SlowQueryThreshold != 500*time.Millisecond {
		t.Fatalf("valores no propagados: %+v", c)
	}
}

func TestLoad_VaciaEsIgualQueSinDefinir(t *testing.T) {
	// .env.example define variables vacías (DB_MAX_CONNS=) que deben tomar el valor por defecto.
	c, err := load(with(map[string]string{"DB_MAX_CONNS": "", "DB_MIN_CONNS": "", "COOKIE_DOMAIN": "", "HTTP_ADDR": ""}))
	if err != nil {
		t.Fatalf("variables vacías no deben fallar: %v", err)
	}
	if c.DBMaxConns != 0 || c.HTTPAddr != ":8080" || c.CookieDomain != "" {
		t.Fatalf("vacías no tomaron el default: %+v", c)
	}
}

func TestLoad_VariableInvalidaFalla(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
		key  string
	}{
		{"APP_ENV desconocido", map[string]string{"APP_ENV": "staging"}, "APP_ENV"},
		{"HTTP_ADDR sin puerto", map[string]string{"HTTP_ADDR": "localhost"}, "HTTP_ADDR"},
		{"HTTP_ADDR puerto fuera de rango", map[string]string{"HTTP_ADDR": ":99999"}, "HTTP_ADDR"},
		{"DATABASE_URL ausente", map[string]string{"DATABASE_URL": ""}, "DATABASE_URL"},
		{"DATABASE_URL de otro motor", map[string]string{"DATABASE_URL": "mysql://u:p@h/db"}, "DATABASE_URL"},
		{"DATABASE_URL no parsea", map[string]string{"DATABASE_URL": "://roto"}, "DATABASE_URL"},
		{"JWT_SECRET ausente", map[string]string{"JWT_SECRET": ""}, "JWT_SECRET"},
		{"JWT_SECRET corto", map[string]string{"JWT_SECRET": "corto"}, "JWT_SECRET"},
		{"ACCESS_TTL no es duración", map[string]string{"ACCESS_TTL": "abc"}, "ACCESS_TTL"},
		{"ACCESS_TTL menor a 1m", map[string]string{"ACCESS_TTL": "30s"}, "ACCESS_TTL"},
		{"ACCESS_TTL mayor a 24h", map[string]string{"ACCESS_TTL": "25h"}, "ACCESS_TTL"},
		{"REFRESH_TTL menor a 1h", map[string]string{"REFRESH_TTL": "10m"}, "REFRESH_TTL"},
		{"REFRESH_TTL mayor a 2160h", map[string]string{"REFRESH_TTL": "2161h"}, "REFRESH_TTL"},
		{"COOKIE_SECURE no bool", map[string]string{"COOKIE_SECURE": "quizá"}, "COOKIE_SECURE"},
		{"DB_MAX_CONNS no numérico", map[string]string{"DB_MAX_CONNS": "diez"}, "DB_MAX_CONNS"},
		{"DB_MAX_CONNS mayor a 500", map[string]string{"DB_MAX_CONNS": "501"}, "DB_MAX_CONNS"},
		{"DB_MIN_CONNS negativo", map[string]string{"DB_MIN_CONNS": "-1"}, "DB_MIN_CONNS"},
		{"DB_MIN_CONNS mayor que MAX", map[string]string{"DB_MAX_CONNS": "5", "DB_MIN_CONNS": "10"}, "DB_MIN_CONNS"},
		{"LOG_LEVEL desconocido", map[string]string{"LOG_LEVEL": "trace"}, "LOG_LEVEL"},
		{"LOG_FORMAT desconocido", map[string]string{"LOG_FORMAT": "xml"}, "LOG_FORMAT"},
		{"SLOW_QUERY_MS demasiado bajo", map[string]string{"SLOW_QUERY_MS": "5"}, "SLOW_QUERY_MS"},
		{"SLOW_QUERY_MS demasiado alto", map[string]string{"SLOW_QUERY_MS": "70000"}, "SLOW_QUERY_MS"},
		{"SLOW_QUERY_MS no numérico", map[string]string{"SLOW_QUERY_MS": "rapido"}, "SLOW_QUERY_MS"},
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

func TestLoad_ProductionRechazaInseguro(t *testing.T) {
	cases := []struct {
		name string
		over map[string]string
		key  string
	}{
		{"cookie insegura", map[string]string{"COOKIE_SECURE": "false"}, "COOKIE_SECURE"},
		{"sslmode=disable", map[string]string{"DATABASE_URL": devDB}, "DATABASE_URL"},
		{"secreto con cambia", map[string]string{"JWT_SECRET": "cambia-esto-por-32-bytes-aleatorios-xxxxxxxx"}, "JWT_SECRET"},
		{"secreto con changeme", map[string]string{"JWT_SECRET": "ChangeMe-ChangeMe-ChangeMe-ChangeMe-1234"}, "JWT_SECRET"},
		{"secreto con secret-secret", map[string]string{"JWT_SECRET": "mi-secret-secret-secret-secret-1234567"}, "JWT_SECRET"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			over := map[string]string{"APP_ENV": "production", "DATABASE_URL": prodDB}
			for k, v := range tc.over {
				over[k] = v
			}
			_, err := load(with(over))
			if !hasProblem(err, tc.key) {
				t.Fatalf("production debió rechazar %s, err = %v", tc.key, err)
			}
		})
	}
}

func TestLoad_DevelopmentAceptaSecretoDeEjemplo(t *testing.T) {
	_, err := load(with(map[string]string{"JWT_SECRET": "cambia-esto-por-32+-bytes-aleatorios-xxxxxxxx", "COOKIE_SECURE": "false"}))
	if err != nil {
		t.Fatalf("development debe aceptar el ejemplo y cookie insegura: %v", err)
	}
}

func TestLoad_ListaTodosLosProblemas(t *testing.T) {
	env := map[string]string{
		"APP_ENV": "staging", "HTTP_ADDR": "mal", "DATABASE_URL": "", "JWT_SECRET": "corto", "LOG_LEVEL": "trace",
	}
	_, err := load(env)
	var ce *config.ConfigError
	if !errors.As(err, &ce) {
		t.Fatalf("se esperaba *ConfigError, es %T", err)
	}
	if len(ce.Problems) != 5 {
		t.Fatalf("problemas = %d, quiero 5: %v", len(ce.Problems), ce.Problems)
	}
	for _, key := range []string{"APP_ENV", "HTTP_ADDR", "DATABASE_URL", "JWT_SECRET", "LOG_LEVEL"} {
		if !strings.Contains(err.Error(), "- "+key+":") {
			t.Errorf("el mensaje no menciona %s:\n%s", key, err)
		}
	}
}

func TestConfigError_UnaLineaPorProblema(t *testing.T) {
	ce := &config.ConfigError{Problems: []string{"A: uno", "B: dos"}}
	lines := 0
	for _, l := range strings.Split(ce.Error(), "\n") {
		if strings.HasPrefix(l, "- ") {
			lines++
		}
	}
	if lines != 2 {
		t.Fatalf("líneas de problema = %d, quiero 2:\n%s", lines, ce.Error())
	}
}

func TestLoad_NoRevelaSecretosEnErrores(t *testing.T) {
	_, err := load(map[string]string{"DATABASE_URL": "://usuario:clave-secreta@host", "JWT_SECRET": "corto-pero-secreto"})
	if err == nil {
		t.Fatal("se esperaba error")
	}
	for _, secreto := range []string{"clave-secreta", "corto-pero-secreto"} {
		if strings.Contains(err.Error(), secreto) {
			t.Fatalf("el error revela un secreto: %s", err)
		}
	}
}
