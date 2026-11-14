package testenv

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"strconv"

	"github.com/google/uuid"

	"prestamos/authkit"
)

// fastHasher usa parámetros mínimos de Argon2id para que sembrar usuarios sea rápido.
var fastHasher = authkit.NewPasswordHasher(authkit.PasswordParams{Memory: 8 * 1024, Time: 1, Threads: 1, KeyLen: 32, SaltLen: 16})

// Tenant crea una empresa y devuelve su id.
func (e *Env) Tenant(ctx context.Context, slug string) (string, error) {
	id := uuid.Must(uuid.NewV7()).String()
	_, err := e.Admin.Exec(ctx, `INSERT INTO tenants (id, slug, name) VALUES ($1::uuid,$2,$2)`, id, slug)
	return id, err
}

// SystemTenant crea una empresa de sistema (is_system=true, como la plataforma): no se elimina ni suspende.
func (e *Env) SystemTenant(ctx context.Context, slug string) (string, error) {
	id := uuid.Must(uuid.NewV7()).String()
	_, err := e.Admin.Exec(ctx, `INSERT INTO tenants (id, slug, name, is_system) VALUES ($1::uuid,$2,$2,true)`, id, slug)
	return id, err
}

// User crea un usuario activo con los roles dados.
func (e *Env) User(ctx context.Context, tenantID, email, password string, roles ...string) (string, error) {
	hash, err := fastHasher.Hash(password)
	if err != nil {
		return "", err
	}
	id := uuid.Must(uuid.NewV7()).String()
	_, err = e.Admin.Exec(ctx, `INSERT INTO users (id, tenant_id, email, full_name, password_hash, roles)
		VALUES ($1::uuid,$2::uuid,$3,$3,$4,$5)`, id, tenantID, email, hash, roles)
	return id, err
}

// BulkCustomers inserta n clientes de golpe (para pruebas de rendimiento) con datos de
// alta entropía, parecidos a los reales: nombre = 6 hex + 8 hex (md5), documento de 8
// dígitos único (g*7919 mod 1e8) y teléfono de 9 dígitos. Ver CustomerName/CustomerDoc.
func (e *Env) BulkCustomers(ctx context.Context, tenantID string, n int) error {
	_, err := e.Admin.Exec(ctx, `INSERT INTO customers (id, tenant_id, full_name, document_id, phone)
		SELECT gen_random_uuid(), $1::uuid,
		       substr(md5(g::text),1,6) || ' ' || substr(md5((g*7)::text),1,8),
		       lpad(((g::bigint*7919) % 100000000)::text, 8, '0'),
		       '9' || lpad(((g::bigint*104729) % 100000000)::text, 8, '0')
		FROM generate_series(1, $2) g`, tenantID, n)
	return err
}

// CustomerName devuelve el nombre del cliente g-ésimo sembrado (para buscarlo en tests).
func CustomerName(g int) string {
	a, b := md5.Sum([]byte(strconv.Itoa(g))), md5.Sum([]byte(strconv.Itoa(g*7)))
	return hex.EncodeToString(a[:])[:6] + " " + hex.EncodeToString(b[:])[:8]
}

// CustomerDoc devuelve el documento del cliente g-ésimo sembrado.
func CustomerDoc(g int) string { return fmt.Sprintf("%08d", (int64(g)*7919)%100000000) }
