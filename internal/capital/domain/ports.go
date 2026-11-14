package domain

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
)

// Balance es la foto de la caja. Todos los montos son >= 0 salvo TotalCash (puede ser negativo
// solo si la empresa permite capital negativo).
type Balance struct {
	TotalCash       decimal.Decimal
	CapitalBalance  decimal.Decimal
	ProfitBalance   decimal.Decimal
	LentOutstanding decimal.Decimal
	MovementsCount  int
	AsOf            time.Time
}

// ListFilter son los criterios de lectura del libro. Vacío = sin filtro.
type ListFilter struct {
	Type    Type
	From    *time.Time
	To      *time.Time
	AfterID string // keyset: devuelve movimientos con id menor que AfterID (orden id DESC)
	Limit   int
}

// ListResult es una página del libro. HasMore indica si existe otra página tras Items.
type ListResult struct {
	Items   []Movement
	HasMore bool
}

// Repository es el puerto de persistencia del libro.
type Repository interface {
	// Append inserta m dentro de un bloque serializado por empresa. Si requireFunds, comprueba
	// el saldo con el mismo bloque y devuelve ErrInsufficientCapital si quedaría negativo.
	// Rellena m.CreatedAt con el valor asignado por la BD.
	Append(ctx context.Context, tenantID string, m *Movement, requireFunds bool) error
	Balance(ctx context.Context, tenantID string) (Balance, error)
	List(ctx context.Context, tenantID string, f ListFilter) (ListResult, error)
}
