package domain_test

import (
	"slices"
	"testing"

	"prestamos/internal/loanproducts/domain"
)

func TestChangedFields_EqualAmountsAreNotChanges(t *testing.T) {
	old := validProduct()
	old.RateDefault = "2.5000" // texto de la BD
	in := validProduct()
	in.RateDefault = "2.50"
	if got := domain.ChangedFields(old, in); len(got) != 0 {
		t.Fatalf("no debe haber cambios, got %v", got)
	}
}

func TestChangedFields_ListsOnlyDifferences(t *testing.T) {
	old := validProduct()
	in := validProduct()
	in.Name = "Otro nombre"
	in.TermsMax = 24
	in.GraceDays = intPtr(7)
	want := []string{"name", "terms_max", "grace_days"}
	if got := domain.ChangedFields(old, in); !slices.Equal(got, want) {
		t.Fatalf("cambios %v, esperaba %v", got, want)
	}
}

func TestChangedFields_NilVersusValueIsChange(t *testing.T) {
	old := validProduct()
	in := validProduct()
	in.LateFeeAmount = strPtr("10.00")
	if got := domain.ChangedFields(old, in); !slices.Equal(got, []string{"late_fee_amount"}) {
		t.Fatalf("cambios %v, esperaba [late_fee_amount]", got)
	}
}
