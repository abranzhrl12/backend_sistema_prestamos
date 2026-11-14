package app_test

import (
	"context"
	"testing"

	"github.com/shopspring/decimal"

	"prestamos/internal/capital/app"
	"prestamos/internal/capital/domain"
)

func TestBalance_SumsBucketsAndLentAmountFromEveryType(t *testing.T) {
	repo := &fakeRepo{}
	ctx := context.Background()
	s := app.NewService(repo, &fakeSettings{allow: true}, &seqIDs{})
	// Estos tipos no son manuales: se insertan directo en el libro para probar la suma de todos.
	add := func(typ domain.Type, amount string) {
		repo.rows = append(repo.rows, domain.Movement{TenantID: "t1", Type: typ, Amount: decimal.RequireFromString(amount)})
	}
	add(domain.TypeContribution, "1000")
	add(domain.TypeDisbursement, "300")
	add(domain.TypePrincipalCollection, "100")
	add(domain.TypeInterestCollection, "50")
	add(domain.TypeLateFeeCollection, "10")
	add(domain.TypeExpense, "20")

	b, err := s.Balance(ctx, "t1")
	if err != nil {
		t.Fatal(err)
	}
	checks := map[string][2]decimal.Decimal{
		"capital":  {b.CapitalBalance, decimal.RequireFromString("800")},
		"utilidad": {b.ProfitBalance, decimal.RequireFromString("40")},
		"total":    {b.TotalCash, decimal.RequireFromString("840")},
		"prestado": {b.LentOutstanding, decimal.RequireFromString("200")},
	}
	for name, c := range checks {
		if !c[0].Equal(c[1]) {
			t.Errorf("%s = %s, esperaba %s", name, c[0], c[1])
		}
	}
	if b.MovementsCount != 6 {
		t.Errorf("movements_count %d, esperaba 6", b.MovementsCount)
	}
}

func TestBalance_IsIsolatedPerTenant(t *testing.T) {
	s := app.NewService(&fakeRepo{}, &fakeSettings{}, &seqIDs{})
	ctx := context.Background()
	if err := record(t, s, "t1", "contribution", "700"); err != nil {
		t.Fatal(err)
	}
	b, _ := s.Balance(ctx, "t2")
	if !b.TotalCash.IsZero() || b.MovementsCount != 0 {
		t.Fatalf("la otra empresa no debe ver saldo: %+v", b)
	}
}
