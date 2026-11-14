package schedule_test

import (
	"github.com/shopspring/decimal"
	"prestamos/internal/loans/schedule"
	"testing"
	"time"
)

func TestPersonal_TotalInterestAndFinalDate(t *testing.T) {
	rows, err := schedule.GeneratePersonal(schedule.PersonalInput{Principal: decimal.NewFromInt(500), InterestType: "percentage", InterestValue: decimal.NewFromInt(10), TermValue: 1, TermUnit: "months", Frequency: "weekly", Start: time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 4 || rows[3].DueDate.Format("2006-01-02") != "2026-02-28" {
		t.Fatalf("dates: %+v", rows)
	}
	p, i := decimal.Zero, decimal.Zero
	for _, r := range rows {
		p = p.Add(r.Principal)
		i = i.Add(r.Interest)
	}
	if !p.Equal(decimal.NewFromInt(500)) || !i.Equal(decimal.NewFromInt(50)) {
		t.Fatalf("totals %s %s", p, i)
	}
}
func TestPersonal_SingleFixedInterest(t *testing.T) {
	rows, err := schedule.GeneratePersonal(schedule.PersonalInput{Principal: decimal.NewFromInt(100), InterestType: "fixed", InterestValue: decimal.NewFromInt(13), TermValue: 3, TermUnit: "weeks", Frequency: "single", Start: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)})
	if err != nil || len(rows) != 1 || rows[0].DueDate.Format("2006-01-02") != "2026-01-22" || !rows[0].Interest.Equal(decimal.NewFromInt(13)) {
		t.Fatalf("rows=%+v err=%v", rows, err)
	}
}
