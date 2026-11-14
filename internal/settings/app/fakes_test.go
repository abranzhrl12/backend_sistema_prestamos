package app_test

import (
	"context"
	"fmt"
	"time"

	"prestamos/internal/settings/domain"
)

// fakeRepo guarda una sola configuración en memoria y registra los audits escritos.
type fakeRepo struct {
	row    *domain.Settings
	saves  int
	audits []domain.Audit
}

func (f *fakeRepo) Get(context.Context, string) (*domain.Settings, error) {
	if f.row == nil {
		return nil, domain.ErrNotFound
	}
	cp := *f.row
	return &cp, nil
}

func (f *fakeRepo) Save(_ context.Context, _ string, s domain.Settings, a domain.Audit) (*domain.Settings, error) {
	f.saves++
	f.audits = append(f.audits, a)
	s.IsDefault = false
	s.UpdatedAt = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	f.row = &s
	cp := s
	return &cp, nil
}

// seqIDs genera IDs predecibles para los tests.
type seqIDs struct{ n int }

func (s *seqIDs) New() (string, error) { s.n++; return fmt.Sprintf("id-%d", s.n), nil }
