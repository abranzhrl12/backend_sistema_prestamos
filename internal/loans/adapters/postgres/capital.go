package postgres

import (
	"context"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
	"prestamos/internal/loans/app"
)

// Both callers hold the tenant's capital lock until their transaction ends.
func requireCapital(ctx context.Context, tx pgx.Tx, principal string) error {
	var funds string
	err := tx.QueryRow(ctx, `SELECT coalesce(sum(CASE WHEN type IN ('contribution','adjustment_in','principal_collection','interest_collection','late_fee_collection') THEN amount WHEN type IN ('withdrawal','adjustment_out','disbursement','expense') THEN -amount END),0)::text FROM capital_movements`).Scan(&funds)
	if err != nil {
		return err
	}
	balance, err := decimal.NewFromString(funds)
	if err != nil {
		return err
	}
	required, err := decimal.NewFromString(principal)
	if err != nil {
		return err
	}
	if balance.LessThan(required) {
		return &app.CapitalShortfall{Available: balance.StringFixed(2)}
	}
	return nil
}
