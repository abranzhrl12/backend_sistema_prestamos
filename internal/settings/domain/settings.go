// Package domain contiene la configuración de la empresa sin dependencias de infraestructura.
package domain

import (
	"context"
	"errors"
	"time"
	_ "time/tzdata" // zonas horarias embebidas: time.LoadLocation funciona en Windows sin tzdata del sistema

	"prestamos/internal/platform/money"
)

// Valores de configuración con opciones cerradas.
const (
	CurrencyPEN = "PEN"
	CurrencyUSD = "USD"

	LateFeeFixed        = "fixed"
	LateFeeProportional = "proportional"

	AllocLateFeeInterestPrincipal = "late_fee_interest_principal"
	AllocInterestPrincipalLateFee = "interest_principal_late_fee"

	maxGraceDays        = 30
	minActiveLoans      = 1
	maxActiveLoans      = 50
	defaultTimezone     = "America/Lima"
	defaultLateFeeValue = "10.00"
	defaultMaxLoans     = 3
)

// ErrNotFound indica que la empresa aún no tiene fila de configuración.
var ErrNotFound = errors.New("configuración no encontrada")

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

// Settings es la configuración editable de una empresa. Los montos viajan como texto decimal.
type Settings struct {
	Currency                  string
	Timezone                  string
	LateFeeMode               string
	LateFeeAmount             string
	LateFeePerDay             bool
	GraceDays                 int
	AllocationOrder           string
	MaxActiveLoansPerCustomer int
	AllowNegativeCapital      bool
	IsDefault                 bool
	UpdatedAt                 time.Time
}

// Defaults devuelve la configuración que rige mientras la empresa no guarde la suya.
func Defaults() Settings {
	return Settings{
		Currency:                  CurrencyPEN,
		Timezone:                  defaultTimezone,
		LateFeeMode:               LateFeeFixed,
		LateFeeAmount:             defaultLateFeeValue,
		AllocationOrder:           AllocLateFeeInterestPrincipal,
		MaxActiveLoansPerCustomer: defaultMaxLoans,
		IsDefault:                 true,
	}
}

// Validate comprueba todas las reglas y devuelve *ValidationError con cada campo incorrecto.
func (s Settings) Validate() error {
	var issues []Issue
	add := func(field, msg string) { issues = append(issues, Issue{field, msg}) }

	if s.Currency != CurrencyPEN && s.Currency != CurrencyUSD {
		add("currency", "Moneda inválida (PEN o USD)")
	}
	if !validTimezone(s.Timezone) {
		add("timezone", "Zona horaria inválida (ej. America/Lima)")
	}
	if s.LateFeeMode != LateFeeFixed && s.LateFeeMode != LateFeeProportional {
		add("late_fee_mode", "Modo de mora inválido (fixed o proportional)")
	}
	if msg := amountIssue(s.LateFeeAmount); msg != "" {
		add("late_fee_amount", msg)
	}
	if s.GraceDays < 0 || s.GraceDays > maxGraceDays {
		add("grace_days", "Los días de gracia deben estar entre 0 y 30")
	}
	if s.AllocationOrder != AllocLateFeeInterestPrincipal && s.AllocationOrder != AllocInterestPrincipalLateFee {
		add("allocation_order", "Orden de aplicación inválido")
	}
	if s.MaxActiveLoansPerCustomer < minActiveLoans || s.MaxActiveLoansPerCustomer > maxActiveLoans {
		add("max_active_loans_per_customer", "El máximo de préstamos activos debe estar entre 1 y 50")
	}
	if len(issues) > 0 {
		return &ValidationError{Issues: issues}
	}
	return nil
}

// amountIssue devuelve el mensaje si el monto no es un decimal >= 0 válido; "" si es correcto.
func amountIssue(s string) string {
	_, err := money.Parse(s)
	switch {
	case errors.Is(err, money.ErrNegative):
		return "El monto de mora no puede ser negativo"
	case err != nil:
		return "El monto de mora debe ser un número válido (ej. 10.00)"
	}
	return ""
}

// validTimezone exige una zona cargable; "" y "Local" dependen del servidor y no son válidas.
func validTimezone(name string) bool {
	if name == "" || name == "Local" {
		return false
	}
	_, err := time.LoadLocation(name)
	return err == nil
}

// ChangedFields lista los nombres JSON de los campos que difieren entre dos configuraciones.
// Los montos se comparan por valor ("10" == "10.00").
func ChangedFields(old, in Settings) []string {
	changed := make([]string, 0)
	add := func(differs bool, field string) {
		if differs {
			changed = append(changed, field)
		}
	}
	add(old.Currency != in.Currency, "currency")
	add(old.Timezone != in.Timezone, "timezone")
	add(old.LateFeeMode != in.LateFeeMode, "late_fee_mode")
	add(!sameAmount(old.LateFeeAmount, in.LateFeeAmount), "late_fee_amount")
	add(old.LateFeePerDay != in.LateFeePerDay, "late_fee_per_day")
	add(old.GraceDays != in.GraceDays, "grace_days")
	add(old.AllocationOrder != in.AllocationOrder, "allocation_order")
	add(old.MaxActiveLoansPerCustomer != in.MaxActiveLoansPerCustomer, "max_active_loans_per_customer")
	add(old.AllowNegativeCapital != in.AllowNegativeCapital, "allow_negative_capital")
	return changed
}

func sameAmount(a, b string) bool {
	da, errA := money.Parse(a)
	db, errB := money.Parse(b)
	return errA == nil && errB == nil && da.Equal(db)
}

// Audit describe la fila de audit_log que se escribe en la misma transacción que el cambio.
type Audit struct {
	ID     string
	UserID string
	Action string
	Data   map[string]any
}

// Repository es el puerto de persistencia.
type Repository interface {
	// Get devuelve ErrNotFound si la empresa no tiene fila propia.
	Get(ctx context.Context, tenantID string) (*Settings, error)
	// Save hace UPSERT y escribe el audit en la misma transacción; devuelve lo guardado.
	Save(ctx context.Context, tenantID string, s Settings, a Audit) (*Settings, error)
}
