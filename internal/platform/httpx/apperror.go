package httpx

import (
	"errors"
	"net/http"
)

// Códigos estables para el frontend (no cambian aunque cambie el mensaje).
const (
	CodeValidation      = "VALIDATION_ERROR"
	CodeInvalidJSON     = "INVALID_JSON"
	CodeUnauthenticated = "UNAUTHENTICATED"
	CodeTokenInvalid    = "TOKEN_INVALID"
	CodeInvalidCreds    = "INVALID_CREDENTIALS"
	CodeSessionExpired  = "SESSION_EXPIRED"
	CodeCSRFInvalid     = "CSRF_INVALID"
	CodeForbidden       = "FORBIDDEN"
	CodeNotFound        = "NOT_FOUND"
	CodeConflict        = "CONFLICT"
	CodeMethodNotAllow  = "METHOD_NOT_ALLOWED"
	CodePayloadTooLarge = "PAYLOAD_TOO_LARGE"
	CodeTooManyRequests = "TOO_MANY_REQUESTS"
	CodeTimeout         = "REQUEST_TIMEOUT"
	CodeInternal        = "INTERNAL_ERROR"
)

// FieldError describe un problema en un campo concreto del request.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// AppError es el único error que los handlers deben devolver hacia el cliente.
type AppError struct {
	Status  int
	Code    string
	Message string // en español, apto para mostrar al usuario
	Fields  []FieldError
	Err     error // causa interna (no se expone)
}

func (e *AppError) Error() string { return e.Code + ": " + e.Message }
func (e *AppError) Unwrap() error { return e.Err }

func New(status int, code, msg string) *AppError {
	return &AppError{Status: status, Code: code, Message: msg}
}

func Validation(fields ...FieldError) *AppError {
	return &AppError{Status: http.StatusBadRequest, Code: CodeValidation, Message: "Hay datos inválidos en la solicitud", Fields: fields}
}

func InvalidJSON() *AppError {
	return New(http.StatusBadRequest, CodeInvalidJSON, "El cuerpo de la solicitud no es un JSON válido")
}

func Unauthenticated() *AppError {
	return New(http.StatusUnauthorized, CodeUnauthenticated, "Debes iniciar sesión para continuar")
}

func Forbidden() *AppError {
	return New(http.StatusForbidden, CodeForbidden, "No tienes permisos para realizar esta acción")
}

func NotFound(msg string) *AppError { return New(http.StatusNotFound, CodeNotFound, msg) }

func Conflict(code, msg string) *AppError { return New(http.StatusConflict, code, msg) }

// As extrae un *AppError de una cadena de errores.
func As(err error) (*AppError, bool) {
	var ae *AppError
	ok := errors.As(err, &ae)
	return ae, ok
}
