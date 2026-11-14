//go:build e2e

package e2e

import (
	"context"
	"testing"
)

type onboardingTokens struct {
	Access  string `json:"access_token"`
	Refresh string `json:"refresh_token"`
}

func loginWithoutTenant(t *testing.T, email, password string) onboardingTokens {
	t.Helper()
	r := do(t, "POST", "/auth/login", map[string]string{"email": email, "password": password})
	expectStatus(t, r, 200)
	var tokens onboardingTokens
	r.decode(t, &tokens)
	if tokens.Access == "" || tokens.Refresh == "" {
		t.Fatal("login must return both bearer tokens")
	}
	return tokens
}

func TestOnboarding_EmailOnlyLogin(t *testing.T) {
	tokens := loginWithoutTenant(t, "owner@acme.com", pass)
	expectStatus(t, do(t, "GET", "/auth/me", nil, bearer(tokens.Access)), 200)
}

func TestOnboarding_TemporaryPasswordAndForcedChange(t *testing.T) {
	slug := uniqueSlug("onboard")
	email := "owner@" + slug + ".com"
	r := do(t, "POST", "/admin/tenants", map[string]any{
		"slug": slug, "name": "Empresa " + slug,
		"owner": map[string]string{"email": email, "full_name": "Owner"},
	}, bearer(superToken(t)))
	expectStatus(t, r, 201)
	var created struct {
		TemporaryPassword string `json:"temporary_password"`
	}
	r.decode(t, &created)
	if created.TemporaryPassword == "" {
		t.Fatal("missing temporary_password")
	}

	tokens := loginWithoutTenant(t, email, created.TemporaryPassword)
	me := do(t, "GET", "/auth/me", nil, bearer(tokens.Access))
	expectStatus(t, me, 200)
	var profile struct {
		MustChange bool `json:"must_change_password"`
	}
	me.decode(t, &profile)
	if !profile.MustChange {
		t.Fatal("temporary password must require change")
	}
	expectStatus(t, do(t, "GET", "/customers", nil, bearer(tokens.Access)), 403)
	expectStatus(t, do(t, "POST", "/customers", newCustomer("Restricted"), bearer(tokens.Access)), 403)

	change := func(current, next string) response {
		return do(t, "POST", "/auth/change-password", map[string]string{"current_password": current, "new_password": next}, bearer(tokens.Access))
	}
	if r := change("wrong-password", "NuevaClave123"); r.Status < 400 {
		t.Fatalf("incorrect current password accepted: %d", r.Status)
	}
	if r := change(created.TemporaryPassword, created.TemporaryPassword); r.Status < 400 {
		t.Fatalf("same password accepted: %d", r.Status)
	}
	expectStatus(t, change(created.TemporaryPassword, "NuevaClave123"), 204)
	expectStatus(t, do(t, "POST", "/auth/login", map[string]string{"email": email, "password": created.TemporaryPassword}), 401)
	newTokens := loginWithoutTenant(t, email, "NuevaClave123")
	me = do(t, "GET", "/auth/me", nil, bearer(newTokens.Access))
	expectStatus(t, me, 200)
	me.decode(t, &profile)
	if profile.MustChange {
		t.Fatal("password change must clear restriction")
	}
	expectStatus(t, do(t, "GET", "/customers", nil, bearer(newTokens.Access)), 200)
}

func TestOnboarding_ResetRevokesExistingSessions(t *testing.T) {
	f := newFixture(t)
	tokens := loginWithoutTenant(t, f.Owner, pass)
	var id string
	if err := env.Admin.QueryRow(context.Background(), `SELECT id::text FROM users WHERE tenant_id=$1::uuid AND email=$2`, f.TenantID, f.Owner).Scan(&id); err != nil {
		t.Fatal(err)
	}
	path := "/admin/tenants/" + f.TenantID + "/users/" + id + "/reset-password"
	expectStatus(t, do(t, "POST", path, nil, bearer(tokens.Access)), 403)
	r := do(t, "POST", path, nil, bearer(superToken(t)))
	expectStatus(t, r, 200)
	var out struct {
		TemporaryPassword string `json:"temporary_password"`
	}
	r.decode(t, &out)
	if out.TemporaryPassword == "" {
		t.Fatal("missing reset temporary password")
	}
	expectStatus(t, do(t, "GET", "/customers", nil, bearer(tokens.Access)), 401)
	expectStatus(t, do(t, "POST", "/auth/refresh", map[string]string{"refresh_token": tokens.Refresh}), 401)
	expectStatus(t, do(t, "POST", "/auth/login", map[string]string{"email": f.Owner, "password": pass}), 401)
	loginWithoutTenant(t, f.Owner, out.TemporaryPassword)
}

func TestOnboarding_DuplicateEmailDoesNotChooseTenant(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	other := newFixture(t)
	email := "duplicate@" + f.Slug + ".com"
	if _, err := env.User(ctx, f.TenantID, email, pass, "owner"); err != nil {
		t.Fatal(err)
	}
	if _, err := env.User(ctx, other.TenantID, email, pass, "owner"); err != nil {
		t.Fatal(err)
	}
	r := do(t, "POST", "/auth/login", map[string]string{"email": email, "password": pass})
	if r.Status == 200 {
		t.Fatal("ambiguous email selected a tenant")
	}
}

func TestOnboarding_CookieChangeRequiresCSRF(t *testing.T) {
	f := newFixture(t)
	login := do(t, "POST", "/auth/login", map[string]string{"email": f.Owner, "password": pass, "transport": "cookie"})
	expectStatus(t, login, 200)
	access, csrf := login.cookieValue("access_token"), login.cookieValue("csrf_token")
	if access == "" || csrf == "" {
		t.Fatal("missing session cookies")
	}
	body := map[string]string{"current_password": pass, "new_password": "NuevaClave123"}
	expectStatus(t, do(t, "POST", "/auth/change-password", body, cookie("access_token", access)), 403)
	expectStatus(t, do(t, "POST", "/auth/change-password", body, cookie("access_token", access), cookie("csrf_token", csrf), header("X-CSRF-Token", csrf)), 204)
}

func TestOnboarding_InactiveUserAndCompanyRejectLoginAndExistingAccess(t *testing.T) {
	for _, kind := range []string{"user", "company"} {
		t.Run(kind, func(t *testing.T) {
			f := newFixture(t)
			tokens := loginWithoutTenant(t, f.Owner, pass)
			path := "/admin/tenants/" + f.TenantID
			if kind == "user" {
				var id string
				if err := env.Admin.QueryRow(context.Background(), `SELECT id::text FROM users WHERE tenant_id=$1::uuid AND email=$2`, f.TenantID, f.Owner).Scan(&id); err != nil {
					t.Fatal(err)
				}
				path += "/users/" + id
			}
			expectStatus(t, do(t, "PATCH", path, map[string]bool{"active": false}, bearer(superToken(t))), 200)
			expectStatus(t, do(t, "POST", "/auth/login", map[string]string{"email": f.Owner, "password": pass}), 401)
			expectStatus(t, do(t, "GET", "/customers", nil, bearer(tokens.Access)), 401)
			expectStatus(t, do(t, "POST", "/auth/refresh", map[string]string{"refresh_token": tokens.Refresh}), 401)
		})
	}
}

func TestOnboarding_StaleRefreshVersionRejected(t *testing.T) {
	f := newFixture(t)
	tokens := loginWithoutTenant(t, f.Owner, pass)
	// Model a refresh token persisted by a racing request after a reset increments
	// the user version: the stale row must not renew the session.
	_, err := env.Admin.Exec(context.Background(), `UPDATE users SET session_version=session_version+1 WHERE tenant_id=$1::uuid AND email=$2`, f.TenantID, f.Owner)
	if err != nil {
		t.Fatal(err)
	}
	expectStatus(t, do(t, "POST", "/auth/refresh", map[string]string{"refresh_token": tokens.Refresh}), 401)
}

func TestOnboarding_CrossTenantDuplicateEmailRejected(t *testing.T) {
	first, second := newFixture(t), newFixture(t)
	actor := bearer(superToken(t))
	body := map[string]any{"email": first.Owner, "full_name": "Duplicate", "roles": []string{"cobrador"}}
	r := do(t, "POST", "/admin/tenants/"+second.TenantID+"/users", body, actor)
	expectStatus(t, r, 409)
	if r.apiErr(t).Error.Code != "EMAIL_TAKEN" {
		t.Fatalf("expected EMAIL_TAKEN: %s", r.Body)
	}
}

func TestOnboarding_RestoreDuplicateEmailAcrossTenantsRejected(t *testing.T) {
	first, second := newFixture(t), newFixture(t)
	email := "restore@" + first.Slug + ".com"
	user := adminCreateUser(t, first.TenantID, email)
	actor := bearer(superToken(t))
	path := "/admin/tenants/" + first.TenantID + "/users/" + user.ID
	expectStatus(t, do(t, "DELETE", path, nil, actor), 204)
	adminCreateUser(t, second.TenantID, email)
	r := do(t, "POST", path+"/restore", nil, actor)
	expectStatus(t, r, 409)
	if r.apiErr(t).Error.Code != "EMAIL_TAKEN" {
		t.Fatalf("expected EMAIL_TAKEN: %s", r.Body)
	}
}

func TestOnboarding_ConcurrentDuplicateCreationHasSingleWinner(t *testing.T) {
	first, second := newFixture(t), newFixture(t)
	actor := bearer(superToken(t))
	email := "race@" + first.Slug + ".com"
	body := map[string]any{"email": email, "full_name": "Race", "roles": []string{"cobrador"}}
	start := make(chan struct{})
	results := make(chan response, 2)
	for _, tenantID := range []string{first.TenantID, second.TenantID} {
		go func(id string) {
			<-start
			results <- do(t, "POST", "/admin/tenants/"+id+"/users", body, actor)
		}(tenantID)
	}
	close(start)
	a, b := <-results, <-results
	if !((a.Status == 201 && b.Status == 409) || (a.Status == 409 && b.Status == 201)) {
		t.Fatalf("expected one creation and one conflict, got %d (%s), %d (%s)", a.Status, a.Body, b.Status, b.Body)
	}
	conflict := a
	if b.Status == 409 {
		conflict = b
	}
	if conflict.apiErr(t).Error.Code != "EMAIL_TAKEN" {
		t.Fatalf("expected EMAIL_TAKEN: %s", conflict.Body)
	}
}
