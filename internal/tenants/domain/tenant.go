package domain

import (
	"strings"
	"time"
)

// Tenant es una empresa cliente de la plataforma.
type Tenant struct {
	ID        string
	Slug      string
	Name      string
	Active    bool
	System    bool // empresa de sistema (la plataforma): no se elimina ni se suspende
	UserCount int  // solo lo llena GetTenant
	CreatedAt time.Time
}

// NewTenant son los datos para crear una empresa con su primer usuario (owner).
type NewTenant struct {
	Slug  string
	Name  string
	Owner NewUser
}

// Normalize limpia los datos y valida las invariantes; reporta todos los campos con error.
func (in *NewTenant) Normalize() error {
	in.Slug = strings.ToLower(strings.TrimSpace(in.Slug))
	in.Name = strings.TrimSpace(in.Name)
	in.Owner.Trim()
	issues := append(checkSlug(in.Slug), checkName("name", in.Name)...)
	issues = append(issues, in.Owner.fieldIssues("owner.")...)
	return validationErr(issues)
}

// TenantPatch son los cambios parciales de una empresa (nil = sin cambio).
type TenantPatch struct {
	Name   *string
	Active *bool
}

// Normalize limpia y valida el nombre si viene en el parche.
func (p *TenantPatch) Normalize() error {
	if p.Name == nil {
		return nil
	}
	*p.Name = strings.TrimSpace(*p.Name)
	return validationErr(checkName("name", *p.Name))
}

// TenantFilter son los criterios de listado de empresas.
type TenantFilter struct {
	Query     string // ILIKE sobre slug y nombre
	Active    *bool  // nil = todas
	WithTotal bool
	Limit     int
	Offset    int
}

// TenantPage es una página de empresas. Total es nil salvo que se haya pedido.
type TenantPage struct {
	Items   []Tenant
	HasMore bool
	Total   *int
}
