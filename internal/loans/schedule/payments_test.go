package schedule

import (
	"github.com/shopspring/decimal"
	"testing"
	"time"
)

func TestPersonalPayments_CountStartsOnFirstPayment(t *testing.T) {
	in := PersonalInput{Principal: decimal.NewFromInt(300), InterestType: "percentage", InterestValue: decimal.NewFromInt(20), Frequency: "daily", Start: time.Date(2026, 11, 14, 0, 0, 0, 0, time.UTC), InstallmentsCount: 5, FirstPayment: time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC)}
	rows, err := GeneratePersonal(in)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 5 || rows[0].DueDate.Format("2006-01-02") != "2026-11-15" || rows[4].DueDate.Format("2006-01-02") != "2026-11-19" {
		t.Fatalf("unexpected dates: %+v", rows)
	}
	for _, row := range rows {
		if !row.Principal.Add(row.Interest).Equal(decimal.NewFromInt(72)) {
			t.Fatal("expected five payments of 72")
		}
	}
	in.Frequency = "monthly"
	in.InstallmentsCount = 3
	in.FirstPayment = time.Date(2027, 1, 31, 0, 0, 0, 0, time.UTC)
	rows, err = GeneratePersonal(in)
	if err != nil || rows[1].DueDate.Format("2006-01-02") != "2027-02-28" || rows[2].DueDate.Format("2006-01-02") != "2027-03-31" {
		t.Fatalf("monthly anchor: %+v %v", rows, err)
	}
	in.Frequency = "single"
	in.InstallmentsCount = 2
	if _, err = GeneratePersonal(in); err == nil {
		t.Fatal("single payment must reject multiple installments")
	}
	in.InstallmentsCount = 1
	in.FirstPayment = in.Start.AddDate(0, 0, -1)
	if _, err = GeneratePersonal(in); err == nil {
		t.Fatal("payment before delivery must be rejected")
	}
}
