package domain

import "context"

// PasswordHasher hashea contraseñas (lo satisface authkit.PasswordHasher).
type PasswordHasher interface {
	Hash(password string) (string, error)
}

// Repository es el puerto de persistencia. Las lecturas y escrituras de empresas y usuarios
// ignoran las filas eliminadas, salvo RestoreTenant y RestoreUser. Atomic ejecuta fn en una
// transacción: las operaciones recibidas dentro de fn se confirman o revierten juntas.
type Repository interface {
	Atomic(ctx context.Context, fn func(Repository) error) error

	InsertTenant(ctx context.Context, t *Tenant) error // ErrSlugTaken
	GetTenant(ctx context.Context, id string) (*Tenant, error)
	FindTenantBySlug(ctx context.Context, slug string) (*Tenant, error)
	UpdateTenant(ctx context.Context, t *Tenant) error // solo name y active
	ListTenants(ctx context.Context, f TenantFilter) (TenantPage, error)
	SoftDeleteTenant(ctx context.Context, id, actorID string) error // ErrTenantNotFound si no está viva
	RestoreTenant(ctx context.Context, id string) error             // ErrTenantNotFound si no está eliminada
	MarkSystem(ctx context.Context, id string) error

	InsertUser(ctx context.Context, u *User, passwordHash string) error // ErrEmailTaken
	GetUser(ctx context.Context, tenantID, id string) (*User, error)
	FindUserByEmail(ctx context.Context, tenantID, email string) (*User, error)
	UpdateUser(ctx context.Context, u *User) error // solo full_name, roles y active
	SetPasswordHash(ctx context.Context, tenantID, userID, hash string) error
	ListUsers(ctx context.Context, tenantID string, f UserFilter) (UserPage, error)
	SoftDeleteUser(ctx context.Context, tenantID, userID, actorID string) error // ErrUserNotFound si no está vivo
	RestoreUser(ctx context.Context, tenantID, userID string) error             // ErrEmailTaken si el correo ya lo usa otro vivo
	// LockActiveSuperadmins devuelve los ids de los superadmins activos y vivos, bloqueando sus filas
	// (FOR UPDATE) hasta el fin de la transacción: llamarlo dentro de Atomic.
	LockActiveSuperadmins(ctx context.Context, tenantID string) ([]string, error)

	// RevokeSessions borra los refresh tokens de un usuario; RevokeTenantSessions los de toda la empresa.
	RevokeSessions(ctx context.Context, tenantID, userID string) error
	RevokeTenantSessions(ctx context.Context, tenantID string) error

	Audit(ctx context.Context, e AuditEntry) error
}

// AuditEntry es una fila de audit_log. ActorID vacío = proceso del sistema (seed).
type AuditEntry struct {
	ID       string
	TenantID string // empresa afectada
	ActorID  string // superadmin que actuó
	Action   string
	Entity   string
	EntityID string
	Data     map[string]any // solo nombres de campos y valores no sensibles
}
