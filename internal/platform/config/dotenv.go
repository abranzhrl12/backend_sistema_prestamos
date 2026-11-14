package config

import (
	"errors"
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

// LoadDotEnv carga un archivo .env si existe. Nunca sobrescribe variables ya definidas
// en el entorno, y no hace nada en producción (APP_ENV=production en el entorno real).
func LoadDotEnv(path string) error {
	if os.Getenv(envKey) == envProduction {
		return nil
	}
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return fmt.Errorf("config: no se pudo leer %s: %w", path, err)
	}
	if err := godotenv.Load(path); err != nil {
		return fmt.Errorf("config: no se pudo cargar %s: %w", path, err)
	}
	return nil
}
