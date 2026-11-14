package bootstrap

import (
	"github.com/gofiber/fiber/v3"

	"prestamos/internal/platform/db"
	"prestamos/internal/platform/ids"
	tenantshttp "prestamos/internal/tenants/adapters/http"
	tenantspg "prestamos/internal/tenants/adapters/postgres"
	tenantsapp "prestamos/internal/tenants/app"
)

// newTenantsRegistrar cablea el módulo de empresas y devuelve la función que monta sus rutas
// en el grupo que reciba (el coordinador la llama con api.Group("/admin")).
func newTenantsRegistrar(tx db.TxRunner, ids ids.Generator, hasher tenantsapp.PasswordHasher, authn fiber.Handler) func(api fiber.Router) {
	svc := tenantsapp.NewService(tenantspg.NewRepo(tx), hasher, ids)
	h := tenantshttp.NewHandler(svc)
	return func(api fiber.Router) { h.Register(api, authn) }
}
