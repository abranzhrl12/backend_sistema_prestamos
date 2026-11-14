package migrate

import "regexp"

var roleRe = regexp.MustCompile(`^[a-z_][a-z0-9_]{0,62}$`)

// ValidRole indica si el nombre es seguro para usarlo como identificador de rol en SQL.
func ValidRole(role string) bool { return roleRe.MatchString(role) }
