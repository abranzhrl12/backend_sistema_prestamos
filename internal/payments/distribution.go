package payments

import (
	"github.com/shopspring/decimal"
	"prestamos/internal/loans/schedule"
)

// Distribution is an explicit allocation agreed with the lender, independent of payment methods.
type Distribution struct{ Principal, Interest, LateFee decimal.Decimal }

func (d Distribution) valid(amount decimal.Decimal) bool {
	for _, value := range []decimal.Decimal{d.Principal, d.Interest, d.LateFee} {
		if value.IsNegative() || !value.Equal(value.Round(4)) {
			return false
		}
	}
	return d.Principal.Add(d.Interest).Add(d.LateFee).Equal(amount)
}

func allocatePayment(in Input, dues []schedule.Due) ([]schedule.Due, error) {
	if in.Distribution == nil {
		return schedule.Allocate(in.Amount, dues)
	}
	remaining := *in.Distribution
	out := make([]schedule.Due, len(dues))
	for n, d := range dues {
		out[n].Principal = decimal.Min(remaining.Principal, d.Principal)
		out[n].Interest = decimal.Min(remaining.Interest, d.Interest)
		out[n].LateFee = decimal.Min(remaining.LateFee, d.LateFee)
		remaining.Principal = remaining.Principal.Sub(out[n].Principal)
		remaining.Interest = remaining.Interest.Sub(out[n].Interest)
		remaining.LateFee = remaining.LateFee.Sub(out[n].LateFee)
	}
	if !remaining.Principal.Add(remaining.Interest).Add(remaining.LateFee).IsZero() {
		return nil, schedule.ErrOverpayment
	}
	return out, nil
}
