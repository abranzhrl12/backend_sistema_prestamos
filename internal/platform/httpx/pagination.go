package httpx

import (
	"github.com/gofiber/fiber/v3"
)

const (
	DefaultLimit = 20
	MaxLimit     = 100
)

// PageParams son los parámetros de paginación por desplazamiento (?limit=&offset=).
type PageParams struct{ Limit, Offset int }

// ParsePage lee y valida limit/offset. Un valor inválido devuelve VALIDATION_ERROR con el campo afectado.
func ParsePage(c fiber.Ctx) (PageParams, error) {
	p := PageParams{
		Limit:  fiber.Query[int](c, "limit", DefaultLimit),
		Offset: fiber.Query[int](c, "offset", 0),
	}
	var issues []FieldError
	if p.Limit < 1 || p.Limit > MaxLimit {
		issues = append(issues, FieldError{"limit", "Debe estar entre 1 y 100"})
	}
	if p.Offset < 0 {
		issues = append(issues, FieldError{"offset", "No puede ser negativo"})
	}
	if len(issues) > 0 {
		return p, Validation(issues...)
	}
	return p, nil
}

// PageMeta describe la página devuelta. Total solo aparece si se pidió ?include_total=true
// (contar toda la tabla en cada página es el costo dominante en listados grandes).
type PageMeta struct {
	Total   *int `json:"total,omitempty"`
	Limit   int  `json:"limit"`
	Offset  int  `json:"offset"`
	HasMore bool `json:"has_more"`
}

// Page es la envoltura estándar de todos los listados.
type Page[T any] struct {
	Items []T      `json:"items"`
	Meta  PageMeta `json:"meta"`
}

// NewPage construye la respuesta; Items nunca es null (siempre []).
func NewPage[T any](items []T, hasMore bool, total *int, p PageParams) Page[T] {
	if items == nil {
		items = []T{}
	}
	return Page[T]{Items: items, Meta: PageMeta{Total: total, Limit: p.Limit, Offset: p.Offset, HasMore: hasMore}}
}

// WantTotal indica si el cliente pidió el total exacto (?include_total=true).
func WantTotal(c fiber.Ctx) bool { return fiber.Query[bool](c, "include_total", false) }
