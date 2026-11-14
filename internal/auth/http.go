package auth

import (
	"context"
	"errors"
	"time"

	"github.com/gofiber/fiber/v3"

	"prestamos/authkit"
	"prestamos/internal/platform/httpauth"
	"prestamos/internal/platform/httpx"
)

// Puertos que necesita el handler (los implementan authkit.Service y Store).
type authService interface {
	Login(ctx context.Context, tenantID, login, password string) (*authkit.TokenPair, *authkit.Account, error)
	Refresh(ctx context.Context, refreshToken string) (*authkit.TokenPair, *authkit.Account, error)
	Logout(ctx context.Context, tenantID, userID string) error
}

type tenantResolver interface {
	ResolveTenant(ctx context.Context, slug string) (string, error)
	ResolveEmail(ctx context.Context, email string) (string, error)
	ChangePassword(ctx context.Context, tenantID, userID, current, next string) error
}

type Handler struct {
	svc     authService
	tenants tenantResolver
	authn   fiber.Handler // middleware de autenticación inyectado
	cookies httpauth.CookieSettings
}

func NewHandler(svc authService, tenants tenantResolver, authn fiber.Handler, cookies httpauth.CookieSettings) *Handler {
	return &Handler{svc: svc, tenants: tenants, authn: authn, cookies: cookies}
}

// Register monta las rutas bajo el grupo dado (p. ej. /api/v1/auth).
func (h *Handler) Register(g fiber.Router) {
	g.Post("/login", h.Login)
	g.Post("/refresh", h.Refresh)
	g.Post("/logout", h.authn, h.Logout)
	g.Post("/change-password", h.authn, h.ChangePassword)
	g.Get("/me", h.authn, h.Me)
}

type LoginRequest struct {
	Tenant    string `json:"tenant,omitempty" example:"acme"`
	Email     string `json:"email" validate:"required,email" example:"admin@acme.com"`
	Password  string `json:"password" validate:"required" example:"s3cret"`
	Transport string `json:"transport" enums:"cookie,bearer" example:"cookie"` // cookie (web) | bearer (default)
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"` // solo para clientes bearer; web usa cookie
}

type TokenResponse struct {
	AccessToken      string    `json:"access_token,omitempty"`
	AccessExpiresAt  time.Time `json:"access_expires_at"`
	RefreshToken     string    `json:"refresh_token,omitempty"`
	RefreshExpiresAt time.Time `json:"refresh_expires_at"`
	TokenType        string    `json:"token_type"`
}

type MeResponse struct {
	UserID             string   `json:"user_id"`
	TenantID           string   `json:"tenant_id"`
	Roles              []string `json:"roles"`
	MustChangePassword bool     `json:"must_change_password"`
}

// Login godoc
// @Summary  Iniciar sesión (híbrido cookie/bearer)
// @Tags     auth
// @Accept   json
// @Produce  json
// @Param    body body LoginRequest true "credenciales"
// @Success  200 {object} TokenResponse
// @Failure  400 {object} httpx.ErrorResponse
// @Failure  401 {object} httpx.ErrorResponse
// @Router   /auth/login [post]
func (h *Handler) Login(c fiber.Ctx) error {
	var req LoginRequest
	if err := c.Bind().Body(&req); err != nil {
		return httpx.InvalidJSON()
	}
	if err := validateLogin(req); err != nil {
		return err
	}
	var tenantID string
	var err error
	if req.Tenant != "" {
		tenantID, err = h.tenants.ResolveTenant(c.Context(), req.Tenant)
	} else {
		tenantID, err = h.tenants.ResolveEmail(c.Context(), req.Email)
	}
	if errors.Is(err, authkit.ErrNotFound) {
		_, _, _ = h.svc.Login(c.Context(), "00000000-0000-0000-0000-000000000000", req.Email, req.Password)
		return errInvalidCredentials
	}
	if err != nil {
		return err
	}
	pair, _, err := h.svc.Login(c.Context(), tenantID, req.Email, req.Password)
	if err != nil {
		return mapErr(err)
	}
	return h.respond(c, pair, req.Transport == "cookie")
}

// Refresh godoc
// @Summary  Rotar refresh token
// @Tags     auth
// @Accept   json
// @Produce  json
// @Param    body body RefreshRequest false "solo clientes bearer"
// @Success  200 {object} TokenResponse
// @Router   /auth/refresh [post]
func (h *Handler) Refresh(c fiber.Ctx) error {
	useCookie := false
	token := c.Cookies(httpauth.RefreshCookie)
	if token != "" {
		useCookie = true
	} else {
		var req RefreshRequest
		_ = c.Bind().Body(&req)
		token = req.RefreshToken
	}
	pair, _, err := h.svc.Refresh(c.Context(), token)
	if err != nil {
		if useCookie {
			h.cookies.Clear(c)
		}
		return mapErr(err)
	}
	return h.respond(c, pair, useCookie)
}

// Logout godoc
// @Summary  Cerrar sesión (revoca refresh tokens)
// @Tags     auth
// @Security BearerAuth
// @Success  204
// @Router   /auth/logout [post]
func (h *Handler) Logout(c fiber.Ctx) error {
	cl := httpauth.Claims(c)
	if err := h.svc.Logout(c.Context(), cl.TenantID, cl.UserID()); err != nil {
		return err
	}
	h.cookies.Clear(c)
	return c.SendStatus(fiber.StatusNoContent)
}

// Me godoc
// @Summary  Usuario actual
// @Tags     auth
// @Security BearerAuth
// @Produce  json
// @Success  200 {object} MeResponse
// @Router   /auth/me [get]
func (h *Handler) Me(c fiber.Ctx) error {
	cl := httpauth.Claims(c)
	return c.JSON(MeResponse{UserID: cl.UserID(), TenantID: cl.TenantID, Roles: cl.Roles, MustChangePassword: c.Locals("auth.must_change_password") == true})
}

func (h *Handler) respond(c fiber.Ctx, p *authkit.TokenPair, cookie bool) error {
	resp := TokenResponse{AccessExpiresAt: p.AccessExpiresAt, RefreshExpiresAt: p.RefreshExpiresAt, TokenType: "Bearer"}
	if cookie {
		if err := h.cookies.Set(c, p); err != nil {
			return err
		}
	} else {
		resp.AccessToken, resp.RefreshToken = p.AccessToken, p.RefreshToken
	}
	return c.JSON(resp)
}

var errInvalidCredentials = httpx.New(fiber.StatusUnauthorized, httpx.CodeInvalidCreds, "Correo o contraseña incorrectos")

func validateLogin(r LoginRequest) error {
	var issues []httpx.FieldError
	if r.Email == "" {
		issues = append(issues, httpx.FieldError{Field: "email", Message: "El correo es obligatorio"})
	}
	if r.Password == "" {
		issues = append(issues, httpx.FieldError{Field: "password", Message: "La contraseña es obligatoria"})
	}
	if len(issues) > 0 {
		return httpx.Validation(issues...)
	}
	return nil
}

func mapErr(err error) error {
	switch {
	case errors.Is(err, authkit.ErrInvalidCredentials):
		return errInvalidCredentials
	case errors.Is(err, authkit.ErrInvalidRefresh):
		return httpx.New(fiber.StatusUnauthorized, httpx.CodeSessionExpired, "Tu sesión expiró; inicia sesión de nuevo")
	default:
		return err
	}
}
