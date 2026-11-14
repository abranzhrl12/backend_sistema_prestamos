package schedule_test

import (
	"github.com/shopspring/decimal"
	"prestamos/internal/loans/schedule"
	"testing"
	"time"
)

func TestGenerate_SmallPrincipalPreserved(t *testing.T) {
	rows, err := schedule.GeneratePersonal(schedule.PersonalInput{Principal: decimal.RequireFromString("0.0002"), InterestType: "none", TermValue: 4, TermUnit: "weeks", Frequency: "weekly", Start: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatal(err)
	}
	total := decimal.Zero
	for _, r := range rows {
		total = total.Add(r.Principal)
		if r.Balance.IsNegative() {
			t.Fatal("negative balance")
		}
	}
	if !total.Equal(decimal.RequireFromString("0.0002")) {
		t.Fatal(total)
	}
}

func TestAllocate_RejectsFractionalUnits(t *testing.T) {
	_, err := schedule.Allocate(decimal.RequireFromString("0.0001"), []schedule.Due{{Principal: decimal.RequireFromString("0.00001")}, {Principal: decimal.RequireFromString("1")}})
	if err == nil {
		t.Fatal("invalid precision")
	}
}
