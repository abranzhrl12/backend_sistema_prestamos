package http

import (
	"github.com/shopspring/decimal"
	"prestamos/internal/payments"
	"prestamos/internal/platform/httpx"
)

type distributionBody struct {
	Principal string `json:"principal"`
	Interest  string `json:"interest"`
	LateFee   string `json:"late_fee"`
}

func (b distributionBody) parse() (*payments.Distribution, error) {
	values := make([]decimal.Decimal, 3)
	for n, s := range []string{b.Principal, b.Interest, b.LateFee} {
		v, e := decimal.NewFromString(s)
		if e != nil || v.IsNegative() || !v.Equal(v.Round(4)) {
			return nil, httpx.Validation(httpx.FieldError{Field: "distribution", Message: "Capital, interés y mora deben ser importes no negativos con hasta cuatro decimales"})
		}
		values[n] = v
	}
	return &payments.Distribution{Principal: values[0], Interest: values[1], LateFee: values[2]}, nil
}
