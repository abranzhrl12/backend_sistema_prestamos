package domain

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	PlatformSlug   = "platform"
	RoleOwner      = "owner"
	RoleSuperadmin = "superadmin"

	maxEmailLen    = 254
	maxNameLen     = 120
	minPasswordLen = 8
	maxPasswordLen = 128
)

var (
	slugRe        = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,38}[a-z0-9]$`)
	reservedSlugs = map[string]bool{"platform": true, "admin": true, "api": true, "www": true,
		"app": true, "root": true, "system": true}
	assignableRoles = map[string]bool{RoleOwner: true, "admin": true, "cobrador": true, "viewer": true}
)

// checkSlug valida el identificador público de la empresa (ya en minúsculas).
func checkSlug(slug string) []Issue {
	switch {
	case !slugRe.MatchString(slug):
		return []Issue{{"slug", "Usa de 3 a 40 caracteres: minúsculas, números y guiones (sin guion al inicio ni al final)"}}
	case reservedSlugs[slug]:
		return []Issue{{"slug", "Ese identificador está reservado"}}
	}
	return nil
}

func checkName(field, name string) []Issue {
	switch n := utf8.RuneCountInString(name); {
	case n == 0:
		return []Issue{{field, "El nombre es obligatorio"}}
	case n > maxNameLen:
		return []Issue{{field, "El nombre no puede superar 120 caracteres"}}
	}
	return nil
}

func checkEmail(field, email string) []Issue {
	at := strings.LastIndex(email, "@")
	switch {
	case email == "":
		return []Issue{{field, "El correo es obligatorio"}}
	case len(email) > maxEmailLen:
		return []Issue{{field, "El correo no puede superar 254 caracteres"}}
	case at <= 0 || at == len(email)-1:
		return []Issue{{field, "El correo no es válido"}}
	}
	return nil
}

// checkPassword valida las claves de alta o restablecimiento administradas.
func checkPassword(field, pw string) []Issue {
	if n := utf8.RuneCountInString(pw); n < minPasswordLen || n > maxPasswordLen {
		return []Issue{{field, "La contraseña debe tener entre 8 y 128 caracteres"}}
	}
	var letter, digit bool
	for _, r := range pw {
		letter = letter || unicode.IsLetter(r)
		digit = digit || unicode.IsDigit(r)
	}
	if !letter || !digit {
		return []Issue{{field, "La contraseña debe incluir al menos una letra y un número"}}
	}
	return nil
}

// checkRoles exige roles asignables (sin superadmin), no vacíos y sin duplicados.
func checkRoles(roles []string) []Issue {
	if len(roles) == 0 {
		return []Issue{{"roles", "Debes asignar al menos un rol"}}
	}
	var issues []Issue
	seen := map[string]bool{}
	for _, r := range roles {
		switch {
		case r == RoleSuperadmin:
			issues = append(issues, Issue{"roles", "El rol superadmin no se puede asignar desde la API"})
		case !assignableRoles[r]:
			issues = append(issues, Issue{"roles", "Rol inválido: " + r + " (usa owner, admin, cobrador o viewer)"})
		case seen[r]:
			issues = append(issues, Issue{"roles", "Rol duplicado: " + r})
		}
		seen[r] = true
	}
	return issues
}
