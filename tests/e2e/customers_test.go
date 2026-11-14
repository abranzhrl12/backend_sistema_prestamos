//go:build e2e

package e2e

import (
	"fmt"
	"net/url"
	"testing"
)

type customerDTO struct {
	ID               string `json:"id"`
	FullName         string `json:"full_name"`
	DocumentID       string `json:"document_id"`
	Address          string `json:"address"`
	AddressReference string `json:"address_reference"`
	Status           string `json:"status"`
}

type pageDTO struct {
	Items []customerDTO `json:"items"`
	Meta  struct {
		Total   int  `json:"total"`
		Limit   int  `json:"limit"`
		Offset  int  `json:"offset"`
		HasMore bool `json:"has_more"`
	} `json:"meta"`
}

// listPage hace GET /customers?query y espera 200.
func listPage(t *testing.T, tok, query string) pageDTO {
	t.Helper()
	r := do(t, "GET", "/customers?include_total=true&"+query, nil, bearer(tok))
	expectStatus(t, r, 200)
	var p pageDTO
	r.decode(t, &p)
	return p
}

func createCustomer(t *testing.T, tok string, body map[string]string) customerDTO {
	t.Helper()
	r := do(t, "POST", "/customers", body, bearer(tok))
	expectStatus(t, r, 201)
	var c customerDTO
	r.decode(t, &c)
	return c
}

func TestCustomers_CreateAndGet(t *testing.T) {
	tok := loginBearer(t, "acme", "owner@acme.com")
	created := createCustomer(t, tok, newCustomer("Cliente Crear"))
	if created.ID == "" {
		t.Fatal("el cliente creado no tiene id")
	}
	if created.Status != "active" {
		t.Fatalf("status %q, esperaba active", created.Status)
	}
	if created.Address != "Av. Lima 123" {
		t.Fatalf("address %q, esperaba %q", created.Address, "Av. Lima 123")
	}
	if created.AddressReference != "frente al mercado" {
		t.Fatalf("address_reference %q, esperaba %q", created.AddressReference, "frente al mercado")
	}

	get := do(t, "GET", "/customers/"+created.ID, nil, bearer(tok))
	expectStatus(t, get, 200)
}

func TestCustomers_DuplicateDocument_Returns409(t *testing.T) {
	tok := loginBearer(t, "acme", "owner@acme.com")
	body := newCustomer("Cliente Duplicado")

	first := do(t, "POST", "/customers", body, bearer(tok))
	expectStatus(t, first, 201)

	second := do(t, "POST", "/customers", body, bearer(tok))
	expectStatus(t, second, 409)
	e := second.apiErr(t)
	if e.Error.Code != "DUPLICATE_DOCUMENT" {
		t.Fatalf("code %q, esperaba DUPLICATE_DOCUMENT: %s", e.Error.Code, second.Body)
	}
	if e.Error.Message != "Ya existe un cliente con ese documento" {
		t.Fatalf("message %q inesperado", e.Error.Message)
	}
}

func TestCustomers_SameDocumentInOtherTenant_IsAllowed(t *testing.T) {
	acme := loginBearer(t, "acme", "owner@acme.com")
	beta := loginBearer(t, "beta", "owner@beta.com")
	body := newCustomer("Cliente Multi Tenant")

	expectStatus(t, do(t, "POST", "/customers", body, bearer(acme)), 201)
	expectStatus(t, do(t, "POST", "/customers", body, bearer(beta)), 201)
}

func TestCustomers_Validation_ReportsEveryField(t *testing.T) {
	tok := loginBearer(t, "acme", "owner@acme.com")
	r := do(t, "POST", "/customers", map[string]string{"email": "sin-arroba"}, bearer(tok))
	expectStatus(t, r, 400)

	e := r.apiErr(t)
	if e.Error.Code != "VALIDATION_ERROR" {
		t.Fatalf("code %q, esperaba VALIDATION_ERROR: %s", e.Error.Code, r.Body)
	}
	want := map[string]bool{"full_name": false, "document_id": false, "email": false}
	for _, f := range e.Error.Fields {
		if _, ok := want[f.Field]; ok {
			want[f.Field] = true
			if f.Message == "" {
				t.Fatalf("el campo %s no tiene message", f.Field)
			}
		}
	}
	for field, found := range want {
		if !found {
			t.Fatalf("falta el campo %s en fields: %s", field, r.Body)
		}
	}
}

func TestCustomers_Cobrador_CannotCreate(t *testing.T) {
	tok := loginBearer(t, "acme", "cobrador@acme.com")

	denied := do(t, "POST", "/customers", newCustomer("Cobrador No Puede"), bearer(tok))
	expectStatus(t, denied, 403)
	if code := denied.apiErr(t).Error.Code; code != "FORBIDDEN" {
		t.Fatalf("code %q, esperaba FORBIDDEN: %s", code, denied.Body)
	}

	list := do(t, "GET", "/customers", nil, bearer(tok))
	expectStatus(t, list, 200)
}

func TestCustomers_TenantIsolation(t *testing.T) {
	acme := loginBearer(t, "acme", "owner@acme.com")
	beta := loginBearer(t, "beta", "owner@beta.com")

	c := createCustomer(t, acme, newCustomer("Solo de acme"))

	get := do(t, "GET", "/customers/"+c.ID, nil, bearer(beta))
	expectStatus(t, get, 404)
	if code := get.apiErr(t).Error.Code; code != "NOT_FOUND" {
		t.Fatalf("GET: code %q, esperaba NOT_FOUND", code)
	}

	put := do(t, "PUT", "/customers/"+c.ID, newCustomer("Intento beta"), bearer(beta))
	expectStatus(t, put, 404)

	search := listPage(t, beta, "q="+url.QueryEscape("Solo de acme"))
	if search.Meta.Total != 0 {
		t.Fatalf("beta ve %d clientes de acme, esperaba 0", search.Meta.Total)
	}
}

func TestCustomers_Pagination(t *testing.T) {
	beta := loginBearer(t, "beta", "owner@beta.com")
	for i := 0; i < 25; i++ {
		createCustomer(t, beta, newCustomer(fmt.Sprintf("Paginado %02d", i)))
	}

	first := listPage(t, beta, "q=Paginado&limit=10&offset=0")
	if first.Meta.Total != 25 || len(first.Items) != 10 || !first.Meta.HasMore {
		t.Fatalf("página 1: total=%d items=%d has_more=%v", first.Meta.Total, len(first.Items), first.Meta.HasMore)
	}

	last := listPage(t, beta, "q=Paginado&limit=10&offset=20")
	if len(last.Items) != 5 || last.Meta.HasMore {
		t.Fatalf("página 3: items=%d has_more=%v", len(last.Items), last.Meta.HasMore)
	}

	beyond := listPage(t, beta, "q=Paginado&limit=10&offset=500")
	if len(beyond.Items) != 0 || beyond.Meta.Total != 25 || beyond.Meta.HasMore {
		t.Fatalf("offset 500: items=%d total=%d has_more=%v", len(beyond.Items), beyond.Meta.Total, beyond.Meta.HasMore)
	}
}

func TestCustomers_List_EmptyIsArrayNotNull(t *testing.T) {
	tok := loginBearer(t, "acme", "owner@acme.com")
	r := do(t, "GET", "/customers?q=zzz-no-existe", nil, bearer(tok))
	expectStatus(t, r, 200)

	var raw struct {
		Items any `json:"items"`
	}
	r.decode(t, &raw)
	items, ok := raw.Items.([]any)
	if !ok || items == nil {
		t.Fatalf("items debe ser un arreglo vacío, no null: %s", r.Body)
	}
}

func TestCustomers_Search_ByNameDocumentAndStatus(t *testing.T) {
	tok := loginBearer(t, "acme", "owner@acme.com")
	c := createCustomer(t, tok, newCustomer("Zacarías Búsqueda"))

	if n := listPage(t, tok, "q=zacar").Meta.Total; n != 1 {
		t.Fatalf("q=zacar total %d, esperaba 1", n)
	}
	if n := listPage(t, tok, "q="+url.QueryEscape(c.DocumentID)).Meta.Total; n != 1 {
		t.Fatalf("q=<document_id> total %d, esperaba 1", n)
	}

	st := do(t, "PATCH", "/customers/"+c.ID+"/status", map[string]string{"status": "blocked"}, bearer(tok))
	expectStatus(t, st, 200)

	if n := listPage(t, tok, "q=zacar&status=blocked").Meta.Total; n != 1 {
		t.Fatalf("q=zacar&status=blocked total %d, esperaba 1", n)
	}
	if n := listPage(t, tok, "q=zacar&status=active").Meta.Total; n != 0 {
		t.Fatalf("q=zacar&status=active total %d, esperaba 0", n)
	}
}

func TestCustomers_List_InvalidPagination_ReportsFields(t *testing.T) {
	tok := loginBearer(t, "acme", "owner@acme.com")
	r := do(t, "GET", "/customers?limit=1000&offset=-5", nil, bearer(tok))
	expectStatus(t, r, 400)

	e := r.apiErr(t)
	if e.Error.Code != "VALIDATION_ERROR" || len(e.Error.Fields) != 2 {
		t.Fatalf("esperaba VALIDATION_ERROR con 2 fields: %s", r.Body)
	}
}

func TestCustomers_SQLInjectionInSearch_IsHarmless(t *testing.T) {
	tok := loginBearer(t, "acme", "owner@acme.com")
	inj := do(t, "GET", "/customers?q=%27%3B+DROP+TABLE+customers%3B--", nil, bearer(tok))
	expectStatus(t, inj, 200)

	after := do(t, "GET", "/customers", nil, bearer(tok))
	expectStatus(t, after, 200)
}

func TestCustomers_List_WithoutIncludeTotal_SkipsCountButKeepsHasMore(t *testing.T) {
	tok := loginBearer(t, "beta", "owner@beta.com")
	for i := range 5 {
		expectStatus(t, do(t, "POST", "/customers", newCustomer(fmt.Sprintf("SinTotal %d", i)), bearer(tok)), 201)
	}
	r := do(t, "GET", "/customers?q=SinTotal&limit=2", nil, bearer(tok))
	expectStatus(t, r, 200)
	var raw struct {
		Items []customerDTO  `json:"items"`
		Meta  map[string]any `json:"meta"`
	}
	r.decode(t, &raw)
	if _, has := raw.Meta["total"]; has {
		t.Fatalf("sin include_total no debe venir total: %s", r.Body)
	}
	if len(raw.Items) != 2 || raw.Meta["has_more"] != true {
		t.Fatalf("esperaba 2 items y has_more=true: %s", r.Body)
	}
	last := do(t, "GET", "/customers?q=SinTotal&limit=2&offset=4", nil, bearer(tok))
	last.decode(t, &raw)
	if len(raw.Items) != 1 || raw.Meta["has_more"] != false {
		t.Fatalf("última página inesperada: %s", last.Body)
	}
}
