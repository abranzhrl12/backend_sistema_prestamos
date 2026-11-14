package domain

import (
	"strings"
	"unicode/utf8"

	"prestamos/internal/platform/money"
)

const (
	maxNoteRunes      = 500
	maxReferenceRunes = 100
	minAdjustmentNote = 5
)

// Input son los datos crudos de un movimiento manual (tal como llegan del adaptador).
type Input struct {
	Type      string
	Amount    string
	Note      string
	Reference string
}

// NewManualMovement valida la entrada manual y construye el movimiento (sin ID ni tenant).
func NewManualMovement(in Input) (Movement, error) {
	var issues []Issue
	t := Type(strings.TrimSpace(in.Type))
	if !t.Manual() {
		issues = append(issues, Issue{"type", "Tipo inválido. Permitidos: contribution, withdrawal, adjustment_in, adjustment_out"})
	}
	amount, err := money.ParsePositive(strings.TrimSpace(in.Amount))
	if err != nil {
		issues = append(issues, Issue{"amount", err.Error()})
	}
	note := strings.TrimSpace(in.Note)
	issues = append(issues, validateNote(t, note)...)
	ref := strings.TrimSpace(in.Reference)
	if utf8.RuneCountInString(ref) > maxReferenceRunes {
		issues = append(issues, Issue{"reference", "La referencia no puede superar 100 caracteres"})
	}
	if len(issues) > 0 {
		return Movement{}, &ValidationError{Issues: issues}
	}
	return Movement{Type: t, Amount: amount, Note: note, Reference: ref}, nil
}

// validateNote: la nota es obligatoria en los ajustes porque explica una corrección del libro.
func validateNote(t Type, note string) []Issue {
	n := utf8.RuneCountInString(note)
	switch {
	case n > maxNoteRunes:
		return []Issue{{"note", "La nota no puede superar 500 caracteres"}}
	case isAdjustment(t) && n < minAdjustmentNote:
		return []Issue{{"note", "La nota es obligatoria en los ajustes (mínimo 5 caracteres)"}}
	}
	return nil
}

func isAdjustment(t Type) bool {
	return t == TypeAdjustmentIn || t == TypeAdjustmentOut
}

// RequiresFunds indica si el movimiento saca dinero de la caja y debe comprobarse el saldo.
func (m Movement) RequiresFunds() bool { return m.Direction() == DirectionOut }
