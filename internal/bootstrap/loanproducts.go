package bootstrap

import (
	"github.com/gofiber/fiber/v3"

	lphttp "prestamos/internal/loanproducts/adapters/http"
	lppg "prestamos/internal/loanproducts/adapters/postgres"
	lpapp "prestamos/internal/loanproducts/app"
	"prestamos/internal/platform/db"
	"prestamos/internal/platform/ids"
	"prestamos/internal/platform/obs"
)

// newLoanProductsRegistrar cablea los productos de préstamo y devuelve la función que monta /loan-products.
func newLoanProductsRegistrar(tx db.TxRunner, ids ids.Generator, authn fiber.Handler) func(api fiber.Router) {
	svc := lpapp.NewService(lppg.NewRepo(tx), ids)
	h := lphttp.NewHandler(svc)
	return func(api fiber.Router) { h.Register(api.Group("/loan-products", obs.Module("loan-products")), authn) }
}
