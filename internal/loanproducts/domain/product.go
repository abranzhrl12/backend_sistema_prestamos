// Package domain contiene el modelo de productos de préstamo sin dependencias de infraestructura.
package domain

import (
	"context"
	"errors"
	"strings"
	"time"
)

// Opciones cerradas del producto.
const (
	FrequencyDaily    = "daily"
	FrequencyWeekly   = "weekly"
	FrequencyBiweekly = "biweekly"
	FrequencyMonthly  = "monthly"
	FrequencySingle   = "single"

	InterestFlat   = "flat"
	InterestFrench = "french"
	InterestGerman = "german"

	RatePeriodDaily    = "daily"
	RatePeriodWeekly   = "weekly"
	RatePeriodBiweekly = "biweekly"
	RatePeriodMonthly  = "monthly"
	RatePeriodAnnual   = "annual"

	LateFeeFixed        = "fixed"
	LateFeeProportional = "proportional"

	FeeNone    = "none"
	FeeFixed   = "fixed"
	FeePercent = "percent"

	defaultInterest = InterestFlat
	defaultFee      = FeeNone
	defaultFeeValue = "0"
)

var (
	ErrNotFound  = errors.New("producto no encontrado")
	ErrNameTaken = errors.New("ya existe un producto con ese nombre")
)

// Product es un producto de préstamo. Porcentajes y montos viajan como texto decimal.
// Los overrides nil significan "usar la configuración de la empresa".
type Product struct {
	ID            string
	TenantID      string
	Name          string
	Frequency     string
	InterestType  string
	RatePeriod    string
	RateDefault   string
	RateMin       string
	RateMax       string
	AmountMin     string
	AmountMax     string
	TermsMin      int
	TermsMax      int
	GraceDays     *int
	LateFeeMode   *string
	LateFeeAmount *string
	LateFeePerDay *bool
	FeeType       string
	FeeValue      string
	Active        bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// Normalize quita espacios y aplica los valores por defecto de los campos opcionales.
func (p *Product) Normalize() {
	p.Name = strings.TrimSpace(p.Name)
	for _, f := range []*string{&p.Frequency, &p.InterestType, &p.RatePeriod, &p.RateDefault, &p.RateMin,
		&p.RateMax, &p.AmountMin, &p.AmountMax, &p.FeeType, &p.FeeValue} {
		*f = strings.TrimSpace(*f)
	}
	if p.InterestType == "" {
		p.InterestType = defaultInterest
	}
	if p.FeeType == "" {
		p.FeeType = defaultFee
	}
	if p.FeeValue == "" {
		p.FeeValue = defaultFeeValue
	}
}

// Audit describe la fila de audit_log que se escribe en la misma transacción que el cambio.
type Audit struct {
	ID     string
	UserID string
	Action string
	Data   map[string]any
}

// ListFilter son los criterios del listado. Active nil = todos; Frequency "" = todas.
type ListFilter struct {
	WithTotal bool
	Query     string
	Active    *bool
	Frequency string
	Limit     int
	Offset    int
}

// ListResult es una página de productos. Total es nil salvo que se haya pedido.
type ListResult struct {
	Items   []Product
	HasMore bool
	Total   *int
}

// Repository es el puerto de persistencia. Ninguna operación ve filas eliminadas.
type Repository interface {
	Create(ctx context.Context, p *Product, a Audit) error
	Update(ctx context.Context, p *Product, a Audit) error
	Delete(ctx context.Context, tenantID, id string, a Audit) error
	Get(ctx context.Context, tenantID, id string) (*Product, error)
	List(ctx context.Context, tenantID string, f ListFilter) (ListResult, error)
}
