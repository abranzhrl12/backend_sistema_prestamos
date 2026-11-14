package authkit

import (
	"testing"
	"time"
)

func benchIssuer(b *testing.B) *TokenIssuer {
	b.Helper()
	i, err := NewTokenIssuer("bench-secret-bench-secret-bench-secret-12", "bench", time.Hour, SystemClock{})
	if err != nil {
		b.Fatal(err)
	}
	return i
}

var benchAccount = &Account{UserID: "u1", TenantID: "t1", Roles: []string{"owner"}}

// Costo de cada request autenticado: validar el JWT.
func BenchmarkToken_Parse(b *testing.B) {
	i := benchIssuer(b)
	tok, _, _ := i.Issue(benchAccount)
	b.ReportAllocs()
	for b.Loop() {
		if _, err := i.Parse(tok); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkToken_Issue(b *testing.B) {
	i := benchIssuer(b)
	b.ReportAllocs()
	for b.Loop() {
		if _, _, err := i.Issue(benchAccount); err != nil {
			b.Fatal(err)
		}
	}
}

// Argon2id es lento A PROPÓSITO (resistencia a fuerza bruta). Solo ocurre en login.
func BenchmarkPassword_Verify_Default(b *testing.B) {
	h := NewPasswordHasher(DefaultPasswordParams)
	enc, _ := h.Hash("secreto")
	b.ReportAllocs()
	for b.Loop() {
		if ok, _ := h.Verify("secreto", enc); !ok {
			b.Fatal("no verifica")
		}
	}
}
