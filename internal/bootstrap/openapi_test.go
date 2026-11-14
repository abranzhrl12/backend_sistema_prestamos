package bootstrap

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDocumentation_ExposesSwaggerUIAndMachineReadableContracts(t *testing.T) {
	app := newFiberApp(slog.New(slog.NewTextHandler(io.Discard, nil)))
	registerDocumentation(app)

	for _, path := range []string{"/docs", "/openapi.json", "/openapi.yaml"} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		response, err := app.Test(request)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		if response.StatusCode != http.StatusOK {
			t.Fatalf("%s: status=%d", path, response.StatusCode)
		}
	}
}
