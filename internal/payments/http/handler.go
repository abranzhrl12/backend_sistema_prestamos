package http

import (
	"errors"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"prestamos/internal/payments"
	"prestamos/internal/platform/httpauth"
	"prestamos/internal/platform/httpx"
	"time"
)

type Handler struct{ repo *payments.Repo }

func NewHandler(repo *payments.Repo) *Handler { return &Handler{repo: repo} }
func (h *Handler) Register(g fiber.Router, authn fiber.Handler) {
	g.Post("/:id/extensions", authn, httpauth.RequireRole("owner", "admin"), h.Extend)
	g.Get("/:id/extensions", authn, httpauth.RequireRole("owner", "admin"), h.Extensions)
	g.Post("/:id/payments", authn, httpauth.RequireRole("owner", "admin"), h.Create)
	g.Get("/:id/payments", authn, h.List)
}

type part struct {
	Method    string `json:"method"`
	Amount    string `json:"amount"`
	Reference string `json:"reference"`
	Bank      string `json:"bank"`
	Phone     string `json:"phone"`
}
type body struct {
	Amount       string            `json:"amount"`
	Parts        []part            `json:"parts"`
	Distribution *distributionBody `json:"distribution"`
}
type allocation struct {
	ScheduleID string `json:"schedule_id"`
	Principal  string `json:"principal"`
	Interest   string `json:"interest"`
	LateFee    string `json:"late_fee"`
}
type response struct {
	ID          string       `json:"id"`
	LoanID      string       `json:"loan_id"`
	Amount      string       `json:"amount"`
	PaidAt      time.Time    `json:"paid_at"`
	Status      string       `json:"status"`
	Parts       []part       `json:"parts"`
	Allocations []allocation `json:"allocations"`
}

// PaymentPageResponse documents the standard paged payment response.
type PaymentPageResponse struct {
	Items []response     `json:"items"`
	Meta  httpx.PageMeta `json:"meta"`
}

func toResponse(p payments.Payment) response {
	out := response{ID: p.ID, LoanID: p.LoanID, Amount: p.Amount.StringFixed(4), PaidAt: p.CreatedAt, Status: "valid", Parts: []part{}, Allocations: []allocation{}}
	for _, v := range p.Parts {
		out.Parts = append(out.Parts, part{v.Method, v.Amount.StringFixed(4), v.Reference, v.Bank, v.Phone})
	}
	for _, v := range p.Allocations {
		out.Allocations = append(out.Allocations, allocation{v.ScheduleID, v.Principal.StringFixed(4), v.Interest.StringFixed(4), v.LateFee.StringFixed(4)})
	}
	return out
}
func mapErr(e error) error {
	switch {
	case errors.Is(e, payments.ErrExtensionDate):
		return httpx.Conflict("EXTENSION_DATE_INVALID", "La nueva fecha debe ser posterior a la próxima cuota pendiente")
	case errors.Is(e, payments.ErrInvalidInput):
		return httpx.Validation(httpx.FieldError{Field: "payment", Message: "Los datos del pago son inválidos"})
	case errors.Is(e, payments.ErrNotFound):
		return httpx.NotFound("El préstamo no existe")
	case errors.Is(e, payments.ErrLoanNotActive):
		return httpx.Conflict("LOAN_INVALID_STATUS", "El préstamo no está activo")
	case errors.Is(e, payments.ErrOverpayment):
		return httpx.Conflict("PAYMENT_EXCEEDS_BALANCE", "El pago supera el saldo")
	case errors.Is(e, payments.ErrIdempotencyConflict):
		return httpx.Conflict("IDEMPOTENCY_CONFLICT", "La clave ya se usó con otros datos")
	case errors.Is(e, payments.ErrDuplicateReference):
		return httpx.Conflict("PAYMENT_REFERENCE_TAKEN", "La referencia ya está registrada")
	default:
		return e
	}
}

// Create godoc
// @Summary Registrar un pago
// @Tags cobros
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "ID del préstamo"
// @Param Idempotency-Key header string true "Clave idempotente única"
// @Param body body body true "Pago y medios de recepción"
// @Success 201 {object} response
// @Failure 400 {object} httpx.ErrorResponse
// @Failure 409 {object} httpx.ErrorResponse
// @Router /loans/{id}/payments [post]
func (h *Handler) Create(c fiber.Ctx) error {
	id := c.Params("id")
	if _, e := uuid.Parse(id); e != nil {
		return httpx.NotFound("El préstamo no existe")
	}
	var b body
	if e := c.Bind().Body(&b); e != nil {
		return httpx.InvalidJSON()
	}
	amount, e := decimal.NewFromString(b.Amount)
	if e != nil {
		return mapErr(payments.ErrInvalidInput)
	}
	cl := httpauth.Claims(c)
	in := payments.Input{LoanID: id, UserID: cl.UserID(), IdempotencyKey: c.Get("Idempotency-Key"), Amount: amount}
	if b.Distribution != nil {
		in.Distribution, e = b.Distribution.parse()
		if e != nil {
			return e
		}
		if !in.Distribution.Principal.Add(in.Distribution.Interest).Add(in.Distribution.LateFee).Equal(amount) {
			return httpx.Validation(httpx.FieldError{Field: "distribution", Message: "La suma de capital, interés y mora debe coincidir con el total recibido"})
		}
	}
	partsTotal := decimal.Zero
	for _, p := range b.Parts {
		v, e := decimal.NewFromString(p.Amount)
		if e != nil {
			return mapErr(payments.ErrInvalidInput)
		}
		in.Parts = append(in.Parts, payments.Part{Method: p.Method, Amount: v, Reference: p.Reference, Bank: p.Bank, Phone: p.Phone})
		partsTotal = partsTotal.Add(v)
	}
	if len(b.Parts) == 0 || !partsTotal.Equal(amount) {
		return httpx.Validation(httpx.FieldError{Field: "parts", Message: "La suma de las partes debe coincidir con el monto del pago"})
	}
	out, e := h.repo.Record(c.Context(), cl.TenantID, in)
	if e != nil {
		return mapErr(e)
	}
	return c.Status(201).JSON(toResponse(out))
}

// List godoc
// @Summary Listar pagos de un préstamo
// @Tags cobros
// @Security BearerAuth
// @Produce json
// @Param id path string true "ID del préstamo"
// @Param limit query int false "1-100 (def. 20)"
// @Param offset query int false "desplazamiento"
// @Success 200 {object} PaymentPageResponse
// @Router /loans/{id}/payments [get]
func (h *Handler) List(c fiber.Ctx) error {
	id := c.Params("id")
	if _, e := uuid.Parse(id); e != nil {
		return httpx.NotFound("El préstamo no existe")
	}
	page, e := httpx.ParsePage(c)
	if e != nil {
		return e
	}
	cl := httpauth.Claims(c)
	rows, more, total, e := h.repo.List(c.Context(), cl.TenantID, id, page.Limit, page.Offset, httpx.WantTotal(c))
	if e != nil {
		return mapErr(e)
	}
	out := make([]response, 0, len(rows))
	for _, v := range rows {
		out = append(out, toResponse(v))
	}
	return c.JSON(httpx.NewPage(out, more, total, page))
}
