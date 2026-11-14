package app_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"prestamos/internal/customers/app"
	"prestamos/internal/customers/domain"
)

const tenantA, actorA = "tenant-a", "user-a"

// uuidIDs genera UUID predecibles: el servicio valida que el id sea UUID antes de borrar.
type uuidIDs struct{ n int }

func (u *uuidIDs) New() (string, error) {
	u.n++
	return fmt.Sprintf("00000000-0000-4000-8000-%012d", u.n), nil
}

func newCustomer(doc string) domain.Customer {
	return domain.Customer{FullName: "Cliente " + doc, DocumentID: doc}
}

func TestDelete_HidesCustomerAndAuditsIt(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo()
	s := app.NewService(repo, &uuidIDs{})
	c, err := s.Create(ctx, tenantA, newCustomer("100"))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(ctx, tenantA, actorA, c.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(ctx, tenantA, c.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("un cliente eliminado no debe obtenerse: %v", err)
	}
	if !repo.deleted[c.ID] {
		t.Fatal("la fila debe seguir existiendo, marcada como eliminada")
	}
	if len(repo.audits) != 1 || repo.audits[0].ActorID != actorA || repo.audits[0].AuditID == "" {
		t.Fatalf("debe registrarse una auditoría con el actor: %+v", repo.audits)
	}
}

func TestDelete_TwiceIsNotFound(t *testing.T) {
	ctx := context.Background()
	s := app.NewService(newFakeRepo(), &uuidIDs{})
	c, err := s.Create(ctx, tenantA, newCustomer("200"))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(ctx, tenantA, actorA, c.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(ctx, tenantA, actorA, c.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("el segundo borrado debe ser ErrNotFound, llegó %v", err)
	}
}

func TestDelete_OtherTenantIsNotFound(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo()
	s := app.NewService(repo, &uuidIDs{})
	c, err := s.Create(ctx, tenantA, newCustomer("300"))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(ctx, "tenant-b", actorA, c.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("otra empresa no debe borrar el cliente: %v", err)
	}
	if repo.deleted[c.ID] {
		t.Fatal("el cliente ajeno no debe quedar marcado como eliminado")
	}
}

func TestDelete_InvalidIDIsNotFound(t *testing.T) {
	s := app.NewService(newFakeRepo(), &uuidIDs{})
	if err := s.Delete(context.Background(), tenantA, actorA, "no-es-uuid"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("un id que no es UUID debe ser ErrNotFound, llegó %v", err)
	}
}

func TestDelete_FreesDocumentForReuse(t *testing.T) {
	ctx := context.Background()
	s := app.NewService(newFakeRepo(), &uuidIDs{})
	c, err := s.Create(ctx, tenantA, newCustomer("400"))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(ctx, tenantA, actorA, c.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Create(ctx, tenantA, newCustomer("400")); err != nil {
		t.Fatalf("el documento de un cliente eliminado debe poder reutilizarse: %v", err)
	}
}
