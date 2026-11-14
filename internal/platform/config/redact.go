package config

import "fmt"

const redactedValue = "***"

// plainConfig no tiene métodos: formatearlo no vuelve a llamar a String/GoString.
type plainConfig Config

// String oculta los secretos (JWT, URLs de BD con contraseña, contraseña del seed) para que
// %v, %+v y cualquier log que imprima la configuración no los revele.
func (c Config) String() string {
	return fmt.Sprintf("%+v", plainConfig(c.redacted()))
}

// GoString aplica la misma redacción que String para %#v.
func (c Config) GoString() string {
	return fmt.Sprintf("%#v", plainConfig(c.redacted()))
}

func (c Config) redacted() Config {
	c.JWTSecret = maskSecret(c.JWTSecret)
	c.DatabaseURL = maskSecret(c.DatabaseURL)
	c.MigrateDatabaseURL = maskSecret(c.MigrateDatabaseURL)
	c.SeedAdminPassword = maskSecret(c.SeedAdminPassword)
	return c
}

// maskSecret conserva la ausencia (vacío) y oculta cualquier valor definido.
func maskSecret(v string) string {
	if v == "" {
		return ""
	}
	return redactedValue
}
