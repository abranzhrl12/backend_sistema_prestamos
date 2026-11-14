package domain

import "errors"

// ErrInsufficientCapital: el movimiento dejaría la caja con saldo negativo y la empresa no lo permite.
var ErrInsufficientCapital = errors.New("no hay capital suficiente para esta operación")

// Issue es un problema de validación asociado a un campo (nombre JSON).
type Issue struct{ Field, Msg string }

// ValidationError agrupa todos los problemas de validación encontrados.
type ValidationError struct{ Issues []Issue }

func (e *ValidationError) Error() string {
	if len(e.Issues) == 0 {
		return "datos inválidos"
	}
	return e.Issues[0].Field + ": " + e.Issues[0].Msg
}
