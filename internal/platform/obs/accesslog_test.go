package obs_test

import (
	"bytes"
	"net/http"
	"testing"

	"prestamos/internal/platform/obs"
)

func TestAccessLog_IncluyeCamposDelRequest(t *testing.T) {
	var buf bytes.Buffer
	app := newTestApp(t, obs.NewLogger(&buf, "info", "json"))

	res := get(t, app, "/api/v1/customers/missing/123", nil)
	m := findLog(t, &buf, "http_request")

	want := map[string]any{
		"module": "customers", "method": "GET", "level": "WARN",
		"route": "/api/v1/customers/missing/:id", "path": "/api/v1/customers/missing/123",
		"status": float64(http.StatusNotFound), "error_code": "NOT_FOUND",
	}
	for k, v := range want {
		if m[k] != v {
			t.Errorf("%s = %v, quiero %v", k, m[k], v)
		}
	}
	if rid := res.Header.Get("X-Request-Id"); rid == "" || m["request_id"] != rid {
		t.Errorf("request_id = %v, quiero %q", m["request_id"], rid)
	}
	for _, k := range []string{"latency_ms", "bytes", "ip"} {
		if _, ok := m[k]; !ok {
			t.Errorf("falta el atributo %s", k)
		}
	}
	if _, ok := m["tenant_id"]; ok {
		t.Error("tenant_id no debe aparecer sin autenticación")
	}
}

func TestAccessLog_NivelSegunStatus(t *testing.T) {
	var buf bytes.Buffer
	app := newTestApp(t, obs.NewLogger(&buf, "info", "json"))
	cases := map[string]string{
		"/api/v1/customers/ok":        "INFO",
		"/api/v1/customers/missing/1": "WARN",
		"/api/v1/customers/duplicado": "WARN",
		"/api/v1/customers/fallo":     "ERROR",
	}
	for path := range cases {
		get(t, app, path, nil)
	}
	for _, m := range logLines(t, &buf) {
		if m["msg"] != "http_request" {
			continue
		}
		if want := cases[m["path"].(string)]; m["level"] != want {
			t.Errorf("%s: level = %v, quiero %s", m["path"], m["level"], want)
		}
	}
}

func TestAccessLog_5xxIncluyeErrorCode(t *testing.T) {
	var buf bytes.Buffer
	app := newTestApp(t, obs.NewLogger(&buf, "info", "json"))

	res := get(t, app, "/api/v1/customers/fallo", nil)
	m := findLog(t, &buf, "http_request")
	if res.StatusCode != http.StatusInternalServerError || m["level"] != "ERROR" || m["error_code"] != "INTERNAL_ERROR" {
		t.Fatalf("5xx mal registrado: status=%d log=%v", res.StatusCode, m)
	}
}

func TestAccessLog_HealthzNoSaleAInfo(t *testing.T) {
	var buf bytes.Buffer
	app := newTestApp(t, obs.NewLogger(&buf, "info", "json"))
	get(t, app, "/healthz", nil)
	for _, m := range logLines(t, &buf) {
		if m["msg"] == "http_request" {
			t.Fatalf("/healthz no debe loguearse a info: %v", m)
		}
	}

	var dbg bytes.Buffer
	app = newTestApp(t, obs.NewLogger(&dbg, "debug", "json"))
	get(t, app, "/healthz", nil)
	if m := findLog(t, &dbg, "http_request"); m["level"] != "DEBUG" {
		t.Fatalf("/healthz debe ser DEBUG, es %v", m["level"])
	}
}

func TestAccessLog_IncluyeTenantYUsuarioConAuth(t *testing.T) {
	var buf bytes.Buffer
	app := newTestApp(t, obs.NewLogger(&buf, "info", "json"))

	get(t, app, "/api/v1/customers/secure", map[string]string{
		"Authorization": "Bearer " + bearer(t, "user-1", "tenant-1"),
	})
	m := findLog(t, &buf, "http_request")
	if m["tenant_id"] != "tenant-1" || m["user_id"] != "user-1" || m["status"] != float64(http.StatusNoContent) {
		t.Fatalf("falta contexto de auth: %v", m)
	}
}
