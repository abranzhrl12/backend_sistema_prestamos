// Package httpx agrupa utilidades HTTP compartidas: errores, paginación y respuestas.
package httpx

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/requestid"
)

// Claves de c.Locals. Se usan literales (no el paquete obs) para no crear un ciclo de imports.
const (
	errorCodeLocal = "error.code"
	moduleLocal    = "obs.module"
	coreModule     = "core"
)

type errorBody struct {
	Code      string       `json:"code"`
	Message   string       `json:"message"`
	Fields    []FieldError `json:"fields,omitempty"`
	RequestID string       `json:"request_id,omitempty"`
}

// ErrorResponse es el formato único de error de toda la API.
type ErrorResponse struct {
	Error errorBody `json:"error"`
}

// ErrorHandler convierte cualquier error en {"error": {code, message, fields?, request_id}}.
// Los errores no controlados se registran con detalle (con el módulo que los originó)
// pero se ocultan al cliente. El módulo nunca viaja en la respuesta.
func ErrorHandler(c fiber.Ctx, err error) error {
	ae := toAppError(err)
	rid := requestid.FromContext(c)
	c.Locals(errorCodeLocal, ae.Code)
	if ae.Status >= http.StatusInternalServerError {
		slog.Error("internal_error",
			"request_id", rid,
			"module", moduleOf(c),
			"method", c.Method(),
			"route", c.Route().Path,
			"err", err.Error(),
		)
	}
	return c.Status(ae.Status).JSON(ErrorResponse{Error: errorBody{
		Code: ae.Code, Message: ae.Message, Fields: ae.Fields, RequestID: rid,
	}})
}

func moduleOf(c fiber.Ctx) string {
	if m, ok := c.Locals(moduleLocal).(string); ok && m != "" {
		return m
	}
	return coreModule
}

func toAppError(err error) *AppError {
	if ae, ok := As(err); ok {
		return ae
	}
	var fe *fiber.Error
	if errors.As(err, &fe) {
		return fromFiber(fe)
	}
	return internal(err)
}

// fromFiber traduce los errores del propio framework (404 de ruta, 405, 413...) al formato de la API.
func fromFiber(fe *fiber.Error) *AppError {
	switch fe.Code {
	case http.StatusNotFound:
		return NotFound("El recurso solicitado no existe")
	case http.StatusMethodNotAllowed:
		return New(fe.Code, CodeMethodNotAllow, "Método no permitido para este recurso")
	case http.StatusRequestEntityTooLarge:
		return New(fe.Code, CodePayloadTooLarge, "El cuerpo de la solicitud es demasiado grande")
	case http.StatusTooManyRequests:
		return New(fe.Code, CodeTooManyRequests, "Demasiadas solicitudes, intenta de nuevo en unos segundos")
	case http.StatusRequestTimeout:
		return New(fe.Code, CodeTimeout, "La solicitud tardó demasiado")
	case http.StatusBadRequest:
		return InvalidJSON()
	default:
		if fe.Code >= http.StatusInternalServerError {
			return internal(fe)
		}
		return New(fe.Code, CodeValidation, fe.Message)
	}
}

func internal(cause error) *AppError {
	return &AppError{Status: http.StatusInternalServerError, Code: CodeInternal,
		Message: "Ocurrió un error interno. Intenta de nuevo; si persiste, contacta a soporte", Err: cause}
}
