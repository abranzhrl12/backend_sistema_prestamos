package domain_test

import (
	"errors"
	"strings"
	"testing"

	"prestamos/internal/capital/domain"
)

// fieldsOf devuelve los campos con error (nil si la entrada es válida).
func fieldsOf(t *testing.T, err error) map[string]bool {
	t.Helper()
	if err == nil {
		return nil
	}
	var ve *domain.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("esperaba ValidationError, got %v", err)
	}
	out := map[string]bool{}
	for _, is := range ve.Issues {
		out[is.Field] = true
	}
	return out
}

func TestInput_AcceptsEachManualType(t *testing.T) {
	for _, typ := range []string{"contribution", "withdrawal", "adjustment_in", "adjustment_out"} {
		note := ""
		if strings.HasPrefix(typ, "adjustment") {
			note = "Corrección de arqueo"
		}
		if _, err := domain.NewManualMovement(domain.Input{Type: typ, Amount: "10.00", Note: note}); err != nil {
			t.Errorf("%s: no debería fallar: %v", typ, err)
		}
	}
}

func TestInput_RejectsNonManualTypes(t *testing.T) {
	for _, typ := range []string{"disbursement", "principal_collection", "expense", "inventado", ""} {
		_, err := domain.NewManualMovement(domain.Input{Type: typ, Amount: "10"})
		if f := fieldsOf(t, err); !f["type"] {
			t.Errorf("%q: esperaba error en type, got %v", typ, err)
		}
	}
}

func TestInput_AmountRules(t *testing.T) {
	bad := []string{"abc", "-5", "0", "1.23456", "", "1,50"}
	for _, a := range bad {
		_, err := domain.NewManualMovement(domain.Input{Type: "contribution", Amount: a})
		if f := fieldsOf(t, err); !f["amount"] {
			t.Errorf("monto %q: esperaba error en amount, got %v", a, err)
		}
	}
	if _, err := domain.NewManualMovement(domain.Input{Type: "contribution", Amount: "1000.5000"}); err != nil {
		t.Fatalf("1000.5000 debería ser válido: %v", err)
	}
}

func TestInput_NoteRequiredForAdjustments(t *testing.T) {
	for _, typ := range []string{"adjustment_in", "adjustment_out"} {
		_, err := domain.NewManualMovement(domain.Input{Type: typ, Amount: "5", Note: "  abc  "})
		if f := fieldsOf(t, err); !f["note"] {
			t.Errorf("%s con nota de 3 caracteres: esperaba error en note", typ)
		}
		if _, err := domain.NewManualMovement(domain.Input{Type: typ, Amount: "5", Note: ""}); err == nil {
			t.Errorf("%s sin nota debería fallar", typ)
		}
	}
}

func TestInput_NoteOptionalForContributionAndWithdrawal(t *testing.T) {
	for _, typ := range []string{"contribution", "withdrawal"} {
		if _, err := domain.NewManualMovement(domain.Input{Type: typ, Amount: "5"}); err != nil {
			t.Errorf("%s sin nota no debería fallar: %v", typ, err)
		}
	}
}

func TestInput_NoteAndReferenceLengthLimits(t *testing.T) {
	long := strings.Repeat("ñ", 501)
	_, err := domain.NewManualMovement(domain.Input{Type: "contribution", Amount: "5", Note: long})
	if f := fieldsOf(t, err); !f["note"] {
		t.Error("una nota de 501 caracteres debería fallar")
	}
	_, err = domain.NewManualMovement(domain.Input{Type: "contribution", Amount: "5", Reference: strings.Repeat("r", 101)})
	if f := fieldsOf(t, err); !f["reference"] {
		t.Error("una referencia de 101 caracteres debería fallar")
	}
	if _, err := domain.NewManualMovement(domain.Input{Type: "contribution", Amount: "5", Note: strings.Repeat("n", 500)}); err != nil {
		t.Errorf("una nota de 500 caracteres debería ser válida: %v", err)
	}
}

func TestInput_ReportsEveryInvalidField(t *testing.T) {
	_, err := domain.NewManualMovement(domain.Input{Type: "disbursement", Amount: "abc"})
	f := fieldsOf(t, err)
	if !f["type"] || !f["amount"] {
		t.Fatalf("esperaba type y amount juntos, got %v", err)
	}
}
