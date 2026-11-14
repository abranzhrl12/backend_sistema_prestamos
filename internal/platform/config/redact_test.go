package config_test

import (
	"fmt"
	"strings"
	"testing"
)

func TestConfigFormato_NoRevelaSecretos(t *testing.T) {
	c, err := load(with(seedEnv(map[string]string{
		"MIGRATE_DATABASE_URL": "postgres://owner:owner-secreto@localhost:5432/prestamos?sslmode=disable",
	})))
	if err != nil {
		t.Fatalf("válido falló: %v", err)
	}
	secretos := []string{seedPassword, strongJW, "owner-secreto", "pw@"}
	formatos := map[string]string{
		"%v":  fmt.Sprintf("%v", c),
		"%+v": fmt.Sprintf("%+v", c),
		"%#v": fmt.Sprintf("%#v", c),
		"%s":  fmt.Sprintf("%s", c),
		"ptr": fmt.Sprintf("%v", &c),
	}
	for nombre, s := range formatos {
		for _, secreto := range secretos {
			if strings.Contains(s, secreto) {
				t.Fatalf("formato %s revela un secreto", nombre)
			}
		}
	}
}

func TestConfigFormato_MantieneCamposNoSecretos(t *testing.T) {
	c, err := load(with(seedEnv(nil)))
	if err != nil {
		t.Fatalf("válido falló: %v", err)
	}
	s := fmt.Sprintf("%+v", c)
	for _, esperado := range []string{"admin@plataforma.test", "Platform Admin", "HTTPAddr:"} {
		if !strings.Contains(s, esperado) {
			t.Fatalf("%%+v debería mostrar %q: %s", esperado, s)
		}
	}
}
