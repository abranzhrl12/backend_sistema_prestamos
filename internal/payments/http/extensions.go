package http

import (
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"prestamos/internal/payments"
	"prestamos/internal/platform/httpauth"
	"prestamos/internal/platform/httpx"
	"strings"
	"time"
)

type extensionBody struct {
	FirstPaymentDate   string `json:"first_payment_date"`
	AdditionalInterest string `json:"additional_interest"`
	Note               string `json:"note"`
}

// Extend records an explicit agreement and preserves the previous schedule.
// @Summary Ampliar el plazo de un préstamo
// @Tags Cobros
// @Security BearerAuth
// @Param id path string true "Préstamo"
// @Param Idempotency-Key header string true "Clave del acuerdo"
// @Param body body extensionBody true "Nueva fecha e interés adicional acordado"
// @Success 201 {object} payments.Extension
// @Router /loans/{id}/extensions [post]
func (h *Handler) Extend(c fiber.Ctx) error {
	if _, e := uuid.Parse(c.Params("id")); e != nil {
		return httpx.NotFound("El préstamo no existe")
	}
	var b extensionBody
	if e := c.Bind().Body(&b); e != nil {
		return httpx.InvalidJSON()
	}
	interest, e := decimal.NewFromString(b.AdditionalInterest)
	if e != nil || interest.IsNegative() || !interest.Equal(interest.Round(4)) || interest.GreaterThanOrEqual(decimal.RequireFromString("1000000000000000")) {
		return httpx.Validation(httpx.FieldError{Field: "additional_interest", Message: "Ingrese un interés adicional válido"})
	}
	if _, e := time.Parse("2006-01-02", b.FirstPaymentDate); e != nil {
		return httpx.Validation(httpx.FieldError{Field: "first_payment_date", Message: "Seleccione una fecha válida"})
	}
	if strings.TrimSpace(b.Note) == "" || len(b.Note) > 500 {
		return httpx.Validation(httpx.FieldError{Field: "note", Message: "Describa el acuerdo en hasta 500 caracteres"})
	}
	cl := httpauth.Claims(c)
	out, e := h.repo.Extend(c.Context(), cl.TenantID, payments.ExtensionInput{LoanID: c.Params("id"), UserID: cl.UserID(), Key: c.Get("Idempotency-Key"), FirstPaymentDate: b.FirstPaymentDate, AdditionalInterest: interest, Note: b.Note})
	if e != nil {
		return mapErr(e)
	}
	return c.Status(201).JSON(out)
}

// Extensions lists recorded agreements without modifying payment history.
// @Summary Historial de ampliaciones
// @Tags Cobros
// @Security BearerAuth
// @Param id path string true "Préstamo"
// @Success 200 {object} map[string]interface{}
// @Router /loans/{id}/extensions [get]
func (h *Handler) Extensions(c fiber.Ctx) error {
	if _, e := uuid.Parse(c.Params("id")); e != nil {
		return httpx.NotFound("El préstamo no existe")
	}
	page, e := httpx.ParsePage(c)
	if e != nil {
		return e
	}
	rows, more, e := h.repo.Extensions(c.Context(), httpauth.Claims(c).TenantID, c.Params("id"), page.Limit, page.Offset)
	if e != nil {
		return mapErr(e)
	}
	return c.JSON(httpx.NewPage(rows, more, nil, page))
}
