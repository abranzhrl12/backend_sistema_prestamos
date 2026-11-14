package app

type CapitalShortfall struct{ Available string }

func (e *CapitalShortfall) Error() string {
	return "Capital insuficiente. Disponible: S/ " + e.Available + ". Registra un aporte en Capital para continuar."
}
func (e *CapitalShortfall) Unwrap() error { return ErrInsufficientCapital }
