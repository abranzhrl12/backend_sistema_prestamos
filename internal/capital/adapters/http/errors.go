package http

import (
	"errors"

	"prestamos/internal/capital/domain"
	"prestamos/internal/platform/httpx"
)

// mapErr traduce los errores de dominio a AppError. Lo no reconocido sigue siendo 500.
func mapErr(err error) error {
	var ve *domain.ValidationError
	switch {
	case errors.As(err, &ve):
		fields := make([]httpx.FieldError, len(ve.Issues))
		for i, is := range ve.Issues {
			fields[i] = httpx.FieldError{Field: is.Field, Message: is.Msg}
		}
		return httpx.Validation(fields...)
	case errors.Is(err, domain.ErrInsufficientCapital):
		return httpx.Conflict("INSUFFICIENT_CAPITAL", "No hay capital suficiente para esta operación")
	default:
		return err
	}
}
