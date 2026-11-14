// Package http expone los casos de uso de clientes por REST (Fiber).
package http

import (
	"errors"
	"time"

	"github.com/gofiber/fiber/v3"

	"prestamos/internal/customers/app"
	"prestamos/internal/customers/domain"
	"prestamos/internal/platform/httpauth"
	"prestamos/internal/platform/httpx"
)

type Handler struct{ svc *app.Service }

func NewHandler(svc *app.Service) *Handler { return &Handler{svc: svc} }

// Register monta /customers. Lectura: cualquier usuario autenticado; escritura: owner/admin.
func (h *Handler) Register(g fiber.Router, authn fiber.Handler) {
	write := httpauth.RequireRole("owner", "admin")
	g.Get("/", authn, h.List)
	g.Get("/:id", authn, h.Get)
	g.Post("/", authn, write, h.Create)
	g.Put("/:id", authn, write, h.Update)
	g.Patch("/:id/status", authn, write, h.SetStatus)
	g.Delete("/:id", authn, write, h.Delete)
}

// Delete godoc
// @Summary  Eliminar cliente (soft delete, owner/admin). Un cliente eliminado responde 404.
// @Tags     customers
// @Security BearerAuth
// @Param    id path string true "ID del cliente"
// @Success  204
// @Failure  401 {object} httpx.ErrorResponse
// @Failure  403 {object} httpx.ErrorResponse
// @Failure  404 {object} httpx.ErrorResponse
// @Router   /customers/{id} [delete]
func (h *Handler) Delete(c fiber.Ctx) error {
	cl := httpauth.Claims(c)
	if err := h.svc.Delete(c.Context(), cl.TenantID, cl.UserID(), c.Params("id")); err != nil {
		return mapErr(err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

type CustomerRequest struct {
	FullName    string  `json:"full_name" example:"Juan Pérez"`
	DocumentID  string  `json:"document_id" example:"45678912"`
	Phone       string  `json:"phone" example:"987654321"`
	Phone2      string  `json:"phone2"`
	WhatsApp    string  `json:"whatsapp"`
	Email       string  `json:"email"`
	Occupation  string  `json:"occupation"`
	Address     string  `json:"address" example:"Av. Lima 123"`
	AddressRef  string  `json:"address_reference" example:"frente al mercado"`
	Notes       string  `json:"notes"`
	CollectorID *string `json:"collector_id"`
}

type StatusRequest struct {
	Status string `json:"status" enums:"active,inactive,blocked"`
}

type CustomerResponse struct {
	CustomerRequest
	ID        string    `json:"id"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Create godoc
// @Summary  Crear cliente
// @Tags     customers
// @Security BearerAuth
// @Accept   json
// @Produce  json
// @Param    body body CustomerRequest true "datos del cliente"
// @Success  201 {object} CustomerResponse
// @Failure  400 {object} httpx.ErrorResponse
// @Failure  409 {object} httpx.ErrorResponse
// @Router   /customers [post]
func (h *Handler) Create(c fiber.Ctx) error {
	req, err := bind(c)
	if err != nil {
		return err
	}
	out, err := h.svc.Create(c.Context(), tenant(c), req.toDomain())
	if err != nil {
		return mapErr(err)
	}
	return c.Status(fiber.StatusCreated).JSON(toResponse(out))
}

// Update godoc
// @Summary  Actualizar cliente
// @Tags     customers
// @Security BearerAuth
// @Accept   json
// @Produce  json
// @Param    id   path string true "ID del cliente"
// @Param    body body CustomerRequest true "datos del cliente"
// @Success  200 {object} CustomerResponse
// @Router   /customers/{id} [put]
func (h *Handler) Update(c fiber.Ctx) error {
	req, err := bind(c)
	if err != nil {
		return err
	}
	out, err := h.svc.Update(c.Context(), tenant(c), c.Params("id"), req.toDomain())
	if err != nil {
		return mapErr(err)
	}
	return c.JSON(toResponse(out))
}

// SetStatus godoc
// @Summary  Cambiar estado (active/inactive/blocked)
// @Tags     customers
// @Security BearerAuth
// @Accept   json
// @Produce  json
// @Param    id   path string true "ID del cliente"
// @Param    body body StatusRequest true "nuevo estado"
// @Success  200 {object} CustomerResponse
// @Router   /customers/{id}/status [patch]
func (h *Handler) SetStatus(c fiber.Ctx) error {
	var req StatusRequest
	if err := c.Bind().Body(&req); err != nil {
		return httpx.InvalidJSON()
	}
	out, err := h.svc.SetStatus(c.Context(), tenant(c), c.Params("id"), domain.Status(req.Status))
	if err != nil {
		return mapErr(err)
	}
	return c.JSON(toResponse(out))
}

// Get godoc
// @Summary  Obtener cliente
// @Tags     customers
// @Security BearerAuth
// @Produce  json
// @Param    id path string true "ID del cliente"
// @Success  200 {object} CustomerResponse
// @Router   /customers/{id} [get]
func (h *Handler) Get(c fiber.Ctx) error {
	out, err := h.svc.Get(c.Context(), tenant(c), c.Params("id"))
	if err != nil {
		return mapErr(err)
	}
	return c.JSON(toResponse(out))
}

// List godoc
// @Summary  Listar/buscar clientes
// @Tags     customers
// @Security BearerAuth
// @Produce  json
// @Param    q      query string false "nombre, documento o teléfono"
// @Param    status query string false "active|inactive|blocked"
// @Param    limit  query int    false "1-100 (def. 20)"
// @Param    offset query int    false "desplazamiento"
// @Param    include_total query bool false "calcular total exacto (más lento)"
// @Success  200 {object} httpx.Page[CustomerResponse]
// @Failure  400 {object} httpx.ErrorResponse
// @Router   /customers [get]
func (h *Handler) List(c fiber.Ctx) error {
	page, err := httpx.ParsePage(c)
	if err != nil {
		return err
	}
	f := domain.ListFilter{Query: c.Query("q"), Status: domain.Status(c.Query("status")),
		Limit: page.Limit, Offset: page.Offset, WithTotal: httpx.WantTotal(c)}
	res, err := h.svc.List(c.Context(), tenant(c), f)
	if err != nil {
		return mapErr(err)
	}
	out := make([]CustomerResponse, 0, len(res.Items))
	for i := range res.Items {
		out = append(out, toResponse(&res.Items[i]))
	}
	return c.JSON(httpx.NewPage(out, res.HasMore, res.Total, page))
}

func tenant(c fiber.Ctx) string { return httpauth.Claims(c).TenantID }

func bind(c fiber.Ctx) (*CustomerRequest, error) {
	var req CustomerRequest
	if err := c.Bind().Body(&req); err != nil {
		return nil, httpx.InvalidJSON()
	}
	return &req, nil
}

func (r *CustomerRequest) toDomain() domain.Customer {
	return domain.Customer{FullName: r.FullName, DocumentID: r.DocumentID, Phone: r.Phone, Phone2: r.Phone2,
		WhatsApp: r.WhatsApp, Email: r.Email, Occupation: r.Occupation, Address: r.Address,
		AddressRef: r.AddressRef, Notes: r.Notes, CollectorID: r.CollectorID}
}

func toResponse(c *domain.Customer) CustomerResponse {
	return CustomerResponse{
		CustomerRequest: CustomerRequest{FullName: c.FullName, DocumentID: c.DocumentID, Phone: c.Phone,
			Phone2: c.Phone2, WhatsApp: c.WhatsApp, Email: c.Email, Occupation: c.Occupation,
			Address: c.Address, AddressRef: c.AddressRef, Notes: c.Notes, CollectorID: c.CollectorID},
		ID: c.ID, Status: string(c.Status), CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt,
	}
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
		return httpx.NotFound("El cliente no existe")
	case errors.Is(err, domain.ErrDuplicateDocument):
		return httpx.Conflict("DUPLICATE_DOCUMENT", "Ya existe un cliente con ese documento")
	default:
		return err
	}
}
