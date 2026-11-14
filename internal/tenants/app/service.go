// Package app contiene los casos de uso de administración de empresas y usuarios (solo superadmin).
package app

import (
	"prestamos/internal/platform/ids"
	"prestamos/internal/tenants/domain"
)

// PasswordHasher lo satisface authkit.PasswordHasher.
type PasswordHasher = domain.PasswordHasher

type Service struct {
	repo   domain.Repository
	hasher PasswordHasher
	ids    ids.Generator
}

func NewService(repo domain.Repository, hasher PasswordHasher, ids ids.Generator) *Service {
	return &Service{repo: repo, hasher: hasher, ids: ids}
}
