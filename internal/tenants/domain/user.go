package domain

import (
	"slices"
	"strings"
	"time"
)

// User es un usuario de una empresa. La contraseña nunca forma parte de la entidad.
type User struct {
	ID        string
	TenantID  string
	Email     string
	FullName  string
	Roles     []string
	Active    bool
	CreatedAt time.Time
}

// NewUser son los datos para crear un usuario (también sirve para el owner de una empresa nueva).
type NewUser struct {
	Email    string
	FullName string
	Password string
	Roles    []string
}

// IsSuperadmin indica si el usuario administra la plataforma.
func (u *User) IsSuperadmin() bool { return slices.Contains(u.Roles, RoleSuperadmin) }

// Trim quita espacios y pasa el correo a minúsculas.
func (u *NewUser) Trim() {
	u.Email = strings.ToLower(strings.TrimSpace(u.Email))
	u.FullName = strings.TrimSpace(u.FullName)
}

// ValidateAccount valida correo, nombre y contraseña (sin roles; lo usa el seed del superadmin).
func (u NewUser) ValidateAccount() error { return validationErr(u.fieldIssues("")) }

// Validate valida la cuenta y los roles asignables desde la API.
func (u NewUser) Validate() error {
	return validationErr(append(u.fieldIssues(""), checkRoles(u.Roles)...))
}

func (u NewUser) fieldIssues(prefix string) []Issue {
	issues := checkEmail(prefix+"email", u.Email)
	issues = append(issues, checkName(prefix+"full_name", u.FullName)...)
	return append(issues, checkPassword(prefix+"password", u.Password)...)
}

// UserPatch son los cambios parciales de un usuario (nil = sin cambio).
type UserPatch struct {
	FullName *string
	Roles    *[]string
	Active   *bool
	Password *string
}

// Normalize limpia y valida los campos presentes en el parche.
func (p *UserPatch) Normalize() error {
	var issues []Issue
	if p.FullName != nil {
		*p.FullName = strings.TrimSpace(*p.FullName)
		issues = append(issues, checkName("full_name", *p.FullName)...)
	}
	if p.Roles != nil {
		issues = append(issues, checkRoles(*p.Roles)...)
	}
	if p.Password != nil {
		issues = append(issues, checkPassword("password", *p.Password)...)
	}
	return validationErr(issues)
}

// UserFilter son los criterios de listado de usuarios de una empresa.
type UserFilter struct {
	Query     string // ILIKE sobre correo y nombre
	Active    *bool  // nil = todos
	WithTotal bool
	Limit     int
	Offset    int
}

// UserPage es una página de usuarios. Total es nil salvo que se haya pedido.
type UserPage struct {
	Items   []User
	HasMore bool
	Total   *int
}
