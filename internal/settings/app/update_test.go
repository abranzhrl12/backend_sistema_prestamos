package app_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"prestamos/internal/settings/app"
	"prestamos/internal/settings/domain"
)

func TestUpdate_SavesAndMarksNotDefault(t *testing.T) {
	repo := &fakeRepo{}
	s := app.NewService(repo, &seqIDs{})
	in := domain.Defaults()
	in.Currency = domain.CurrencyUSD
	out, err := s.Update(context.Background(), "t1", "user-1", in)
	if err != nil {
		t.Fatal(err)
	}
	if out.IsDefault || out.Currency != domain.CurrencyUSD {
		t.Fatalf("resultado inesperado: %+v", out)
	}
	if repo.saves != 1 {
		t.Fatalf("esperaba 1 escritura, hubo %d", repo.saves)
	}
}

func TestUpdate_AuditListsOnlyChangedFields(t *testing.T) {
	repo := &fakeRepo{}
	s := app.NewService(repo, &seqIDs{})
	in := domain.Defaults()
	in.GraceDays = 5
	in.LateFeeAmount = "10" // igual que el default ("10.00"): no debe figurar como cambio
	if _, err := s.Update(context.Background(), "t1", "user-1", in); err != nil {
		t.Fatal(err)
	}
	a := repo.audits[0]
	if a.Action != "settings.update" || a.UserID != "user-1" {
		t.Fatalf("audit inesperado: %+v", a)
	}
	fields, _ := a.Data["fields"].([]string)
	if !slices.Equal(fields, []string{"grace_days"}) {
		t.Fatalf("campos cambiados %v, esperaba [grace_days]", fields)
	}
}

func TestUpdate_InvalidDoesNotSave(t *testing.T) {
	repo := &fakeRepo{}
	s := app.NewService(repo, &seqIDs{})
	in := domain.Defaults()
	in.Timezone = "Mars/Olympus"
	_, err := s.Update(context.Background(), "t1", "user-1", in)
	var ve *domain.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("esperaba ValidationError, got %v", err)
	}
	if repo.saves != 0 {
		t.Fatal("una configuración inválida no debe guardarse")
	}
}
