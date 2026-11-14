// Package httpauth implementa la autenticación híbrida sobre Fiber:
//   - Bearer JWT (apps móviles / servicios): sin CSRF y sin renovación automática.
//   - Cookie HttpOnly (web): exige doble envío CSRF en métodos que modifican estado y
//     renueva la sesión de forma transparente cuando el access token venció.
package httpauth

import (
	"context"
	"crypto/subtle"
	"errors"
	"strings"

	"github.com/gofiber/fiber/v3"

	"prestamos/authkit"
	"prestamos/internal/platform/httpx"
)

const (
	AccessCookie  = "access_token"
	RefreshCookie = "refresh_token"
	CSRFCookie    = "csrf_token"
	CSRFHeader    = "X-CSRF-Token"

	claimsKey = "auth.claims"
)

// TokenParser es el puerto que valida access tokens (lo implementa authkit.TokenIssuer).
type TokenParser interface {
	Parse(raw string) (*authkit.Claims, error)
}

// Authenticate valida la identidad de cada petición. Con Bearer solo valida el token. Con
// cookies, si el access falta o venció y hay refresh_token, rota la sesión en el mismo request
// y escribe las cookies nuevas. La validación CSRF de las mutaciones va ANTES de renovar.
type SessionChecker interface {
	FindByID(context.Context, string, string) (*authkit.Account, error)
}

func Authenticate(parser TokenParser, renewer *Renewer, cookies CookieSettings, checkers ...SessionChecker) fiber.Handler {
	return func(c fiber.Ctx) error {
		if h := c.Get(fiber.HeaderAuthorization); h != "" {
			return authenticateBearer(c, parser, h, checkers)
		}
		return authenticateCookie(c, parser, renewer, cookies, checkers)
	}
}

func authenticateBearer(c fiber.Ctx, parser TokenParser, header string, checkers []SessionChecker) error {
	scheme, tok, ok := strings.Cut(header, " ")
	tok = strings.TrimSpace(tok)
	if !ok || !strings.EqualFold(scheme, "Bearer") || tok == "" {
		return httpx.Unauthenticated()
	}
	claims, err := parser.Parse(tok)
	if err != nil {
		return tokenInvalid()
	}
	return next(c, claims, checkers)
}

func authenticateCookie(c fiber.Ctx, parser TokenParser, renewer *Renewer, cookies CookieSettings, checkers []SessionChecker) error {
	access, refresh := c.Cookies(AccessCookie), c.Cookies(RefreshCookie)
	if access == "" && refresh == "" {
		return httpx.Unauthenticated()
	}
	if !safeMethod(c.Method()) && !validCSRF(c) {
		return httpx.New(fiber.StatusForbidden, httpx.CodeCSRFInvalid, "Falta o es inválido el token CSRF")
	}
	if access != "" {
		claims, err := parser.Parse(access)
		if err == nil {
			return next(c, claims, checkers)
		}
		if !errors.Is(err, authkit.ErrTokenExpired) || refresh == "" {
			return tokenInvalid()
		}
	}
	return renewSession(c, renewer, cookies, refresh, checkers)
}

// renewSession rota la sesión. Si no se puede (token inválido, expirado, usuario o empresa
// eliminados o suspendidos) limpia las cookies y responde 401.
func renewSession(c fiber.Ctx, renewer *Renewer, cookies CookieSettings, refresh string, checkers []SessionChecker) error {
	res, err := renewer.Renew(c.Context(), refresh)
	if err != nil {
		cookies.Clear(c)
		return httpx.New(fiber.StatusUnauthorized, httpx.CodeSessionExpired, "Tu sesión expiró; inicia sesión de nuevo")
	}
	cookies.SetWithCSRF(c, res.Pair, res.CSRF)
	return next(c, res.Claims, checkers)
}

func next(c fiber.Ctx, claims *authkit.Claims, checkers []SessionChecker) error {
	if len(checkers) > 0 {
		account, err := checkers[0].FindByID(c.Context(), claims.TenantID, claims.UserID())
		if err != nil || !account.Active || account.SessionVersion != claims.SessionVersion {
			return tokenInvalid()
		}
		claims.Roles = account.Roles
		c.Locals("auth.must_change_password", account.MustChangePassword)
		if account.MustChangePassword && c.Path() != "/api/v1/auth/me" && c.Path() != "/api/v1/auth/logout" && c.Path() != "/api/v1/auth/change-password" {
			return httpx.New(fiber.StatusForbidden, "PASSWORD_CHANGE_REQUIRED", "Debes cambiar tu contraseña")
		}
	}
	c.Locals(claimsKey, claims)
	return c.Next()
}

func tokenInvalid() error {
	return httpx.New(fiber.StatusUnauthorized, httpx.CodeTokenInvalid, "Tu sesión es inválida o expiró; inicia sesión de nuevo")
}

// RequireRole permite el acceso si el usuario tiene alguno de los roles.
func RequireRole(roles ...string) fiber.Handler {
	return func(c fiber.Ctx) error {
		claims := Claims(c)
		if claims != nil {
			for _, have := range claims.Roles {
				for _, need := range roles {
					if have == need {
						return c.Next()
					}
				}
			}
		}
		return httpx.Forbidden()
	}
}

// Claims devuelve los claims del request autenticado (nil si no hay).
func Claims(c fiber.Ctx) *authkit.Claims {
	cl, _ := c.Locals(claimsKey).(*authkit.Claims)
	return cl
}

func safeMethod(m string) bool {
	return m == fiber.MethodGet || m == fiber.MethodHead || m == fiber.MethodOptions
}

func validCSRF(c fiber.Ctx) bool {
	cookie, header := c.Cookies(CSRFCookie), c.Get(CSRFHeader)
	return cookie != "" && subtle.ConstantTimeCompare([]byte(cookie), []byte(header)) == 1
}
