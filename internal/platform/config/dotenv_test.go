package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"prestamos/internal/platform/config"
)

// writeEnvFile crea un .env temporal con el contenido dado.
func writeEnvFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// unset deja la variable sin definir y la restaura al final del test.
func unset(t *testing.T, key string) {
	t.Helper()
	t.Setenv(key, "")
	if err := os.Unsetenv(key); err != nil {
		t.Fatal(err)
	}
}

func TestLoadDotEnv_ArchivoInexistenteNoFalla(t *testing.T) {
	if err := config.LoadDotEnv(filepath.Join(t.TempDir(), "no-existe.env")); err != nil {
		t.Fatalf("un .env ausente no debe fallar: %v", err)
	}
}

func TestLoadDotEnv_CargaVariables(t *testing.T) {
	const key = "PRESTAMOS_TEST_CARGA"
	unset(t, key)
	path := writeEnvFile(t, key+"=valor-del-archivo\n")
	if err := config.LoadDotEnv(path); err != nil {
		t.Fatalf("LoadDotEnv: %v", err)
	}
	if got := os.Getenv(key); got != "valor-del-archivo" {
		t.Fatalf("%s = %q, quiero valor-del-archivo", key, got)
	}
}

func TestLoadDotEnv_NoSobrescribeEntorno(t *testing.T) {
	const key = "PRESTAMOS_TEST_NOSOBRE"
	t.Setenv(key, "desde-entorno")
	path := writeEnvFile(t, key+"=desde-archivo\n")
	if err := config.LoadDotEnv(path); err != nil {
		t.Fatalf("LoadDotEnv: %v", err)
	}
	if got := os.Getenv(key); got != "desde-entorno" {
		t.Fatalf("el entorno fue sobrescrito: %q", got)
	}
}

func TestLoadDotEnv_ProductionNoCarga(t *testing.T) {
	const key = "PRESTAMOS_TEST_PROD"
	t.Setenv("APP_ENV", "production")
	unset(t, key)
	path := writeEnvFile(t, key+"=no-debe-cargarse\n")
	if err := config.LoadDotEnv(path); err != nil {
		t.Fatalf("LoadDotEnv: %v", err)
	}
	if got, ok := os.LookupEnv(key); ok {
		t.Fatalf("en production no debe cargarse .env, se cargó %q", got)
	}
}
