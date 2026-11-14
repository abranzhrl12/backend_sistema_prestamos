// Package http expone la configuración de la empresa por REST (Fiber).
package http

import (
	"errors"

	"github.com/gofiber/fiber/v3"

	"prestamos/internal/platform/httpauth"
	"prestamos/internal/platform/httpx"
	"prestamos/internal/settings/app"
	"prestamos/internal/settings/domain"
)

type Handler struct{ svc *app.Service }

func NewHandler(svc *app.Service) *Handler { return &Handler{svc: svc} }

// Register monta /settings. Lectura: cualquier usuario autenticado; escritura: owner/admin.
func (h *Handler) Register(g fiber.Router, authn fiber.Handler) {
	write := httpauth.RequireRole("owner", "admin")
	g.Get("/", authn, h.Get)
	g.Put("/", authn, write, h.Update)
}

// Get godoc
// @Summary  Obtener configuración de la empresa
// @Tags     settings
// @Security BearerAuth
// @Produce  json
// @Success  200 {object} SettingsResponse
// @Failure  401 {object} httpx.ErrorResponse
// @Router   /settings [get]
func (h *Handler) Get(c fiber.Ctx) error {
	out, err := h.svc.Get(c.Context(), httpauth.Claims(c).TenantID)
	if err != nil {
		return mapErr(err)
	}
	return c.JSON(toResponse(out))
}

// Update godoc
// @Summary  Reemplazar configuración de la empresa (owner/admin)
// @Description Reemplazo total: todos los campos son obligatorios.
// @Tags     settings
// @Security BearerAuth
// @Accept   json
// @Produce  json
// @Param    body body SettingsRequest true "configuración completa"
// @Success  200 {object} SettingsResponse
// @Failure  400 {object} httpx.ErrorResponse
// @Failure  403 {object} httpx.ErrorResponse
// @Router   /settings [put]
func (h *Handler) Update(c fiber.Ctx) error {
	var req SettingsRequest
	if err := c.Bind().Body(&req); err != nil {
		return httpx.InvalidJSON()
	}
	in, missing := req.toDomain()
	if len(missing) > 0 {
		return httpx.Validation(missing...)
	}
	cl := httpauth.Claims(c)
	out, err := h.svc.Update(c.Context(), cl.TenantID, cl.UserID(), in)
	if err != nil {
		return mapErr(err)
	}
	return c.JSON(toResponse(out))
}

func mapErr(err error) error {
	var ve *domain.ValidationError
	if errors.As(err, &ve) {
		fields := make([]httpx.FieldError, len(ve.Issues))
		for i, is := range ve.Issues {
			fields[i] = httpx.FieldError{Field: is.Field, Message: is.Msg}
		}
		return httpx.Validation(fields...)
	}
	return err
}
