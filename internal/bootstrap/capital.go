package bootstrap

import (
	"github.com/gofiber/fiber/v3"

	capitalhttp "prestamos/internal/capital/adapters/http"
	capitalpg "prestamos/internal/capital/adapters/postgres"
	capitalapp "prestamos/internal/capital/app"
	"prestamos/internal/platform/db"
	"prestamos/internal/platform/ids"
	"prestamos/internal/platform/obs"
)

// newCapitalRegistrar cablea el módulo de caja única y devuelve la función que monta /capital
// en el grupo que reciba (el coordinador la llama con api).
func newCapitalRegistrar(tx db.TxRunner, ids ids.Generator, authn fiber.Handler, settings capitalapp.SettingsReader) func(api fiber.Router) {
	svc := capitalapp.NewService(capitalpg.NewRepo(tx), settings, ids)
	h := capitalhttp.NewHandler(svc)
	return func(api fiber.Router) { h.Register(api.Group("/capital", obs.Module("capital")), authn) }
}
