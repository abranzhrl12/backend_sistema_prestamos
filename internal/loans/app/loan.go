package app

import (
	"context"
	"prestamos/internal/loans/schedule"
)

type Loan struct {
	ID, CustomerID, CustomerName, ProductID, Principal, Rate, AnnualRate, RatePeriod, InterestType, Frequency, Status, DisbursementDate string
	InterestValue, TermUnit                                                                                                             string
	TermCount, TermValue                                                                                                                int
}
type Actor struct{ TenantID, UserID string }
type Store interface {
	Create(context.Context, Actor, Loan, []schedule.Installment) (Loan, error)
	Disburse(context.Context, Actor, string) (Loan, error)
	List(context.Context, string, int, int, bool) ([]Loan, bool, *int, error)
	Schedule(context.Context, string, string) ([]ScheduleRow, error)
}
type ScheduleRow struct {
	ID                                                                                  string
	Number                                                                              int
	DueDate                                                                             string
	PrincipalDue, InterestDue, LateFeeAccrued, PrincipalPaid, InterestPaid, LateFeePaid string
}
