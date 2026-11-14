//go:build perf

package perf

import (
	"fmt"
	"math/rand/v2"
	"net/url"
	"os"
	"testing"
	"time"

	"prestamos/tests/testenv"
)

func duration() time.Duration {
	if d, err := time.ParseDuration(os.Getenv("PERF_DURATION")); err == nil {
		return d
	}
	return 5 * time.Second
}

func newBody(worker, iter int) map[string]string {
	doc := fmt.Sprintf("L%02d%08d", worker, iter) // único por worker e iteración
	return map[string]string{"full_name": "Carga " + doc, "document_id": doc, "phone": "999111222",
		"address": "Av. Carga 1", "address_reference": "esquina"}
}

// TestLoad mide latencia (p50/p95/p99) y throughput de los endpoints críticos con
// la base poblada. Falla si hay errores o si algún p95 supera PERF_P95_MS.
func TestLoad(t *testing.T) {
	workers, d := envInt("PERF_WORKERS", 32), duration()
	limit := time.Duration(envInt("PERF_P95_MS", 250)) * time.Millisecond
	searchWorkers := min(workers, envInt("PERF_SEARCH_WORKERS", 8)) // la búsqueda recorre filas del tenant: uso concurrente realista es bajo

	scenarios := []scenario{
		{"list (paginas 1-10)", workers, func(_, _ int) (int, error) {
			return call("GET", fmt.Sprintf("/customers?limit=20&offset=%d", 20*rand.IntN(10)), nil, token)
		}},
		{"search (nombre)", searchWorkers, func(_, _ int) (int, error) {
			return call("GET", "/customers?q="+url.QueryEscape(testenv.CustomerName(1+rand.IntN(rows))), nil, token)
		}},
		{"search (documento)", searchWorkers, func(_, _ int) (int, error) {
			return call("GET", "/customers?q="+testenv.CustomerDoc(1+rand.IntN(rows)), nil, token)
		}},
		{"get por id", workers, func(_, _ int) (int, error) {
			return call("GET", "/customers/"+ids[rand.IntN(len(ids))], nil, token)
		}},
		{"create", workers, func(w, i int) (int, error) {
			return call("POST", "/customers", newBody(w, i), token)
		}},
		{"mixto 80/20 lectura/esc.", workers, func(w, i int) (int, error) {
			if rand.IntN(10) < 8 {
				return call("GET", fmt.Sprintf("/customers?limit=20&offset=%d", 20*rand.IntN(10)), nil, token)
			}
			return call("POST", "/customers", newBody(w+100, i), token)
		}},
	}

	var results []result
	for _, s := range scenarios {
		results = append(results, runScenario(t, s, d))
	}
	printReport(results)

	for _, r := range results {
		if r.errors > 0 {
			t.Errorf("%s: %d errores de %d requests", r.name, r.errors, r.requests)
		}
		if r.p95 > limit {
			t.Errorf("%s: p95 %s supera el umbral %s", r.name, r.p95, limit)
		}
	}
}

// TestLoad_DeepOffset es INFORMATIVO: muestra cómo se degrada OFFSET en páginas profundas
// (por eso los listados grandes append-only usan cursor). Sin umbral.
func TestLoad_DeepOffset(t *testing.T) {
	r := runScenario(t, scenario{"list (offset profundo)", 32, func(_, _ int) (int, error) {
		return call("GET", fmt.Sprintf("/customers?limit=20&offset=%d", rand.IntN(rows-20)), nil, token)
	}}, duration())
	printReport([]result{r})
	if r.errors > 0 {
		t.Fatalf("%d errores", r.errors)
	}
}

// TestLoad_Login mide el costo de Argon2id (intencionalmente lento): pocos workers y sin umbral estricto.
func TestLoad_Login(t *testing.T) {
	r := runScenario(t, scenario{"login (argon2id)", 4, func(_, _ int) (int, error) {
		_, err := loginToken()
		return 200, err
	}}, duration())
	printReport([]result{r})
	if r.errors > 0 {
		t.Fatalf("%d logins fallaron", r.errors)
	}
}

// TestRLS_TotalMatchesOwnTenant verifica que, con otro tenant poblado, el total que ve el
// tenant "perf" coincide exactamente con SUS filas (RLS no filtra de más ni de menos).
func TestRLS_TotalMatchesOwnTenant(t *testing.T) {
	other, err := env.Tenant(t.Context(), "perf-otro")
	if err != nil {
		t.Fatal(err)
	}
	if err := env.BulkCustomers(t.Context(), other, 500); err != nil {
		t.Fatal(err)
	}
	var want int
	if err := env.Admin.QueryRow(t.Context(), `SELECT count(*) FROM customers WHERE tenant_id=$1::uuid`, tenantID).Scan(&want); err != nil {
		t.Fatal(err)
	}
	got, err := listTotal()
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("el tenant ve %d clientes y tiene %d: posible fuga o filtrado incorrecto", got, want)
	}
}
