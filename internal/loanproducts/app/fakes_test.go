package app_test

import (
	"context"
	"fmt"
	"strings"

	"prestamos/internal/loanproducts/domain"
)

type row struct {
	p       domain.Product
	deleted bool
}

// fakeRepo respeta las reglas de la BD: unicidad de nombre (sin distinguir mayúsculas) entre
// filas vivas y soft delete (las filas eliminadas no se ven ni se modifican).
type fakeRepo struct {
	rows   map[string]*row
	audits []domain.Audit
}

func newFakeRepo() *fakeRepo { return &fakeRepo{rows: map[string]*row{}} }

func (f *fakeRepo) nameTaken(tenantID, name, exceptID string) bool {
	for id, r := range f.rows {
		if id != exceptID && !r.deleted && r.p.TenantID == tenantID && strings.EqualFold(r.p.Name, name) {
			return true
		}
	}
	return false
}

func (f *fakeRepo) live(tenantID, id string) *row {
	if r, ok := f.rows[id]; ok && !r.deleted && r.p.TenantID == tenantID {
		return r
	}
	return nil
}

func (f *fakeRepo) Create(_ context.Context, p *domain.Product, a domain.Audit) error {
	if f.nameTaken(p.TenantID, p.Name, "") {
		return domain.ErrNameTaken
	}
	f.rows[p.ID] = &row{p: *p}
	f.audits = append(f.audits, a)
	return nil
}

func (f *fakeRepo) Update(_ context.Context, p *domain.Product, a domain.Audit) error {
	r := f.live(p.TenantID, p.ID)
	if r == nil {
		return domain.ErrNotFound
	}
	if f.nameTaken(p.TenantID, p.Name, p.ID) {
		return domain.ErrNameTaken
	}
	r.p = *p
	f.audits = append(f.audits, a)
	return nil
}

func (f *fakeRepo) Delete(_ context.Context, tenantID, id string, a domain.Audit) error {
	r := f.live(tenantID, id)
	if r == nil {
		return domain.ErrNotFound
	}
	r.deleted = true
	f.audits = append(f.audits, a)
	return nil
}

func (f *fakeRepo) Get(_ context.Context, tenantID, id string) (*domain.Product, error) {
	r := f.live(tenantID, id)
	if r == nil {
		return nil, domain.ErrNotFound
	}
	cp := r.p
	return &cp, nil
}

func (f *fakeRepo) List(_ context.Context, tenantID string, _ domain.ListFilter) (domain.ListResult, error) {
	var res domain.ListResult
	for _, r := range f.rows {
		if !r.deleted && r.p.TenantID == tenantID {
			res.Items = append(res.Items, r.p)
		}
	}
	return res, nil
}

// seqIDs genera UUID predecibles (el servicio exige formato UUID en los ids de la URL).
type seqIDs struct{ n int }

func (s *seqIDs) New() (string, error) {
	s.n++
	return fmt.Sprintf("00000000-0000-7000-8000-%012d", s.n), nil
}
