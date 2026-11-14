// Package app contiene los casos de uso de la caja única.
package app

import (
	"context"
	"fmt"

	"prestamos/internal/capital/domain"
	"prestamos/internal/platform/ids"
)

// SettingsReader lo implementa el módulo de configuración de la empresa.
type SettingsReader interface {
	AllowNegativeCapital(ctx context.Context, tenantID string) (bool, error)
}

// Service orquesta el registro manual de movimientos, el saldo y el listado.
type Service struct {
	repo     domain.Repository
	settings SettingsReader
	ids      ids.Generator
}

func NewService(repo domain.Repository, settings SettingsReader, ids ids.Generator) *Service {
	return &Service{repo: repo, settings: settings, ids: ids}
}

// Record valida y registra un movimiento manual. Los retiros y ajustes de salida no pueden dejar
// la caja en negativo, salvo que la empresa lo permita (la comprobación ocurre en el repositorio, atómica).
func (s *Service) Record(ctx context.Context, tenantID, actorID string, in domain.Input) (domain.Movement, error) {
	m, err := domain.NewManualMovement(in)
	if err != nil {
		return domain.Movement{}, err
	}
	if m.ID, err = s.ids.New(); err != nil {
		return domain.Movement{}, appErr("Record", err)
	}
	m.TenantID, m.CreatedBy = tenantID, &actorID
	requireFunds, err := s.needsFundsCheck(ctx, tenantID, m)
	if err != nil {
		return domain.Movement{}, err
	}
	if err := s.repo.Append(ctx, tenantID, &m, requireFunds); err != nil {
		return domain.Movement{}, appErr("Record", err)
	}
	return m, nil
}

// needsFundsCheck consulta la configuración solo cuando el movimiento saca dinero.
func (s *Service) needsFundsCheck(ctx context.Context, tenantID string, m domain.Movement) (bool, error) {
	if !m.RequiresFunds() {
		return false, nil
	}
	allowNegative, err := s.settings.AllowNegativeCapital(ctx, tenantID)
	if err != nil {
		return false, appErr("Record.settings", err)
	}
	return !allowNegative, nil
}

func (s *Service) Balance(ctx context.Context, tenantID string) (domain.Balance, error) {
	b, err := s.repo.Balance(ctx, tenantID)
	return b, appErr("Balance", err)
}

func (s *Service) List(ctx context.Context, tenantID string, f domain.ListFilter) (domain.ListResult, error) {
	res, err := s.repo.List(ctx, tenantID, f)
	return res, appErr("List", err)
}

// appErr envuelve un error de infraestructura con módulo, capa y operación (%w conserva los centinelas).
func appErr(op string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("capital.app.%s: %w", op, err)
}
