package app_test

import (
	"context"
	"testing"

	"prestamos/internal/settings/app"
	"prestamos/internal/settings/domain"
)

func TestGet_WithoutRowReturnsDefaultsAndDoesNotWrite(t *testing.T) {
	repo := &fakeRepo{}
	s := app.NewService(repo, &seqIDs{})
	got, err := s.Get(context.Background(), "t1")
	if err != nil {
		t.Fatal(err)
	}
	if !got.IsDefault || got.Currency != domain.CurrencyPEN || got.LateFeeAmount != "10.00" {
		t.Fatalf("esperaba defaults, got %+v", got)
	}
	if repo.saves != 0 {
		t.Fatalf("Get no debe escribir, hubo %d escrituras", repo.saves)
	}
}

func TestGet_ReturnsStoredRow(t *testing.T) {
	d := domain.Defaults()
	d.Currency = domain.CurrencyUSD
	d.IsDefault = false
	repo := &fakeRepo{row: &d}
	got, err := app.NewService(repo, &seqIDs{}).Get(context.Background(), "t1")
	if err != nil {
		t.Fatal(err)
	}
	if got.IsDefault || got.Currency != domain.CurrencyUSD {
		t.Fatalf("esperaba la fila guardada, got %+v", got)
	}
}
