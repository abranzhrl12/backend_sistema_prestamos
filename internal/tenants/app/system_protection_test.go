package app_test

import (
	"context"
	"errors"
	"testing"

	"prestamos/internal/tenants/domain"
)

func TestUpdateTenant_DeactivatingSystemTenantIsRefused(t *testing.T) {
	repo := newFakeRepo()
	repo.addTenant("p", domain.PlatformSlug, true)
	inactive := false
	_, err := newService(repo).UpdateTenant(context.Background(), actor, "p", domain.TenantPatch{Active: &inactive})
	if !errors.Is(err, domain.ErrPlatformProtected) {
		t.Fatalf("suspender la empresa de sistema debe ser ErrPlatformProtected, llegó %v", err)
	}
	if !repo.tenants["p"].t.Active {
		t.Fatal("la empresa de sistema debe seguir activa")
	}
}

func TestCreateUser_SystemTenantIsRefused(t *testing.T) {
	repo := newFakeRepo()
	repo.addTenant("p", domain.PlatformSlug, true)
	nuevo := domain.NewUser{Email: "x@platform.com", FullName: "X", Password: "Secreto12345", Roles: []string{"cobrador"}}
	if _, err := newService(repo).CreateUser(context.Background(), actor, "p", nuevo); !errors.Is(err, domain.ErrPlatformProtected) {
		t.Fatalf("no se deben crear usuarios en la empresa de sistema, llegó %v", err)
	}
}

func TestUpdateTenant_SystemTenantNameCanChange(t *testing.T) {
	repo := newFakeRepo()
	repo.addTenant("p", domain.PlatformSlug, true)
	name := "Plataforma Central"
	got, err := newService(repo).UpdateTenant(context.Background(), actor, "p", domain.TenantPatch{Name: &name})
	if err != nil {
		t.Fatalf("renombrar la empresa de sistema sí está permitido: %v", err)
	}
	if got.Name != name || !got.System {
		t.Fatalf("resultado inesperado: %+v", got)
	}
}
