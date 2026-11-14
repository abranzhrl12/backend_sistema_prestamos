package obs

import (
	"fmt"
	"log/slog"
	"runtime/debug"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/gofiber/fiber/v3/middleware/requestid"
)

// Recover captura panics y los registra con stack y módulo. El panic termina como
// error normal, así que el cliente recibe 500 INTERNAL_ERROR vía ErrorHandler.
func Recover(log *slog.Logger) fiber.Handler {
	return recover.New(recover.Config{
		EnableStackTrace: true,
		StackTraceHandler: func(c fiber.Ctx, r any) {
			log.Error("panic",
				"request_id", requestid.FromContext(c),
				"module", ModuleOf(c),
				"panic", fmt.Sprint(r),
				"stack", string(debug.Stack()),
			)
		},
	})
}
