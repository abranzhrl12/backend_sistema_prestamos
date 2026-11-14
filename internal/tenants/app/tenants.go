package app

import (
	"context"

	"prestamos/internal/tenants/domain"
)

// CreateTenant crea la empresa y su owner en una sola transacción. actor = id del superadmin.
func (s *Service) CreateTenant(ctx context.Context, actor string, in domain.NewTenant) (*domain.Tenant, *domain.User, error) {
	if err := in.Normalize(); err != nil {
		return nil, nil, err
	}
	tenantID, err := s.ids.New()
	if err != nil {
		return nil, nil, err
	}
	ownerID, err := s.ids.New()
	if err != nil {
		return nil, nil, err
	}
	hash, err := s.hasher.Hash(in.Owner.Password)
	if err != nil {
		return nil, nil, err
	}
	t := &domain.Tenant{ID: tenantID, Slug: in.Slug, Name: in.Name, Active: true}
	owner := &domain.User{ID: ownerID, TenantID: tenantID, Email: in.Owner.Email, FullName: in.Owner.FullName,
		Roles: []string{domain.RoleOwner}, Active: true}
	if err := s.insertTenantWithOwner(ctx, actor, t, owner, hash); err != nil {
		return nil, nil, err
	}
	return s.createdDetails(ctx, tenantID, ownerID)
}

func (s *Service) insertTenantWithOwner(ctx context.Context, actor string, t *domain.Tenant, owner *domain.User, hash string) error {
	return s.repo.Atomic(ctx, func(r domain.Repository) error {
		if err := r.InsertTenant(ctx, t); err != nil {
			return err
		}
		if err := r.InsertUser(ctx, owner, hash); err != nil {
			return err
		}
		if err := s.record(ctx, r, actor, change{action: actionTenantCreate, entity: entityTenant,
			entityID: t.ID, tenantID: t.ID, fields: []string{"slug", "name"},
			values: map[string]any{"slug": t.Slug, "name": t.Name}}); err != nil {
			return err
		}
		return s.record(ctx, r, actor, change{action: actionUserCreate, entity: entityUser,
			entityID: owner.ID, tenantID: t.ID, fields: []string{"email", "full_name", "roles"},
			values: userValues(owner)})
	})
}

func (s *Service) createdDetails(ctx context.Context, tenantID, ownerID string) (*domain.Tenant, *domain.User, error) {
	t, err := s.repo.GetTenant(ctx, tenantID)
	if err != nil {
		return nil, nil, err
	}
	owner, err := s.repo.GetUser(ctx, tenantID, ownerID)
	if err != nil {
		return nil, nil, err
	}
	return t, owner, nil
}

func (s *Service) ListTenants(ctx context.Context, f domain.TenantFilter) (domain.TenantPage, error) {
	return s.repo.ListTenants(ctx, f)
}

// GetTenant devuelve la empresa con su número de usuarios.
func (s *Service) GetTenant(ctx context.Context, id string) (*domain.Tenant, error) {
	return s.repo.GetTenant(ctx, id)
}

// TenantBySlug busca una empresa por su identificador público.
func (s *Service) TenantBySlug(ctx context.Context, slug string) (*domain.Tenant, error) {
	return s.repo.FindTenantBySlug(ctx, slug)
}

// UpdateTenant cambia nombre y/o estado. Desactivar revoca todas las sesiones de la empresa.
func (s *Service) UpdateTenant(ctx context.Context, actor, id string, p domain.TenantPatch) (*domain.Tenant, error) {
	if err := p.Normalize(); err != nil {
		return nil, err
	}
	cur, err := s.repo.GetTenant(ctx, id)
	if err != nil {
		return nil, err
	}
	deactivating := p.Active != nil && !*p.Active
	if cur.System && deactivating {
		return nil, domain.ErrPlatformProtected
	}
	fields, values := applyTenantPatch(cur, p)
	if len(fields) == 0 {
		return cur, nil
	}
	err = s.repo.Atomic(ctx, func(r domain.Repository) error {
		if err := r.UpdateTenant(ctx, cur); err != nil {
			return err
		}
		if deactivating {
			if err := r.RevokeTenantSessions(ctx, cur.ID); err != nil {
				return err
			}
		}
		return s.record(ctx, r, actor, change{action: actionTenantUpdate, entity: entityTenant,
			entityID: cur.ID, tenantID: cur.ID, fields: fields, values: values})
	})
	if err != nil {
		return nil, err
	}
	return s.repo.GetTenant(ctx, id)
}
