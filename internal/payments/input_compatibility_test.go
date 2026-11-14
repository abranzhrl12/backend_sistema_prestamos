package payments_test

import (
	"encoding/json"
	"github.com/shopspring/decimal"
	"prestamos/internal/payments"
	"testing"
)

func TestInput_AutomaticPaymentsPreserveLegacyIdempotencyPayload(t *testing.T) {
	in := payments.Input{LoanID: "loan", UserID: "owner", IdempotencyKey: "retry", Amount: decimal.NewFromInt(20), Parts: []payments.Part{{Method: "cash", Amount: decimal.NewFromInt(20)}}}
	legacy := struct {
		LoanID, UserID, IdempotencyKey string
		Amount                         decimal.Decimal
		Parts                          []payments.Part
	}{in.LoanID, in.UserID, in.IdempotencyKey, in.Amount, in.Parts}
	got, _ := json.Marshal(in)
	want, _ := json.Marshal(legacy)
	if string(got) != string(want) {
		t.Fatal("adding explicit distribution must not change existing automatic payment request hashes")
	}
}
