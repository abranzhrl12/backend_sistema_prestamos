package app

import (
	"context"

	"prestamos/internal/tenants/domain"
)

// DeleteTenant elimina (soft delete) una empresa: deja de aceptar logins y revoca sus sesiones.
// La empresa de sistema no se puede eliminar.
func (s *Service) DeleteTenant(ctx context.Context, actor, id string) error {
	cur, err := s.repo.GetTenant(ctx, id)
	if err != nil {
		return err
	}
	if cur.System {
		return domain.ErrPlatformProtected
	}
	return s.repo.Atomic(ctx, func(r domain.Repository) error {
		if err := r.SoftDeleteTenant(ctx, cur.ID, actor); err != nil {
			return err
		}
		if err := r.RevokeTenantSessions(ctx, cur.ID); err != nil {
			return err
		}
		return s.record(ctx, r, actor, change{action: actionTenantDelete, entity: entityTenant,
			entityID: cur.ID, tenantID: cur.ID, fields: []string{"deleted_at"}})
	})
}

// RestoreTenant vuelve a activar una empresa eliminada. Una empresa viva o inexistente es ErrTenantNotFound.
func (s *Service) RestoreTenant(ctx context.Context, actor, id string) (*domain.Tenant, error) {
	err := s.repo.Atomic(ctx, func(r domain.Repository) error {
		if err := r.RestoreTenant(ctx, id); err != nil {
			return err
		}
		return s.record(ctx, r, actor, change{action: actionTenantRestore, entity: entityTenant,
			entityID: id, tenantID: id, fields: []string{"deleted_at"}})
	})
	if err != nil {
		return nil, err
	}
	return s.repo.GetTenant(ctx, id)
}
