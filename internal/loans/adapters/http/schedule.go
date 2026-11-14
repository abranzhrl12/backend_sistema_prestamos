package http

import (
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"prestamos/internal/platform/httpx"
)

type scheduleResponse struct {
	ID             string `json:"id"`
	Number         int    `json:"installment_no"`
	DueDate        string `json:"due_date"`
	PrincipalDue   string `json:"principal_due"`
	InterestDue    string `json:"interest_due"`
	LateFeeAccrued string `json:"late_fee_accrued"`
	PrincipalPaid  string `json:"principal_paid"`
	InterestPaid   string `json:"interest_paid"`
	LateFeePaid    string `json:"late_fee_paid"`
}

// ScheduleListResponse documents the schedule of a single loan.
type ScheduleListResponse struct {
	Items []scheduleResponse `json:"items"`
}

// Schedule godoc
// @Summary Consultar cronograma de cuotas
// @Tags prestamos
// @Security BearerAuth
// @Produce json
// @Param id path string true "ID del préstamo"
// @Success 200 {object} ScheduleListResponse
// @Failure 404 {object} httpx.ErrorResponse
// @Router /loans/{id}/schedule [get]
func (h *Handler) Schedule(c fiber.Ctx) error {
	id := c.Params("id")
	if _, e := uuid.Parse(id); e != nil {
		return httpx.NotFound("El préstamo no existe")
	}
	rows, e := h.repo.Schedule(c.Context(), actor(c).TenantID, id)
	if e != nil {
		return mapErr(e)
	}
	out := make([]scheduleResponse, 0, len(rows))
	for _, r := range rows {
		out = append(out, scheduleResponse{r.ID, r.Number, r.DueDate, r.PrincipalDue, r.InterestDue, r.LateFeeAccrued, r.PrincipalPaid, r.InterestPaid, r.LateFeePaid})
	}
	return c.JSON(fiber.Map{"items": out})
}
