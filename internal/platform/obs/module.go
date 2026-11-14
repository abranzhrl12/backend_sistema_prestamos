package obs

import (
	"context"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/requestid"
)

// ModuleKey es la clave en c.Locals donde se guarda el nombre del módulo del request.
const ModuleKey = "obs.module"

// CoreModule es el módulo asignado a los requests que no pasan por un grupo de módulo.
const CoreModule = "core"

// Meta viaja en el context.Context para que el código de dominio y la BD registren
// request y módulo sin depender de Fiber.
type Meta struct {
	RequestID string
	Module    string
}

type metaKey struct{}

// WithMeta devuelve un context con la metadata de trazabilidad.
func WithMeta(ctx context.Context, m Meta) context.Context {
	return context.WithValue(ctx, metaKey{}, m)
}

// MetaFrom devuelve la metadata de trazabilidad (vacía si no existe).
func MetaFrom(ctx context.Context) Meta {
	m, _ := ctx.Value(metaKey{}).(Meta)
	return m
}

// Context guarda el request_id en el context.Context. Debe registrarse justo después de requestid.
func Context() fiber.Handler {
	return func(c fiber.Ctx) error {
		c.SetContext(WithMeta(c.Context(), Meta{RequestID: requestid.FromContext(c)}))
		return c.Next()
	}
}

// Module fija el módulo del request (middleware de grupo, p. ej. api.Group("/customers", obs.Module("customers"))).
func Module(name string) fiber.Handler {
	return func(c fiber.Ctx) error {
		c.Locals(ModuleKey, name)
		m := MetaFrom(c.Context())
		m.Module = name
		if m.RequestID == "" {
			m.RequestID = requestid.FromContext(c)
		}
		c.SetContext(WithMeta(c.Context(), m))
		return c.Next()
	}
}

// ModuleOf devuelve el módulo del request, o "core" si no tiene.
func ModuleOf(c fiber.Ctx) string {
	if m, ok := c.Locals(ModuleKey).(string); ok && m != "" {
		return m
	}
	return CoreModule
}
