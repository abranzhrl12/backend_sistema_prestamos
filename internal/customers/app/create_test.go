package app_test

import (
	"context"
	"errors"
	"testing"

	"prestamos/internal/customers/app"
	"prestamos/internal/customers/domain"
)

func TestCreate_RequiresName(t *testing.T) {
	s := app.NewService(newFakeRepo(), &seqIDs{})
	var ve *domain.ValidationError
	if _, err := s.Create(context.Background(), "t1", domain.Customer{DocumentID: "1"}); !errors.As(err, &ve) {
		t.Fatalf("esperaba ValidationError por nombre vacío, got %v", err)
	}
}

func TestCreate_DefaultsActiveAndTrims(t *testing.T) {
	s := app.NewService(newFakeRepo(), &seqIDs{})
	c, err := s.Create(context.Background(), "t1", domain.Customer{FullName: " Ana ", DocumentID: "123", Address: "Av. Lima 1"})
	if err != nil {
		t.Fatal(err)
	}
	if c.Status != domain.StatusActive || c.FullName != "Ana" || c.Address != "Av. Lima 1" {
		t.Fatalf("cliente inesperado: %+v", c)
	}
}

func TestCreate_RejectsDuplicateDocumentInSameTenant(t *testing.T) {
	s := app.NewService(newFakeRepo(), &seqIDs{})
	ctx := context.Background()
	if _, err := s.Create(ctx, "t1", domain.Customer{FullName: "Ana", DocumentID: "123"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Create(ctx, "t1", domain.Customer{FullName: "Otra", DocumentID: "123"}); !errors.Is(err, domain.ErrDuplicateDocument) {
		t.Fatalf("esperaba duplicado, got %v", err)
	}
	if _, err := s.Create(ctx, "t2", domain.Customer{FullName: "Otra", DocumentID: "123"}); err != nil {
		t.Fatalf("otro tenant puede repetir documento: %v", err)
	}
}
