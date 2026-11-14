//go:build perf

package perf

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
)

// call ejecuta una peticiÃ³n en proceso y devuelve el status.
func call(method, path string, body any, tok string) (int, error) {
	var rd io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return 0, err
		}
		rd = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, "/api/v1"+path, rd)
	if err != nil {
		return 0, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	res, err := app.Test(req, fiber.TestConfig{Timeout: 30 * time.Second})
	if err != nil {
		return 0, err
	}
	defer res.Body.Close()
	_, _ = io.Copy(io.Discard, res.Body)
	return res.StatusCode, nil
}

func loginToken() (string, error) {
	req, _ := http.NewRequest("POST", "/api/v1/auth/login",
		bytes.NewBufferString(`{"tenant":"perf","email":"owner@perf.com","password":"`+pass+`"}`))
	req.Header.Set("Content-Type", "application/json")
	res, err := app.Test(req, fiber.TestConfig{Timeout: 30 * time.Second})
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	var out struct {
		Access string `json:"access_token"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil || out.Access == "" {
		return "", fmt.Errorf("login fallÃ³ (status %d): %v", res.StatusCode, err)
	}
	return out.Access, nil
}

// scenario es una operaciÃ³n repetida por N workers en paralelo.
type scenario struct {
	name    string
	workers int
	fn      func(worker, iter int) (status int, err error)
}

type result struct {
	name             string
	requests, errors int
	rps              float64
	p50, p95, p99    time.Duration
	max              time.Duration
}

func runScenario(t *testing.T, s scenario, d time.Duration) result {
	t.Helper()
	lat := make([][]time.Duration, s.workers)
	var errs atomic.Int64
	var wg sync.WaitGroup
	begin := time.Now()
	deadline := begin.Add(d)
	for w := range s.workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; time.Now().Before(deadline); i++ {
				start := time.Now()
				status, err := s.fn(w, i)
				lat[w] = append(lat[w], time.Since(start))
				if err != nil || status >= 400 {
					errs.Add(1)
				}
			}
		}()
	}
	wg.Wait()
	return summarize(s.name, lat, int(errs.Load()), time.Since(begin))
}

func summarize(name string, lat [][]time.Duration, errs int, elapsed time.Duration) result {
	var all []time.Duration
	for _, l := range lat {
		all = append(all, l...)
	}
	sort.Slice(all, func(i, j int) bool { return all[i] < all[j] })
	pct := func(p float64) time.Duration {
		if len(all) == 0 {
			return 0
		}
		return all[min(len(all)-1, int(float64(len(all))*p))]
	}
	r := result{name: name, requests: len(all), errors: errs, rps: float64(len(all)) / elapsed.Seconds(),
		p50: pct(0.50), p95: pct(0.95), p99: pct(0.99)}
	if len(all) > 0 {
		r.max = all[len(all)-1]
	}
	return r
}

func printReport(rs []result) {
	fmt.Printf("\n%-22s %9s %7s %10s %9s %9s %9s %9s\n", "escenario", "requests", "errores", "req/s", "p50", "p95", "p99", "max")
	for _, r := range rs {
		fmt.Printf("%-22s %9d %7d %10.0f %9s %9s %9s %9s\n", r.name, r.requests, r.errors, r.rps,
			r.p50.Round(time.Microsecond), r.p95.Round(time.Microsecond), r.p99.Round(time.Microsecond), r.max.Round(time.Microsecond))
	}
	fmt.Println()
}

// listTotal devuelve meta.total del listado de clientes del tenant autenticado.
func listTotal() (int, error) {
	req, _ := http.NewRequest("GET", "/api/v1/customers?limit=1&include_total=true", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := app.Test(req, fiber.TestConfig{Timeout: 30 * time.Second})
	if err != nil {
		return 0, err
	}
	defer res.Body.Close()
	var out struct {
		Meta struct {
			Total int `json:"total"`
		} `json:"meta"`
	}
	err = json.NewDecoder(res.Body).Decode(&out)
	return out.Meta.Total, err
}
