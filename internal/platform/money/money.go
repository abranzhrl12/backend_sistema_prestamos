// Package money centraliza cómo la API recibe y devuelve dinero: SIEMPRE como string decimal
// ("1500.50"), nunca como número JSON (evita errores de punto flotante en el frontend).
// En BD es NUMERIC(19,4); en Go, decimal.Decimal.
package money

import (
	"errors"
	"regexp"

	"github.com/shopspring/decimal"
)

var (
	ErrFormat   = errors.New("monto inválido: usa un texto decimal como \"1500.50\" (hasta 4 decimales)")
	ErrNegative = errors.New("el monto no puede ser negativo")
	ErrZero     = errors.New("el monto debe ser mayor que cero")
)

// 15 dígitos enteros + 4 decimales = NUMERIC(19,4).
var pattern = regexp.MustCompile(`^\d{1,15}(\.\d{1,4})?$`)

// Parse lee un monto >= 0 en formato "123", "123.4" o "123.4567".
func Parse(s string) (decimal.Decimal, error) {
	if len(s) > 0 && s[0] == '-' {
		return decimal.Zero, ErrNegative
	}
	if !pattern.MatchString(s) {
		return decimal.Zero, ErrFormat
	}
	return decimal.RequireFromString(s), nil
}

// ParsePositive lee un monto estrictamente mayor que cero.
func ParsePositive(s string) (decimal.Decimal, error) {
	d, err := Parse(s)
	if err != nil {
		return d, err
	}
	if !d.IsPositive() {
		return d, ErrZero
	}
	return d, nil
}

// Format devuelve 2 decimales, o 4 si el valor realmente los necesita.
func Format(d decimal.Decimal) string {
	if d.Equal(d.Round(2)) {
		return d.StringFixed(2)
	}
	return d.StringFixed(4)
}
