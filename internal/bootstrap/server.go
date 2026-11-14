package bootstrap

import (
	"log/slog"
	"time"

	"github.com/bytedance/sonic"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/requestid"

	"prestamos/internal/auth"
	customershttp "prestamos/internal/customers/adapters/http"
	"prestamos/internal/platform/httpx"
	"prestamos/internal/platform/obs"
)

// newFiberApp crea Fiber con el JSON de la API, los límites HTTP y la cadena de middlewares
// de trazabilidad: request_id -> contexto -> access log -> recover.
func newFiberApp(log *slog.Logger) *fiber.App {
	app := fiber.New(fiber.Config{
		JSONEncoder:  sonic.Marshal,
		JSONDecoder:  sonic.Unmarshal,
		ErrorHandler: httpx.ErrorHandler,
		BodyLimit:    1 << 20, // 1 MiB
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	})
	app.Use(requestid.New(), obs.Context(), obs.AccessLog(log), obs.Recover(log))
	app.Get("/healthz", func(c fiber.Ctx) error { return c.SendString("ok") })
	return app
}

// registerRoutes monta /api/v1; cada grupo fija su módulo para los logs y las métricas.
// Los módulos nuevos aportan su propio registrador (cada uno monta su grupo con obs.Module).
func registerRoutes(app *fiber.App, authHandler *auth.Handler, customers *customershttp.Handler,
	authn fiber.Handler, registrars ...func(fiber.Router)) {
	api := app.Group("/api/v1")
	authHandler.Register(api.Group("/auth", obs.Module("auth")))
	customers.Register(api.Group("/customers", obs.Module("customers")), authn)
	for _, register := range registrars {
		register(api)
	}
}

// adminGroup monta las rutas de administración de la plataforma bajo /admin (módulo "tenants").
func adminGroup(register func(fiber.Router)) func(fiber.Router) {
	return func(api fiber.Router) { register(api.Group("/admin", obs.Module("tenants"))) }
}
