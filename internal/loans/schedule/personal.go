package schedule

import (
	"github.com/shopspring/decimal"
	"time"
)

type PersonalInput struct {
	Principal         decimal.Decimal
	InterestType      string
	InterestValue     decimal.Decimal
	TermValue         int
	TermUnit          string
	Frequency         string
	Start             time.Time
	InstallmentsCount int
	FirstPayment      time.Time
}

// GeneratePersonal calculates the entire interest once and splits it across due dates.
func GeneratePersonal(in PersonalInput) ([]Installment, error) {
	if !in.Principal.IsPositive() || !in.Principal.Equal(in.Principal.Round(4)) || in.Principal.GreaterThanOrEqual(decimal.RequireFromString("1000000000000000")) || in.InterestValue.IsNegative() || !in.InterestValue.Equal(in.InterestValue.Round(4)) || in.InterestValue.GreaterThanOrEqual(decimal.RequireFromString("1000000000000000")) || in.Start.IsZero() {
		return nil, ErrInvalidInput
	}
	var interest decimal.Decimal
	switch in.InterestType {
	case "none", "":
		if !in.InterestValue.IsZero() {
			return nil, ErrInvalidInput
		}
	case "fixed":
		interest = in.InterestValue
	case "percentage":
		interest = in.Principal.Mul(in.InterestValue).Div(decimal.NewFromInt(100)).Round(4)
	default:
		return nil, ErrInvalidInput
	}
	if interest.GreaterThanOrEqual(decimal.RequireFromString("1000000000000000")) {
		return nil, ErrInvalidInput
	}
	if in.Frequency != "daily" && in.Frequency != "weekly" && in.Frequency != "biweekly" && in.Frequency != "monthly" && in.Frequency != "single" {
		return nil, ErrInvalidInput
	}
	dates, err := paymentDates(in)
	if err != nil {
		return nil, err
	}
	count := decimal.NewFromInt(int64(len(dates)))
	partP := in.Principal.Div(count).Truncate(4)
	partI := interest.Div(count).Truncate(4)
	balance := in.Principal
	remainingInterest := interest
	rows := make([]Installment, 0, len(dates))
	for i, d := range dates {
		p, j := partP, partI
		if i == len(dates)-1 {
			p = balance
			j = remainingInterest
		}
		balance = balance.Sub(p)
		remainingInterest = remainingInterest.Sub(j)
		rows = append(rows, Installment{Number: i + 1, DueDate: d, Principal: p, Interest: j, Balance: balance})
	}
	return rows, nil
}

func paymentDates(in PersonalInput) ([]time.Time, error) {
	if in.InstallmentsCount != 0 || !in.FirstPayment.IsZero() {
		if in.InstallmentsCount < 1 || in.InstallmentsCount > 1000 || in.FirstPayment.IsZero() || in.FirstPayment.Before(in.Start) || in.TermValue != 0 || in.TermUnit != "" || (in.Frequency == "single" && in.InstallmentsCount != 1) {
			return nil, ErrInvalidInput
		}
		dates := make([]time.Time, in.InstallmentsCount)
		for n := range dates {
			dates[n] = dueDate(in.FirstPayment, in.Frequency, n)
			if dates[n].Year() > 9999 {
				return nil, ErrInvalidInput
			}
		}
		return dates, nil
	}
	if in.TermValue < 1 || in.TermValue > 1000 {
		return nil, ErrInvalidInput
	}
	var end time.Time
	switch in.TermUnit {
	case "days":
		end = in.Start.AddDate(0, 0, in.TermValue)
	case "weeks":
		end = in.Start.AddDate(0, 0, 7*in.TermValue)
	case "months":
		end = dueDate(in.Start, "monthly", in.TermValue)
	default:
		return nil, ErrInvalidInput
	}
	dates := make([]time.Time, 0)
	if in.Frequency != "single" {
		for n := 1; n <= 1000; n++ {
			d := dueDate(in.Start, in.Frequency, n)
			if !d.Before(end) {
				break
			}
			dates = append(dates, d)
		}
	}
	if len(dates) >= 1000 {
		return nil, ErrInvalidInput
	}
	return append(dates, end), nil
}
