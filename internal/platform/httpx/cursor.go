package httpx

import (
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

// CursorParams son los parámetros de paginación por cursor (?limit=&after=).
// After es el id del último elemento visto; vacío = primera página.
type CursorParams struct {
	Limit int
	After string
}

// ParseCursor lee y valida limit/after. Devuelve VALIDATION_ERROR con todos los campos inválidos.
func ParseCursor(c fiber.Ctx) (CursorParams, error) {
	p := CursorParams{
		Limit: fiber.Query[int](c, "limit", DefaultLimit),
		After: c.Query("after"),
	}
	var issues []FieldError
	if p.Limit < 1 || p.Limit > MaxLimit {
		issues = append(issues, FieldError{"limit", "Debe estar entre 1 y 100"})
	}
	if p.After != "" {
		if _, err := uuid.Parse(p.After); err != nil {
			issues = append(issues, FieldError{"after", "Debe ser un identificador válido (UUID)"})
		}
	}
	if len(issues) > 0 {
		return p, Validation(issues...)
	}
	return p, nil
}

// CursorMeta describe la página. NextCursor es el id del último item si hay más páginas; si no, nil.
type CursorMeta struct {
	Limit      int     `json:"limit"`
	NextCursor *string `json:"next_cursor"`
	HasMore    bool    `json:"has_more"`
}

// CursorPage es la envoltura de los listados append-only paginados por cursor.
type CursorPage[T any] struct {
	Items []T        `json:"items"`
	Meta  CursorMeta `json:"meta"`
}

// NewCursorPage construye la respuesta; Items nunca es null (siempre []).
// idOf extrae el id de un elemento para usarlo como next_cursor.
func NewCursorPage[T any](items []T, p CursorParams, hasMore bool, idOf func(T) string) CursorPage[T] {
	if items == nil {
		items = []T{}
	}
	meta := CursorMeta{Limit: p.Limit, HasMore: hasMore}
	if hasMore && len(items) > 0 {
		next := idOf(items[len(items)-1])
		meta.NextCursor = &next
	}
	return CursorPage[T]{Items: items, Meta: meta}
}
