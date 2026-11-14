package http

import (
	"time"

	"prestamos/internal/loanproducts/domain"
	"prestamos/internal/platform/httpx"
)

// ProductRequest es el cuerpo de POST y PUT (PUT reemplaza todo el producto).
// Los opcionales sin valor toman su default: interest_type=flat, fee_type=none, fee_value=0, active=true.
// Los overrides null significan "usar la configuración de la empresa".
type ProductRequest struct {
	Name          string  `json:"name" example:"Préstamo diario"`
	Frequency     string  `json:"frequency" enums:"daily,weekly,biweekly,monthly,single"`
	InterestType  string  `json:"interest_type" enums:"flat,french,german"`
	RatePeriod    string  `json:"rate_period" enums:"daily,weekly,biweekly,monthly,annual"`
	RateDefault   string  `json:"rate_default" example:"2.50"`
	RateMin       string  `json:"rate_min" example:"1.00"`
	RateMax       string  `json:"rate_max" example:"5.00"`
	AmountMin     string  `json:"amount_min" example:"500.00"`
	AmountMax     string  `json:"amount_max" example:"5000.00"`
	TermsMin      int     `json:"terms_min" example:"1"`
	TermsMax      int     `json:"terms_max" example:"12"`
	GraceDays     *int    `json:"grace_days"`
	LateFeeMode   *string `json:"late_fee_mode" enums:"fixed,proportional"`
	LateFeeAmount *string `json:"late_fee_amount" example:"10.00"`
	LateFeePerDay *bool   `json:"late_fee_per_day"`
	FeeType       string  `json:"fee_type" enums:"none,fixed,percent"`
	FeeValue      string  `json:"fee_value" example:"0"`
	Active        *bool   `json:"active"`
}

// ProductResponse es el producto tal como lo ve el frontend.
type ProductResponse struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Frequency     string    `json:"frequency"`
	InterestType  string    `json:"interest_type"`
	RatePeriod    string    `json:"rate_period"`
	RateDefault   string    `json:"rate_default"`
	RateMin       string    `json:"rate_min"`
	RateMax       string    `json:"rate_max"`
	AmountMin     string    `json:"amount_min"`
	AmountMax     string    `json:"amount_max"`
	TermsMin      int       `json:"terms_min"`
	TermsMax      int       `json:"terms_max"`
	GraceDays     *int      `json:"grace_days"`
	LateFeeMode   *string   `json:"late_fee_mode"`
	LateFeeAmount *string   `json:"late_fee_amount"`
	LateFeePerDay *bool     `json:"late_fee_per_day"`
	FeeType       string    `json:"fee_type"`
	FeeValue      string    `json:"fee_value"`
	Active        bool      `json:"active"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// ProductPageResponse documents the standard paged product response.
type ProductPageResponse struct {
	Items []ProductResponse `json:"items"`
	Meta  httpx.PageMeta    `json:"meta"`
}

func (r ProductRequest) toDomain() domain.Product {
	active := true
	if r.Active != nil {
		active = *r.Active
	}
	return domain.Product{
		Name: r.Name, Frequency: r.Frequency, InterestType: r.InterestType, RatePeriod: r.RatePeriod,
		RateDefault: r.RateDefault, RateMin: r.RateMin, RateMax: r.RateMax,
		AmountMin: r.AmountMin, AmountMax: r.AmountMax, TermsMin: r.TermsMin, TermsMax: r.TermsMax,
		GraceDays: r.GraceDays, LateFeeMode: r.LateFeeMode, LateFeeAmount: r.LateFeeAmount,
		LateFeePerDay: r.LateFeePerDay, FeeType: r.FeeType, FeeValue: r.FeeValue, Active: active,
	}
}

func toResponse(p *domain.Product) ProductResponse {
	return ProductResponse{
		ID: p.ID, Name: p.Name, Frequency: p.Frequency, InterestType: p.InterestType, RatePeriod: p.RatePeriod,
		RateDefault: p.RateDefault, RateMin: p.RateMin, RateMax: p.RateMax,
		AmountMin: p.AmountMin, AmountMax: p.AmountMax, TermsMin: p.TermsMin, TermsMax: p.TermsMax,
		GraceDays: p.GraceDays, LateFeeMode: p.LateFeeMode, LateFeeAmount: p.LateFeeAmount,
		LateFeePerDay: p.LateFeePerDay, FeeType: p.FeeType, FeeValue: p.FeeValue, Active: p.Active,
		CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
}
