package http

import (
	"errors"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"prestamos/internal/platform/httpx"
	"prestamos/internal/tenants/domain"
)

const (
	msgTenantNotFound = "La empresa no existe"
	msgUserNotFound   = "El usuario no existe"
)

// mapErr traduce los errores de dominio a la respuesta HTTP (mensajes en español).
func mapErr(err error) error {
	var ve *domain.ValidationError
	switch {
	case errors.As(err, &ve):
		fields := make([]httpx.FieldError, len(ve.Issues))
		for i, is := range ve.Issues {
			fields[i] = httpx.FieldError{Field: is.Field, Message: is.Msg}
		}
		return httpx.Validation(fields...)
	case errors.Is(err, domain.ErrTenantNotFound):
		return httpx.NotFound(msgTenantNotFound)
	case errors.Is(err, domain.ErrUserNotFound):
		return httpx.NotFound(msgUserNotFound)
	case errors.Is(err, domain.ErrSlugTaken):
		return httpx.Conflict("TENANT_SLUG_TAKEN", "Ya existe una empresa con ese identificador")
	case errors.Is(err, domain.ErrEmailTaken):
		return httpx.Conflict("EMAIL_TAKEN", "Ya existe un usuario con ese correo")
	case errors.Is(err, domain.ErrPlatformProtected):
		return httpx.Conflict("PLATFORM_TENANT_PROTECTED", "La empresa de la plataforma está protegida: no se puede eliminar, suspender ni modificar así")
	case errors.Is(err, domain.ErrLastSuperadmin):
		return httpx.Conflict("LAST_SUPERADMIN", "No se puede eliminar al último superadmin activo de la plataforma")
	default:
		return err
	}
}

// uuidParam valida que el parámetro sea un UUID antes de tocar la BD; si no, 404 (no existe).
func uuidParam(c fiber.Ctx, name, notFound string) (string, error) {
	id, err := uuid.Parse(c.Params(name))
	if err != nil {
		return "", httpx.NotFound(notFound)
	}
	return id.String(), nil
}

// activeFilter lee ?active=true|false (vacío = sin filtro).
func activeFilter(c fiber.Ctx) (*bool, error) {
	switch c.Query("active") {
	case "":
		return nil, nil
	case "true":
		v := true
		return &v, nil
	case "false":
		v := false
		return &v, nil
	}
	return nil, httpx.Validation(httpx.FieldError{Field: "active", Message: "Debe ser true o false"})
}
