package app_test

import (
	"context"
	"fmt"

	"prestamos/internal/customers/domain"
)

// fakeRepo es un repositorio en memoria que respeta el aislamiento por tenant, la unicidad de
// documento solo entre clientes vivos y el soft delete (las filas eliminadas no se ven ni se modifican).
type fakeRepo struct {
	data    map[string]*domain.Customer
	deleted map[string]bool
	audits  []domain.Deletion
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{data: map[string]*domain.Customer{}, deleted: map[string]bool{}}
}

func (f *fakeRepo) live(tenantID, id string) (*domain.Customer, bool) {
	c, ok := f.data[id]
	return c, ok && !f.deleted[id] && c.TenantID == tenantID
}

func (f *fakeRepo) Create(_ context.Context, c *domain.Customer) error {
	for id, o := range f.data {
		if !f.deleted[id] && o.TenantID == c.TenantID && o.DocumentID == c.DocumentID {
			return domain.ErrDuplicateDocument
		}
	}
	cp := *c
	f.data[c.ID] = &cp
	return nil
}

func (f *fakeRepo) Update(_ context.Context, c *domain.Customer) error {
	if _, ok := f.live(c.TenantID, c.ID); !ok {
		return domain.ErrNotFound
	}
	cp := *c
	f.data[c.ID] = &cp
	return nil
}

func (f *fakeRepo) Get(_ context.Context, tenantID, id string) (*domain.Customer, error) {
	if c, ok := f.live(tenantID, id); ok {
		cp := *c
		return &cp, nil
	}
	return nil, domain.ErrNotFound
}

func (f *fakeRepo) List(context.Context, string, domain.ListFilter) (domain.ListResult, error) {
	return domain.ListResult{}, nil
}

func (f *fakeRepo) SoftDelete(_ context.Context, d domain.Deletion) error {
	if _, ok := f.live(d.TenantID, d.ID); !ok {
		return domain.ErrNotFound
	}
	f.deleted[d.ID] = true
	f.audits = append(f.audits, d)
	return nil
}

// seqIDs genera IDs predecibles para los tests.
type seqIDs struct{ n int }

func (s *seqIDs) New() (string, error) { s.n++; return fmt.Sprintf("id-%d", s.n), nil }
