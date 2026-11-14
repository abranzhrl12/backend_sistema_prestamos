package http

import (
	"github.com/gofiber/fiber/v3"

	"prestamos/internal/platform/httpauth"
	"prestamos/internal/platform/httpx"
	"prestamos/internal/tenants/app"
	"prestamos/internal/tenants/domain"
)

type Handler struct{ svc *app.Service }

func NewHandler(svc *app.Service) *Handler { return &Handler{svc: svc} }

// Register monta las rutas bajo el grupo /admin. TODAS exigen sesión y rol superadmin
// (authn antes que RequireRole: el orden de los middlewares importa).
func (h *Handler) Register(g fiber.Router, authn fiber.Handler) {
	super := httpauth.RequireRole(domain.RoleSuperadmin)
	g.Post("/tenants", authn, super, h.CreateTenant)
	g.Get("/tenants", authn, super, h.ListTenants)
	g.Get("/tenants/:id", authn, super, h.GetTenant)
	g.Patch("/tenants/:id", authn, super, h.UpdateTenant)
	g.Delete("/tenants/:id", authn, super, h.DeleteTenant)
	g.Post("/tenants/:id/restore", authn, super, h.RestoreTenant)
	g.Get("/tenants/:id/users", authn, super, h.ListUsers)
	g.Post("/tenants/:id/users", authn, super, h.CreateUser)
	g.Patch("/tenants/:id/users/:userId", authn, super, h.UpdateUser)
	g.Delete("/tenants/:id/users/:userId", authn, super, h.DeleteUser)
	g.Post("/tenants/:id/users/:userId/restore", authn, super, h.RestoreUser)
	g.Post("/tenants/:id/users/:userId/reset-password", authn, super, h.ResetPassword)
}

// CreateTenant godoc
// @Summary  Crear empresa con su owner (solo superadmin)
// @Tags     admin
// @Security BearerAuth
// @Accept   json
// @Produce  json
// @Param    body body CreateTenantRequest true "empresa y owner"
// @Success  201 {object} CreateTenantResponse
// @Failure  400 {object} httpx.ErrorResponse
// @Failure  401 {object} httpx.ErrorResponse
// @Failure  403 {object} httpx.ErrorResponse
// @Failure  409 {object} httpx.ErrorResponse
// @Router   /admin/tenants [post]
func (h *Handler) CreateTenant(c fiber.Ctx) error {
	var req CreateTenantRequest
	if err := c.Bind().Body(&req); err != nil {
		return httpx.InvalidJSON()
	}
	temporary := ""
	if req.Owner.Password == "" {
		var err error
		temporary, err = temporaryPassword()
		if err != nil {
			return err
		}
		req.Owner.Password = temporary
	}
	in := domain.NewTenant{Slug: req.Slug, Name: req.Name,
		Owner: domain.NewUser{Email: req.Owner.Email, FullName: req.Owner.FullName, Password: req.Owner.Password}}
	t, owner, err := h.svc.CreateTenant(c.Context(), httpauth.Claims(c).UserID(), in)
	if err != nil {
		return mapErr(err)
	}
	return c.Status(fiber.StatusCreated).JSON(CreateTenantResponse{TenantDetail: tenantDetail(t), Owner: userSummary(owner), TemporaryPassword: temporary})
}

// ListTenants godoc
// @Summary  Listar empresas (solo superadmin)
// @Tags     admin
// @Security BearerAuth
// @Produce  json
// @Param    q       query string false "slug o nombre"
// @Param    active  query bool   false "true|false"
// @Param    limit   query int    false "1-100 (def. 20)"
// @Param    offset  query int    false "desplazamiento"
// @Param    include_total query bool false "calcular total exacto (más lento)"
// @Success  200 {object} httpx.Page[TenantSummary]
// @Failure  400 {object} httpx.ErrorResponse
// @Failure  403 {object} httpx.ErrorResponse
// @Router   /admin/tenants [get]
func (h *Handler) ListTenants(c fiber.Ctx) error {
	page, err := httpx.ParsePage(c)
	if err != nil {
		return err
	}
	active, err := activeFilter(c)
	if err != nil {
		return err
	}
	res, err := h.svc.ListTenants(c.Context(), domain.TenantFilter{Query: c.Query("q"), Active: active,
		WithTotal: httpx.WantTotal(c), Limit: page.Limit, Offset: page.Offset})
	if err != nil {
		return mapErr(err)
	}
	out := make([]TenantSummary, 0, len(res.Items))
	for i := range res.Items {
		out = append(out, tenantSummary(&res.Items[i]))
	}
	return c.JSON(httpx.NewPage(out, res.HasMore, res.Total, page))
}

// GetTenant godoc
// @Summary  Obtener empresa con su número de usuarios
// @Tags     admin
// @Security BearerAuth
// @Produce  json
// @Param    id path string true "ID de la empresa"
// @Success  200 {object} TenantDetail
// @Failure  404 {object} httpx.ErrorResponse
// @Router   /admin/tenants/{id} [get]
func (h *Handler) GetTenant(c fiber.Ctx) error {
	id, err := uuidParam(c, "id", msgTenantNotFound)
	if err != nil {
		return err
	}
	t, err := h.svc.GetTenant(c.Context(), id)
	if err != nil {
		return mapErr(err)
	}
	return c.JSON(tenantDetail(t))
}

// UpdateTenant godoc
// @Summary  Cambiar nombre y/o estado de la empresa (desactivar corta su acceso)
// @Tags     admin
// @Security BearerAuth
// @Accept   json
// @Produce  json
// @Param    id   path string true "ID de la empresa"
// @Param    body body UpdateTenantRequest true "campos a cambiar"
// @Success  200 {object} TenantDetail
// @Failure  400 {object} httpx.ErrorResponse
// @Failure  404 {object} httpx.ErrorResponse
// @Failure  409 {object} httpx.ErrorResponse
// @Router   /admin/tenants/{id} [patch]
func (h *Handler) UpdateTenant(c fiber.Ctx) error {
	id, err := uuidParam(c, "id", msgTenantNotFound)
	if err != nil {
		return err
	}
	var req UpdateTenantRequest
	if err := c.Bind().Body(&req); err != nil {
		return httpx.InvalidJSON()
	}
	t, err := h.svc.UpdateTenant(c.Context(), httpauth.Claims(c).UserID(), id,
		domain.TenantPatch{Name: req.Name, Active: req.Active})
	if err != nil {
		return mapErr(err)
	}
	return c.JSON(tenantDetail(t))
}

// DeleteTenant godoc
// @Summary  Eliminar empresa (soft delete): bloquea sus logins y cierra sus sesiones (solo superadmin)
// @Tags     admin
// @Security BearerAuth
// @Param    id path string true "ID de la empresa"
// @Success  204
// @Failure  404 {object} httpx.ErrorResponse
// @Failure  409 {object} httpx.ErrorResponse
// @Router   /admin/tenants/{id} [delete]
func (h *Handler) DeleteTenant(c fiber.Ctx) error {
	id, err := uuidParam(c, "id", msgTenantNotFound)
	if err != nil {
		return err
	}
	if err := h.svc.DeleteTenant(c.Context(), httpauth.Claims(c).UserID(), id); err != nil {
		return mapErr(err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// RestoreTenant godoc
// @Summary  Restaurar empresa eliminada (solo superadmin)
// @Tags     admin
// @Security BearerAuth
// @Produce  json
// @Param    id path string true "ID de la empresa"
// @Success  200 {object} TenantDetail
// @Failure  404 {object} httpx.ErrorResponse
// @Router   /admin/tenants/{id}/restore [post]
func (h *Handler) RestoreTenant(c fiber.Ctx) error {
	id, err := uuidParam(c, "id", msgTenantNotFound)
	if err != nil {
		return err
	}
	t, err := h.svc.RestoreTenant(c.Context(), httpauth.Claims(c).UserID(), id)
	if err != nil {
		return mapErr(err)
	}
	return c.JSON(tenantDetail(t))
}
