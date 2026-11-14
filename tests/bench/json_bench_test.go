package bench

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/bytedance/sonic"
)

type benchCustomer struct {
	ID         string    `json:"id"`
	FullName   string    `json:"full_name"`
	DocumentID string    `json:"document_id"`
	Phone      string    `json:"phone"`
	Address    string    `json:"address"`
	AddressRef string    `json:"address_reference"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func benchPage(n int) map[string]any {
	items := make([]benchCustomer, n)
	for i := range items {
		items[i] = benchCustomer{ID: fmt.Sprintf("0190a1b2-c3d4-7e5f-8a9b-%012d", i), FullName: fmt.Sprintf("Cliente %d", i),
			DocumentID: fmt.Sprintf("%010d", i), Phone: "987654321", Address: "Av. Lima 123", AddressRef: "frente al mercado",
			Status: "active", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	}
	return map[string]any{"items": items, "meta": map[string]any{"total": 20000, "limit": n, "offset": 0, "has_more": true}}
}

// go test ./tests/bench -bench . -benchmem
func BenchmarkJSON_Marshal_Stdlib(b *testing.B) {
	v := benchPage(100)
	b.ReportAllocs()
	for b.Loop() {
		_, _ = json.Marshal(v)
	}
}

func BenchmarkJSON_Marshal_Sonic(b *testing.B) {
	v := benchPage(100)
	b.ReportAllocs()
	for b.Loop() {
		_, _ = sonic.Marshal(v)
	}
}

func BenchmarkJSON_Unmarshal_Stdlib(b *testing.B) {
	raw, _ := json.Marshal(benchPage(100))
	b.ReportAllocs()
	for b.Loop() {
		var out map[string]any
		_ = json.Unmarshal(raw, &out)
	}
}

func BenchmarkJSON_Unmarshal_Sonic(b *testing.B) {
	raw, _ := json.Marshal(benchPage(100))
	b.ReportAllocs()
	for b.Loop() {
		var out map[string]any
		_ = sonic.Unmarshal(raw, &out)
	}
}
