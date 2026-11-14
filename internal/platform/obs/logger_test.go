package obs_test

import (
	"bytes"
	"strings"
	"testing"

	"prestamos/internal/platform/obs"
)

func TestNewLogger_JSONIncluyeServicio(t *testing.T) {
	var buf bytes.Buffer
	obs.NewLogger(&buf, "info", "json").Info("hola")
	m := logLines(t, &buf)
	if len(m) != 1 || m[0]["service"] != "prestamos" || m[0]["msg"] != "hola" {
		t.Fatalf("salida JSON inesperada: %s", buf.String())
	}
}

func TestNewLogger_NivelFiltraDebug(t *testing.T) {
	var buf bytes.Buffer
	log := obs.NewLogger(&buf, "info", "json")
	log.Debug("oculto")
	if buf.Len() != 0 {
		t.Fatalf("con nivel info no debe salir debug: %s", buf.String())
	}
}

func TestNewLogger_NivelDesconocidoEsInfo(t *testing.T) {
	var buf bytes.Buffer
	log := obs.NewLogger(&buf, "raro", "json")
	log.Debug("oculto")
	log.Info("visible")
	if got := logLines(t, &buf); len(got) != 1 || got[0]["msg"] != "visible" {
		t.Fatalf("nivel desconocido debe equivaler a info: %s", buf.String())
	}
}

func TestNewLogger_FormatoTextoIncluyeServicio(t *testing.T) {
	var buf bytes.Buffer
	obs.NewLogger(&buf, "info", "text").Info("hola")
	if !strings.Contains(buf.String(), "service=prestamos") {
		t.Fatalf("formato text sin service: %s", buf.String())
	}
}
