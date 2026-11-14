package app

import (
	"context"

	"prestamos/internal/tenants/domain"
)

// DeleteUser elimina (soft delete) un usuario de una empresa viva y revoca sus sesiones.
// No se puede dejar la plataforma sin superadmin activo.
func (s *Service) DeleteUser(ctx context.Context, actor, tenantID, userID string) error {
	if _, err := s.repo.GetTenant(ctx, tenantID); err != nil {
		return err
	}
	cur, err := s.repo.GetUser(ctx, tenantID, userID)
	if err != nil {
		return err
	}
	return s.repo.Atomic(ctx, func(r domain.Repository) error {
		if cur.Active && cur.IsSuperadmin() {
			if err := ensureOtherSuperadmin(ctx, r, tenantID, cur.ID); err != nil {
				return err
			}
		}
		if err := r.SoftDeleteUser(ctx, tenantID, cur.ID, actor); err != nil {
			return err
		}
		if err := r.RevokeSessions(ctx, tenantID, cur.ID); err != nil {
			return err
		}
		return s.record(ctx, r, actor, change{action: actionUserDelete, entity: entityUser,
			entityID: cur.ID, tenantID: tenantID, fields: []string{"deleted_at"}})
	})
}

// RestoreUser vuelve a activar un usuario eliminado de una empresa viva.
func (s *Service) RestoreUser(ctx context.Context, actor, tenantID, userID string) (*domain.User, error) {
	if _, err := s.repo.GetTenant(ctx, tenantID); err != nil {
		return nil, err
	}
	err := s.repo.Atomic(ctx, func(r domain.Repository) error {
		if err := r.RestoreUser(ctx, tenantID, userID); err != nil {
			return err
		}
		return s.record(ctx, r, actor, change{action: actionUserRestore, entity: entityUser,
			entityID: userID, tenantID: tenantID, fields: []string{"deleted_at"}})
	})
	if err != nil {
		return nil, err
	}
	return s.repo.GetUser(ctx, tenantID, userID)
}

// ensureOtherSuperadmin exige que, tras eliminar a userID, quede otro superadmin activo.
// Bloquea las filas de superadmin dentro de la transacción: dos eliminaciones concurrentes no pueden
// dejar la plataforma sin administradores.
func ensureOtherSuperadmin(ctx context.Context, r domain.Repository, tenantID, userID string) error {
	ids, err := r.LockActiveSuperadmins(ctx, tenantID)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if id != userID {
			return nil
		}
	}
	return domain.ErrLastSuperadmin
}
