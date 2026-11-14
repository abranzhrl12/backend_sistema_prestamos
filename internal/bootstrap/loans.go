package bootstrap

import (
	"github.com/gofiber/fiber/v3"
	loanhttp "prestamos/internal/loans/adapters/http"
	loanpg "prestamos/internal/loans/adapters/postgres"
	loanapp "prestamos/internal/loans/app"
	"prestamos/internal/payments"
	paymentshttp "prestamos/internal/payments/http"
	"prestamos/internal/platform/db"
	"prestamos/internal/platform/httpauth"
	"prestamos/internal/platform/ids"
	"prestamos/internal/platform/obs"
)

func newLoansRegistrar(tx db.TxRunner, gen ids.Generator, authn fiber.Handler) func(fiber.Router) {
	repo := loanpg.NewRepo(tx, gen)
	handler := loanhttp.NewHandler(repo, loanapp.NewService())
	return func(api fiber.Router) {
		group := api.Group("/loans", obs.Module("loans"))
		handler.Register(group, authn)
		paymentHandler := paymentshttp.NewHandler(payments.NewRepo(tx, gen))
		paymentHandler.Register(group, authn)
		api.Get("/collections", authn, httpauth.RequireRole("owner", "admin"), paymentHandler.Collections)
	}
}
