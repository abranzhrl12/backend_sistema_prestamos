// Package http expone la caja única de la empresa por REST (Fiber).
package http

import (
	"time"

	"github.com/gofiber/fiber/v3"

	"prestamos/internal/capital/app"
	"prestamos/internal/platform/httpauth"
	"prestamos/internal/platform/money"
)

type Handler struct{ svc *app.Service }

func NewHandler(svc *app.Service) *Handler { return &Handler{svc: svc} }

// Register monta /capital. Lectura: owner, admin y viewer. Escritura: owner y admin
// (el retiro de capital exige owner; se comprueba en Record tras leer el tipo).
func (h *Handler) Register(g fiber.Router, authn fiber.Handler) {
	read := httpauth.RequireRole("owner", "admin", "viewer")
	write := httpauth.RequireRole("owner", "admin")
	g.Get("/", authn, read, h.Balance)
	g.Get("/movements", authn, read, h.List)
	g.Post("/movements", authn, write, h.Record)
}

// BalanceResponse es la foto de la caja. Montos como texto decimal.
type BalanceResponse struct {
	TotalCash       string    `json:"total_cash" example:"1000.00"`
	CapitalBalance  string    `json:"capital_balance" example:"800.00"`
	ProfitBalance   string    `json:"profit_balance" example:"200.00"`
	LentOutstanding string    `json:"lent_outstanding" example:"0.00"`
	MovementsCount  int       `json:"movements_count" example:"3"`
	AsOf            time.Time `json:"as_of"`
}

// Balance godoc
// @Summary  Saldo de la caja (total, capital, utilidad y capital prestado)
// @Tags     capital
// @Security BearerAuth
// @Produce  json
// @Success  200 {object} BalanceResponse
// @Failure  403 {object} httpx.ErrorResponse
// @Router   /capital [get]
func (h *Handler) Balance(c fiber.Ctx) error {
	b, err := h.svc.Balance(c.Context(), tenant(c))
	if err != nil {
		return mapErr(err)
	}
	return c.JSON(BalanceResponse{
		TotalCash: money.Format(b.TotalCash), CapitalBalance: money.Format(b.CapitalBalance),
		ProfitBalance: money.Format(b.ProfitBalance), LentOutstanding: money.Format(b.LentOutstanding),
		MovementsCount: b.MovementsCount, AsOf: b.AsOf,
	})
}

func tenant(c fiber.Ctx) string { return httpauth.Claims(c).TenantID }

// hasRole indica si el usuario autenticado tiene el rol indicado.
func hasRole(c fiber.Ctx, role string) bool {
	cl := httpauth.Claims(c)
	if cl == nil {
		return false
	}
	for _, r := range cl.Roles {
		if r == role {
			return true
		}
	}
	return false
}
