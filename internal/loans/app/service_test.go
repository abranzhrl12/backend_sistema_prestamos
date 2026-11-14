package app_test

import (
	"context"
	"prestamos/internal/loans/app"
	"testing"
)

func TestSimulate_WithoutProduct(t *testing.T) {
	rows, err := app.NewService().Simulate(context.Background(), "tenant", app.Input{Principal: "100", InterestType: "none", InterestValue: "0", TermValue: 5, TermUnit: "weeks", Frequency: "weekly", DisbursementDate: "2026-01-01"})
	if err != nil || len(rows) != 5 {
		t.Fatalf("rows=%d err=%v", len(rows), err)
	}
}
