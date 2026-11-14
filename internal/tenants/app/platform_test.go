package app_test

import (
	"context"
	"testing"

	"prestamos/internal/tenants/domain"
)

const (
	rootEmail    = "root@platform.com"
	rootPassword = "Secreto12345"
)

func TestEnsurePlatformAdmin_CreatesSystemTenantWithSuperadmin(t *testing.T) {
	repo := newFakeRepo()
	created, err := newService(repo).EnsurePlatformAdmin(context.Background(), rootEmail, "Root", rootPassword)
	if err != nil {
		t.Fatal(err)
	}
	if !created {
		t.Fatal("la primera ejecución debe crear el superadmin")
	}
	plat, err := repo.FindTenantBySlug(context.Background(), domain.PlatformSlug)
	if err != nil {
		t.Fatalf("la empresa platform debe existir: %v", err)
	}
	if !plat.System {
		t.Fatal("la empresa platform debe marcarse como de sistema (is_system)")
	}
	admin, err := repo.FindUserByEmail(context.Background(), plat.ID, rootEmail)
	if err != nil {
		t.Fatalf("el superadmin debe existir: %v", err)
	}
	if !admin.IsSuperadmin() {
		t.Fatalf("el usuario sembrado debe ser superadmin: %v", admin.Roles)
	}
}

func TestEnsurePlatformAdmin_IsIdempotent(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo()
	svc := newService(repo)
	if _, err := svc.EnsurePlatformAdmin(ctx, rootEmail, "Root", rootPassword); err != nil {
		t.Fatal(err)
	}
	auditsAfterFirst := len(repo.audits)
	created, err := svc.EnsurePlatformAdmin(ctx, rootEmail, "Root", rootPassword)
	if err != nil {
		t.Fatal(err)
	}
	if created {
		t.Fatal("la segunda ejecución no debe crear nada")
	}
	if len(repo.tenants) != 1 || len(repo.users) != 1 || len(repo.audits) != auditsAfterFirst {
		t.Fatalf("la segunda ejecución no debe modificar datos: tenants=%d users=%d audits=%d",
			len(repo.tenants), len(repo.users), len(repo.audits))
	}
}

func TestEnsurePlatformAdmin_MarksExistingPlatformAsSystem(t *testing.T) {
	repo := newFakeRepo()
	repo.addTenant("p", domain.PlatformSlug, false)
	created, err := newService(repo).EnsurePlatformAdmin(context.Background(), rootEmail, "Root", rootPassword)
	if err != nil {
		t.Fatal(err)
	}
	if !created {
		t.Fatal("el superadmin no existía y debe crearse")
	}
	if !repo.tenants["p"].t.System {
		t.Fatal("una plataforma preexistente debe quedar marcada como de sistema")
	}
	if got := repo.auditActions(); got[0] != "tenant.update" {
		t.Fatalf("el cambio de is_system debe auditarse: %v", got)
	}
}

func TestEnsurePlatformAdmin_RejectsWeakPassword(t *testing.T) {
	repo := newFakeRepo()
	if _, err := newService(repo).EnsurePlatformAdmin(context.Background(), rootEmail, "Root", "corta"); err == nil {
		t.Fatal("una contraseña débil debe rechazarse")
	}
	if len(repo.tenants) != 0 || len(repo.users) != 0 {
		t.Fatal("con una contraseña inválida no debe crearse nada")
	}
}
