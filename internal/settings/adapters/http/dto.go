package http

import (
	"time"

	"prestamos/internal/platform/httpx"
	"prestamos/internal/settings/domain"
)

// SettingsRequest es el reemplazo total de la configuración. Los punteros permiten detectar
// los campos obligatorios que no vienen en el body.
type SettingsRequest struct {
	Currency                  *string `json:"currency" example:"PEN" enums:"PEN,USD"`
	Timezone                  *string `json:"timezone" example:"America/Lima"`
	LateFeeMode               *string `json:"late_fee_mode" enums:"fixed,proportional"`
	LateFeeAmount             *string `json:"late_fee_amount" example:"10.00"`
	LateFeePerDay             *bool   `json:"late_fee_per_day"`
	GraceDays                 *int    `json:"grace_days" example:"0"`
	AllocationOrder           *string `json:"allocation_order" enums:"late_fee_interest_principal,interest_principal_late_fee"`
	MaxActiveLoansPerCustomer *int    `json:"max_active_loans_per_customer" example:"3"`
	AllowNegativeCapital      *bool   `json:"allow_negative_capital"`
}

// SettingsResponse es la configuración efectiva. updated_at es null mientras son los valores por defecto.
type SettingsResponse struct {
	Currency                  string     `json:"currency"`
	Timezone                  string     `json:"timezone"`
	LateFeeMode               string     `json:"late_fee_mode"`
	LateFeeAmount             string     `json:"late_fee_amount" example:"10.00"`
	LateFeePerDay             bool       `json:"late_fee_per_day"`
	GraceDays                 int        `json:"grace_days"`
	AllocationOrder           string     `json:"allocation_order"`
	MaxActiveLoansPerCustomer int        `json:"max_active_loans_per_customer"`
	AllowNegativeCapital      bool       `json:"allow_negative_capital"`
	IsDefault                 bool       `json:"is_default"`
	UpdatedAt                 *time.Time `json:"updated_at"`
}

// binder convierte punteros en valores y anota los campos obligatorios ausentes.
type binder struct{ missing []httpx.FieldError }

func (b *binder) str(p *string, field string) string {
	if p == nil {
		b.missing = append(b.missing, httpx.FieldError{Field: field, Message: "es obligatorio"})
		return ""
	}
	return *p
}

func (b *binder) flag(p *bool, field string) bool {
	if p == nil {
		b.missing = append(b.missing, httpx.FieldError{Field: field, Message: "es obligatorio"})
		return false
	}
	return *p
}

func (b *binder) num(p *int, field string) int {
	if p == nil {
		b.missing = append(b.missing, httpx.FieldError{Field: field, Message: "es obligatorio"})
		return 0
	}
	return *p
}

// toDomain devuelve la configuración y los campos obligatorios que faltan.
func (r SettingsRequest) toDomain() (domain.Settings, []httpx.FieldError) {
	var b binder
	in := domain.Settings{
		Currency:                  b.str(r.Currency, "currency"),
		Timezone:                  b.str(r.Timezone, "timezone"),
		LateFeeMode:               b.str(r.LateFeeMode, "late_fee_mode"),
		LateFeeAmount:             b.str(r.LateFeeAmount, "late_fee_amount"),
		LateFeePerDay:             b.flag(r.LateFeePerDay, "late_fee_per_day"),
		GraceDays:                 b.num(r.GraceDays, "grace_days"),
		AllocationOrder:           b.str(r.AllocationOrder, "allocation_order"),
		MaxActiveLoansPerCustomer: b.num(r.MaxActiveLoansPerCustomer, "max_active_loans_per_customer"),
		AllowNegativeCapital:      b.flag(r.AllowNegativeCapital, "allow_negative_capital"),
	}
	return in, b.missing
}

func toResponse(s *domain.Settings) SettingsResponse {
	out := SettingsResponse{
		Currency: s.Currency, Timezone: s.Timezone, LateFeeMode: s.LateFeeMode,
		LateFeeAmount: s.LateFeeAmount, LateFeePerDay: s.LateFeePerDay, GraceDays: s.GraceDays,
		AllocationOrder: s.AllocationOrder, MaxActiveLoansPerCustomer: s.MaxActiveLoansPerCustomer,
		AllowNegativeCapital: s.AllowNegativeCapital, IsDefault: s.IsDefault,
	}
	if !s.IsDefault {
		out.UpdatedAt = &s.UpdatedAt
	}
	return out
}
