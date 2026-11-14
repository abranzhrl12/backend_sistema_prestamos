package httpauth

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"time"

	"github.com/gofiber/fiber/v3"

	"prestamos/authkit"
)

const (
	// RefreshPath es el path del refresh_token: toda la API, para que la renovación
	// transparente funcione en cualquier ruta protegida.
	RefreshPath = "/api/v1"
	// legacyRefreshPath es el path con el que se emitían las cookies antes de RefreshPath.
	// Se limpia en cada emisión y en logout para no dejar dos refresh_token en el navegador.
	legacyRefreshPath = "/api/v1/auth"
)

// CookieSettings configura las cookies de sesión web.
type CookieSettings struct {
	Secure      bool
	Domain      string
	RefreshPath string // por defecto RefreshPath; se acepta otro valor para pruebas
}

// Set escribe la sesión completa con un CSRF nuevo (login y /auth/refresh).
func (s CookieSettings) Set(c fiber.Ctx, p *authkit.TokenPair) error {
	csrf, err := randomToken()
	if err != nil {
		return err
	}
	s.SetWithCSRF(c, p, csrf)
	return nil
}

// SetWithCSRF escribe access (HttpOnly), refresh (HttpOnly, path de la API) y csrf (legible
// por JS). La renovación recibe el CSRF ya generado para que las peticiones concurrentes
// que comparten una rotación devuelvan el mismo valor.
func (s CookieSettings) SetWithCSRF(c fiber.Ctx, p *authkit.TokenPair, csrf string) {
	s.put(c, AccessCookie, p.AccessToken, "/", p.AccessExpiresAt, true)
	s.put(c, RefreshCookie, p.RefreshToken, s.refreshPath(), p.RefreshExpiresAt, true)
	s.expireLegacyRefresh(c)
	s.put(c, CSRFCookie, csrf, "/", p.RefreshExpiresAt, false)
}

// Clear caduca las cookies de sesión, incluida la de refresh en el path legado.
func (s CookieSettings) Clear(c fiber.Ctx) {
	past := time.Unix(0, 0)
	s.put(c, AccessCookie, "", "/", past, true)
	s.put(c, RefreshCookie, "", s.refreshPath(), past, true)
	s.expireLegacyRefresh(c)
	s.put(c, CSRFCookie, "", "/", past, false)
}

func (s CookieSettings) refreshPath() string {
	if s.RefreshPath == "" {
		return RefreshPath
	}
	return s.RefreshPath
}

func (s CookieSettings) expireLegacyRefresh(c fiber.Ctx) {
	if s.refreshPath() == legacyRefreshPath {
		return
	}
	s.put(c, RefreshCookie, "", legacyRefreshPath, time.Unix(0, 0), true)
}

// put añade una cabecera Set-Cookie independiente. No usa c.Cookie: fasthttp guarda las cookies
// de respuesta indexadas solo por nombre, y necesitamos dos refresh_token (path nuevo y legado)
// en la misma respuesta.
func (s CookieSettings) put(c fiber.Ctx, name, value, path string, exp time.Time, httpOnly bool) {
	ck := &http.Cookie{
		Name: name, Value: value, Path: path, Domain: s.Domain, Expires: exp,
		Secure: s.Secure, HttpOnly: httpOnly, SameSite: http.SameSiteLaxMode,
	}
	c.Response().Header.Add(fiber.HeaderSetCookie, ck.String())
}

func randomToken() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
