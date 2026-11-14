// Package app contiene los casos de uso de productos de préstamo.
package app

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"prestamos/internal/loanproducts/domain"
	"prestamos/internal/platform/ids"
)

const (
	defaultLimit = 20
	maxLimit     = 100
)

// Actor es quien ejecuta la operación: su tenant (de los claims) y su usuario (para el audit).
type Actor struct{ TenantID, UserID string }

type Service struct {
	repo domain.Repository
	ids  ids.Generator
}

func NewService(repo domain.Repository, ids ids.Generator) *Service {
	return &Service{repo: repo, ids: ids}
}

func (s *Service) Create(ctx context.Context, a Actor, in domain.Product) (*domain.Product, error) {
	in.Normalize()
	if err := in.Validate(); err != nil {
		return nil, err
	}
	id, err := s.ids.New()
	if err != nil {
		return nil, appErr("Create", err)
	}
	audit, err := s.newAudit(a, "product.create", map[string]any{"name": in.Name})
	if err != nil {
		return nil, appErr("Create", err)
	}
	in.ID, in.TenantID = id, a.TenantID
	if err := s.repo.Create(ctx, &in, audit); err != nil {
		return nil, appErr("Create", err)
	}
	out, err := s.repo.Get(ctx, a.TenantID, id)
	return out, appErr("Create", err)
}

// Update reemplaza todos los datos del producto; registra en audit los campos que cambiaron.
func (s *Service) Update(ctx context.Context, a Actor, id string, in domain.Product) (*domain.Product, error) {
	cur, err := s.Get(ctx, a.TenantID, id)
	if err != nil {
		return nil, err
	}
	in.Normalize()
	if err := in.Validate(); err != nil {
		return nil, err
	}
	audit, err := s.newAudit(a, "product.update", map[string]any{"fields": domain.ChangedFields(*cur, in)})
	if err != nil {
		return nil, appErr("Update", err)
	}
	in.ID, in.TenantID = cur.ID, cur.TenantID
	if err := s.repo.Update(ctx, &in, audit); err != nil {
		return nil, appErr("Update", err)
	}
	out, err := s.repo.Get(ctx, a.TenantID, id)
	return out, appErr("Update", err)
}

// Delete marca el producto como eliminado (soft delete). Un producto ya eliminado es ErrNotFound.
func (s *Service) Delete(ctx context.Context, a Actor, id string) error {
	cur, err := s.Get(ctx, a.TenantID, id)
	if err != nil {
		return err
	}
	audit, err := s.newAudit(a, "product.delete", map[string]any{"name": cur.Name})
	if err != nil {
		return appErr("Delete", err)
	}
	return appErr("Delete", s.repo.Delete(ctx, a.TenantID, id, audit))
}

func (s *Service) Get(ctx context.Context, tenantID, id string) (*domain.Product, error) {
	if !validID(id) {
		return nil, appErr("Get", domain.ErrNotFound)
	}
	out, err := s.repo.Get(ctx, tenantID, id)
	return out, appErr("Get", err)
}

func (s *Service) List(ctx context.Context, tenantID string, f domain.ListFilter) (domain.ListResult, error) {
	if f.Frequency != "" && !domain.ValidFrequency(f.Frequency) {
		return domain.ListResult{}, domain.NewValidationError("frequency", "Frecuencia inválida (daily, weekly, biweekly, monthly o single)")
	}
	if f.Limit <= 0 {
		f.Limit = defaultLimit
	}
	f.Limit = min(f.Limit, maxLimit)
	f.Offset = max(f.Offset, 0)
	res, err := s.repo.List(ctx, tenantID, f)
	return res, appErr("List", err)
}

func (s *Service) newAudit(a Actor, action string, data map[string]any) (domain.Audit, error) {
	id, err := s.ids.New()
	if err != nil {
		return domain.Audit{}, err
	}
	return domain.Audit{ID: id, UserID: a.UserID, Action: action, Data: data}, nil
}

// validID evita enviar a Postgres un id que no es UUID (daría error 500 en vez de 404).
func validID(id string) bool {
	_, err := uuid.Parse(id)
	return err == nil
}

// appErr envuelve un error con módulo, capa y operación; %w conserva los centinelas.
func appErr(op string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("loanproducts.app.%s: %w", op, err)
}
