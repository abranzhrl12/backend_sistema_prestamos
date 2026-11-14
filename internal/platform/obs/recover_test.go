package obs_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"prestamos/internal/platform/obs"
)

func TestRecover_PanicSeLogueConStackYModulo(t *testing.T) {
	var buf bytes.Buffer
	app := newTestApp(t, obs.NewLogger(&buf, "info", "json"))

	res := get(t, app, "/api/v1/customers/panic", nil)

	m := findLog(t, &buf, "panic")
	if m["level"] != "ERROR" || m["module"] != "customers" || m["panic"] != "explotó" {
		t.Fatalf("panic mal registrado: %v", m)
	}
	if stack, _ := m["stack"].(string); !strings.Contains(stack, "goroutine") {
		t.Fatalf("el log de panic no incluye stack: %v", m["stack"])
	}
	if rid, _ := m["request_id"].(string); rid == "" || rid != res.Header.Get("X-Request-Id") {
		t.Errorf("request_id del panic = %q, cabecera = %q", rid, res.Header.Get("X-Request-Id"))
	}
}

func TestRecover_PanicResponde500JSONEstandar(t *testing.T) {
	var buf bytes.Buffer
	app := newTestApp(t, obs.NewLogger(&buf, "info", "json"))

	res := get(t, app, "/api/v1/customers/panic", nil)
	body, _ := io.ReadAll(res.Body)
	var out struct {
		Error struct {
			Code      string `json:"code"`
			RequestID string `json:"request_id"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("respuesta no es JSON: %s", body)
	}
	if res.StatusCode != http.StatusInternalServerError || out.Error.Code != "INTERNAL_ERROR" || out.Error.RequestID == "" {
		t.Fatalf("respuesta de panic = %d %s", res.StatusCode, body)
	}
	if strings.Contains(string(body), "explotó") {
		t.Fatal("el detalle del panic no debe llegar al cliente")
	}
}
