package bootstrap

import (
	"github.com/gofiber/fiber/v3"

	"prestamos/internal/platform/db"
	"prestamos/internal/platform/ids"
	"prestamos/internal/platform/obs"
	settingshttp "prestamos/internal/settings/adapters/http"
	settingspg "prestamos/internal/settings/adapters/postgres"
	settingsapp "prestamos/internal/settings/app"
)

// newSettingsRegistrar cablea la configuración de la empresa. register monta /settings (el coordinador
// la llama con api); svc lo consumen otros módulos, p. ej. para AllowNegativeCapital.
func newSettingsRegistrar(tx db.TxRunner, ids ids.Generator, authn fiber.Handler) (register func(api fiber.Router), svc *settingsapp.Service) {
	svc = settingsapp.NewService(settingspg.NewRepo(tx), ids)
	h := settingshttp.NewHandler(svc)
	register = func(api fiber.Router) { h.Register(api.Group("/settings", obs.Module("settings")), authn) }
	return register, svc
}
