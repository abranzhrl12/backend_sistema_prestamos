package postgres

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"

	"prestamos/internal/tenants/domain"
)

// Audit inserta una fila en audit_log dentro de la empresa afectada (RLS). Sin actor = NULL.
func (r *Repo) Audit(ctx context.Context, e domain.AuditEntry) error {
	data, err := json.Marshal(e.Data)
	if err != nil {
		return wrap("Audit", err)
	}
	err = r.scoped(ctx, e.TenantID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO audit_log (id, tenant_id, user_id, action, entity, entity_id, data)
			VALUES ($1::uuid, $2::uuid, nullif($3, '')::uuid, $4, $5, $6::uuid, $7::jsonb)`,
			e.ID, e.TenantID, e.ActorID, e.Action, e.Entity, e.EntityID, string(data))
		return err
	})
	return wrap("Audit", err)
}
