package httpauth_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"prestamos/internal/platform/httpauth"
)

// Varias pestañas o peticiones paralelas con la misma cookie vieja: todas deben salir bien
// y el refresh solo debe rotarse una vez.
func TestAuthenticate_ConcurrentRequestsRotateOnce(t *testing.T) {
	const n = 20
	h := newHarness(t)
	access, refresh := h.session(t)
	h.clock.Advance(16 * time.Minute)

	gate := make(chan struct{})
	h.sessions.setGate(gate)

	results := make([]result, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i], errs[i] = h.send("GET", "/api/v1/me",
				withCookie(httpauth.AccessCookie, access), withCookie(httpauth.RefreshCookie, refresh))
		}()
	}
	close(gate) // libera la rotación; las llegadas tardías caen en la ventana de gracia
	wg.Wait()

	if got := h.sessions.callCount(); got != 1 {
		t.Fatalf("esperaba 1 rotación, hubo %d", got)
	}
	var wantRefresh, wantCSRF string
	for i, r := range results {
		if errs[i] != nil {
			t.Fatalf("petición %d: %v", i, errs[i])
		}
		expectStatus(t, r, 200)
		rc := r.cookie(httpauth.RefreshCookie, httpauth.RefreshPath)
		cc := r.cookie(httpauth.CSRFCookie, "/")
		if rc == nil || cc == nil {
			t.Fatalf("petición %d sin cookies de sesión", i)
		}
		if i == 0 {
			wantRefresh, wantCSRF = rc.Value, cc.Value
			continue
		}
		if rc.Value != wantRefresh || cc.Value != wantCSRF {
			t.Fatalf("petición %d recibió otro par de cookies; debían ser el mismo", i)
		}
	}
}

func TestRenewer_ReusesResultWithinGraceThenRejects(t *testing.T) {
	h := newHarness(t)
	_, refresh := h.session(t)
	renewer := httpauth.NewRenewer(h.sessions, h.issuer, h.clock)
	ctx := context.Background()

	first, err := renewer.Renew(ctx, refresh)
	if err != nil {
		t.Fatalf("primera rotación: %v", err)
	}
	h.clock.Advance(9 * time.Second)
	again, err := renewer.Renew(ctx, refresh)
	if err != nil {
		t.Fatalf("dentro de la gracia debe reutilizar el resultado: %v", err)
	}
	if again.Pair.RefreshToken != first.Pair.RefreshToken || again.CSRF != first.CSRF {
		t.Fatal("dentro de la gracia debe devolver el mismo par y el mismo CSRF")
	}
	if got := h.sessions.callCount(); got != 1 {
		t.Fatalf("esperaba 1 rotación dentro de la gracia, hubo %d", got)
	}

	h.clock.Advance(2 * time.Second) // ya pasaron más de 10 s desde la rotación
	if _, err := renewer.Renew(ctx, refresh); err == nil {
		t.Fatal("pasada la gracia el token consumido debe rechazarse")
	}
}

func TestRenewer_FailedRotationIsNotCached(t *testing.T) {
	h := newHarness(t)
	renewer := httpauth.NewRenewer(h.sessions, h.issuer, h.clock)
	for range 2 {
		if _, err := renewer.Renew(context.Background(), "tenant.nope"); err == nil {
			t.Fatal("un token desconocido no debe renovar")
		}
	}
	if got := h.sessions.callCount(); got != 2 {
		t.Fatalf("los fallos no se retienen: esperaba 2 llamadas al servicio, hubo %d", got)
	}
}
