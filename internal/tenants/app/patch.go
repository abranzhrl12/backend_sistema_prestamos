package app

import "prestamos/internal/tenants/domain"

// applyTenantPatch aplica los cambios reales a t y devuelve los campos modificados (sin valores sensibles).
func applyTenantPatch(t *domain.Tenant, p domain.TenantPatch) ([]string, map[string]any) {
	var fields []string
	values := map[string]any{}
	if p.Name != nil && *p.Name != t.Name {
		t.Name = *p.Name
		fields = append(fields, "name")
		values["name"] = t.Name
	}
	if p.Active != nil && *p.Active != t.Active {
		t.Active = *p.Active
		fields = append(fields, "active")
		values["active"] = t.Active
	}
	return fields, values
}

// applyUserPatch aplica nombre, roles y estado a u. La contraseña no pasa por aquí (no se audita su valor).
func applyUserPatch(u *domain.User, p domain.UserPatch) ([]string, map[string]any) {
	var fields []string
	values := map[string]any{}
	if p.FullName != nil && *p.FullName != u.FullName {
		u.FullName = *p.FullName
		fields = append(fields, "full_name")
		values["full_name"] = u.FullName
	}
	if p.Roles != nil {
		u.Roles = *p.Roles
		fields = append(fields, "roles")
		values["roles"] = u.Roles
	}
	if p.Active != nil && *p.Active != u.Active {
		u.Active = *p.Active
		fields = append(fields, "active")
		values["active"] = u.Active
	}
	return fields, values
}
