package app_test

import (
	"context"
	"fmt"
	"time"

	"github.com/shopspring/decimal"

	"prestamos/internal/capital/domain"
)

// fakeRepo guarda el libro en memoria. Replica la regla atómica del repositorio real:
// el chequeo de fondos se hace justo antes de insertar.
type fakeRepo struct{ rows []domain.Movement }

func (f *fakeRepo) Append(_ context.Context, tenantID string, m *domain.Movement, requireFunds bool) error {
	if requireFunds && f.sumTenant(tenantID).TotalCash.Sub(m.Amount).IsNegative() {
		return domain.ErrInsufficientCapital
	}
	m.CreatedAt = time.Date(2026, 1, 1, 0, 0, len(f.rows), 0, time.UTC)
	f.rows = append(f.rows, *m)
	return nil
}

func (f *fakeRepo) Balance(_ context.Context, tenantID string) (domain.Balance, error) {
	return f.sumTenant(tenantID), nil
}

// sumTenant suma los movimientos del tenant según su dirección y bucket.
func (f *fakeRepo) sumTenant(tenantID string) domain.Balance {
	var capital, profit, lent decimal.Decimal
	var b domain.Balance
	for _, m := range f.rows {
		if m.TenantID != tenantID {
			continue
		}
		b.MovementsCount++
		signed := m.Amount
		if m.Direction() == domain.DirectionOut {
			signed = signed.Neg()
		}
		if m.Bucket() == domain.BucketCapital {
			capital = capital.Add(signed)
		} else {
			profit = profit.Add(signed)
		}
		switch m.Type {
		case domain.TypeDisbursement:
			lent = lent.Add(m.Amount)
		case domain.TypePrincipalCollection:
			lent = lent.Sub(m.Amount)
		}
	}
	b.CapitalBalance, b.ProfitBalance, b.LentOutstanding = capital, profit, lent
	b.TotalCash = capital.Add(profit)
	return b
}

func (f *fakeRepo) List(_ context.Context, tenantID string, _ domain.ListFilter) (domain.ListResult, error) {
	var out domain.ListResult
	for _, m := range f.rows {
		if m.TenantID == tenantID {
			out.Items = append(out.Items, m)
		}
	}
	return out, nil
}

// fakeSettings simula la configuración de la empresa y cuenta las consultas.
type fakeSettings struct {
	allow bool
	err   error
	calls int
}

func (s *fakeSettings) AllowNegativeCapital(context.Context, string) (bool, error) {
	s.calls++
	return s.allow, s.err
}

// seqIDs genera IDs predecibles.
type seqIDs struct{ n int }

func (s *seqIDs) New() (string, error) { s.n++; return fmt.Sprintf("mov-%d", s.n), nil }
