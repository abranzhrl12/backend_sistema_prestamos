package obs_test

import (
	"io"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/requestid"

	"prestamos/internal/platform/obs"
)

func TestModule_PoneElModuloEnElContexto(t *testing.T) {
	app := fiber.New()
	app.Use(requestid.New(), obs.Context())
	loans := app.Group("/loans", obs.Module("loans"))
	loans.Get("/x", func(c fiber.Ctx) error {
		m := obs.MetaFrom(c.Context())
		return c.SendString(m.Module + "|" + m.RequestID + "|" + obs.ModuleOf(c))
	})

	res := get(t, app, "/loans/x", nil)
	body, _ := io.ReadAll(res.Body)
	want := "loans|" + res.Header.Get("X-Request-Id") + "|loans"
	if string(body) != want || res.Header.Get("X-Request-Id") == "" {
		t.Fatalf("meta = %q, quiero %q", body, want)
	}
}

func TestModuleOf_SinGrupoDevuelveCore(t *testing.T) {
	app := fiber.New()
	app.Get("/x", func(c fiber.Ctx) error { return c.SendString(obs.ModuleOf(c)) })

	res := get(t, app, "/x", nil)
	body, _ := io.ReadAll(res.Body)
	if string(body) != obs.CoreModule {
		t.Fatalf("ModuleOf sin módulo = %q, quiero %q", body, obs.CoreModule)
	}
}
