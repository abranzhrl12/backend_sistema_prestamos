package http

import (
	"errors"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"prestamos/internal/loans/app"
	"prestamos/internal/loans/schedule"
	"prestamos/internal/platform/httpauth"
	"prestamos/internal/platform/httpx"
	"time"
)

type Handler struct {
	repo app.Store
	svc  *app.Service
}

func NewHandler(repo app.Store, svc *app.Service) *Handler { return &Handler{repo: repo, svc: svc} }
func (h *Handler) Register(g fiber.Router, authn fiber.Handler) {
	write := httpauth.RequireRole("owner", "admin")
	g.Post("/simulate", authn, write, h.Simulate)
	g.Post("/", authn, write, h.Create)
	g.Get("/", authn, h.List)
	g.Get("/:id/schedule", authn, h.Schedule)
	g.Post("/:id/disburse", authn, write, h.Disburse)
}

type request struct {
	CustomerID        string `json:"customer_id"`
	Principal         string `json:"principal"`
	InterestType      string `json:"interest_type"`
	InterestValue     string `json:"interest_value"`
	TermValue         int    `json:"term_value"`
	TermUnit          string `json:"term_unit"`
	Frequency         string `json:"frequency"`
	DisbursementDate  string `json:"disbursement_date"`
	InstallmentsCount *int   `json:"installments_count"`
	FirstPaymentDate  string `json:"first_payment_date"`
}

func (r request) input() app.Input {
	count := 0
	if r.InstallmentsCount != nil {
		count = *r.InstallmentsCount
	}
	return app.Input{CustomerID: r.CustomerID, Principal: r.Principal, InterestType: r.InterestType, InterestValue: r.InterestValue, TermValue: r.TermValue, TermUnit: r.TermUnit, Frequency: r.Frequency, DisbursementDate: r.DisbursementDate, InstallmentsCount: count, FirstPaymentDate: r.FirstPaymentDate}
}
func actor(c fiber.Ctx) app.Actor {
	cl := httpauth.Claims(c)
	return app.Actor{TenantID: cl.TenantID, UserID: cl.UserID()}
}
func bind(c fiber.Ctx) (request, error) {
	var r request
	if e := c.Bind().Body(&r); e != nil {
		return r, httpx.InvalidJSON()
	}
	if r.InterestType == "" {
		r.InterestType = "none"
	}
	if r.InterestValue == "" {
		r.InterestValue = "0"
	}
	p, e := decimal.NewFromString(r.Principal)
	if e != nil || !p.IsPositive() || !p.Equal(p.Round(4)) || p.GreaterThanOrEqual(decimal.RequireFromString("1000000000000000")) {
		return r, httpx.Validation(httpx.FieldError{Field: "principal", Message: "Ingrese un monto positivo con hasta cuatro decimales"})
	}
	v, e := decimal.NewFromString(r.InterestValue)
	if e != nil || v.IsNegative() || !v.Equal(v.Round(4)) {
		return r, httpx.Validation(httpx.FieldError{Field: "interest_value", Message: "Ingrese un interés no negativo con hasta cuatro decimales"})
	}
	if r.InterestType != "none" && r.InterestType != "fixed" && r.InterestType != "percentage" {
		return r, httpx.Validation(httpx.FieldError{Field: "interest_type", Message: "Tipo de interés inválido"})
	}
	if r.InterestType == "none" && !v.IsZero() {
		return r, httpx.Validation(httpx.FieldError{Field: "interest_value", Message: "Sin interés el valor debe ser cero"})
	}
	if r.InstallmentsCount != nil || r.FirstPaymentDate != "" {
		if e := validatePaymentDates(r); e != nil {
			return r, e
		}
	} else {
		if r.TermValue < 1 || r.TermValue > 1000 {
			return r, httpx.Validation(httpx.FieldError{Field: "term_value", Message: "Ingrese un plazo entre 1 y 1000"})
		}
		if r.TermUnit != "days" && r.TermUnit != "weeks" && r.TermUnit != "months" {
			return r, httpx.Validation(httpx.FieldError{Field: "term_unit", Message: "Unidad de plazo inválida"})
		}
	}
	if r.Frequency != "daily" && r.Frequency != "weekly" && r.Frequency != "biweekly" && r.Frequency != "monthly" && r.Frequency != "single" {
		return r, httpx.Validation(httpx.FieldError{Field: "frequency", Message: "Frecuencia inválida"})
	}
	if _, e = time.Parse("2006-01-02", r.DisbursementDate); e != nil {
		return r, httpx.Validation(httpx.FieldError{Field: "disbursement_date", Message: "Ingrese una fecha válida"})
	}
	return r, nil
}
func mapErr(e error) error {
	var shortfall *app.CapitalShortfall
	if errors.As(e, &shortfall) {
		return httpx.Conflict("INSUFFICIENT_CAPITAL", shortfall.Error())
	}
	switch {
	case errors.Is(e, app.ErrInvalid):
		return httpx.Validation(httpx.FieldError{Field: "loan", Message: "Los datos del préstamo son inválidos o no están admitidos"})
	case errors.Is(e, app.ErrNotFound):
		return httpx.NotFound("El préstamo no existe")
	case errors.Is(e, app.ErrInsufficientCapital):
		return httpx.Conflict("INSUFFICIENT_CAPITAL", "El capital disponible es insuficiente")
	case errors.Is(e, app.ErrInvalidStatus):
		return httpx.Conflict("LOAN_INVALID_STATUS", "El préstamo no permite esta operación")
	default:
		return e
	}
}

type loanResponse struct {
	ID               string `json:"id"`
	CustomerID       string `json:"customer_id"`
	CustomerName     string `json:"customer_name"`
	AnnualRate       string `json:"annual_rate"`
	ProductID        string `json:"product_id"`
	Principal        string `json:"principal"`
	Rate             string `json:"rate"`
	RatePeriod       string `json:"rate_period"`
	InterestType     string `json:"interest_type"`
	Frequency        string `json:"frequency"`
	TermCount        int    `json:"term_count"`
	Status           string `json:"status"`
	DisbursementDate string `json:"disbursement_date"`
	InterestValue    string `json:"interest_value"`
	TermValue        int    `json:"term_value"`
	TermUnit         string `json:"term_unit"`
}

// LoanPageResponse documents the standard paged loan response.
type LoanPageResponse struct {
	Items []loanResponse `json:"items"`
	Meta  httpx.PageMeta `json:"meta"`
}

func response(l app.Loan) loanResponse {
	return loanResponse{ID: l.ID, CustomerID: l.CustomerID, CustomerName: l.CustomerName, AnnualRate: l.AnnualRate, ProductID: l.ProductID, Principal: l.Principal, Rate: l.Rate, RatePeriod: l.RatePeriod, InterestType: l.InterestType, Frequency: l.Frequency, TermCount: l.TermCount, Status: l.Status, DisbursementDate: l.DisbursementDate, InterestValue: l.InterestValue, TermValue: l.TermValue, TermUnit: l.TermUnit}
}

type simulationRow struct {
	Number    int    `json:"installment_no"`
	DueDate   string `json:"due_date"`
	Principal string `json:"principal_due"`
	Interest  string `json:"interest_due"`
	Total     string `json:"total_due"`
	Balance   string `json:"balance"`
}

// SimulationResponse documents the calculated schedule before it is recorded.
type SimulationResponse struct {
	Schedule       []simulationRow `json:"schedule"`
	TotalPrincipal string          `json:"total_principal"`
	TotalInterest  string          `json:"total_interest"`
	TotalDue       string          `json:"total_due"`
}

func simulation(rows []schedule.Installment) SimulationResponse {
	result := make([]simulationRow, 0, len(rows))
	principal, interest := decimal.Zero, decimal.Zero
	for _, r := range rows {
		principal = principal.Add(r.Principal)
		interest = interest.Add(r.Interest)
		result = append(result, simulationRow{r.Number, r.DueDate.Format("2006-01-02"), r.Principal.StringFixed(4), r.Interest.StringFixed(4), r.Principal.Add(r.Interest).StringFixed(4), r.Balance.StringFixed(4)})
	}
	return SimulationResponse{Schedule: result, TotalPrincipal: principal.StringFixed(4), TotalInterest: interest.StringFixed(4), TotalDue: principal.Add(interest).StringFixed(4)}
}

// Simulate godoc
// @Summary Simular cuotas de un préstamo personal
// @Tags prestamos
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body request true "Condiciones del préstamo"
// @Success 200 {object} SimulationResponse
// @Failure 400 {object} httpx.ErrorResponse
// @Router /loans/simulate [post]
func (h *Handler) Simulate(c fiber.Ctx) error {
	r, e := bind(c)
	if e != nil {
		return e
	}
	rows, e := h.svc.Simulate(c.Context(), actor(c).TenantID, r.input())
	if e != nil {
		return mapErr(e)
	}
	return c.JSON(simulation(rows))
}

// Create godoc
// @Summary Registrar préstamo pendiente de entrega
// @Tags prestamos
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body request true "Condiciones del préstamo"
// @Success 201 {object} loanResponse
// @Failure 400 {object} httpx.ErrorResponse
// @Failure 409 {object} httpx.ErrorResponse
// @Router /loans [post]
func (h *Handler) Create(c fiber.Ctx) error {
	r, e := bind(c)
	if e != nil {
		return e
	}
	if _, e = uuid.Parse(r.CustomerID); e != nil {
		return httpx.Validation(httpx.FieldError{Field: "customer_id", Message: "Selecciona un cliente válido"})
	}
	a := actor(c)
	rows, e := h.svc.Simulate(c.Context(), a.TenantID, r.input())
	if e != nil {
		return mapErr(e)
	}
	l := app.Loan{CustomerID: r.CustomerID, Principal: r.Principal, InterestType: r.InterestType, InterestValue: r.InterestValue, TermValue: r.TermValue, TermUnit: r.TermUnit, Frequency: r.Frequency, TermCount: len(rows), DisbursementDate: r.DisbursementDate}
	out, e := h.repo.Create(c.Context(), a, l, rows)
	if e != nil {
		return mapErr(e)
	}
	return c.Status(201).JSON(response(out))
}

// Disburse godoc
// @Summary Entregar el dinero de un préstamo pendiente
// @Tags prestamos
// @Security BearerAuth
// @Produce json
// @Param id path string true "ID del préstamo"
// @Success 200 {object} loanResponse
// @Failure 409 {object} httpx.ErrorResponse
// @Router /loans/{id}/disburse [post]
func (h *Handler) Disburse(c fiber.Ctx) error {
	id := c.Params("id")
	if _, e := uuid.Parse(id); e != nil {
		return httpx.NotFound("El préstamo no existe")
	}
	l, e := h.repo.Disburse(c.Context(), actor(c), id)
	if e != nil {
		return mapErr(e)
	}
	return c.JSON(response(l))
}

// List godoc
// @Summary Listar préstamos
// @Tags prestamos
// @Security BearerAuth
// @Produce json
// @Param limit query int false "1-100 (def. 20)"
// @Param offset query int false "desplazamiento"
// @Success 200 {object} LoanPageResponse
// @Router /loans [get]
func (h *Handler) List(c fiber.Ctx) error {
	p, e := httpx.ParsePage(c)
	if e != nil {
		return e
	}
	rows, more, total, e := h.repo.List(c.Context(), actor(c).TenantID, p.Limit, p.Offset, httpx.WantTotal(c))
	if e != nil {
		return mapErr(e)
	}
	out := make([]loanResponse, 0, len(rows))
	for _, r := range rows {
		out = append(out, response(r))
	}
	return c.JSON(httpx.NewPage(out, more, total, p))
}
