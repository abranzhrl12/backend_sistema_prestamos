package app

import (
	"context"

	"prestamos/internal/tenants/domain"
)

const (
	actionTenantCreate  = "tenant.create"
	actionTenantUpdate  = "tenant.update"
	actionTenantDelete  = "tenant.delete"
	actionTenantRestore = "tenant.restore"
	actionUserCreate    = "user.create"
	actionUserUpdate    = "user.update"
	actionUserDelete    = "user.delete"
	actionUserRestore   = "user.restore"
	entityTenant        = "tenant"
	entityUser          = "user"
)

// change describe una escritura a auditar. Nunca incluye passwords ni hashes.
type change struct {
	action, entity, entityID, tenantID string
	fields                             []string       // nombres de los campos cambiados
	values                             map[string]any // valores no sensibles de esos campos
}

// record escribe la auditoría dentro de la transacción r (misma tx que el cambio).
func (s *Service) record(ctx context.Context, r domain.Repository, actor string, c change) error {
	id, err := s.ids.New()
	if err != nil {
		return err
	}
	if c.values == nil {
		c.values = map[string]any{}
	}
	return r.Audit(ctx, domain.AuditEntry{
		ID: id, TenantID: c.tenantID, ActorID: actor, Action: c.action, Entity: c.entity, EntityID: c.entityID,
		Data: map[string]any{"changed": c.fields, "values": c.values},
	})
}

func userValues(u *domain.User) map[string]any {
	return map[string]any{"email": u.Email, "full_name": u.FullName, "roles": u.Roles}
}
