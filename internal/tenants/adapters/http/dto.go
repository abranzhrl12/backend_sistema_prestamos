// Package http expone la administración de empresas y usuarios (solo superadmin) por REST (Fiber).
package http

import (
	"time"

	"prestamos/internal/tenants/domain"
)

// TenantSummary es una empresa en listados. System = empresa de sistema (no se elimina ni se suspende).
type TenantSummary struct {
	ID        string    `json:"id"`
	Slug      string    `json:"slug" example:"acme"`
	Name      string    `json:"name" example:"Acme S.A.C."`
	Active    bool      `json:"active"`
	System    bool      `json:"system"`
	CreatedAt time.Time `json:"created_at"`
}

// TenantDetail es una empresa con su número de usuarios.
type TenantDetail struct {
	TenantSummary
	UserCount int `json:"user_count"`
}

// UserSummary es un usuario de una empresa. Nunca incluye la contraseña ni su hash.
type UserSummary struct {
	ID        string    `json:"id"`
	Email     string    `json:"email" example:"owner@acme.com"`
	FullName  string    `json:"full_name" example:"Ana Ruiz"`
	Roles     []string  `json:"roles" example:"owner"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"created_at"`
}

// OwnerRequest son los datos del primer usuario (owner) de la empresa nueva.
type OwnerRequest struct {
	Email    string `json:"email" example:"owner@acme.com"`
	FullName string `json:"full_name" example:"Ana Ruiz"`
	Password string `json:"password" example:"Secreto12345"`
}

// CreateTenantRequest crea la empresa y su owner.
type CreateTenantRequest struct {
	Slug  string       `json:"slug" example:"acme"`
	Name  string       `json:"name" example:"Acme S.A.C."`
	Owner OwnerRequest `json:"owner"`
}

// CreateTenantResponse es la empresa creada con su owner.
type CreateTenantResponse struct {
	TenantDetail
	Owner             UserSummary `json:"owner"`
	TemporaryPassword string      `json:"temporary_password,omitempty"`
}

// UpdateTenantRequest cambia campos opcionales de la empresa.
type UpdateTenantRequest struct {
	Name   *string `json:"name,omitempty" example:"Acme Holding"`
	Active *bool   `json:"active,omitempty"`
}

// CreateUserRequest crea un usuario en la empresa.
type CreateUserRequest struct {
	Email    string   `json:"email" example:"cobrador@acme.com"`
	FullName string   `json:"full_name" example:"Luis Torres"`
	Password string   `json:"password" example:"Secreto12345"`
	Roles    []string `json:"roles" enums:"owner,admin,cobrador,viewer"`
}

// UpdateUserRequest cambia campos opcionales del usuario. Cambiar password o desactivar cierra sus sesiones.
type UpdateUserRequest struct {
	FullName *string   `json:"full_name,omitempty"`
	Roles    *[]string `json:"roles,omitempty"`
	Active   *bool     `json:"active,omitempty"`
	Password *string   `json:"password,omitempty"`
}

func tenantSummary(t *domain.Tenant) TenantSummary {
	return TenantSummary{ID: t.ID, Slug: t.Slug, Name: t.Name, Active: t.Active, System: t.System, CreatedAt: t.CreatedAt}
}

func tenantDetail(t *domain.Tenant) TenantDetail {
	return TenantDetail{TenantSummary: tenantSummary(t), UserCount: t.UserCount}
}

func userSummary(u *domain.User) UserSummary {
	roles := u.Roles
	if roles == nil {
		roles = []string{}
	}
	return UserSummary{ID: u.ID, Email: u.Email, FullName: u.FullName, Roles: roles, Active: u.Active, CreatedAt: u.CreatedAt}
}
