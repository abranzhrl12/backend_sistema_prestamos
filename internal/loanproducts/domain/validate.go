package domain

import (
	"errors"
	"unicode/utf8"

	"github.com/shopspring/decimal"

	"prestamos/internal/platform/money"
)

const (
	minNameRunes = 2
	maxNameRunes = 80
	maxTerms     = 1000
	maxRate      = 1000
	maxPercent   = 100
	maxGraceDays = 30
)

// Issue es un problema de validación asociado a un campo (nombre JSON).
type Issue struct{ Field, Msg string }

// ValidationError agrupa todos los problemas de validación encontrados.
type ValidationError struct{ Issues []Issue }

func (e *ValidationError) Error() string {
	if len(e.Issues) == 0 {
		return "datos inválidos"
	}
	return e.Issues[0].Field + ": " + e.Issues[0].Msg
}

// NewValidationError crea un error de validación de un solo campo.
func NewValidationError(field, msg string) *ValidationError {
	return &ValidationError{Issues: []Issue{{field, msg}}}
}

// collector acumula los problemas para reportar todos los campos de una vez.
type collector struct{ issues []Issue }

func (c *collector) add(field, msg string) { c.issues = append(c.issues, Issue{field, msg}) }

func (c *collector) err() error {
	if len(c.issues) == 0 {
		return nil
	}
	return &ValidationError{Issues: c.issues}
}

// Validate aplica todas las reglas del producto y devuelve *ValidationError con cada campo incorrecto.
func (p Product) Validate() error {
	c := &collector{}
	p.validateName(c)
	p.validateOptions(c)
	p.validateRates(c)
	p.validateAmounts(c)
	p.validateTerms(c)
	p.validateOverrides(c)
	p.validateFee(c)
	return c.err()
}

func (p Product) validateName(c *collector) {
	n := utf8.RuneCountInString(p.Name)
	switch {
	case n == 0:
		c.add("name", "El nombre es obligatorio")
	case n < minNameRunes || n > maxNameRunes:
		c.add("name", "El nombre debe tener entre 2 y 80 caracteres")
	}
}

// ValidFrequency indica si la frecuencia es una de las opciones del producto.
func ValidFrequency(v string) bool {
	return inSet(v, FrequencyDaily, FrequencyWeekly, FrequencyBiweekly, FrequencyMonthly, FrequencySingle)
}

func (p Product) validateOptions(c *collector) {
	if !ValidFrequency(p.Frequency) {
		c.add("frequency", "Frecuencia inválida (daily, weekly, biweekly, monthly o single)")
	}
	if !inSet(p.InterestType, InterestFlat, InterestFrench, InterestGerman) {
		c.add("interest_type", "Tipo de interés inválido (flat, french o german)")
	}
	if !inSet(p.RatePeriod, RatePeriodDaily, RatePeriodWeekly, RatePeriodBiweekly, RatePeriodMonthly, RatePeriodAnnual) {
		c.add("rate_period", "Periodo de la tasa inválido (daily, weekly, biweekly, monthly o annual)")
	}
}

// validateRates exige 0 <= min <= default <= max <= 1000.
func (p Product) validateRates(c *collector) {
	def, okD := c.number("rate_default", p.RateDefault, "Porcentaje inválido (ej. 2.50)")
	lo, okL := c.number("rate_min", p.RateMin, "Porcentaje inválido (ej. 2.50)")
	hi, okH := c.number("rate_max", p.RateMax, "Porcentaje inválido (ej. 2.50)")
	if !(okD && okL && okH) {
		return
	}
	if lo.GreaterThan(def) {
		c.add("rate_min", "No puede ser mayor que la tasa por defecto")
	}
	if def.GreaterThan(hi) {
		c.add("rate_max", "No puede ser menor que la tasa por defecto")
	}
	if hi.GreaterThan(decimal.NewFromInt(maxRate)) {
		c.add("rate_max", "No puede superar 1000")
	}
}

// validateAmounts exige 0 < min <= max.
func (p Product) validateAmounts(c *collector) {
	lo, okL := c.number("amount_min", p.AmountMin, "Monto inválido (ej. 500.00)")
	hi, okH := c.number("amount_max", p.AmountMax, "Monto inválido (ej. 5000.00)")
	if okL && !lo.IsPositive() {
		c.add("amount_min", "Debe ser mayor que cero")
	}
	if okL && okH && hi.LessThan(lo) {
		c.add("amount_max", "No puede ser menor que el monto mínimo")
	}
}

// validateTerms exige 1 <= min <= max <= 1000; con frecuencia única el plazo es exactamente 1.
func (p Product) validateTerms(c *collector) {
	if p.TermsMin < 1 || p.TermsMin > maxTerms {
		c.add("terms_min", "Debe estar entre 1 y 1000")
	}
	if p.TermsMax < 1 || p.TermsMax > maxTerms {
		c.add("terms_max", "Debe estar entre 1 y 1000")
	}
	if p.TermsMin > p.TermsMax {
		c.add("terms_max", "No puede ser menor que el plazo mínimo")
	}
	if p.Frequency != FrequencySingle {
		return
	}
	if p.TermsMin != 1 {
		c.add("terms_min", "Con frecuencia única el plazo debe ser 1")
	}
	if p.TermsMax != 1 {
		c.add("terms_max", "Con frecuencia única el plazo debe ser 1")
	}
}

// validateOverrides revisa solo los overrides presentes (nil = usar la configuración de la empresa).
func (p Product) validateOverrides(c *collector) {
	if p.GraceDays != nil && (*p.GraceDays < 0 || *p.GraceDays > maxGraceDays) {
		c.add("grace_days", "Debe estar entre 0 y 30")
	}
	if p.LateFeeMode != nil && !inSet(*p.LateFeeMode, LateFeeFixed, LateFeeProportional) {
		c.add("late_fee_mode", "Modo de mora inválido (fixed o proportional)")
	}
	if p.LateFeeAmount != nil {
		c.number("late_fee_amount", *p.LateFeeAmount, "Monto inválido (ej. 10.00)")
	}
}

// validateFee: none exige valor 0; percent no supera 100.
func (p Product) validateFee(c *collector) {
	if !inSet(p.FeeType, FeeNone, FeeFixed, FeePercent) {
		c.add("fee_type", "Tipo de cargo inválido (none, fixed o percent)")
	}
	v, ok := c.number("fee_value", p.FeeValue, "Monto inválido (ej. 15.00)")
	if !ok {
		return
	}
	if p.FeeType == FeeNone && !v.IsZero() {
		c.add("fee_value", "Sin cargo, el valor debe ser 0")
	}
	if p.FeeType == FeePercent && v.GreaterThan(decimal.NewFromInt(maxPercent)) {
		c.add("fee_value", "El porcentaje no puede superar 100")
	}
}

// number lee un monto >= 0; registra el problema y devuelve false si no es válido.
func (c *collector) number(field, s, invalidMsg string) (decimal.Decimal, bool) {
	if s == "" {
		c.add(field, "es obligatorio")
		return decimal.Zero, false
	}
	d, err := money.Parse(s)
	switch {
	case errors.Is(err, money.ErrNegative):
		c.add(field, "No puede ser negativo")
		return d, false
	case err != nil:
		c.add(field, invalidMsg)
		return d, false
	}
	return d, true
}

func inSet(v string, options ...string) bool {
	for _, o := range options {
		if v == o {
			return true
		}
	}
	return false
}
