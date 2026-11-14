package app_test

import (
	"context"
	"testing"

	"prestamos/internal/settings/app"
	"prestamos/internal/settings/domain"
)

func TestAllowNegativeCapital_DefaultsToFalse(t *testing.T) {
	s := app.NewService(&fakeRepo{}, &seqIDs{})
	ok, err := s.AllowNegativeCapital(context.Background(), "t1")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("por defecto no se permite capital negativo")
	}
}

func TestAllowNegativeCapital_RespectsStoredValue(t *testing.T) {
	d := domain.Defaults()
	d.AllowNegativeCapital = true
	s := app.NewService(&fakeRepo{row: &d}, &seqIDs{})
	ok, err := s.AllowNegativeCapital(context.Background(), "t1")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("debe respetar el valor guardado (true)")
	}
}
