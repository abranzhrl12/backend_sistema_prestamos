// Package http expone los productos de préstamo por REST (Fiber).
package http

import (
	"errors"
	"strconv"

	"github.com/gofiber/fiber/v3"

	"prestamos/internal/loanproducts/app"
	"prestamos/internal/loanproducts/domain"
	"prestamos/internal/platform/httpauth"
	"prestamos/internal/platform/httpx"
)

type Handler struct{ svc *app.Service }

func NewHandler(svc *app.Service) *Handler { return &Handler{svc: svc} }

// Register monta /loan-products. Lectura: cualquier usuario autenticado; escritura: owner/admin.
func (h *Handler) Register(g fiber.Router, authn fiber.Handler) {
	write := httpauth.RequireRole("owner", "admin")
	g.Get("/", authn, h.List)
	g.Get("/:id", authn, h.Get)
	g.Post("/", authn, write, h.Create)
	g.Put("/:id", authn, write, h.Update)
	g.Delete("/:id", authn, write, h.Delete)
}

// Create godoc
// @Summary  Crear producto de préstamo
// @Tags     loan-products
// @Security BearerAuth
// @Accept   json
// @Produce  json
// @Param    body body ProductRequest true "datos del producto"
// @Success  201 {object} ProductResponse
// @Failure  400 {object} httpx.ErrorResponse
// @Failure  403 {object} httpx.ErrorResponse
// @Failure  409 {object} httpx.ErrorResponse
// @Router   /loan-products [post]
func (h *Handler) Create(c fiber.Ctx) error {
	req, err := bind(c)
	if err != nil {
		return err
	}
	out, err := h.svc.Create(c.Context(), actor(c), req.toDomain())
	if err != nil {
		return mapErr(err)
	}
	return c.Status(fiber.StatusCreated).JSON(toResponse(out))
}

// Update godoc
// @Summary  Reemplazar producto de préstamo (owner/admin)
// @Tags     loan-products
// @Security BearerAuth
// @Accept   json
// @Produce  json
// @Param    id   path string true "ID del producto"
// @Param    body body ProductRequest true "datos completos del producto"
// @Success  200 {object} ProductResponse
// @Failure  400 {object} httpx.ErrorResponse
// @Failure  404 {object} httpx.ErrorResponse
// @Failure  409 {object} httpx.ErrorResponse
// @Router   /loan-products/{id} [put]
func (h *Handler) Update(c fiber.Ctx) error {
	req, err := bind(c)
	if err != nil {
		return err
	}
	out, err := h.svc.Update(c.Context(), actor(c), c.Params("id"), req.toDomain())
	if err != nil {
		return mapErr(err)
	}
	return c.JSON(toResponse(out))
}

// Delete godoc
// @Summary  Eliminar producto de préstamo (soft delete, owner/admin)
// @Tags     loan-products
// @Security BearerAuth
// @Param    id path string true "ID del producto"
// @Success  204
// @Failure  404 {object} httpx.ErrorResponse
// @Router   /loan-products/{id} [delete]
func (h *Handler) Delete(c fiber.Ctx) error {
	if err := h.svc.Delete(c.Context(), actor(c), c.Params("id")); err != nil {
		return mapErr(err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// Get godoc
// @Summary  Obtener producto de préstamo
// @Tags     loan-products
// @Security BearerAuth
// @Produce  json
// @Param    id path string true "ID del producto"
// @Success  200 {object} ProductResponse
// @Failure  404 {object} httpx.ErrorResponse
// @Router   /loan-products/{id} [get]
func (h *Handler) Get(c fiber.Ctx) error {
	out, err := h.svc.Get(c.Context(), httpauth.Claims(c).TenantID, c.Params("id"))
	if err != nil {
		return mapErr(err)
	}
	return c.JSON(toResponse(out))
}

// List godoc
// @Summary  Listar productos de préstamo
// @Tags     loan-products
// @Security BearerAuth
// @Produce  json
// @Param    q          query string false "texto en el nombre"
// @Param    active     query bool   false "true|false (omitir = todos)"
// @Param    frequency  query string false "daily|weekly|biweekly|monthly|single"
// @Param    limit      query int    false "1-100 (def. 20)"
// @Param    offset     query int    false "desplazamiento"
// @Param    include_total query bool false "calcular total exacto (más lento)"
// @Success  200 {object} ProductPageResponse
// @Failure  400 {object} httpx.ErrorResponse
// @Router   /loan-products [get]
func (h *Handler) List(c fiber.Ctx) error {
	page, err := httpx.ParsePage(c)
	if err != nil {
		return err
	}
	f, err := listFilter(c, page)
	if err != nil {
		return err
	}
	res, err := h.svc.List(c.Context(), httpauth.Claims(c).TenantID, f)
	if err != nil {
		return mapErr(err)
	}
	out := make([]ProductResponse, 0, len(res.Items))
	for i := range res.Items {
		out = append(out, toResponse(&res.Items[i]))
	}
	return c.JSON(httpx.NewPage(out, res.HasMore, res.Total, page))
}

func actor(c fiber.Ctx) app.Actor {
	cl := httpauth.Claims(c)
	return app.Actor{TenantID: cl.TenantID, UserID: cl.UserID()}
}

func bind(c fiber.Ctx) (*ProductRequest, error) {
	var req ProductRequest
	if err := c.Bind().Body(&req); err != nil {
		return nil, httpx.InvalidJSON()
	}
	return &req, nil
}

// listFilter lee los filtros; ?active solo acepta true o false.
func listFilter(c fiber.Ctx, page httpx.PageParams) (domain.ListFilter, error) {
	f := domain.ListFilter{Query: c.Query("q"), Frequency: c.Query("frequency"),
		Limit: page.Limit, Offset: page.Offset, WithTotal: httpx.WantTotal(c)}
	if raw := c.Query("active"); raw != "" {
		v, err := strconv.ParseBool(raw)
		if err != nil {
			return f, httpx.Validation(httpx.FieldError{Field: "active", Message: "Debe ser true o false"})
		}
		f.Active = &v
	}
	return f, nil
}

func mapErr(err error) error {
	var ve *domain.ValidationError
	switch {
	case errors.As(err, &ve):
		fields := make([]httpx.FieldError, len(ve.Issues))
		for i, is := range ve.Issues {
			fields[i] = httpx.FieldError{Field: is.Field, Message: is.Msg}
		}
		return httpx.Validation(fields...)
	case errors.Is(err, domain.ErrNotFound):
		return httpx.NotFound("El plan de préstamo no existe")
	case errors.Is(err, domain.ErrNameTaken):
		return httpx.Conflict("PRODUCT_NAME_TAKEN", "Ya existe un plan de préstamo con ese nombre")
	default:
		return err
	}
}
