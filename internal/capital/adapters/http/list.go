package http

import (
	"time"

	"github.com/gofiber/fiber/v3"

	"prestamos/internal/capital/domain"
	"prestamos/internal/platform/httpx"
)

// List godoc
// @Summary  Listar movimientos de caja (paginación por cursor, id DESC)
// @Tags     capital
// @Security BearerAuth
// @Produce  json
// @Param    limit query int    false "1-100 (def. 20)"
// @Param    after query string false "id del último elemento visto (cursor)"
// @Param    type  query string false "tipo de movimiento"
// @Param    from  query string false "RFC3339; created_at >= from"
// @Param    to    query string false "RFC3339; created_at <= to"
// @Success  200 {object} httpx.CursorPage[MovementResponse]
// @Failure  400 {object} httpx.ErrorResponse
// @Router   /capital/movements [get]
func (h *Handler) List(c fiber.Ctx) error {
	page, perr := httpx.ParseCursor(c)
	f, ferr := parseFilter(c)
	issues := append(fieldsOf(perr), ferr...)
	if len(issues) > 0 {
		return httpx.Validation(issues...)
	}
	f.Limit, f.AfterID = page.Limit, page.After
	res, err := h.svc.List(c.Context(), tenant(c), f)
	if err != nil {
		return mapErr(err)
	}
	out := make([]MovementResponse, 0, len(res.Items))
	for _, m := range res.Items {
		out = append(out, toMovementResponse(m))
	}
	return c.JSON(httpx.NewCursorPage(out, page, res.HasMore, func(r MovementResponse) string { return r.ID }))
}

// parseFilter lee type/from/to. Devuelve todos los campos inválidos, no solo el primero.
func parseFilter(c fiber.Ctx) (domain.ListFilter, []httpx.FieldError) {
	var f domain.ListFilter
	var issues []httpx.FieldError
	if t := c.Query("type"); t != "" {
		if domain.KnownType(t) {
			f.Type = domain.Type(t)
		} else {
			issues = append(issues, httpx.FieldError{Field: "type", Message: "Tipo de movimiento inválido"})
		}
	}
	var err error
	if f.From, err = parseTime(c.Query("from")); err != nil {
		issues = append(issues, httpx.FieldError{Field: "from", Message: "Debe ser una fecha RFC3339, p. ej. 2026-01-31T00:00:00Z"})
	}
	if f.To, err = parseTime(c.Query("to")); err != nil {
		issues = append(issues, httpx.FieldError{Field: "to", Message: "Debe ser una fecha RFC3339, p. ej. 2026-01-31T00:00:00Z"})
	}
	return f, issues
}

// parseTime devuelve nil si el parámetro no vino.
func parseTime(s string) (*time.Time, error) {
	if s == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// fieldsOf extrae los campos de un VALIDATION_ERROR (nil si el error no lo es).
func fieldsOf(err error) []httpx.FieldError {
	if ae, ok := httpx.As(err); ok {
		return ae.Fields
	}
	return nil
}
