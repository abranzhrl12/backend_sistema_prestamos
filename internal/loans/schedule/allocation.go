package schedule

import (
	"errors"
	"github.com/shopspring/decimal"
)

var ErrOverpayment = errors.New("el pago supera el saldo pendiente")

// Due contains outstanding amounts for one installment.
type Due struct {
	Principal, Interest, LateFee decimal.Decimal
	Overdue                      bool
}

// Allocate applies components across installments in chronological order.
// The caller supplies overdue installments before future installments.
func Allocate(amount decimal.Decimal, dues []Due) ([]Due, error) {
	if !amount.IsPositive() || !amount.Equal(amount.Round(4)) {
		return nil, ErrInvalidInput
	}
	total := decimal.Zero
	for _, d := range dues {
		if d.Principal.IsNegative() || d.Interest.IsNegative() || d.LateFee.IsNegative() || !d.Principal.Equal(d.Principal.Round(4)) || !d.Interest.Equal(d.Interest.Round(4)) || !d.LateFee.Equal(d.LateFee.Round(4)) {
			return nil, ErrInvalidInput
		}
		total = total.Add(d.Principal).Add(d.Interest).Add(d.LateFee)
	}
	if amount.GreaterThan(total) {
		return nil, ErrOverpayment
	}
	result := make([]Due, len(dues))
	remaining := amount
	for component := 0; component < 3; component++ {
		for i, d := range dues {
			if !d.Overdue {
				continue
			}
			var due decimal.Decimal
			switch component {
			case 0:
				due = d.LateFee
			case 1:
				due = d.Interest
			default:
				due = d.Principal
			}
			paid := decimal.Min(remaining, due)
			remaining = remaining.Sub(paid)
			switch component {
			case 0:
				result[i].LateFee = paid
			case 1:
				result[i].Interest = paid
			default:
				result[i].Principal = paid
			}
		}
	}
	for i, d := range dues {
		if d.Overdue {
			continue
		}
		result[i].LateFee = decimal.Min(remaining, d.LateFee)
		remaining = remaining.Sub(result[i].LateFee)
		result[i].Interest = decimal.Min(remaining, d.Interest)
		remaining = remaining.Sub(result[i].Interest)
		result[i].Principal = decimal.Min(remaining, d.Principal)
		remaining = remaining.Sub(result[i].Principal)
	}
	return result, nil
}
