package postgres

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5"
	"prestamos/internal/tenants/domain"
)

// checkGlobalEmail serializes creation and restoration of the same email, then
// inspects each tenant using its RLS context, including suspended tenants.
func checkGlobalEmail(ctx context.Context, tx pgx.Tx, tenantID, userID, email string) error {
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended(lower($1), 0))`, strings.TrimSpace(email)); err != nil {
		return err
	}
	rows, err := tx.Query(ctx, `SELECT id::text FROM tenants`)
	if err != nil {
		return err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, id := range ids {
		if _, err := tx.Exec(ctx, `SELECT set_config('app.tenant_id', $1, true)`, id); err != nil {
			return err
		}
		var exists bool
		err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM users WHERE lower(email)=lower($1) AND deleted_at IS NULL AND id::text <> $2)`, email, userID).Scan(&exists)
		if err != nil {
			return err
		}
		if exists {
			return domain.ErrEmailTaken
		}
	}
	_, err = tx.Exec(ctx, `SELECT set_config('app.tenant_id', $1, true)`, tenantID)
	return err
}
