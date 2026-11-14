package http

import (
	"prestamos/internal/platform/httpx"
	"time"
)

func validatePaymentDates(r request) error {
	var issues []httpx.FieldError
	if r.InstallmentsCount == nil || *r.InstallmentsCount < 1 || *r.InstallmentsCount > 1000 {
		issues = append(issues, httpx.FieldError{Field: "installments_count", Message: "Ingrese entre 1 y 1000 pagos"})
	} else if r.Frequency == "single" && *r.InstallmentsCount != 1 {
		issues = append(issues, httpx.FieldError{Field: "installments_count", Message: "Un solo pago requiere una cuota"})
	}
	first, err := time.Parse("2006-01-02", r.FirstPaymentDate)
	if err != nil {
		issues = append(issues, httpx.FieldError{Field: "first_payment_date", Message: "Selecciona una fecha válida para el primer pago"})
	} else if start, e := time.Parse("2006-01-02", r.DisbursementDate); e == nil && first.Before(start) {
		issues = append(issues, httpx.FieldError{Field: "first_payment_date", Message: "El primer pago no puede ser anterior a la entrega"})
	}
	if r.TermValue != 0 || r.TermUnit != "" {
		issues = append(issues, httpx.FieldError{Field: "installments_count", Message: "Indica el número de pagos sin combinarlo con un plazo"})
	}
	if len(issues) > 0 {
		return httpx.Validation(issues...)
	}
	return nil
}
