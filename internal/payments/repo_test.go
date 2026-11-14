package payments_test

import (
	"context"
	"errors"
	"github.com/shopspring/decimal"
	"prestamos/internal/payments"
	"testing"
)

func TestRecordRejectsPartsThatDoNotSum(t *testing.T) {
	repo := payments.NewRepo(nil, nil)
	_, err := repo.Record(context.Background(), "tenant", payments.Input{
		LoanID: "loan", UserID: "user", IdempotencyKey: "key",
		Amount: decimal.RequireFromString("10"),
		Parts:  []payments.Part{{Method: "cash", Amount: decimal.RequireFromString("9")}},
	})
	if !errors.Is(err, payments.ErrInvalidInput) {
		t.Fatalf("expected invalid input, got %v", err)
	}
}
