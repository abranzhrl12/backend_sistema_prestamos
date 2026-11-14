package app_test

import (
	"context"
	"errors"
	"testing"

	"prestamos/internal/tenants/domain"
)

func TestDeleteTenant_HidesTenantAndKeepsRow(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo()
	repo.addTenant("t1", "acme", false)
	svc := newService(repo)
	if err := svc.DeleteTenant(ctx, actor, "t1"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.GetTenant(ctx, "t1"); !errors.Is(err, domain.ErrTenantNotFound) {
		t.Fatalf("una empresa eliminada no debe obtenerse: %v", err)
	}
	if !repo.tenantDeleted("t1") {
		t.Fatal("la fila debe conservarse, marcada como eliminada")
	}
	if got := repo.auditActions(); len(got) != 1 || got[0] != "tenant.delete" {
		t.Fatalf("auditoría inesperada: %v", got)
	}
}

func TestDeleteTenant_RevokesOnlyItsOwnSessions(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo()
	repo.addTenant("t1", "acme", false)
	repo.addTenant("t2", "beta", false)
	repo.addUser("t1", "u1", "a@acme.com", "owner")
	repo.addUser("t1", "u2", "b@acme.com", "cobrador")
	repo.addUser("t2", "u3", "a@beta.com", "owner")
	repo.addToken("t1", "u1")
	repo.addToken("t1", "u2")
	repo.addToken("t2", "u3")
	if err := newService(repo).DeleteTenant(ctx, actor, "t1"); err != nil {
		t.Fatal(err)
	}
	if n := repo.tokensOf("u1") + repo.tokensOf("u2"); n != 0 {
		t.Fatalf("las sesiones de la empresa eliminada deben revocarse, quedan %d", n)
	}
	if repo.tokensOf("u3") != 1 {
		t.Fatal("las sesiones de otra empresa no deben tocarse")
	}
}

func TestDeleteTenant_TwiceIsNotFound(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo()
	repo.addTenant("t1", "acme", false)
	svc := newService(repo)
	if err := svc.DeleteTenant(ctx, actor, "t1"); err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteTenant(ctx, actor, "t1"); !errors.Is(err, domain.ErrTenantNotFound) {
		t.Fatalf("el segundo borrado debe ser ErrTenantNotFound, llegó %v", err)
	}
}

func TestDeleteTenant_SystemTenantIsProtected(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo()
	repo.addTenant("p", "platform", true)
	if err := newService(repo).DeleteTenant(ctx, actor, "p"); !errors.Is(err, domain.ErrPlatformProtected) {
		t.Fatalf("la empresa de sistema no se puede eliminar, llegó %v", err)
	}
	if repo.tenantDeleted("p") {
		t.Fatal("la empresa de sistema debe seguir viva")
	}
}

func TestDeleteTenant_SlugStaysReserved(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo()
	repo.addTenant("t1", "acme", false)
	svc := newService(repo)
	if err := svc.DeleteTenant(ctx, actor, "t1"); err != nil {
		t.Fatal(err)
	}
	in := domain.NewTenant{Slug: "acme", Name: "Acme nueva", Owner: owner("owner@nueva.com")}
	if _, _, err := svc.CreateTenant(ctx, actor, in); !errors.Is(err, domain.ErrSlugTaken) {
		t.Fatalf("el slug de una empresa eliminada no se reutiliza, llegó %v", err)
	}
}

func TestRestoreTenant_ReactivatesTenant(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo()
	repo.addTenant("t1", "acme", false)
	svc := newService(repo)
	if err := svc.DeleteTenant(ctx, actor, "t1"); err != nil {
		t.Fatal(err)
	}
	restored, err := svc.RestoreTenant(ctx, actor, "t1")
	if err != nil {
		t.Fatal(err)
	}
	if restored.ID != "t1" || repo.tenantDeleted("t1") {
		t.Fatal("la empresa debe volver a estar viva")
	}
	if got := repo.auditActions(); got[len(got)-1] != "tenant.restore" {
		t.Fatalf("debe auditarse tenant.restore: %v", got)
	}
}

func TestRestoreTenant_LiveTenantIsNotFound(t *testing.T) {
	repo := newFakeRepo()
	repo.addTenant("t1", "acme", false)
	if _, err := newService(repo).RestoreTenant(context.Background(), actor, "t1"); !errors.Is(err, domain.ErrTenantNotFound) {
		t.Fatalf("restaurar una empresa que no está eliminada debe ser ErrTenantNotFound, llegó %v", err)
	}
}
