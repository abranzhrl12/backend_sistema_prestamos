package app_test

import (
	"context"
	"fmt"
	"strings"

	"prestamos/internal/tenants/app"
	"prestamos/internal/tenants/domain"
)

// fakeRepo respeta las reglas de la BD: el slug de empresa es único global (también entre eliminadas),
// el correo de usuario es único entre vivos de cada empresa, y el soft delete oculta las filas.
type fakeRepo struct {
	tenants map[string]*tenantRow
	users   map[string]*userRow
	tokens  map[string]tokenRow // refresh tokens: clave arbitraria -> dueño
	audits  []domain.AuditEntry
}

type tenantRow struct {
	t       domain.Tenant
	deleted bool
}

type userRow struct {
	u       domain.User
	hash    string
	deleted bool
}

type tokenRow struct{ tenantID, userID string }

func newFakeRepo() *fakeRepo {
	return &fakeRepo{tenants: map[string]*tenantRow{}, users: map[string]*userRow{}, tokens: map[string]tokenRow{}}
}

// addTenant siembra una empresa viva.
func (f *fakeRepo) addTenant(id, slug string, system bool) {
	f.tenants[id] = &tenantRow{t: domain.Tenant{ID: id, Slug: slug, Name: slug, Active: true, System: system}}
}

// addUser siembra un usuario activo y vivo con los roles dados.
func (f *fakeRepo) addUser(tenantID, id, email string, roles ...string) {
	f.users[id] = &userRow{u: domain.User{ID: id, TenantID: tenantID, Email: email, FullName: email,
		Roles: roles, Active: true}, hash: "hash"}
}

// addToken siembra una sesión (refresh token) de un usuario.
func (f *fakeRepo) addToken(tenantID, userID string) {
	f.tokens[fmt.Sprintf("tok-%d", len(f.tokens)+1)] = tokenRow{tenantID: tenantID, userID: userID}
}

func (f *fakeRepo) tokensOf(userID string) int {
	n := 0
	for _, t := range f.tokens {
		if t.userID == userID {
			n++
		}
	}
	return n
}

func (f *fakeRepo) tenantDeleted(id string) bool { return f.tenants[id].deleted }

func (f *fakeRepo) userDeleted(id string) bool { return f.users[id].deleted }

func (f *fakeRepo) auditActions() []string {
	out := make([]string, 0, len(f.audits))
	for _, a := range f.audits {
		out = append(out, a.Action)
	}
	return out
}

func (f *fakeRepo) liveTenant(id string) *tenantRow {
	if r, ok := f.tenants[id]; ok && !r.deleted {
		return r
	}
	return nil
}

func (f *fakeRepo) liveUserIn(tenantID, id string) *userRow {
	if r, ok := f.users[id]; ok && !r.deleted && r.u.TenantID == tenantID {
		return r
	}
	return nil
}

func (f *fakeRepo) emailTaken(tenantID, email, exceptID string) bool {
	for id, r := range f.users {
		if id != exceptID && !r.deleted && r.u.TenantID == tenantID && strings.EqualFold(r.u.Email, email) {
			return true
		}
	}
	return false
}

func (f *fakeRepo) Atomic(_ context.Context, fn func(domain.Repository) error) error { return fn(f) }

func (f *fakeRepo) InsertTenant(_ context.Context, t *domain.Tenant) error {
	for _, r := range f.tenants {
		if r.t.Slug == t.Slug {
			return domain.ErrSlugTaken
		}
	}
	f.tenants[t.ID] = &tenantRow{t: *t}
	return nil
}

func (f *fakeRepo) GetTenant(_ context.Context, id string) (*domain.Tenant, error) {
	r := f.liveTenant(id)
	if r == nil {
		return nil, domain.ErrTenantNotFound
	}
	t := r.t
	t.UserCount = 0
	for _, u := range f.users {
		if !u.deleted && u.u.TenantID == id {
			t.UserCount++
		}
	}
	return &t, nil
}

func (f *fakeRepo) FindTenantBySlug(_ context.Context, slug string) (*domain.Tenant, error) {
	for _, r := range f.tenants {
		if r.t.Slug == slug && !r.deleted {
			t := r.t
			return &t, nil
		}
	}
	return nil, domain.ErrTenantNotFound
}

func (f *fakeRepo) UpdateTenant(_ context.Context, t *domain.Tenant) error {
	r := f.liveTenant(t.ID)
	if r == nil {
		return domain.ErrTenantNotFound
	}
	r.t.Name, r.t.Active = t.Name, t.Active
	return nil
}

func (f *fakeRepo) ListTenants(context.Context, domain.TenantFilter) (domain.TenantPage, error) {
	return domain.TenantPage{}, nil
}

func (f *fakeRepo) SoftDeleteTenant(_ context.Context, id, _ string) error {
	r := f.liveTenant(id)
	if r == nil {
		return domain.ErrTenantNotFound
	}
	r.deleted = true
	return nil
}

func (f *fakeRepo) RestoreTenant(_ context.Context, id string) error {
	r, ok := f.tenants[id]
	if !ok || !r.deleted {
		return domain.ErrTenantNotFound
	}
	r.deleted = false
	return nil
}

func (f *fakeRepo) MarkSystem(_ context.Context, id string) error {
	r := f.liveTenant(id)
	if r == nil {
		return domain.ErrTenantNotFound
	}
	r.t.System = true
	return nil
}

func (f *fakeRepo) InsertUser(_ context.Context, u *domain.User, hash string) error {
	if f.emailTaken(u.TenantID, u.Email, "") {
		return domain.ErrEmailTaken
	}
	cp := *u
	f.users[u.ID] = &userRow{u: cp, hash: hash}
	return nil
}

func (f *fakeRepo) GetUser(_ context.Context, tenantID, id string) (*domain.User, error) {
	r := f.liveUserIn(tenantID, id)
	if r == nil {
		return nil, domain.ErrUserNotFound
	}
	u := r.u
	return &u, nil
}

func (f *fakeRepo) FindUserByEmail(_ context.Context, tenantID, email string) (*domain.User, error) {
	for id, r := range f.users {
		if !r.deleted && r.u.TenantID == tenantID && strings.EqualFold(r.u.Email, email) {
			u := f.users[id].u
			return &u, nil
		}
	}
	return nil, domain.ErrUserNotFound
}

func (f *fakeRepo) UpdateUser(_ context.Context, u *domain.User) error {
	r := f.liveUserIn(u.TenantID, u.ID)
	if r == nil {
		return domain.ErrUserNotFound
	}
	r.u.FullName, r.u.Roles, r.u.Active = u.FullName, u.Roles, u.Active
	return nil
}

func (f *fakeRepo) SetPasswordHash(_ context.Context, tenantID, userID, hash string) error {
	r := f.liveUserIn(tenantID, userID)
	if r == nil {
		return domain.ErrUserNotFound
	}
	r.hash = hash
	return nil
}

func (f *fakeRepo) ListUsers(context.Context, string, domain.UserFilter) (domain.UserPage, error) {
	return domain.UserPage{}, nil
}

func (f *fakeRepo) SoftDeleteUser(_ context.Context, tenantID, userID, _ string) error {
	r := f.liveUserIn(tenantID, userID)
	if r == nil {
		return domain.ErrUserNotFound
	}
	r.deleted = true
	return nil
}

func (f *fakeRepo) RestoreUser(_ context.Context, tenantID, userID string) error {
	r, ok := f.users[userID]
	if !ok || !r.deleted || r.u.TenantID != tenantID {
		return domain.ErrUserNotFound
	}
	if f.emailTaken(tenantID, r.u.Email, userID) {
		return domain.ErrEmailTaken
	}
	r.deleted = false
	return nil
}

func (f *fakeRepo) LockActiveSuperadmins(_ context.Context, tenantID string) ([]string, error) {
	var ids []string
	for id, r := range f.users {
		if !r.deleted && r.u.TenantID == tenantID && r.u.Active && r.u.IsSuperadmin() {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

func (f *fakeRepo) RevokeSessions(_ context.Context, tenantID, userID string) error {
	for k, t := range f.tokens {
		if t.tenantID == tenantID && t.userID == userID {
			delete(f.tokens, k)
		}
	}
	return nil
}

func (f *fakeRepo) RevokeTenantSessions(_ context.Context, tenantID string) error {
	for k, t := range f.tokens {
		if t.tenantID == tenantID {
			delete(f.tokens, k)
		}
	}
	return nil
}

func (f *fakeRepo) Audit(_ context.Context, e domain.AuditEntry) error {
	f.audits = append(f.audits, e)
	return nil
}

// fakeHasher no es seguro: solo sirve para comprobar que la contraseña se hashea y nunca se guarda en claro.
type fakeHasher struct{}

func (fakeHasher) Hash(pw string) (string, error) { return "hash:" + pw, nil }

// seqIDs genera IDs predecibles para los tests.
type seqIDs struct{ n int }

func (s *seqIDs) New() (string, error) { s.n++; return fmt.Sprintf("id-%d", s.n), nil }

// actor es el superadmin que ejecuta las operaciones de los tests.
const actor = "superadmin-1"

// newService cablea el servicio con los fakes.
func newService(repo *fakeRepo) *app.Service {
	return app.NewService(repo, fakeHasher{}, &seqIDs{})
}

// owner devuelve datos válidos de un usuario nuevo con la contraseña de prueba.
func owner(email string) domain.NewUser {
	return domain.NewUser{Email: email, FullName: "Ana Ruiz", Password: "Secreto12345"}
}
