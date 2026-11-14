package app_test

import (
	"context"
	"errors"
	"testing"

	"prestamos/internal/loanproducts/app"
	"prestamos/internal/loanproducts/domain"
)

func TestDelete_IsSoftAndHidesProduct(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo()
	s := app.NewService(repo, &seqIDs{})
	p, err := s.Create(ctx, actor, newProduct("Temporal"))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(ctx, actor, p.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(ctx, actor.TenantID, p.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("un producto eliminado no debe obtenerse: %v", err)
	}
	list, err := s.List(ctx, actor.TenantID, domain.ListFilter{Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 0 {
		t.Fatalf("el listado no debe mostrar eliminados: %+v", list.Items)
	}
	if r := repo.rows[p.ID]; r == nil || !r.deleted {
		t.Fatal("la fila debe seguir existiendo, marcada como eliminada")
	}
}

func TestDelete_FreesNameForReuse(t *testing.T) {
	ctx := context.Background()
	s := app.NewService(newFakeRepo(), &seqIDs{})
	p, err := s.Create(ctx, actor, newProduct("Reutilizable"))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(ctx, actor, p.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Create(ctx, actor, newProduct("reutilizable")); err != nil {
		t.Fatalf("el nombre de un producto eliminado debe poder reutilizarse: %v", err)
	}
}

func TestDelete_TwiceIsNotFound(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo()
	s := app.NewService(repo, &seqIDs{})
	p, err := s.Create(ctx, actor, newProduct("Doble"))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(ctx, actor, p.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(ctx, actor, p.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("segundo delete debe ser ErrNotFound, got %v", err)
	}
	if n := len(repo.audits); n != 2 {
		t.Fatalf("esperaba 2 audits (create y delete), hubo %d", n)
	}
	if a := repo.audits[1]; a.Action != "product.delete" {
		t.Fatalf("audit de borrado inesperado: %+v", a)
	}
}

func TestDelete_OtherTenantIsNotFound(t *testing.T) {
	ctx := context.Background()
	s := app.NewService(newFakeRepo(), &seqIDs{})
	p, err := s.Create(ctx, actor, newProduct("Ajeno"))
	if err != nil {
		t.Fatal(err)
	}
	other := app.Actor{TenantID: "otra", UserID: "x"}
	if err := s.Delete(ctx, other, p.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("otra empresa no debe borrar el producto: %v", err)
	}
}
