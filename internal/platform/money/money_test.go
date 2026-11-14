package money_test

import (
	"errors"
	"testing"

	"github.com/shopspring/decimal"

	"prestamos/internal/platform/money"
)

func TestParse_Valid(t *testing.T) {
	for _, in := range []string{"0", "10", "1500.5", "1500.50", "0.0001", "999999999999999.9999"} {
		if _, err := money.Parse(in); err != nil {
			t.Errorf("%q debería ser válido: %v", in, err)
		}
	}
}

func TestParse_Invalid(t *testing.T) {
	for _, in := range []string{"", "abc", "1,5", "1e3", "1.23456", " 1", "1 ", ".5", "5.", "+5", "1000000000000000"} {
		if _, err := money.Parse(in); !errors.Is(err, money.ErrFormat) {
			t.Errorf("%q debería dar ErrFormat, got %v", in, err)
		}
	}
	if _, err := money.Parse("-5"); !errors.Is(err, money.ErrNegative) {
		t.Errorf("-5 debería dar ErrNegative, got %v", err)
	}
}

func TestParsePositive_RejectsZero(t *testing.T) {
	if _, err := money.ParsePositive("0.00"); !errors.Is(err, money.ErrZero) {
		t.Fatalf("esperaba ErrZero, got %v", err)
	}
}

func TestFormat(t *testing.T) {
	cases := map[string]string{"1500": "1500.00", "1500.5": "1500.50", "0.1234": "0.1234", "10.005": "10.0050"}
	for in, want := range cases {
		if got := money.Format(decimal.RequireFromString(in)); got != want {
			t.Errorf("Format(%s)=%s, esperaba %s", in, got, want)
		}
	}
}

func TestNoFloatDrift(t *testing.T) {
	a, _ := money.Parse("0.1")
	b, _ := money.Parse("0.2")
	if got := money.Format(a.Add(b)); got != "0.30" {
		t.Fatalf("0.1+0.2 = %s, esperaba 0.30", got)
	}
}
