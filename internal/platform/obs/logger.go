// Package obs concentra la trazabilidad: logs estructurados por request, middleware de
// módulo, recuperación de panics y tracer de consultas lentas o fallidas.
package obs

import (
	"io"
	"log/slog"
)

const serviceName = "prestamos"

// NewLogger crea el logger de la aplicación. format: "json" (por defecto) o "text";
// level: debug, info, warn o error (cualquier otro valor equivale a info).
func NewLogger(w io.Writer, level, format string) *slog.Logger {
	opts := &slog.HandlerOptions{Level: parseLevel(level)}
	var h slog.Handler = slog.NewJSONHandler(w, opts)
	if format == "text" {
		h = slog.NewTextHandler(w, opts)
	}
	return slog.New(h).With("service", serviceName)
}

func parseLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
