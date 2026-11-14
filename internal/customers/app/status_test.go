package app_test

import (
	"context"
	"errors"
	"testing"

	"prestamos/internal/customers/app"
	"prestamos/internal/customers/domain"
)

func TestGet_OtherTenantCannotSeeCustomer(t *testing.T) {
	s := app.NewService(newFakeRepo(), &seqIDs{})
	ctx := context.Background()
	c, _ := s.Create(ctx, "t1", domain.Customer{FullName: "Ana", DocumentID: "1"})
	if _, err := s.Get(ctx, "t2", c.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("otro tenant no debe verlo, got %v", err)
	}
}

func TestSetStatus_RejectsInvalid(t *testing.T) {
	s := app.NewService(newFakeRepo(), &seqIDs{})
	ctx := context.Background()
	c, _ := s.Create(ctx, "t1", domain.Customer{FullName: "Ana", DocumentID: "1"})
	if _, err := s.SetStatus(ctx, "t1", c.ID, "zzz"); err == nil {
		t.Fatal("estado inválido debe fallar")
	}
}

func TestSetStatus_BlocksCustomer(t *testing.T) {
	s := app.NewService(newFakeRepo(), &seqIDs{})
	ctx := context.Background()
	c, _ := s.Create(ctx, "t1", domain.Customer{FullName: "Ana", DocumentID: "1"})
	got, err := s.SetStatus(ctx, "t1", c.ID, domain.StatusBlocked)
	if err != nil || got.Status != domain.StatusBlocked {
		t.Fatalf("bloqueo falló: %v %+v", err, got)
	}
}
