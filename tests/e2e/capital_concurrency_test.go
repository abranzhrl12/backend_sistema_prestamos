//go:build e2e

package e2e

import (
	"sync"
	"testing"
)

// Veinte retiros simultáneos de 100 sobre un saldo de 1000: el bloqueo por empresa
// debe dejar exactamente 10 aceptados y 10 rechazados, sin sobregiro.
func TestCapital_ConcurrentWithdrawals_NeverOverdraw(t *testing.T) {
	f := newFixture(t)
	tok := f.ownerToken(t)
	capPost(t, tok, "contribution", "1000")

	const workers = 20
	type result struct {
		status int
		err    error
	}
	results := make(chan result, workers)
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			st, err := doRaw("POST", "/capital/movements",
				map[string]string{"type": "withdrawal", "amount": "100"}, bearer(tok))
			results <- result{st, err}
		}()
	}
	wg.Wait()
	close(results)

	var accepted, rejected int
	for r := range results {
		if r.err != nil {
			t.Fatalf("fallo de transporte: %v", r.err)
		}
		switch r.status {
		case 201:
			accepted++
		case 409:
			rejected++
		default:
			t.Fatalf("status inesperado %d", r.status)
		}
	}
	if accepted != 10 || rejected != 10 {
		t.Fatalf("aceptados %d y rechazados %d; esperaba 10 y 10", accepted, rejected)
	}
	b := capGetBalance(t, tok)
	if b.TotalCash != "0.00" || b.MovementsCount != 11 {
		t.Fatalf("saldo final %+v, esperaba 0.00 con 11 movimientos", b)
	}
}
