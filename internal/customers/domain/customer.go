// Package domain contiene el modelo de clientes sin dependencias de infraestructura.
package domain

import (
	"context"
	"errors"
	"strings"
	"time"
)

type Status string

const (
	StatusActive   Status = "active"
	StatusInactive Status = "inactive"
	StatusBlocked  Status = "blocked"
)

func (s Status) Valid() bool {
	return s == StatusActive || s == StatusInactive || s == StatusBlocked
}

var (
	ErrNotFound          = errors.New("cliente no encontrado")
	ErrDuplicateDocument = errors.New("ya existe un cliente con ese documento")
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

func NewValidationError(field, msg string) *ValidationError {
	return &ValidationError{Issues: []Issue{{field, msg}}}
}

type Customer struct {
	ID          string
	TenantID    string
	FullName    string
	DocumentID  string
	Phone       string
	Phone2      string
	WhatsApp    string
	Email       string
	Occupation  string
	Address     string
	AddressRef  string // referencia de ubicación (ej. "frente al mercado")
	Notes       string
	Status      Status
	CollectorID *string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Normalize limpia espacios y valida las invariantes del cliente.
func (c *Customer) Normalize() error {
	for _, f := range []*string{&c.FullName, &c.DocumentID, &c.Phone, &c.Phone2, &c.WhatsApp,
		&c.Email, &c.Occupation, &c.Address, &c.AddressRef, &c.Notes} {
		*f = strings.TrimSpace(*f)
	}
	var issues []Issue
	add := func(cond bool, field, msg string) {
		if cond {
			issues = append(issues, Issue{field, msg})
		}
	}
	add(c.FullName == "", "full_name", "El nombre es obligatorio")
	add(c.DocumentID == "", "document_id", "El documento es obligatorio")
	add(len(c.DocumentID) > 20, "document_id", "El documento no puede superar 20 caracteres")
	add(c.Email != "" && !strings.Contains(c.Email, "@"), "email", "El correo no es válido")
	add(!c.Status.Valid(), "status", "Estado inválido (active, inactive o blocked)")
	if len(issues) > 0 {
		return &ValidationError{Issues: issues}
	}
	return nil
}

// ListResult es una página de clientes. Total es nil salvo que se haya pedido.
type ListResult struct {
	Items   []Customer
	HasMore bool
	Total   *int
}

type ListFilter struct {
	WithTotal bool   // calcular el total exacto (costoso en tablas grandes)
	Query     string // nombre, documento o teléfono
	Status    Status // vacío = todos
	Limit     int
	Offset    int
}

// Deletion es el borrado lógico de un cliente: la fila se conserva y su auditoría se escribe
// en la misma transacción. ActorID es el usuario que borra.
type Deletion struct {
	TenantID, ID, ActorID, AuditID string
}

// Repository es el puerto de persistencia. Ninguna operación ve los clientes eliminados.
type Repository interface {
	Create(ctx context.Context, c *Customer) error
	Update(ctx context.Context, c *Customer) error
	Get(ctx context.Context, tenantID, id string) (*Customer, error)
	List(ctx context.Context, tenantID string, f ListFilter) (ListResult, error)
	SoftDelete(ctx context.Context, d Deletion) error // ErrNotFound si no existe o ya está eliminado
}
