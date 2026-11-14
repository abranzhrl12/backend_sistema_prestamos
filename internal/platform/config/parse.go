package config

import (
	"fmt"
	"net"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
)

// reader lee variables con DI (get) y acumula TODOS los problemas en lugar de fallar al primero.
// Una variable vacía se trata como no definida y toma su valor por defecto.
type reader struct {
	get      func(string) string
	problems []string
}

func (r *reader) raw(key string) string {
	return strings.TrimSpace(r.get(key))
}

func (r *reader) fail(key, motivo string) {
	r.problems = append(r.problems, key+": "+motivo)
}

// The reader never echoes values: some (DATABASE_URL, JWT_SECRET) are secrets.

func (r *reader) str(key, def string) string {
	if v := r.raw(key); v != "" {
		return v
	}
	return def
}

func (r *reader) enum(key, def string, allowed ...string) string {
	v := r.str(key, def)
	if !slices.Contains(allowed, v) {
		r.fail(key, "debe ser uno de: "+strings.Join(allowed, ", "))
	}
	return v
}

func (r *reader) boolean(key string, def bool) bool {
	v := r.raw(key)
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		r.fail(key, "debe ser true o false")
		return def
	}
	return b
}

func (r *reader) integer(key string, def, lo, hi int) int {
	v := r.raw(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < lo || n > hi {
		r.fail(key, fmt.Sprintf("debe ser un entero entre %d y %d", lo, hi))
		return def
	}
	return n
}

func (r *reader) duration(key string, def, lo, hi time.Duration) time.Duration {
	v := r.raw(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		r.fail(key, "formato de duración inválido (ej. 15m, 720h)")
		return def
	}
	if d < lo || d > hi {
		r.fail(key, fmt.Sprintf("debe estar entre %s y %s", humanDuration(lo), humanDuration(hi)))
	}
	return d
}

// addr valida host:puerto; el host puede estar vacío (":8080").
func (r *reader) addr(key, def string) string {
	v := r.str(key, def)
	if !validAddr(v) {
		r.fail(key, "debe tener formato host:puerto (ej. :8080)")
	}
	return v
}

// postgresURL exige un esquema postgres/postgresql. No repite el error de url.Parse
// porque incluye la URL completa (puede contener la contraseña).
func (r *reader) postgresURL(key string) string {
	if r.raw(key) == "" {
		r.fail(key, "es obligatoria")
		return ""
	}
	return r.optionalPostgresURL(key)
}

// optionalPostgresURL valida la URL solo si está definida; vacía significa "no usar".
func (r *reader) optionalPostgresURL(key string) string {
	v := r.raw(key)
	if v == "" {
		return ""
	}
	u, err := url.Parse(v)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") {
		r.fail(key, "debe ser una URL que empiece por postgres:// o postgresql://")
	}
	return v
}

func validAddr(a string) bool {
	_, port, err := net.SplitHostPort(a)
	if err != nil {
		return false
	}
	p, err := strconv.Atoi(port)
	return err == nil && p >= 1 && p <= 65535
}

// humanDuration imprime 24h o 15m en vez de 24h0m0s.
func humanDuration(d time.Duration) string {
	switch {
	case d%time.Hour == 0:
		return fmt.Sprintf("%dh", d/time.Hour)
	case d%time.Minute == 0:
		return fmt.Sprintf("%dm", d/time.Minute)
	default:
		return d.String()
	}
}
