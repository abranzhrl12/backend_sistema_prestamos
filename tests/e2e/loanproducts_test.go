//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"testing"
)

type productDTO struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	Frequency     string  `json:"frequency"`
	InterestType  string  `json:"interest_type"`
	RatePeriod    string  `json:"rate_period"`
	RateDefault   string  `json:"rate_default"`
	RateMin       string  `json:"rate_min"`
	RateMax       string  `json:"rate_max"`
	AmountMin     string  `json:"amount_min"`
	AmountMax     string  `json:"amount_max"`
	TermsMin      int     `json:"terms_min"`
	TermsMax      int     `json:"terms_max"`
	GraceDays     *int    `json:"grace_days"`
	LateFeeMode   *string `json:"late_fee_mode"`
	LateFeeAmount *string `json:"late_fee_amount"`
	LateFeePerDay *bool   `json:"late_fee_per_day"`
	FeeType       string  `json:"fee_type"`
	FeeValue      string  `json:"fee_value"`
	Active        bool    `json:"active"`
}

type productPageDTO struct {
	Items []productDTO `json:"items"`
	Meta  struct {
		Total   *int `json:"total"`
		Limit   int  `json:"limit"`
		Offset  int  `json:"offset"`
		HasMore bool `json:"has_more"`
	} `json:"meta"`
}

// productBody es un producto válido de cargo fijo; cada test ajusta lo que necesita.
func productBody(name string) map[string]any {
	return map[string]any{
		"name": name, "frequency": "monthly", "interest_type": "flat", "rate_period": "monthly",
		"rate_default": "2.50", "rate_min": "1.00", "rate_max": "5.00",
		"amount_min": "500.00", "amount_max": "5000.00", "terms_min": 1, "terms_max": 12,
		"fee_type": "fixed", "fee_value": "15.00",
	}
}

func createProduct(t *testing.T, tok string, body map[string]any) productDTO {
	t.Helper()
	r := do(t, "POST", "/loan-products", body, bearer(tok))
	expectStatus(t, r, 201)
	var p productDTO
	r.decode(t, &p)
	return p
}

func listProducts(t *testing.T, tok, query string) productPageDTO {
	t.Helper()
	r := do(t, "GET", "/loan-products?"+query, nil, bearer(tok))
	expectStatus(t, r, 200)
	var p productPageDTO
	r.decode(t, &p)
	return p
}

func TestLoanProducts_CRUDCycle(t *testing.T) {
	f := newFixture(t)
	tok := f.ownerToken(t)

	created := createProduct(t, tok, productBody("Diario Ciclo"))
	if created.ID == "" || created.RateDefault != "2.50" || created.AmountMin != "500.00" ||
		created.FeeValue != "15.00" || !created.Active || created.GraceDays != nil || created.LateFeeMode != nil {
		t.Fatalf("producto creado inesperado: %+v", created)
	}

	got := do(t, "GET", "/loan-products/"+created.ID, nil, bearer(tok))
	expectStatus(t, got, 200)

	list := listProducts(t, tok, "q=ciclo")
	if len(list.Items) != 1 || list.Items[0].ID != created.ID {
		t.Fatalf("el listado debe contener el producto creado: %+v", list.Items)
	}

	upd := productBody("Diario Ciclo Editado")
	upd["rate_default"], upd["rate_max"] = "3.00", "6.00"
	upd["grace_days"], upd["late_fee_mode"], upd["late_fee_amount"] = 2, "proportional", "1.5"
	r := do(t, "PUT", "/loan-products/"+created.ID, upd, bearer(tok))
	expectStatus(t, r, 200)
	var updated productDTO
	r.decode(t, &updated)
	if updated.Name != "Diario Ciclo Editado" || updated.RateDefault != "3.00" || updated.GraceDays == nil ||
		*updated.GraceDays != 2 || updated.LateFeeAmount == nil || *updated.LateFeeAmount != "1.50" {
		t.Fatalf("actualización inesperada: %s", r.Body)
	}

	expectStatus(t, do(t, "DELETE", "/loan-products/"+created.ID, nil, bearer(tok)), 204)
	gone := do(t, "GET", "/loan-products/"+created.ID, nil, bearer(tok))
	expectStatus(t, gone, 404)
	if e := gone.apiErr(t); e.Error.Code != "NOT_FOUND" || e.Error.Message != "El plan de préstamo no existe" {
		t.Fatalf("error inesperado tras eliminar: %s", gone.Body)
	}
}

func TestLoanProducts_NonUUIDIDIs404(t *testing.T) {
	f := newFixture(t)
	r := do(t, "GET", "/loan-products/no-es-uuid", nil, bearer(f.ownerToken(t)))
	expectStatus(t, r, 404)
	if code := r.apiErr(t).Error.Code; code != "NOT_FOUND" {
		t.Fatalf("code %q, esperaba NOT_FOUND", code)
	}
}

func TestLoanProducts_ValidationReturnsFields(t *testing.T) {
	f := newFixture(t)
	tok := f.ownerToken(t)
	cases := []struct {
		name   string
		mutate func(map[string]any)
		fields []string
	}{
		{"monto mínimo mayor que el máximo", func(b map[string]any) { b["amount_min"] = "9000.00" }, []string{"amount_max"}},
		{"frecuencia única con plazo 5", func(b map[string]any) {
			b["frequency"], b["terms_min"], b["terms_max"] = "single", 5, 5
		}, []string{"terms_min", "terms_max"}},
		{"cargo porcentual de 150", func(b map[string]any) { b["fee_type"], b["fee_value"] = "percent", "150" }, []string{"fee_value"}},
		{"nombre vacío", func(b map[string]any) { b["name"] = "" }, []string{"name"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := productBody("Validacion " + tc.name)
			tc.mutate(body)
			r := do(t, "POST", "/loan-products", body, bearer(tok))
			expectStatus(t, r, 400)
			e := r.apiErr(t)
			if e.Error.Code != "VALIDATION_ERROR" {
				t.Fatalf("code %q, esperaba VALIDATION_ERROR", e.Error.Code)
			}
			got := fieldSet(e)
			for _, want := range tc.fields {
				if _, ok := got[want]; !ok {
					t.Fatalf("falta el campo %q en %s", want, r.Body)
				}
			}
		})
	}
}

func TestLoanProducts_DuplicateNameIsCaseInsensitive409(t *testing.T) {
	f := newFixture(t)
	tok := f.ownerToken(t)
	createProduct(t, tok, productBody("Diario Dup"))

	r := do(t, "POST", "/loan-products", productBody("DIARIO DUP"), bearer(tok))
	expectStatus(t, r, 409)
	e := r.apiErr(t)
	if e.Error.Code != "PRODUCT_NAME_TAKEN" || e.Error.Message != "Ya existe un plan de préstamo con ese nombre" {
		t.Fatalf("error inesperado: %s", r.Body)
	}
}

func TestLoanProducts_SoftDeleteKeepsRowAndFreesName(t *testing.T) {
	f := newFixture(t)
	tok := f.ownerToken(t)
	first := createProduct(t, tok, productBody("Reutilizable"))

	expectStatus(t, do(t, "DELETE", "/loan-products/"+first.ID, nil, bearer(tok)), 204)
	expectStatus(t, do(t, "DELETE", "/loan-products/"+first.ID, nil, bearer(tok)), 404)
	if list := listProducts(t, tok, "q=reutilizable"); len(list.Items) != 0 {
		t.Fatalf("el eliminado no debe aparecer en el listado: %+v", list.Items)
	}

	second := createProduct(t, tok, productBody("reutilizable"))
	if second.ID == first.ID {
		t.Fatal("el nuevo producto debe tener otro id")
	}

	var deleted bool
	err := env.Admin.QueryRow(context.Background(),
		`SELECT deleted_at IS NOT NULL FROM loan_products WHERE id=$1::uuid`, first.ID).Scan(&deleted)
	if err != nil {
		t.Fatalf("la fila eliminada debe seguir en la BD: %v", err)
	}
	if !deleted {
		t.Fatal("la fila eliminada debe tener deleted_at")
	}
}

func TestLoanProducts_CobradorReadsButCannotWrite(t *testing.T) {
	f := newFixture(t)
	created := createProduct(t, f.ownerToken(t), productBody("Solo Lectura"))
	cob := f.cobradorToken(t)

	expectStatus(t, do(t, "GET", "/loan-products", nil, bearer(cob)), 200)
	expectStatus(t, do(t, "GET", "/loan-products/"+created.ID, nil, bearer(cob)), 200)

	for _, req := range []struct{ method, path string }{
		{"POST", "/loan-products"},
		{"PUT", "/loan-products/" + created.ID},
		{"DELETE", "/loan-products/" + created.ID},
	} {
		r := do(t, req.method, req.path, productBody("Intento Cobrador"), bearer(cob))
		expectStatus(t, r, 403)
		if code := r.apiErr(t).Error.Code; code != "FORBIDDEN" {
			t.Fatalf("%s %s: code %q, esperaba FORBIDDEN", req.method, req.path, code)
		}
	}
}

func TestLoanProducts_TenantsAreIsolated(t *testing.T) {
	a := newFixture(t)
	b := newFixture(t)
	created := createProduct(t, a.ownerToken(t), productBody("Privado"))
	tokB := b.ownerToken(t)

	expectStatus(t, do(t, "GET", "/loan-products/"+created.ID, nil, bearer(tokB)), 404)
	if list := listProducts(t, tokB, ""); len(list.Items) != 0 {
		t.Fatalf("la otra empresa no debe ver productos ajenos: %+v", list.Items)
	}
	expectStatus(t, do(t, "DELETE", "/loan-products/"+created.ID, nil, bearer(tokB)), 404)
}

func TestLoanProducts_Pagination(t *testing.T) {
	f := newFixture(t)
	tok := f.ownerToken(t)
	for i := 1; i <= 3; i++ {
		createProduct(t, tok, productBody(fmt.Sprintf("Paginado %d", i)))
	}

	first := listProducts(t, tok, "limit=2&include_total=true")
	if len(first.Items) != 2 || !first.Meta.HasMore || first.Meta.Total == nil || *first.Meta.Total != 3 {
		t.Fatalf("primera página inesperada: %+v", first.Meta)
	}
	second := listProducts(t, tok, "limit=2&offset=2")
	if len(second.Items) != 1 || second.Meta.HasMore || second.Meta.Total != nil {
		t.Fatalf("segunda página inesperada (total no pedido debe omitirse): %+v", second.Meta)
	}
	expectStatus(t, do(t, "GET", "/loan-products?limit=0", nil, bearer(tok)), 400)
}

func TestLoanProducts_Filters(t *testing.T) {
	f := newFixture(t)
	tok := f.ownerToken(t)
	createProduct(t, tok, productBody("Mensual Activo"))
	inactive := productBody("Diario Inactivo")
	inactive["frequency"], inactive["terms_min"], inactive["terms_max"] = "daily", 1, 30
	inactive["active"] = false
	createProduct(t, tok, inactive)

	if p := listProducts(t, tok, "q=inactivo"); len(p.Items) != 1 || p.Items[0].Name != "Diario Inactivo" {
		t.Fatalf("filtro q: %+v", p.Items)
	}
	if p := listProducts(t, tok, "active=false"); len(p.Items) != 1 || p.Items[0].Active {
		t.Fatalf("filtro active=false: %+v", p.Items)
	}
	if p := listProducts(t, tok, "frequency=daily"); len(p.Items) != 1 || p.Items[0].Frequency != "daily" {
		t.Fatalf("filtro frequency: %+v", p.Items)
	}
	r := do(t, "GET", "/loan-products?frequency=yearly", nil, bearer(tok))
	expectStatus(t, r, 400)
	if _, ok := fieldSet(r.apiErr(t))["frequency"]; !ok {
		t.Fatalf("frecuencia inválida debe reportar el campo frequency: %s", r.Body)
	}
	r = do(t, "GET", "/loan-products?active=abc", nil, bearer(tok))
	expectStatus(t, r, 400)
	if _, ok := fieldSet(r.apiErr(t))["active"]; !ok {
		t.Fatalf("active inválido debe reportar el campo active: %s", r.Body)
	}
}

func TestLoanProducts_WritesAuditRows(t *testing.T) {
	f := newFixture(t)
	tok := f.ownerToken(t)
	p := createProduct(t, tok, productBody("Auditoria"))
	expectStatus(t, do(t, "PUT", "/loan-products/"+p.ID, productBody("Auditoria Editada"), bearer(tok)), 200)
	expectStatus(t, do(t, "DELETE", "/loan-products/"+p.ID, nil, bearer(tok)), 204)

	for _, action := range []string{"product.create", "product.update", "product.delete"} {
		var n int
		err := env.Admin.QueryRow(context.Background(),
			`SELECT count(*) FROM audit_log WHERE tenant_id=$1::uuid AND action=$2`, f.TenantID, action).Scan(&n)
		if err != nil {
			t.Fatal(err)
		}
		if n != 1 {
			t.Fatalf("%s: esperaba 1 fila de audit, hubo %d", action, n)
		}
	}
}
