// Package app contiene los casos de uso de la configuración de la empresa.
package app

import (
	"context"
	"errors"
	"fmt"

	"prestamos/internal/platform/ids"
	"prestamos/internal/settings/domain"
)

const actionUpdate = "settings.update"

type Service struct {
	repo domain.Repository
	ids  ids.Generator
}

func NewService(repo domain.Repository, ids ids.Generator) *Service {
	return &Service{repo: repo, ids: ids}
}

// Get devuelve la configuración guardada o, si no existe, los valores por defecto (sin escribir).
func (s *Service) Get(ctx context.Context, tenantID string) (*domain.Settings, error) {
	cur, err := s.repo.Get(ctx, tenantID)
	if errors.Is(err, domain.ErrNotFound) {
		d := domain.Defaults()
		return &d, nil
	}
	return cur, appErr("Get", err)
}

// Update reemplaza toda la configuración y registra en audit los campos que cambiaron.
func (s *Service) Update(ctx context.Context, tenantID, actorID string, in domain.Settings) (*domain.Settings, error) {
	if err := in.Validate(); err != nil {
		return nil, err
	}
	cur, err := s.Get(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	id, err := s.ids.New()
	if err != nil {
		return nil, appErr("Update", err)
	}
	audit := domain.Audit{
		ID:     id,
		UserID: actorID,
		Action: actionUpdate,
		Data:   map[string]any{"fields": domain.ChangedFields(*cur, in)},
	}
	out, err := s.repo.Save(ctx, tenantID, in, audit)
	return out, appErr("Update", err)
}

// AllowNegativeCapital indica si la empresa permite que el capital disponible quede negativo.
func (s *Service) AllowNegativeCapital(ctx context.Context, tenantID string) (bool, error) {
	cur, err := s.Get(ctx, tenantID)
	if err != nil {
		return false, err
	}
	return cur.AllowNegativeCapital, nil
}

// appErr envuelve un error de infraestructura con módulo, capa y operación (%w conserva los centinelas).
func appErr(op string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("settings.app.%s: %w", op, err)
}
