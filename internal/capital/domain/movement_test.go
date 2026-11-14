package domain_test

import (
	"testing"

	"prestamos/internal/capital/domain"
)

func TestType_DirectionAndBucketForAllNineTypes(t *testing.T) {
	cases := []struct {
		typ    domain.Type
		dir    domain.Direction
		bucket domain.Bucket
	}{
		{domain.TypeContribution, domain.DirectionIn, domain.BucketCapital},
		{domain.TypeWithdrawal, domain.DirectionOut, domain.BucketCapital},
		{domain.TypeAdjustmentIn, domain.DirectionIn, domain.BucketCapital},
		{domain.TypeAdjustmentOut, domain.DirectionOut, domain.BucketCapital},
		{domain.TypeDisbursement, domain.DirectionOut, domain.BucketCapital},
		{domain.TypePrincipalCollection, domain.DirectionIn, domain.BucketCapital},
		{domain.TypeInterestCollection, domain.DirectionIn, domain.BucketProfit},
		{domain.TypeLateFeeCollection, domain.DirectionIn, domain.BucketProfit},
		{domain.TypeExpense, domain.DirectionOut, domain.BucketProfit},
	}
	for _, c := range cases {
		if got := c.typ.Direction(); got != c.dir {
			t.Errorf("%s: dirección %q, esperaba %q", c.typ, got, c.dir)
		}
		if got := c.typ.Bucket(); got != c.bucket {
			t.Errorf("%s: bucket %q, esperaba %q", c.typ, got, c.bucket)
		}
	}
}

func TestType_OnlyFourManualTypesAreAllowed(t *testing.T) {
	manual := []domain.Type{domain.TypeContribution, domain.TypeWithdrawal, domain.TypeAdjustmentIn, domain.TypeAdjustmentOut}
	for _, typ := range manual {
		if !typ.Manual() {
			t.Errorf("%s debería ser manual", typ)
		}
	}
	internal := []domain.Type{domain.TypeDisbursement, domain.TypePrincipalCollection,
		domain.TypeInterestCollection, domain.TypeLateFeeCollection, domain.TypeExpense}
	for _, typ := range internal {
		if typ.Manual() {
			t.Errorf("%s no debería ser manual", typ)
		}
	}
}

func TestKnownType_AcceptsNineAndRejectsUnknown(t *testing.T) {
	if !domain.KnownType("late_fee_collection") || !domain.KnownType("contribution") {
		t.Fatal("los tipos conocidos deberían aceptarse")
	}
	if domain.KnownType("prestamo") || domain.KnownType("") {
		t.Fatal("un tipo desconocido no debería aceptarse")
	}
}

func TestMovement_RequiresFundsOnlyWhenCashLeaves(t *testing.T) {
	out := domain.Movement{Type: domain.TypeWithdrawal}
	in := domain.Movement{Type: domain.TypeContribution}
	if !out.RequiresFunds() || in.RequiresFunds() {
		t.Fatal("solo los movimientos de salida deben exigir fondos")
	}
}
