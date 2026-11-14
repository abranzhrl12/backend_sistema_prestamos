// Package domain modela la caja única de la empresa como un libro de movimientos append-only.
package domain

import (
	"time"

	"github.com/shopspring/decimal"
)

// Type es el tipo de movimiento. Determina la dirección y el bucket (capital o utilidad).
type Type string

const (
	TypeContribution        Type = "contribution"
	TypeWithdrawal          Type = "withdrawal"
	TypeAdjustmentIn        Type = "adjustment_in"
	TypeAdjustmentOut       Type = "adjustment_out"
	TypeDisbursement        Type = "disbursement"
	TypePrincipalCollection Type = "principal_collection"
	TypeInterestCollection  Type = "interest_collection"
	TypeLateFeeCollection   Type = "late_fee_collection"
	TypeExpense             Type = "expense"
)

// Direction indica si el movimiento entra o sale de la caja.
type Direction string

const (
	DirectionIn  Direction = "in"
	DirectionOut Direction = "out"
)

// Bucket separa el saldo en capital (lo prestable) y utilidad (intereses, mora, gastos).
type Bucket string

const (
	BucketCapital Bucket = "capital"
	BucketProfit  Bucket = "profit"
)

// Movement es una fila del libro. Amount es siempre positivo; la dirección sale del tipo.
type Movement struct {
	ID        string
	TenantID  string
	Type      Type
	Amount    decimal.Decimal
	Note      string
	Reference string
	LoanID    *string
	PaymentID *string
	CreatedBy *string
	CreatedAt time.Time
}

type effect struct {
	dir    Direction
	bucket Bucket
}

var effects = map[Type]effect{
	TypeContribution:        {DirectionIn, BucketCapital},
	TypeWithdrawal:          {DirectionOut, BucketCapital},
	TypeAdjustmentIn:        {DirectionIn, BucketCapital},
	TypeAdjustmentOut:       {DirectionOut, BucketCapital},
	TypeDisbursement:        {DirectionOut, BucketCapital},
	TypePrincipalCollection: {DirectionIn, BucketCapital},
	TypeInterestCollection:  {DirectionIn, BucketProfit},
	TypeLateFeeCollection:   {DirectionIn, BucketProfit},
	TypeExpense:             {DirectionOut, BucketProfit},
}

// manualTypes son los únicos tipos que la API permite registrar a mano. Los demás los escriben
// los módulos de préstamos y pagos.
var manualTypes = map[Type]bool{
	TypeContribution:  true,
	TypeWithdrawal:    true,
	TypeAdjustmentIn:  true,
	TypeAdjustmentOut: true,
}

// KnownType indica si el texto corresponde a alguno de los 9 tipos (útil para filtros de lectura).
func KnownType(s string) bool {
	_, ok := effects[Type(s)]
	return ok
}

// Manual indica si el tipo puede registrarse por la API manual.
func (t Type) Manual() bool { return manualTypes[t] }

// Direction devuelve si el tipo entra o sale de la caja. Tipo desconocido: vacío.
func (t Type) Direction() Direction { return effects[t].dir }

// Bucket devuelve el bucket (capital o utilidad) que afecta el tipo. Tipo desconocido: vacío.
func (t Type) Bucket() Bucket { return effects[t].bucket }

// Direction devuelve la dirección del movimiento (la define su tipo).
func (m Movement) Direction() Direction { return m.Type.Direction() }

// Bucket devuelve el bucket del movimiento (lo define su tipo).
func (m Movement) Bucket() Bucket { return m.Type.Bucket() }
