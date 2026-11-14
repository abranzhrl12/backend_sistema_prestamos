//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
)

// rootEmail es el superadmin de la empresa de sistema "platform" (sembrado en main_test.go).
const rootEmail = "root@platform.com"

var adminSeq atomic.Int64

type tenantDTO struct {
	ID        string `json:"id"`
	Slug      string `json:"slug"`
	Name      string `json:"name"`
	Active    bool   `json:"active"`
	System    bool   `json:"system"`
	UserCount int    `json:"user_count"`
}

type userDTO struct {
	ID       string   `json:"id"`
	Email    string   `json:"email"`
	FullName string   `json:"full_name"`
	Roles    []string `json:"roles"`
	Active   bool     `json:"active"`
}

// superToken inicia sesión como superadmin de la plataforma.
func superToken(t *testing.T) string { return loginBearer(t, "platform", rootEmail) }

// loginPair inicia sesión (transporte bearer) y devuelve el access y el refresh token.
func loginPair(t *testing.T, tenant, email string) (string, string) {
	t.Helper()
	r := do(t, "POST", "/auth/login", map[string]string{"tenant": tenant, "email": email, "password": pass})
	expectStatus(t, r, 200)
	var out struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	r.decode(t, &out)
	return out.AccessToken, out.RefreshToken
}

// uniqueSlug devuelve un slug válido y nuevo para crear empresas por la API.
func uniqueSlug(prefix string) string { return fmt.Sprintf("%s%d", prefix, adminSeq.Add(1)) }

// adminCreateTenant crea una empresa con su owner por la API (espera 201).
func adminCreateTenant(t *testing.T, slug string) tenantDTO {
	t.Helper()
	body := map[string]any{"slug": slug, "name": "Empresa " + slug,
		"owner": map[string]string{"email": "owner@" + slug + ".com", "full_name": "Owner " + slug, "password": pass}}
	r := do(t, "POST", "/admin/tenants", body, bearer(superToken(t)))
	expectStatus(t, r, 201)
	var out tenantDTO
	r.decode(t, &out)
	return out
}

// adminCreateUser crea un cobrador en la empresa por la API (espera 201).
func adminCreateUser(t *testing.T, tenantID, email string) userDTO {
	t.Helper()
	body := map[string]any{"email": email, "full_name": "Usuario", "password": pass, "roles": []string{"cobrador"}}
	r := do(t, "POST", "/admin/tenants/"+tenantID+"/users", body, bearer(superToken(t)))
	expectStatus(t, r, 201)
	var out userDTO
	r.decode(t, &out)
	return out
}

// tenantIDBySlug lee el id de una empresa directamente de la BD (superusuario).
func tenantIDBySlug(t *testing.T, slug string) string {
	t.Helper()
	var id string
	if err := env.Admin.QueryRow(context.Background(), `SELECT id::text FROM tenants WHERE slug=$1`, slug).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

// deletedAt indica si la fila tiene marca de borrado lógico (y que la fila sigue existiendo).
func deletedAt(t *testing.T, table, id string) bool {
	t.Helper()
	var deleted bool
	q := fmt.Sprintf(`SELECT deleted_at IS NOT NULL FROM %s WHERE id=$1::uuid`, table)
	if err := env.Admin.QueryRow(context.Background(), q, id).Scan(&deleted); err != nil {
		t.Fatalf("la fila de %s debe seguir existiendo: %v", table, err)
	}
	return deleted
}

func TestAdmin_CreateTenantReturnsOwnerWhoCanLogin(t *testing.T) {
	slug := uniqueSlug("adm")
	created := adminCreateTenant(t, slug)
	if created.System || !created.Active {
		t.Fatalf("empresa nueva: system=%v active=%v, esperaba false y true", created.System, created.Active)
	}
	loginBearer(t, slug, "owner@"+slug+".com")
}

func TestAdmin_ValidationReportsFields(t *testing.T) {
	body := map[string]any{"slug": "A", "name": "X",
		"owner": map[string]string{"email": "no-es-correo", "full_name": "Ana", "password": "corta"}}
	r := do(t, "POST", "/admin/tenants", body, bearer(superToken(t)))
	expectStatus(t, r, 400)
	e := r.apiErr(t)
	if e.Error.Code != "VALIDATION_ERROR" {
		t.Fatalf("código %q, esperaba VALIDATION_ERROR", e.Error.Code)
	}
	fields := map[string]bool{}
	for _, f := range e.Error.Fields {
		fields[f.Field] = true
	}
	for _, want := range []string{"slug", "owner.email", "owner.password"} {
		if !fields[want] {
			t.Fatalf("falta el campo %q en fields: %+v", want, e.Error.Fields)
		}
	}
}

func TestAdmin_RequiresSuperadmin(t *testing.T) {
	f := newFixture(t)
	if r := do(t, "GET", "/admin/tenants", nil); r.Status != 401 {
		t.Fatalf("sin token: status %d, esperaba 401", r.Status)
	}
	owner := bearer(f.ownerToken(t))
	expectStatus(t, do(t, "GET", "/admin/tenants", nil, owner), 403)
	expectStatus(t, do(t, "DELETE", "/admin/tenants/"+f.TenantID, nil, owner), 403)
	expectStatus(t, do(t, "DELETE", "/admin/tenants/"+f.TenantID+"/users/x", nil, owner), 403)
}

func TestAdmin_InvalidIDIsNotFound(t *testing.T) {
	r := do(t, "DELETE", "/admin/tenants/no-es-uuid", nil, bearer(superToken(t)))
	expectStatus(t, r, 404)
	if e := r.apiErr(t); e.Error.Code != "NOT_FOUND" {
		t.Fatalf("código %q, esperaba NOT_FOUND", e.Error.Code)
	}
}

func TestAdmin_DeleteTenantBlocksLoginAndRefresh(t *testing.T) {
	slug := uniqueSlug("del")
	created := adminCreateTenant(t, slug)
	ownerEmail := "owner@" + slug + ".com"
	_, refresh := loginPair(t, slug, ownerEmail)
	actor := bearer(superToken(t))

	expectStatus(t, do(t, "DELETE", "/admin/tenants/"+created.ID, nil, actor), 204)

	r := do(t, "POST", "/auth/login", map[string]string{"tenant": slug, "email": ownerEmail, "password": pass})
	expectStatus(t, r, 401)
	if e := r.apiErr(t); e.Error.Code != "INVALID_CREDENTIALS" {
		t.Fatalf("login tras eliminar: código %q, esperaba INVALID_CREDENTIALS", e.Error.Code)
	}
	expectStatus(t, do(t, "POST", "/auth/refresh", map[string]string{"refresh_token": refresh}), 401)
	expectStatus(t, do(t, "GET", "/admin/tenants/"+created.ID, nil, actor), 404)
	expectStatus(t, do(t, "DELETE", "/admin/tenants/"+created.ID, nil, actor), 404)
	if !deletedAt(t, "tenants", created.ID) {
		t.Fatal("la empresa debe conservarse con deleted_at")
	}
}

func TestAdmin_RestoreTenantUnblocksLogin(t *testing.T) {
	slug := uniqueSlug("rst")
	created := adminCreateTenant(t, slug)
	actor := bearer(superToken(t))
	expectStatus(t, do(t, "DELETE", "/admin/tenants/"+created.ID, nil, actor), 204)
	expectStatus(t, do(t, "POST", "/admin/tenants/"+created.ID+"/restore", nil, actor), 200)
	loginBearer(t, slug, "owner@"+slug+".com")
}

func TestAdmin_RestoreLiveTenantIsNotFound(t *testing.T) {
	created := adminCreateTenant(t, uniqueSlug("liv"))
	expectStatus(t, do(t, "POST", "/admin/tenants/"+created.ID+"/restore", nil, bearer(superToken(t))), 404)
}

func TestAdmin_DeletedTenantSlugIsNotReusable(t *testing.T) {
	slug := uniqueSlug("slg")
	created := adminCreateTenant(t, slug)
	actor := bearer(superToken(t))
	expectStatus(t, do(t, "DELETE", "/admin/tenants/"+created.ID, nil, actor), 204)

	body := map[string]any{"slug": slug, "name": "Otra",
		"owner": map[string]string{"email": "nuevo@" + slug + ".com", "full_name": "Nuevo", "password": pass}}
	r := do(t, "POST", "/admin/tenants", body, actor)
	expectStatus(t, r, 409)
	if e := r.apiErr(t); e.Error.Code != "TENANT_SLUG_TAKEN" {
		t.Fatalf("código %q, esperaba TENANT_SLUG_TAKEN", e.Error.Code)
	}
}

func TestAdmin_SystemTenantIsProtected(t *testing.T) {
	actor := bearer(superToken(t))
	platformID := tenantIDBySlug(t, "platform")

	r := do(t, "DELETE", "/admin/tenants/"+platformID, nil, actor)
	expectStatus(t, r, 409)
	if e := r.apiErr(t); e.Error.Code != "PLATFORM_TENANT_PROTECTED" {
		t.Fatalf("código %q, esperaba PLATFORM_TENANT_PROTECTED", e.Error.Code)
	}
	expectStatus(t, do(t, "PATCH", "/admin/tenants/"+platformID, map[string]bool{"active": false}, actor), 409)
	if deletedAt(t, "tenants", platformID) {
		t.Fatal("la empresa de sistema no debe quedar eliminada")
	}
	loginBearer(t, "platform", rootEmail)
}

func TestAdmin_DeleteUserRevokesSessionsAndFreesEmail(t *testing.T) {
	slug := uniqueSlug("usr")
	tenant := adminCreateTenant(t, slug)
	email := "cobrador@" + slug + ".com"
	user := adminCreateUser(t, tenant.ID, email)
	_, refresh := loginPair(t, slug, email)
	actor := bearer(superToken(t))

	expectStatus(t, do(t, "DELETE", "/admin/tenants/"+tenant.ID+"/users/"+user.ID, nil, actor), 204)

	expectStatus(t, do(t, "POST", "/auth/login", map[string]string{"tenant": slug, "email": email, "password": pass}), 401)
	expectStatus(t, do(t, "POST", "/auth/refresh", map[string]string{"refresh_token": refresh}), 401)

	adminCreateUser(t, tenant.ID, email) // el correo de un usuario eliminado se reutiliza
	r := do(t, "POST", "/admin/tenants/"+tenant.ID+"/users/"+user.ID+"/restore", nil, actor)
	expectStatus(t, r, 409)
	if e := r.apiErr(t); e.Error.Code != "EMAIL_TAKEN" {
		t.Fatalf("restaurar con correo ocupado: código %q, esperaba EMAIL_TAKEN", e.Error.Code)
	}
}

func TestAdmin_RestoreUserReactivatesLogin(t *testing.T) {
	slug := uniqueSlug("rsu")
	tenant := adminCreateTenant(t, slug)
	email := "cobrador@" + slug + ".com"
	user := adminCreateUser(t, tenant.ID, email)
	actor := bearer(superToken(t))
	expectStatus(t, do(t, "DELETE", "/admin/tenants/"+tenant.ID+"/users/"+user.ID, nil, actor), 204)
	expectStatus(t, do(t, "POST", "/admin/tenants/"+tenant.ID+"/users/"+user.ID+"/restore", nil, actor), 200)
	loginBearer(t, slug, email)
}

func TestAdmin_CrossTenantUserPathIsNotFound(t *testing.T) {
	ctx := context.Background()
	betaID := tenantIDBySlug(t, "beta")
	acmeID := tenantIDBySlug(t, "acme")
	var cobradorID string
	if err := env.Admin.QueryRow(ctx, `SELECT id::text FROM users WHERE tenant_id=$1::uuid AND email='cobrador@acme.com'`,
		acmeID).Scan(&cobradorID); err != nil {
		t.Fatal(err)
	}
	expectStatus(t, do(t, "DELETE", "/admin/tenants/"+betaID+"/users/"+cobradorID, nil, bearer(superToken(t))), 404)
	if deletedAt(t, "users", cobradorID) {
		t.Fatal("el usuario de otra empresa no debe eliminarse por una ruta ajena")
	}
	loginBearer(t, "acme", "cobrador@acme.com")
}

func TestAdmin_LastSuperadminIsProtected(t *testing.T) {
	ctx := context.Background()
	slug := uniqueSlug("sys")
	sysID, err := env.SystemTenant(ctx, slug)
	if err != nil {
		t.Fatal(err)
	}
	solo, err := env.User(ctx, sysID, "solo@"+slug+".com", pass, "superadmin")
	if err != nil {
		t.Fatal(err)
	}
	actor := bearer(superToken(t))
	path := "/admin/tenants/" + sysID + "/users/" + solo

	r := do(t, "DELETE", path, nil, actor)
	expectStatus(t, r, 409)
	if e := r.apiErr(t); e.Error.Code != "LAST_SUPERADMIN" {
		t.Fatalf("código %q, esperaba LAST_SUPERADMIN", e.Error.Code)
	}
	if deletedAt(t, "users", solo) {
		t.Fatal("el último superadmin no debe quedar eliminado")
	}

	if _, err := env.User(ctx, sysID, "segundo@"+slug+".com", pass, "superadmin"); err != nil {
		t.Fatal(err)
	}
	expectStatus(t, do(t, "DELETE", path, nil, actor), 204)
	var other string
	if err := env.Admin.QueryRow(ctx, `SELECT id::text FROM users WHERE tenant_id=$1::uuid AND email=$2`,
		sysID, "segundo@"+slug+".com").Scan(&other); err != nil {
		t.Fatal(err)
	}
	expectStatus(t, do(t, "DELETE", "/admin/tenants/"+sysID+"/users/"+other, nil, actor), 409)
}
