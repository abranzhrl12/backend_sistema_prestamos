package app_test

import (
	"context"
	"errors"
	"testing"

	"prestamos/internal/capital/app"
	"prestamos/internal/capital/domain"
)

func record(t *testing.T, s *app.Service, tenant string, typ, amount string) error {
	t.Helper()
	note := ""
	if typ == "adjustment_in" || typ == "adjustment_out" {
		note = "Ajuste de prueba"
	}
	_, err := s.Record(context.Background(), tenant, "user-1", domain.Input{Type: typ, Amount: amount, Note: note})
	return err
}

func newSvc(allow bool) (*app.Service, *fakeRepo, *fakeSettings) {
	repo, set := &fakeRepo{}, &fakeSettings{allow: allow}
	return app.NewService(repo, set, &seqIDs{}), repo, set
}

func TestRecord_ContributionIsAlwaysAllowedAndSkipsSettings(t *testing.T) {
	s, _, set := newSvc(false)
	if err := record(t, s, "t1", "contribution", "100"); err != nil {
		t.Fatalf("un aporte siempre debe aceptarse: %v", err)
	}
	if set.calls != 0 {
		t.Fatalf("un aporte no debe consultar la configuración (llamadas: %d)", set.calls)
	}
}

func TestRecord_WithdrawalAboveBalance_ReturnsInsufficientCapital(t *testing.T) {
	s, repo, _ := newSvc(false)
	_ = record(t, s, "t1", "contribution", "1000")
	err := record(t, s, "t1", "withdrawal", "1000.01")
	if !errors.Is(err, domain.ErrInsufficientCapital) {
		t.Fatalf("esperaba ErrInsufficientCapital, got %v", err)
	}
	if n := repo.sumTenant("t1").MovementsCount; n != 1 {
		t.Fatalf("el retiro rechazado no debe insertarse; movimientos: %d", n)
	}
}

func TestRecord_WithdrawalEqualToBalance_IsAllowed(t *testing.T) {
	s, repo, _ := newSvc(false)
	_ = record(t, s, "t1", "contribution", "1000")
	if err := record(t, s, "t1", "withdrawal", "1000"); err != nil {
		t.Fatalf("retirar exactamente el saldo debe permitirse: %v", err)
	}
	if got := repo.sumTenant("t1").TotalCash.String(); got != "0" {
		t.Fatalf("saldo %s, esperaba 0", got)
	}
}

func TestRecord_WithdrawalAboveBalance_AllowedWhenNegativeCapitalEnabled(t *testing.T) {
	s, repo, _ := newSvc(true)
	if err := record(t, s, "t1", "withdrawal", "500"); err != nil {
		t.Fatalf("con capital negativo permitido el retiro debe pasar: %v", err)
	}
	if got := repo.sumTenant("t1").TotalCash.String(); got != "-500" {
		t.Fatalf("saldo %s, esperaba -500", got)
	}
}

func TestRecord_AdjustmentOutRespectsFunds(t *testing.T) {
	s, _, _ := newSvc(false)
	_ = record(t, s, "t1", "contribution", "50")
	if err := record(t, s, "t1", "adjustment_out", "60"); !errors.Is(err, domain.ErrInsufficientCapital) {
		t.Fatalf("un ajuste de salida sin fondos debe fallar, got %v", err)
	}
	if err := record(t, s, "t1", "adjustment_in", "60"); err != nil {
		t.Fatalf("un ajuste de entrada siempre pasa: %v", err)
	}
}

func TestRecord_InvalidInput_ReturnsValidationErrorAndWritesNothing(t *testing.T) {
	s, repo, _ := newSvc(false)
	var ve *domain.ValidationError
	err := record(t, s, "t1", "adjustment_in", "abc")
	if !errors.As(err, &ve) {
		t.Fatalf("esperaba ValidationError, got %v", err)
	}
	if n := repo.sumTenant("t1").MovementsCount; n != 0 {
		t.Fatalf("una entrada inválida no debe escribir; movimientos: %d", n)
	}
}

func TestRecord_SettingsErrorIsPropagated(t *testing.T) {
	s, _, set := newSvc(false)
	set.err = errors.New("db caída")
	if err := record(t, s, "t1", "withdrawal", "1"); err == nil || !errors.Is(err, set.err) {
		t.Fatalf("el error de configuración debe propagarse, got %v", err)
	}
}
