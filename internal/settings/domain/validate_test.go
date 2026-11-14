package domain_test

import (
	"errors"
	"slices"
	"testing"

	"prestamos/internal/settings/domain"
)

// fieldsOf devuelve los campos con error, o nil si la configuración es válida.
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

func TestValidate_DefaultsAreValid(t *testing.T) {
	if err := domain.Defaults().Validate(); err != nil {
		t.Fatalf("los defaults deben ser válidos: %v", err)
	}
}

func TestDefaults_Values(t *testing.T) {
	d := domain.Defaults()
	if !d.IsDefault || d.Currency != "PEN" || d.Timezone != "America/Lima" || d.LateFeeMode != "fixed" ||
		d.LateFeeAmount != "10.00" || d.GraceDays != 0 || d.LateFeePerDay ||
		d.AllocationOrder != "late_fee_interest_principal" || d.MaxActiveLoansPerCustomer != 3 ||
		d.AllowNegativeCapital {
		t.Fatalf("defaults inesperados: %+v", d)
	}
}

func TestValidate_Rules(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*domain.Settings)
		field  string
	}{
		{"moneda inválida", func(s *domain.Settings) { s.Currency = "EUR" }, "currency"},
		{"zona inexistente", func(s *domain.Settings) { s.Timezone = "Mars/Olympus" }, "timezone"},
		{"zona vacía", func(s *domain.Settings) { s.Timezone = "" }, "timezone"},
		{"zona Local no fija", func(s *domain.Settings) { s.Timezone = "Local" }, "timezone"},
		{"modo de mora inválido", func(s *domain.Settings) { s.LateFeeMode = "mixed" }, "late_fee_mode"},
		{"monto con letras", func(s *domain.Settings) { s.LateFeeAmount = "abc" }, "late_fee_amount"},
		{"monto negativo", func(s *domain.Settings) { s.LateFeeAmount = "-1" }, "late_fee_amount"},
		{"monto vacío", func(s *domain.Settings) { s.LateFeeAmount = "" }, "late_fee_amount"},
		{"gracia negativa", func(s *domain.Settings) { s.GraceDays = -1 }, "grace_days"},
		{"gracia mayor a 30", func(s *domain.Settings) { s.GraceDays = 31 }, "grace_days"},
		{"orden de aplicación inválido", func(s *domain.Settings) { s.AllocationOrder = "x" }, "allocation_order"},
		{"máximo de préstamos en 0", func(s *domain.Settings) { s.MaxActiveLoansPerCustomer = 0 }, "max_active_loans_per_customer"},
		{"máximo de préstamos en 51", func(s *domain.Settings) { s.MaxActiveLoansPerCustomer = 51 }, "max_active_loans_per_customer"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := domain.Defaults()
			tc.mutate(&s)
			got := fieldsOf(s.Validate())
			if !slices.Equal(got, []string{tc.field}) {
				t.Fatalf("campos con error %v, esperaba [%s]", got, tc.field)
			}
		})
	}
}

func TestValidate_AcceptsBoundaryValues(t *testing.T) {
	s := domain.Defaults()
	s.Timezone = "UTC"
	s.Currency = domain.CurrencyUSD
	s.LateFeeMode = domain.LateFeeProportional
	s.LateFeeAmount = "0"
	s.GraceDays = 30
	s.AllocationOrder = domain.AllocInterestPrincipalLateFee
	s.MaxActiveLoansPerCustomer = 50
	if err := s.Validate(); err != nil {
		t.Fatalf("valores frontera deben ser válidos: %v", err)
	}
}

func TestValidate_ReportsAllFields(t *testing.T) {
	s := domain.Defaults()
	s.Currency, s.GraceDays, s.LateFeeAmount = "XXX", 99, "abc"
	got := fieldsOf(s.Validate())
	want := []string{"currency", "late_fee_amount", "grace_days"}
	if !slices.Equal(got, want) {
		t.Fatalf("campos %v, esperaba %v", got, want)
	}
}
