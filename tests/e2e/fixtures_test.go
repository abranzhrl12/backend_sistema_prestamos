//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
)

var tenantSeq atomic.Int64

// fixture es una empresa NUEVA y aislada (para tests que dependen de estado exacto,
// como saldos de capital o configuración). Sembrada con la BD de pruebas (superusuario).
type fixture struct {
	Slug     string
	TenantID string
	Owner    string // email del owner
	Cobrador string // email del cobrador
}

// newFixture crea una empresa con un owner y un cobrador (password = pass).
func newFixture(t *testing.T) fixture {
	t.Helper()
	ctx := context.Background()
	slug := fmt.Sprintf("fx%d", tenantSeq.Add(1))
	id, err := env.Tenant(ctx, slug)
	if err != nil {
		t.Fatal(err)
	}
	f := fixture{Slug: slug, TenantID: id, Owner: "owner@" + slug + ".com", Cobrador: "cobrador@" + slug + ".com"}
	if _, err := env.User(ctx, id, f.Owner, pass, "owner"); err != nil {
		t.Fatal(err)
	}
	if _, err := env.User(ctx, id, f.Cobrador, pass, "cobrador"); err != nil {
		t.Fatal(err)
	}
	return f
}

// ownerToken / cobradorToken inician sesión (bearer) y devuelven el access token.
func (f fixture) ownerToken(t *testing.T) string { t.Helper(); return loginBearer(t, f.Slug, f.Owner) }
func (f fixture) cobradorToken(t *testing.T) string {
	t.Helper()
	return loginBearer(t, f.Slug, f.Cobrador)
}
