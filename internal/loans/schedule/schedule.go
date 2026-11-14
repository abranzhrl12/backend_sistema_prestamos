// Package schedule calculates installments without persistence.
package schedule

import (
	"errors"
	"github.com/shopspring/decimal"
	"time"
)

var ErrInvalidInput = errors.New("parámetros de cronograma inválidos")

type Installment struct {
	Number                       int
	DueDate                      time.Time
	Principal, Interest, Balance decimal.Decimal
}

func dueDate(start time.Time, frequency string, n int) time.Time {
	switch frequency {
	case "daily":
		return start.AddDate(0, 0, n)
	case "weekly":
		return start.AddDate(0, 0, 7*n)
	case "biweekly":
		return start.AddDate(0, 0, 15*n)
	default:
		first := time.Date(start.Year(), start.Month()+time.Month(n), 1, 0, 0, 0, 0, start.Location())
		last := first.AddDate(0, 1, -1).Day()
		day := start.Day()
		if day > last {
			day = last
		}
		return first.AddDate(0, 0, day-1)
	}
}
