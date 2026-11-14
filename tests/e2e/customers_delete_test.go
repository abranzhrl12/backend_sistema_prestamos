//go:build e2e

package e2e

import (
	"context"
	"testing"
)

// deleteCustomer hace DELETE /customers/:id con el token dado.
func deleteCustomer(t *testing.T, tok, id string) response {
	t.Helper()
	return do(t, "DELETE", "/customers/"+id, nil, bearer(tok))
}

func TestCustomersDelete_HappyPathHidesCustomerAndAudits(t *testing.T) {
	ctx := context.Background()
	tok := loginBearer(t, "acme", "owner@acme.com")
	c := createCustomer(t, tok, newCustomer("Cliente Borrar"))

	expectStatus(t, deleteCustomer(t, tok, c.ID), 204)

	r := do(t, "GET", "/customers/"+c.ID, nil, bearer(tok))
	expectStatus(t, r, 404)
	if e := r.apiErr(t); e.Error.Code != "NOT_FOUND" {
		t.Fatalf("código %q, esperaba NOT_FOUND", e.Error.Code)
	}
	for _, item := range listPage(t, tok, "q=Cliente+Borrar").Items {
		if item.ID == c.ID {
			t.Fatal("el listado no debe mostrar un cliente eliminado")
		}
	}
	if !deletedAt(t, "customers", c.ID) {
		t.Fatal("la fila debe conservarse con deleted_at")
	}
	var n int
	if err := env.Admin.QueryRow(ctx, `SELECT count(*) FROM audit_log WHERE action='customer.delete' AND entity_id=$1::uuid`,
		c.ID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("debe existir una fila de auditoría customer.delete, hay %d", n)
	}
}

func TestCustomersDelete_AlreadyDeletedIsNotFound(t *testing.T) {
	tok := loginBearer(t, "acme", "owner@acme.com")
	c := createCustomer(t, tok, newCustomer("Cliente Doble"))
	expectStatus(t, deleteCustomer(t, tok, c.ID), 204)

	r := deleteCustomer(t, tok, c.ID)
	expectStatus(t, r, 404)
	if e := r.apiErr(t); e.Error.Code != "NOT_FOUND" {
		t.Fatalf("código %q, esperaba NOT_FOUND", e.Error.Code)
	}
}

// Sin cuerpo que validar, la entrada inválida es el id: un valor que no es UUID responde 404 sin tocar la BD.
func TestCustomersDelete_InvalidIDIsNotFound(t *testing.T) {
	tok := loginBearer(t, "acme", "owner@acme.com")
	expectStatus(t, deleteCustomer(t, tok, "no-es-uuid"), 404)
}

func TestCustomersDelete_RequiresAuthAndWriteRole(t *testing.T) {
	owner := loginBearer(t, "acme", "owner@acme.com")
	cobrador := loginBearer(t, "acme", "cobrador@acme.com")
	c := createCustomer(t, owner, newCustomer("Cliente Permisos"))

	expectStatus(t, do(t, "DELETE", "/customers/"+c.ID, nil), 401)
	expectStatus(t, deleteCustomer(t, cobrador, c.ID), 403)
	expectStatus(t, do(t, "GET", "/customers/"+c.ID, nil, bearer(owner)), 200)
}

func TestCustomersDelete_OtherTenantIsNotFound(t *testing.T) {
	acme := loginBearer(t, "acme", "owner@acme.com")
	beta := loginBearer(t, "beta", "owner@beta.com")
	c := createCustomer(t, acme, newCustomer("Cliente Aislado"))

	expectStatus(t, deleteCustomer(t, beta, c.ID), 404)
	expectStatus(t, do(t, "GET", "/customers/"+c.ID, nil, bearer(acme)), 200)
}

func TestCustomersDelete_FreesDocumentForReuse(t *testing.T) {
	tok := loginBearer(t, "acme", "owner@acme.com")
	body := newCustomer("Cliente Documento")
	first := createCustomer(t, tok, body)
	expectStatus(t, deleteCustomer(t, tok, first.ID), 204)

	r := do(t, "POST", "/customers", body, bearer(tok))
	expectStatus(t, r, 201)
}
