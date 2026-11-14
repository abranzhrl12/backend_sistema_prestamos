package app_test

import (
	"context"
	"errors"
	"testing"

	"prestamos/internal/tenants/domain"
)

func TestDeleteUser_HidesUserRevokesSessionsAndAudits(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo()
	repo.addTenant("t1", "acme", false)
	repo.addUser("t1", "u1", "cobrador@acme.com", "cobrador")
	repo.addToken("t1", "u1")
	svc := newService(repo)
	if err := svc.DeleteUser(ctx, actor, "t1", "u1"); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.GetUser(ctx, "t1", "u1"); !errors.Is(err, domain.ErrUserNotFound) {
		t.Fatalf("un usuario eliminado no debe obtenerse: %v", err)
	}
	if !repo.userDeleted("u1") {
		t.Fatal("la fila debe conservarse, marcada como eliminada")
	}
	if repo.tokensOf("u1") != 0 {
		t.Fatal("eliminar al usuario debe revocar sus sesiones")
	}
	if got := repo.auditActions(); got[len(got)-1] != "user.delete" {
		t.Fatalf("debe auditarse user.delete: %v", got)
	}
}

func TestDeleteUser_TwiceIsNotFound(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo()
	repo.addTenant("t1", "acme", false)
	repo.addUser("t1", "u1", "cobrador@acme.com", "cobrador")
	svc := newService(repo)
	if err := svc.DeleteUser(ctx, actor, "t1", "u1"); err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteUser(ctx, actor, "t1", "u1"); !errors.Is(err, domain.ErrUserNotFound) {
		t.Fatalf("el segundo borrado debe ser ErrUserNotFound, llegó %v", err)
	}
}

func TestDeleteUser_OtherTenantIsNotFound(t *testing.T) {
	repo := newFakeRepo()
	repo.addTenant("t1", "acme", false)
	repo.addTenant("t2", "beta", false)
	repo.addUser("t1", "u1", "cobrador@acme.com", "cobrador")
	if err := newService(repo).DeleteUser(context.Background(), actor, "t2", "u1"); !errors.Is(err, domain.ErrUserNotFound) {
		t.Fatalf("un usuario de otra empresa no se puede borrar por esta ruta: %v", err)
	}
	if repo.userDeleted("u1") {
		t.Fatal("el usuario ajeno no debe quedar eliminado")
	}
}

func TestDeleteUser_RequiresLiveTenant(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo()
	repo.addTenant("t1", "acme", false)
	repo.addUser("t1", "u1", "cobrador@acme.com", "cobrador")
	svc := newService(repo)
	if err := svc.DeleteTenant(ctx, actor, "t1"); err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteUser(ctx, actor, "t1", "u1"); !errors.Is(err, domain.ErrTenantNotFound) {
		t.Fatalf("en una empresa eliminada no se puede borrar usuarios, llegó %v", err)
	}
}

func TestDeleteUser_LastActiveSuperadminIsRefused(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo()
	repo.addTenant("p", "platform", true)
	repo.addUser("p", "s1", "root@platform.com", "superadmin")
	repo.addToken("p", "s1")
	if err := newService(repo).DeleteUser(ctx, actor, "p", "s1"); !errors.Is(err, domain.ErrLastSuperadmin) {
		t.Fatalf("no se debe eliminar al último superadmin activo, llegó %v", err)
	}
	if repo.userDeleted("s1") || repo.tokensOf("s1") != 1 {
		t.Fatal("al rechazarse, no debe tocarse al superadmin ni sus sesiones")
	}
}

func TestDeleteUser_SuperadminWithPeerIsAllowed(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo()
	repo.addTenant("p", "platform", true)
	repo.addUser("p", "s1", "root@platform.com", "superadmin")
	repo.addUser("p", "s2", "ops@platform.com", "superadmin")
	if err := newService(repo).DeleteUser(ctx, actor, "p", "s1"); err != nil {
		t.Fatalf("hay otro superadmin activo, debe permitirse: %v", err)
	}
	if !repo.userDeleted("s1") || repo.userDeleted("s2") {
		t.Fatal("solo debe eliminarse el superadmin pedido")
	}
}

func TestDeleteUser_InactiveSuperadminDoesNotCountAsPeer(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo()
	repo.addTenant("p", "platform", true)
	repo.addUser("p", "s1", "root@platform.com", "superadmin")
	repo.addUser("p", "s2", "ops@platform.com", "superadmin")
	repo.users["s2"].u.Active = false
	svc := newService(repo)
	if err := svc.DeleteUser(ctx, actor, "p", "s2"); err != nil {
		t.Fatalf("eliminar un superadmin inactivo no deja a la plataforma sin admins: %v", err)
	}
	if err := svc.DeleteUser(ctx, actor, "p", "s1"); !errors.Is(err, domain.ErrLastSuperadmin) {
		t.Fatalf("s1 es el único superadmin activo y no se debe eliminar, llegó %v", err)
	}
}

func TestDeleteUser_FreesEmailForReuse(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo()
	repo.addTenant("t1", "acme", false)
	repo.addUser("t1", "u1", "cobrador@acme.com", "cobrador")
	svc := newService(repo)
	if err := svc.DeleteUser(ctx, actor, "t1", "u1"); err != nil {
		t.Fatal(err)
	}
	nuevo := domain.NewUser{Email: "cobrador@acme.com", FullName: "Otro", Password: "Secreto12345", Roles: []string{"cobrador"}}
	if _, err := svc.CreateUser(ctx, actor, "t1", nuevo); err != nil {
		t.Fatalf("el correo de un usuario eliminado debe poder reutilizarse: %v", err)
	}
}

func TestRestoreUser_ReactivatesUser(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo()
	repo.addTenant("t1", "acme", false)
	repo.addUser("t1", "u1", "cobrador@acme.com", "cobrador")
	svc := newService(repo)
	if err := svc.DeleteUser(ctx, actor, "t1", "u1"); err != nil {
		t.Fatal(err)
	}
	u, err := svc.RestoreUser(ctx, actor, "t1", "u1")
	if err != nil {
		t.Fatal(err)
	}
	if u.ID != "u1" || repo.userDeleted("u1") {
		t.Fatal("el usuario debe volver a estar vivo")
	}
}

func TestRestoreUser_EmailTakenIsRefused(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo()
	repo.addTenant("t1", "acme", false)
	repo.addUser("t1", "u1", "cobrador@acme.com", "cobrador")
	svc := newService(repo)
	if err := svc.DeleteUser(ctx, actor, "t1", "u1"); err != nil {
		t.Fatal(err)
	}
	nuevo := domain.NewUser{Email: "cobrador@acme.com", FullName: "Otro", Password: "Secreto12345", Roles: []string{"cobrador"}}
	if _, err := svc.CreateUser(ctx, actor, "t1", nuevo); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RestoreUser(ctx, actor, "t1", "u1"); !errors.Is(err, domain.ErrEmailTaken) {
		t.Fatalf("restaurar con un correo ya ocupado debe ser ErrEmailTaken, llegó %v", err)
	}
}
