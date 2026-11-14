package schedule_test

import (
	"github.com/shopspring/decimal"
	"prestamos/internal/loans/schedule"
	"testing"
)

func TestAllocate_OverpaymentRejected(t *testing.T) {
	_, err := schedule.Allocate(decimal.RequireFromString("11"), []schedule.Due{{Principal: decimal.RequireFromString("10")}})
	if err == nil {
		t.Fatal("overpayment must be rejected")
	}
}

func TestAllocate_LateFeeThenInterestThenPrincipal(t *testing.T) {
	due := []schedule.Due{{Principal: decimal.RequireFromString("10"), Interest: decimal.RequireFromString("2"), LateFee: decimal.RequireFromString("1")}}
	got, err := schedule.Allocate(decimal.RequireFromString("5"), due)
	if err != nil {
		t.Fatal(err)
	}
	if !got[0].LateFee.Equal(decimal.RequireFromString("1")) || !got[0].Interest.Equal(decimal.RequireFromString("2")) || !got[0].Principal.Equal(decimal.RequireFromString("2")) {
		t.Fatalf("allocation: %+v", got[0])
	}
}
