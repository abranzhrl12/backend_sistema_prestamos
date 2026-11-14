package app

import (
	"context"
	"errors"
	"github.com/shopspring/decimal"
	"prestamos/internal/loans/schedule"
	"time"
)

var ErrInvalid = errors.New("datos de préstamo inválidos")
var ErrNotFound = errors.New("préstamo no encontrado")
var ErrInsufficientCapital = errors.New("capital insuficiente")
var ErrInvalidStatus = errors.New("estado de préstamo inválido")

type Input struct {
	CustomerID, Principal, InterestType, InterestValue, TermUnit, Frequency, DisbursementDate string
	TermValue                                                                                 int
	InstallmentsCount                                                                         int
	FirstPaymentDate                                                                          string
}
type Service struct{}

func NewService() *Service { return &Service{} }
func (s *Service) Simulate(_ context.Context, _ string, in Input) ([]schedule.Installment, error) {
	principal, err := decimal.NewFromString(in.Principal)
	if err != nil {
		return nil, ErrInvalid
	}
	value := decimal.Zero
	if in.InterestValue != "" {
		value, err = decimal.NewFromString(in.InterestValue)
		if err != nil {
			return nil, ErrInvalid
		}
	}
	start, err := time.Parse("2006-01-02", in.DisbursementDate)
	if err != nil {
		return nil, ErrInvalid
	}
	var first time.Time
	if in.FirstPaymentDate != "" {
		first, err = time.Parse("2006-01-02", in.FirstPaymentDate)
		if err != nil {
			return nil, ErrInvalid
		}
	}
	rows, err := schedule.GeneratePersonal(schedule.PersonalInput{Principal: principal, InterestType: in.InterestType, InterestValue: value, TermValue: in.TermValue, TermUnit: in.TermUnit, Frequency: in.Frequency, Start: start, InstallmentsCount: in.InstallmentsCount, FirstPayment: first})
	if err != nil {
		return nil, ErrInvalid
	}
	return rows, nil
}
