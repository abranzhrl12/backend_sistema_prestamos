package domain_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"prestamos/internal/loanproducts/domain"
)

// validProduct es un producto correcto que cada caso modifica en un solo punto.
func validProduct() domain.Product {
	return domain.Product{
		Name: "Préstamo diario", Frequency: "daily", InterestType: "flat", RatePeriod: "monthly",
		RateDefault: "2.50", RateMin: "1.00", RateMax: "5.00", AmountMin: "500.00", AmountMax: "5000.00",
		TermsMin: 1, TermsMax: 12, FeeType: "none", FeeValue: "0", Active: true,
	}
}

func fieldsOf(err error) []string {
	if err == nil {
		return nil
	}
	var ve *domain.ValidationError
	if !errors.As(err, &ve) {
		return []string{"no es ValidationError"}
	}
	var out []string
	for _, is := range ve.Issues {
		out = append(out, is.Field)
	}
	return out
}

func intPtr(v int) *int       { return &v }
func strPtr(v string) *string { return &v }
func boolPtr(v bool) *bool    { return &v }

func TestValidate_ValidProductPasses(t *testing.T) {
	if err := validProduct().Validate(); err != nil {
		t.Fatalf("el producto base debe ser válido: %v", err)
	}
}

func TestValidate_Rules(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*domain.Product)
		want   []string
	}{
		{"nombre vacío", func(p *domain.Product) { p.Name = "" }, []string{"name"}},
		{"nombre de 1 carácter", func(p *domain.Product) { p.Name = "A" }, []string{"name"}},
		{"nombre de 81 caracteres", func(p *domain.Product) { p.Name = strings.Repeat("a", 81) }, []string{"name"}},
		{"frecuencia inválida", func(p *domain.Product) { p.Frequency = "yearly" }, []string{"frequency"}},
		{"tipo de interés inválido", func(p *domain.Product) { p.InterestType = "simple" }, []string{"interest_type"}},
		{"periodo de tasa inválido", func(p *domain.Product) { p.RatePeriod = "hourly" }, []string{"rate_period"}},
		{"tasa por defecto no numérica", func(p *domain.Product) { p.RateDefault = "abc" }, []string{"rate_default"}},
		{"tasa mínima mayor que la default", func(p *domain.Product) { p.RateMin = "3.00" }, []string{"rate_min"}},
		{"tasa máxima menor que la default", func(p *domain.Product) { p.RateMax = "1.00" }, []string{"rate_max"}},
		{"tasa máxima mayor a 1000", func(p *domain.Product) { p.RateMax = "1500" }, []string{"rate_max"}},
		{"tasa negativa", func(p *domain.Product) { p.RateMin = "-1" }, []string{"rate_min"}},
		{"monto mínimo cero", func(p *domain.Product) { p.AmountMin = "0" }, []string{"amount_min"}},
		{"monto mínimo no numérico", func(p *domain.Product) { p.AmountMin = "abc" }, []string{"amount_min"}},
		{"monto máximo menor que el mínimo", func(p *domain.Product) { p.AmountMax = "100" }, []string{"amount_max"}},
		{"plazo mínimo cero", func(p *domain.Product) { p.TermsMin = 0 }, []string{"terms_min"}},
		{"plazo máximo mayor a 1000", func(p *domain.Product) { p.TermsMax = 1001 }, []string{"terms_max"}},
		{"plazo mínimo mayor que el máximo", func(p *domain.Product) { p.TermsMin, p.TermsMax = 5, 3 }, []string{"terms_max"}},
		{"frecuencia única con plazo 5", func(p *domain.Product) {
			p.Frequency, p.TermsMin, p.TermsMax = "single", 5, 5
		}, []string{"terms_min", "terms_max"}},
		{"frecuencia única con plazo máximo 5", func(p *domain.Product) {
			p.Frequency, p.TermsMin, p.TermsMax = "single", 1, 5
		}, []string{"terms_max"}},
		{"gracia fuera de rango", func(p *domain.Product) { p.GraceDays = intPtr(31) }, []string{"grace_days"}},
		{"modo de mora inválido", func(p *domain.Product) { p.LateFeeMode = strPtr("mixed") }, []string{"late_fee_mode"}},
		{"monto de mora inválido", func(p *domain.Product) { p.LateFeeAmount = strPtr("abc") }, []string{"late_fee_amount"}},
		{"tipo de cargo inválido", func(p *domain.Product) { p.FeeType = "weird" }, []string{"fee_type"}},
		{"sin cargo con valor distinto de 0", func(p *domain.Product) { p.FeeValue = "5" }, []string{"fee_value"}},
		{"cargo porcentual mayor a 100", func(p *domain.Product) {
			p.FeeType, p.FeeValue = "percent", "150"
		}, []string{"fee_value"}},
		{"cargo fijo no numérico", func(p *domain.Product) {
			p.FeeType, p.FeeValue = "fixed", "abc"
		}, []string{"fee_value"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := validProduct()
			tc.mutate(&p)
			if got := fieldsOf(p.Validate()); !slices.Equal(got, tc.want) {
				t.Fatalf("campos %v, esperaba %v", got, tc.want)
			}
		})
	}
}

func TestValidate_AcceptsBoundaryValues(t *testing.T) {
	p := validProduct()
	p.Name = strings.Repeat("a", 80)
	p.RateMax = "1000"
	p.RateDefault = "1000"
	p.AmountMin = "0.0001"
	p.AmountMax = "0.0001"
	p.TermsMax = 1000
	p.GraceDays = intPtr(30)
	p.LateFeeMode = strPtr("proportional")
	p.LateFeeAmount = strPtr("0")
	p.LateFeePerDay = boolPtr(true)
	p.FeeType, p.FeeValue = "percent", "100"
	if err := p.Validate(); err != nil {
		t.Fatalf("valores frontera deben ser válidos: %v", err)
	}
}

func TestValidate_SingleFrequencyWithOneTermIsValid(t *testing.T) {
	p := validProduct()
	p.Frequency, p.TermsMin, p.TermsMax = "single", 1, 1
	if err := p.Validate(); err != nil {
		t.Fatalf("frecuencia única con plazo 1 debe ser válida: %v", err)
	}
}

func TestValidate_ReportsAllFieldsAtOnce(t *testing.T) {
	p := validProduct()
	p.Name, p.Frequency, p.FeeType, p.FeeValue = "", "yearly", "percent", "150"
	want := []string{"name", "frequency", "fee_value"}
	if got := fieldsOf(p.Validate()); !slices.Equal(got, want) {
		t.Fatalf("campos %v, esperaba %v", got, want)
	}
}

func TestNormalize_TrimsAndAppliesDefaults(t *testing.T) {
	p := domain.Product{Name: "  Diario  ", Frequency: "daily", RatePeriod: "monthly", RateDefault: " 2 "}
	p.Normalize()
	if p.Name != "Diario" || p.InterestType != "flat" || p.FeeType != "none" || p.FeeValue != "0" || p.RateDefault != "2" {
		t.Fatalf("normalización inesperada: %+v", p)
	}
}
