// Package ids provee generadores de identificadores inyectables.
package ids

import "github.com/google/uuid"

// Generator crea identificadores únicos (puerto; se inyecta en los casos de uso).
type Generator interface{ New() (string, error) }

// UUIDv7 genera UUID v7 (ordenables por tiempo).
type UUIDv7 struct{}

func (UUIDv7) New() (string, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return "", err
	}
	return id.String(), nil
}
