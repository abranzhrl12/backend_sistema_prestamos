package http

import (
	"github.com/gofiber/fiber/v3"

	"prestamos/internal/platform/httpauth"
	"prestamos/internal/platform/httpx"
	"prestamos/internal/tenants/domain"
)

// ListUsers godoc
// @Summary  Listar usuarios de una empresa (solo superadmin)
// @Tags     admin
// @Security BearerAuth
// @Produce  json
// @Param    id      path  string true  "ID de la empresa"
// @Param    q       query string false "correo o nombre"
// @Param    active  query bool   false "true|false"
// @Param    limit   query int    false "1-100 (def. 20)"
// @Param    offset  query int    false "desplazamiento"
// @Param    include_total query bool false "calcular total exacto (más lento)"
// @Success  200 {object} httpx.Page[UserSummary]
// @Failure  404 {object} httpx.ErrorResponse
// @Router   /admin/tenants/{id}/users [get]
func (h *Handler) ListUsers(c fiber.Ctx) error {
	tenantID, err := uuidParam(c, "id", msgTenantNotFound)
	if err != nil {
		return err
	}
	page, err := httpx.ParsePage(c)
	if err != nil {
		return err
	}
	active, err := activeFilter(c)
	if err != nil {
		return err
	}
	res, err := h.svc.ListUsers(c.Context(), tenantID, domain.UserFilter{Query: c.Query("q"), Active: active,
		WithTotal: httpx.WantTotal(c), Limit: page.Limit, Offset: page.Offset})
	if err != nil {
		return mapErr(err)
	}
	out := make([]UserSummary, 0, len(res.Items))
	for i := range res.Items {
		out = append(out, userSummary(&res.Items[i]))
	}
	return c.JSON(httpx.NewPage(out, res.HasMore, res.Total, page))
}

// CreateUser godoc
// @Summary  Crear usuario en una empresa (solo superadmin)
// @Tags     admin
// @Security BearerAuth
// @Accept   json
// @Produce  json
// @Param    id   path string true "ID de la empresa"
// @Param    body body CreateUserRequest true "datos del usuario"
// @Success  201 {object} UserSummary
// @Failure  400 {object} httpx.ErrorResponse
// @Failure  404 {object} httpx.ErrorResponse
// @Failure  409 {object} httpx.ErrorResponse
// @Router   /admin/tenants/{id}/users [post]
func (h *Handler) CreateUser(c fiber.Ctx) error {
	tenantID, err := uuidParam(c, "id", msgTenantNotFound)
	if err != nil {
		return err
	}
	var req CreateUserRequest
	if err := c.Bind().Body(&req); err != nil {
		return httpx.InvalidJSON()
	}
	temporary := ""
	if req.Password == "" {
		temporary, err = temporaryPassword()
		if err != nil {
			return err
		}
		req.Password = temporary
	}
	in := domain.NewUser{Email: req.Email, FullName: req.FullName, Password: req.Password, Roles: req.Roles}
	u, err := h.svc.CreateUser(c.Context(), httpauth.Claims(c).UserID(), tenantID, in)
	if err != nil {
		return mapErr(err)
	}
	return c.Status(fiber.StatusCreated).JSON(struct {
		UserSummary
		TemporaryPassword string `json:"temporary_password,omitempty"`
	}{userSummary(u), temporary})
}

// UpdateUser godoc
// @Summary  Actualizar usuario: nombre, roles, estado o contraseña (solo superadmin)
// @Tags     admin
// @Security BearerAuth
// @Accept   json
// @Produce  json
// @Param    id     path string true "ID de la empresa"
// @Param    userId path string true "ID del usuario"
// @Param    body   body UpdateUserRequest true "campos a cambiar"
// @Success  200 {object} UserSummary
// @Failure  400 {object} httpx.ErrorResponse
// @Failure  404 {object} httpx.ErrorResponse
// @Failure  409 {object} httpx.ErrorResponse
// @Router   /admin/tenants/{id}/users/{userId} [patch]
func (h *Handler) UpdateUser(c fiber.Ctx) error {
	tenantID, err := uuidParam(c, "id", msgTenantNotFound)
	if err != nil {
		return err
	}
	userID, err := uuidParam(c, "userId", msgUserNotFound)
	if err != nil {
		return err
	}
	var req UpdateUserRequest
	if err := c.Bind().Body(&req); err != nil {
		return httpx.InvalidJSON()
	}
	patch := domain.UserPatch{FullName: req.FullName, Roles: req.Roles, Active: req.Active, Password: req.Password}
	u, err := h.svc.UpdateUser(c.Context(), httpauth.Claims(c).UserID(), tenantID, userID, patch)
	if err != nil {
		return mapErr(err)
	}
	return c.JSON(userSummary(u))
}

// DeleteUser godoc
// @Summary  Eliminar usuario (soft delete): cierra sus sesiones y bloquea su acceso (solo superadmin)
// @Tags     admin
// @Security BearerAuth
// @Param    id     path string true "ID de la empresa"
// @Param    userId path string true "ID del usuario"
// @Success  204
// @Failure  404 {object} httpx.ErrorResponse
// @Failure  409 {object} httpx.ErrorResponse
// @Router   /admin/tenants/{id}/users/{userId} [delete]
func (h *Handler) DeleteUser(c fiber.Ctx) error {
	tenantID, userID, err := userParams(c)
	if err != nil {
		return err
	}
	if err := h.svc.DeleteUser(c.Context(), httpauth.Claims(c).UserID(), tenantID, userID); err != nil {
		return mapErr(err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// RestoreUser godoc
// @Summary  Restaurar usuario eliminado (solo superadmin)
// @Tags     admin
// @Security BearerAuth
// @Produce  json
// @Param    id     path string true "ID de la empresa"
// @Param    userId path string true "ID del usuario"
// @Success  200 {object} UserSummary
// @Failure  404 {object} httpx.ErrorResponse
// @Failure  409 {object} httpx.ErrorResponse
// @Router   /admin/tenants/{id}/users/{userId}/restore [post]
func (h *Handler) RestoreUser(c fiber.Ctx) error {
	tenantID, userID, err := userParams(c)
	if err != nil {
		return err
	}
	u, err := h.svc.RestoreUser(c.Context(), httpauth.Claims(c).UserID(), tenantID, userID)
	if err != nil {
		return mapErr(err)
	}
	return c.JSON(userSummary(u))
}

// userParams valida los ids de empresa y usuario de la ruta (UUID; si no, 404).
func userParams(c fiber.Ctx) (string, string, error) {
	tenantID, err := uuidParam(c, "id", msgTenantNotFound)
	if err != nil {
		return "", "", err
	}
	userID, err := uuidParam(c, "userId", msgUserNotFound)
	return tenantID, userID, err
}
