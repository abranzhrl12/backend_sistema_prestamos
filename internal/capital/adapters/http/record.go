package http

import (
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"

	"prestamos/internal/capital/domain"
	"prestamos/internal/platform/httpauth"
	"prestamos/internal/platform/httpx"
	"prestamos/internal/platform/money"
)

// MovementRequest es el cuerpo de POST /capital/movements. Amount es texto decimal ("1000.00").
type MovementRequest struct {
	Type      string `json:"type" example:"contribution" enums:"contribution,withdrawal,adjustment_in,adjustment_out"`
	Amount    string `json:"amount" example:"1000.00"`
	Note      string `json:"note" example:"Aporte inicial del socio"`
	Reference string `json:"reference" example:"TRF-0001"`
}

// MovementResponse es un movimiento del libro.
type MovementResponse struct {
	ID        string    `json:"id"`
	Type      string    `json:"type" example:"withdrawal"`
	Direction string    `json:"direction" enums:"in,out"`
	Bucket    string    `json:"bucket" enums:"capital,profit"`
	Amount    string    `json:"amount" example:"300.00"`
	Note      string    `json:"note"`
	Reference string    `json:"reference"`
	CreatedBy *string   `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
}

// Record godoc
// @Summary  Registrar movimiento manual de caja (aporte, retiro o ajuste)
// @Tags     capital
// @Security BearerAuth
// @Accept   json
// @Produce  json
// @Param    body body MovementRequest true "movimiento"
// @Success  201 {object} MovementResponse
// @Failure  400 {object} httpx.ErrorResponse
// @Failure  403 {object} httpx.ErrorResponse
// @Failure  409 {object} httpx.ErrorResponse
// @Router   /capital/movements [post]
func (h *Handler) Record(c fiber.Ctx) error {
	var req MovementRequest
	if err := c.Bind().Body(&req); err != nil {
		return httpx.InvalidJSON()
	}
	if domain.Type(strings.TrimSpace(req.Type)) == domain.TypeWithdrawal && !hasRole(c, "owner") {
		return httpx.New(fiber.StatusForbidden, httpx.CodeForbidden, "Solo el dueño puede retirar capital")
	}
	cl := httpauth.Claims(c)
	m, err := h.svc.Record(c.Context(), cl.TenantID, cl.UserID(), req.toInput())
	if err != nil {
		return mapErr(err)
	}
	return c.Status(fiber.StatusCreated).JSON(toMovementResponse(m))
}

func (r MovementRequest) toInput() domain.Input {
	return domain.Input{Type: r.Type, Amount: r.Amount, Note: r.Note, Reference: r.Reference}
}

func toMovementResponse(m domain.Movement) MovementResponse {
	return MovementResponse{
		ID: m.ID, Type: string(m.Type), Direction: string(m.Direction()), Bucket: string(m.Bucket()),
		Amount: money.Format(m.Amount), Note: m.Note, Reference: m.Reference,
		CreatedBy: m.CreatedBy, CreatedAt: m.CreatedAt,
	}
}
