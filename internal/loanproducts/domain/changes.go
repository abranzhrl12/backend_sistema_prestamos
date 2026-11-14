package domain

import "prestamos/internal/platform/money"

// ChangedFields lista los nombres JSON de los campos que difieren entre dos versiones del producto.
// Los montos se comparan por valor ("5" == "5.00") y los overrides nil cuentan como un valor más.
func ChangedFields(old, in Product) []string {
	changed := make([]string, 0)
	add := func(differs bool, field string) {
		if differs {
			changed = append(changed, field)
		}
	}
	add(old.Name != in.Name, "name")
	add(old.Frequency != in.Frequency, "frequency")
	add(old.InterestType != in.InterestType, "interest_type")
	add(old.RatePeriod != in.RatePeriod, "rate_period")
	add(!sameAmount(old.RateDefault, in.RateDefault), "rate_default")
	add(!sameAmount(old.RateMin, in.RateMin), "rate_min")
	add(!sameAmount(old.RateMax, in.RateMax), "rate_max")
	add(!sameAmount(old.AmountMin, in.AmountMin), "amount_min")
	add(!sameAmount(old.AmountMax, in.AmountMax), "amount_max")
	add(old.TermsMin != in.TermsMin, "terms_min")
	add(old.TermsMax != in.TermsMax, "terms_max")
	add(!samePtr(old.GraceDays, in.GraceDays), "grace_days")
	add(!samePtr(old.LateFeeMode, in.LateFeeMode), "late_fee_mode")
	add(!sameAmountPtr(old.LateFeeAmount, in.LateFeeAmount), "late_fee_amount")
	add(!samePtr(old.LateFeePerDay, in.LateFeePerDay), "late_fee_per_day")
	add(old.FeeType != in.FeeType, "fee_type")
	add(!sameAmount(old.FeeValue, in.FeeValue), "fee_value")
	add(old.Active != in.Active, "active")
	return changed
}

func sameAmount(a, b string) bool {
	da, errA := money.Parse(a)
	db, errB := money.Parse(b)
	return errA == nil && errB == nil && da.Equal(db)
}

func sameAmountPtr(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return sameAmount(*a, *b)
}

// samePtr compara dos punteros opcionales: nil solo es igual a nil.
func samePtr[T comparable](a, b *T) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}
