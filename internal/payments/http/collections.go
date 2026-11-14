package http

import (
	"github.com/gofiber/fiber/v3"
	"prestamos/internal/payments"
	"prestamos/internal/platform/httpauth"
	"prestamos/internal/platform/httpx"
	"strings"
)

// Collections lists outstanding installments belonging to the authenticated company.
// @Summary Cuotas por cobrar
// @Tags Cobros
// @Security BearerAuth
// @Param status query string false "all, overdue, today, upcoming"
// @Param q query string false "Nombre o documento"
// @Success 200 {object} map[string]interface{}
// @Router /collections [get]
func (h *Handler) Collections(c fiber.Ctx) error {
	page, e := httpx.ParsePage(c)
	if e != nil {
		return e
	}
	status := c.Query("status", "all")
	if status != "all" && status != "overdue" && status != "today" && status != "upcoming" {
		return httpx.Validation(httpx.FieldError{Field: "status", Message: "Estado de cobro inválido"})
	}
	search := strings.TrimSpace(c.Query("q"))
	if len(search) > 100 {
		return httpx.Validation(httpx.FieldError{Field: "q", Message: "La búsqueda admite hasta 100 caracteres"})
	}
	rows, more, e := h.repo.Collections(c.Context(), httpauth.Claims(c).TenantID, payments.CollectionQuery{Status: status, Search: search, Limit: page.Limit, Offset: page.Offset})
	if e != nil {
		return e
	}
	return c.JSON(httpx.NewPage(rows, more, nil, page))
}
