package app_test

import (
	"context"
	"errors"
	"testing"

	"prestamos/internal/loanproducts/app"
	"prestamos/internal/loanproducts/domain"
)

var actor = app.Actor{TenantID: "t1", UserID: "u1"}

func newProduct(name string) domain.Product {
	return domain.Product{
		Name: name, Frequency: "monthly", RatePeriod: "annual", RateDefault: "24", RateMin: "20", RateMax: "30",
		AmountMin: "500", AmountMax: "5000", TermsMin: 1, TermsMax: 12, Active: true,
	}
}

func TestCreate_StoresNormalizedProduct(t *testing.T) {
	s := app.NewService(newFakeRepo(), &seqIDs{})
	p, err := s.Create(context.Background(), actor, domain.Product{
		Name: "  Mensual  ", Frequency: "monthly", RatePeriod: "annual", RateDefault: "24", RateMin: "20",
		RateMax: "30", AmountMin: "500", AmountMax: "5000", TermsMin: 1, TermsMax: 12, Active: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if p.ID == "" || p.TenantID != "t1" || p.Name != "Mensual" || p.InterestType != "flat" || p.FeeType != "none" {
		t.Fatalf("producto inesperado: %+v", p)
	}
}

func TestCreate_DuplicateNameIgnoresCase(t *testing.T) {
	s := app.NewService(newFakeRepo(), &seqIDs{})
	ctx := context.Background()
	if _, err := s.Create(ctx, actor, newProduct("Diario Pro")); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Create(ctx, actor, newProduct("diario pro")); !errors.Is(err, domain.ErrNameTaken) {
		t.Fatalf("esperaba ErrNameTaken, got %v", err)
	}
	other := app.Actor{TenantID: "t2", UserID: "u9"}
	if _, err := s.Create(ctx, other, newProduct("diario pro")); err != nil {
		t.Fatalf("otra empresa puede usar el mismo nombre: %v", err)
	}
}

func TestCreate_InvalidProductIsNotStored(t *testing.T) {
	repo := newFakeRepo()
	s := app.NewService(repo, &seqIDs{})
	bad := newProduct("Mal")
	bad.TermsMin, bad.TermsMax = 5, 3
	_, err := s.Create(context.Background(), actor, bad)
	var ve *domain.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("esperaba ValidationError, got %v", err)
	}
	if len(repo.rows) != 0 {
		t.Fatal("un producto inválido no debe guardarse")
	}
}

func TestCreate_WritesAuditRow(t *testing.T) {
	repo := newFakeRepo()
	s := app.NewService(repo, &seqIDs{})
	if _, err := s.Create(context.Background(), actor, newProduct("Auditado")); err != nil {
		t.Fatal(err)
	}
	if len(repo.audits) != 1 || repo.audits[0].Action != "product.create" || repo.audits[0].UserID != "u1" {
		t.Fatalf("audit inesperado: %+v", repo.audits)
	}
}

func TestGet_InvalidIDIsNotFound(t *testing.T) {
	s := app.NewService(newFakeRepo(), &seqIDs{})
	if _, err := s.Get(context.Background(), "t1", "no-es-uuid"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("esperaba ErrNotFound, got %v", err)
	}
}
