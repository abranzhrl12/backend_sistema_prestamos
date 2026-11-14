package app

import (
	"context"
	"errors"

	"prestamos/internal/tenants/domain"
)

const platformName = "Plataforma"

// EnsurePlatformAdmin crea (si faltan) la empresa "platform" y su superadmin. Es idempotente:
// created=false significa que el usuario ya existía y no se tocó nada.
func (s *Service) EnsurePlatformAdmin(ctx context.Context, email, fullName, password string) (bool, error) {
	in := domain.NewUser{Email: email, FullName: fullName, Password: password}
	in.Trim()
	if err := in.ValidateAccount(); err != nil {
		return false, err
	}
	hash, err := s.hasher.Hash(password)
	if err != nil {
		return false, err
	}
	created := false
	err = s.repo.Atomic(ctx, func(r domain.Repository) error {
		tenant, err := s.platformTenant(ctx, r)
		if err != nil {
			return err
		}
		_, err = r.FindUserByEmail(ctx, tenant.ID, in.Email)
		if err == nil {
			return nil
		}
		if !errors.Is(err, domain.ErrUserNotFound) {
			return err
		}
		if err := s.insertSuperadmin(ctx, r, tenant.ID, in, hash); err != nil {
			return err
		}
		created = true
		return nil
	})
	return created, err
}

// platformTenant devuelve la empresa platform, creándola si no existe (dentro de la tx r).
func (s *Service) platformTenant(ctx context.Context, r domain.Repository) (*domain.Tenant, error) {
	t, err := r.FindTenantBySlug(ctx, domain.PlatformSlug)
	if err == nil {
		return t, s.ensureSystem(ctx, r, t)
	}
	if !errors.Is(err, domain.ErrTenantNotFound) {
		return nil, err
	}
	id, err := s.ids.New()
	if err != nil {
		return nil, err
	}
	t = &domain.Tenant{ID: id, Slug: domain.PlatformSlug, Name: platformName, Active: true, System: true}
	if err := r.InsertTenant(ctx, t); err != nil {
		return nil, err
	}
	err = s.record(ctx, r, "", change{action: actionTenantCreate, entity: entityTenant, entityID: id,
		tenantID: id, fields: []string{"slug", "name", "is_system"},
		values: map[string]any{"slug": t.Slug, "name": t.Name, "is_system": true}})
	return t, err
}

// ensureSystem marca una plataforma preexistente como empresa de sistema (idempotente).
func (s *Service) ensureSystem(ctx context.Context, r domain.Repository, t *domain.Tenant) error {
	if t.System {
		return nil
	}
	if err := r.MarkSystem(ctx, t.ID); err != nil {
		return err
	}
	t.System = true
	return s.record(ctx, r, "", change{action: actionTenantUpdate, entity: entityTenant, entityID: t.ID,
		tenantID: t.ID, fields: []string{"is_system"}, values: map[string]any{"is_system": true}})
}

func (s *Service) insertSuperadmin(ctx context.Context, r domain.Repository, tenantID string, in domain.NewUser, hash string) error {
	id, err := s.ids.New()
	if err != nil {
		return err
	}
	u := &domain.User{ID: id, TenantID: tenantID, Email: in.Email, FullName: in.FullName,
		Roles: []string{domain.RoleSuperadmin}, Active: true}
	if err := r.InsertUser(ctx, u, hash); err != nil {
		return err
	}
	return s.record(ctx, r, "", change{action: actionUserCreate, entity: entityUser, entityID: id,
		tenantID: tenantID, fields: []string{"email", "full_name", "roles"}, values: userValues(u)})
}
