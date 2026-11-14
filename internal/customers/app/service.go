// Package app contiene los casos de uso de clientes.
package app

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"prestamos/internal/customers/domain"
	"prestamos/internal/platform/ids"
)

const (
	defaultLimit = 20
	maxLimit     = 100
)

type Service struct {
	repo domain.Repository
	ids  ids.Generator
}

func NewService(repo domain.Repository, ids ids.Generator) *Service {
	return &Service{repo: repo, ids: ids}
}

func (s *Service) Create(ctx context.Context, tenantID string, c domain.Customer) (*domain.Customer, error) {
	id, err := s.ids.New()
	if err != nil {
		return nil, appErr("Create", err)
	}
	c.ID, c.TenantID, c.Status = id, tenantID, domain.StatusActive
	if err := c.Normalize(); err != nil {
		return nil, err
	}
	if err := s.repo.Create(ctx, &c); err != nil {
		return nil, appErr("Create", err)
	}
	out, err := s.repo.Get(ctx, tenantID, c.ID)
	return out, appErr("Create", err)
}

// Update reemplaza los datos editables; el estado se cambia con SetStatus.
func (s *Service) Update(ctx context.Context, tenantID, id string, in domain.Customer) (*domain.Customer, error) {
	cur, err := s.repo.Get(ctx, tenantID, id)
	if err != nil {
		return nil, appErr("Update", err)
	}
	in.ID, in.TenantID, in.Status = cur.ID, cur.TenantID, cur.Status
	if err := in.Normalize(); err != nil {
		return nil, err
	}
	if err := s.repo.Update(ctx, &in); err != nil {
		return nil, appErr("Update", err)
	}
	out, err := s.repo.Get(ctx, tenantID, id)
	return out, appErr("Update", err)
}

func (s *Service) SetStatus(ctx context.Context, tenantID, id string, st domain.Status) (*domain.Customer, error) {
	if !st.Valid() {
		return nil, domain.NewValidationError("status", "Estado inválido (active, inactive o blocked)")
	}
	cur, err := s.repo.Get(ctx, tenantID, id)
	if err != nil {
		return nil, appErr("SetStatus", err)
	}
	cur.Status = st
	if err := s.repo.Update(ctx, cur); err != nil {
		return nil, appErr("SetStatus", err)
	}
	out, err := s.repo.Get(ctx, tenantID, id)
	return out, appErr("SetStatus", err)
}

// Delete marca el cliente como eliminado (soft delete) y registra customer.delete en la auditoría.
// Un id que no es UUID o un cliente ya eliminado es domain.ErrNotFound.
func (s *Service) Delete(ctx context.Context, tenantID, actorID, id string) error {
	if _, err := uuid.Parse(id); err != nil {
		return appErr("Delete", domain.ErrNotFound)
	}
	auditID, err := s.ids.New()
	if err != nil {
		return appErr("Delete", err)
	}
	d := domain.Deletion{TenantID: tenantID, ID: id, ActorID: actorID, AuditID: auditID}
	return appErr("Delete", s.repo.SoftDelete(ctx, d))
}

func (s *Service) Get(ctx context.Context, tenantID, id string) (*domain.Customer, error) {
	out, err := s.repo.Get(ctx, tenantID, id)
	return out, appErr("Get", err)
}

func (s *Service) List(ctx context.Context, tenantID string, f domain.ListFilter) (domain.ListResult, error) {
	if f.Status != "" && !f.Status.Valid() {
		return domain.ListResult{}, domain.NewValidationError("status", "Estado inválido (active, inactive o blocked)")
	}
	if f.Limit <= 0 {
		f.Limit = defaultLimit
	}
	f.Limit = min(f.Limit, maxLimit)
	f.Offset = max(f.Offset, 0)
	res, err := s.repo.List(ctx, tenantID, f)
	return res, appErr("List", err)
}

// appErr envuelve un error de infraestructura con módulo, capa y operación.
// Devuelve nil si no hay error, y envuelve con %w para que los centinelas sigan funcionando.
func appErr(op string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("customers.app.%s: %w", op, err)
}
