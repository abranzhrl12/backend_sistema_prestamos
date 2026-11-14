package obs

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/requestid"

	"prestamos/internal/platform/httpauth"
)

const (
	healthzPath   = "/healthz"
	errorCodeKey  = "error.code"
	msPerDuration = float64(time.Millisecond)
)

// AccessLog escribe un log por request (msg http_request).
// Fiber ejecuta el ErrorHandler DESPUÉS de la cadena de middlewares, así que aquí
// se resuelve el error antes de medir el status final.
func AccessLog(log *slog.Logger) fiber.Handler {
	return func(c fiber.Ctx) error {
		start := time.Now()
		if err := c.Next(); err != nil {
			if herr := c.App().Config().ErrorHandler(c, err); herr != nil {
				_ = c.SendStatus(http.StatusInternalServerError)
			}
		}
		logRequest(log, c, time.Since(start))
		return nil
	}
}

func logRequest(log *slog.Logger, c fiber.Ctx, elapsed time.Duration) {
	status := c.Response().StatusCode()
	attrs := []any{
		"request_id", requestid.FromContext(c),
		"module", ModuleOf(c),
		"method", c.Method(),
		"route", c.Route().Path,
		"path", c.Path(),
		"status", status,
		"latency_ms", float64(elapsed) / msPerDuration,
		"bytes", len(c.Response().Body()),
		"ip", c.IP(),
	}
	if cl := httpauth.Claims(c); cl != nil {
		attrs = append(attrs, "tenant_id", cl.TenantID, "user_id", cl.UserID())
	}
	if code, ok := c.Locals(errorCodeKey).(string); ok && code != "" {
		attrs = append(attrs, "error_code", code)
	}
	log.Log(c.Context(), levelFor(c.Path(), status), "http_request", attrs...)
}

func levelFor(path string, status int) slog.Level {
	switch {
	case path == healthzPath:
		return slog.LevelDebug
	case status >= http.StatusInternalServerError:
		return slog.LevelError
	case status >= http.StatusBadRequest:
		return slog.LevelWarn
	default:
		return slog.LevelInfo
	}
}
