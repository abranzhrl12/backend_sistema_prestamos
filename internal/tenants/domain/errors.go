// Package domain contiene el modelo de empresas (tenants) y de sus usuarios, sin dependencias de infraestructura.
package domain

import "errors"

var (
	ErrTenantNotFound    = errors.New("empresa no encontrada")
	ErrUserNotFound      = errors.New("usuario no encontrado")
	ErrSlugTaken         = errors.New("ya existe una empresa con ese identificador")
	ErrEmailTaken        = errors.New("ya existe un usuario con ese correo en la empresa")
	ErrPlatformProtected = errors.New("la empresa de la plataforma está protegida")
	ErrLastSuperadmin    = errors.New("no se puede eliminar al último superadmin activo")
)

// Issue es un problema de validación asociado a un campo.
type Issue struct{ Field, Msg string }

// ValidationError agrupa todos los problemas de validación encontrados.
type ValidationError struct{ Issues []Issue }

func (e *ValidationError) Error() string {
	if len(e.Issues) == 0 {
		return "datos inválidos"
	}
	return e.Issues[0].Field + ": " + e.Issues[0].Msg
}

// validationErr devuelve nil si no hay problemas (evita devolver un *ValidationError vacío).
func validationErr(issues []Issue) error {
	if len(issues) == 0 {
		return nil
	}
	return &ValidationError{Issues: issues}
}
