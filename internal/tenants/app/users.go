package app

import (
	"context"

	"prestamos/internal/tenants/domain"
)

// ListUsers lista los usuarios de una empresa (404 si la empresa no existe).
func (s *Service) ListUsers(ctx context.Context, tenantID string, f domain.UserFilter) (domain.UserPage, error) {
	if _, err := s.repo.GetTenant(ctx, tenantID); err != nil {
		return domain.UserPage{}, err
	}
	return s.repo.ListUsers(ctx, tenantID, f)
}

// CreateUser crea un usuario en una empresa de clientes (la plataforma está protegida).
func (s *Service) CreateUser(ctx context.Context, actor, tenantID string, in domain.NewUser) (*domain.User, error) {
	in.Trim()
	if err := in.Validate(); err != nil {
		return nil, err
	}
	tenant, err := s.repo.GetTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	if tenant.System {
		return nil, domain.ErrPlatformProtected
	}
	hash, err := s.hasher.Hash(in.Password)
	if err != nil {
		return nil, err
	}
	id, err := s.ids.New()
	if err != nil {
		return nil, err
	}
	u := &domain.User{ID: id, TenantID: tenantID, Email: in.Email, FullName: in.FullName, Roles: in.Roles, Active: true}
	err = s.repo.Atomic(ctx, func(r domain.Repository) error {
		if err := r.InsertUser(ctx, u, hash); err != nil {
			return err
		}
		return s.record(ctx, r, actor, change{action: actionUserCreate, entity: entityUser,
			entityID: id, tenantID: tenantID, fields: []string{"email", "full_name", "roles"}, values: userValues(u)})
	})
	if err != nil {
		return nil, err
	}
	return s.repo.GetUser(ctx, tenantID, id)
}

// UpdateUser cambia nombre, roles, estado y/o contraseña. Cambiar la contraseña o desactivar
// borra los refresh tokens del usuario (los access tokens ya emitidos expiran en ≤15 min).
func (s *Service) UpdateUser(ctx context.Context, actor, tenantID, userID string, p domain.UserPatch) (*domain.User, error) {
	if err := p.Normalize(); err != nil {
		return nil, err
	}
	tenant, err := s.repo.GetTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	if tenant.System {
		return nil, domain.ErrPlatformProtected
	}
	cur, err := s.repo.GetUser(ctx, tenantID, userID)
	if err != nil {
		return nil, err
	}
	fields, values := applyUserPatch(cur, p)
	var hash string
	if p.Password != nil {
		if hash, err = s.hasher.Hash(*p.Password); err != nil {
			return nil, err
		}
		fields = append(fields, "password")
	}
	if len(fields) == 0 {
		return cur, nil
	}
	revoke := p.Password != nil || (p.Active != nil && !*p.Active)
	err = s.repo.Atomic(ctx, func(r domain.Repository) error {
		if err := r.UpdateUser(ctx, cur); err != nil {
			return err
		}
		if p.Password != nil {
			if err := r.SetPasswordHash(ctx, tenantID, userID, hash); err != nil {
				return err
			}
		}
		if revoke {
			if err := r.RevokeSessions(ctx, tenantID, userID); err != nil {
				return err
			}
		}
		return s.record(ctx, r, actor, change{action: actionUserUpdate, entity: entityUser,
			entityID: userID, tenantID: tenantID, fields: fields, values: values})
	})
	if err != nil {
		return nil, err
	}
	return s.repo.GetUser(ctx, tenantID, userID)
}
