// Package auth integra authkit con Postgres y Fiber (adaptadores).
package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"prestamos/authkit"
	"prestamos/internal/platform/db"
)

// Store implementa authkit.AccountRepository y authkit.RefreshStore.
// Los errores se envuelven con el módulo y la operación; los centinelas de authkit
// siguen funcionando con errors.Is.
type Store struct{ tx db.TxRunner }

func NewStore(tx db.TxRunner) *Store { return &Store{tx: tx} }

// ResolveTenant traduce el slug público a tenant_id (tenants no tiene RLS).
func (s *Store) ResolveTenant(ctx context.Context, slug string) (id string, err error) {
	err = s.tx.InTx(ctx, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT id::text FROM tenants WHERE slug=$1 AND active AND deleted_at IS NULL`, slug).Scan(&id)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return "", authkit.ErrNotFound
	}
	return id, storeErr("ResolveTenant", err)
}

const accountCols = `id::text, tenant_id::text, roles, password_hash, active, session_version, must_change_password`

// liveAccount exige usuario y empresa sin eliminar (y empresa activa). Login y refresh dependen de esto.
const liveAccount = ` AND users.deleted_at IS NULL AND EXISTS (SELECT 1 FROM tenants t
	WHERE t.id = users.tenant_id AND t.active AND t.deleted_at IS NULL)`

func scanAccount(row pgx.Row) (*authkit.Account, error) {
	a := &authkit.Account{}
	if err := row.Scan(&a.UserID, &a.TenantID, &a.Roles, &a.PasswordHash, &a.Active, &a.SessionVersion, &a.MustChangePassword); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, authkit.ErrNotFound
		}
		return nil, err
	}
	return a, nil
}

func (s *Store) FindByLogin(ctx context.Context, tenantID, login string) (a *authkit.Account, err error) {
	err = s.tx.InTenant(ctx, tenantID, func(tx pgx.Tx) error {
		a, err = scanAccount(tx.QueryRow(ctx, `SELECT `+accountCols+` FROM users WHERE lower(email)=lower($1)`+liveAccount, login))
		return err
	})
	return a, storeErr("FindByLogin", err)
}

func (s *Store) FindByID(ctx context.Context, tenantID, userID string) (a *authkit.Account, err error) {
	err = s.tx.InTenant(ctx, tenantID, func(tx pgx.Tx) error {
		a, err = scanAccount(tx.QueryRow(ctx, `SELECT `+accountCols+` FROM users WHERE id=$1::uuid`+liveAccount, userID))
		return err
	})
	return a, storeErr("FindByID", err)
}

func (s *Store) UpdatePasswordHash(ctx context.Context, tenantID, userID, hash string) error {
	return storeErr("UpdatePasswordHash", s.tx.InTenant(ctx, tenantID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE users SET password_hash=$1, updated_at=now() WHERE id=$2::uuid AND deleted_at IS NULL`, hash, userID)
		return err
	}))
}

func (s *Store) Save(ctx context.Context, r authkit.RefreshRecord) error {
	return storeErr("Save", s.tx.InTenant(ctx, r.TenantID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO refresh_tokens (token_hash, tenant_id, user_id, expires_at, session_version) VALUES ($1,$2::uuid,$3::uuid,$4,$5)`,
			r.TokenHash, r.TenantID, r.UserID, r.ExpiresAt, r.SessionVersion)
		return err
	}))
}

func (s *Store) Take(ctx context.Context, tenantID, tokenHash string) (r *authkit.RefreshRecord, err error) {
	err = s.tx.InTenant(ctx, tenantID, func(tx pgx.Tx) error {
		r = &authkit.RefreshRecord{}
		e := tx.QueryRow(ctx, `DELETE FROM refresh_tokens WHERE token_hash=$1 AND expires_at>now()
			RETURNING token_hash, tenant_id::text, user_id::text, expires_at, session_version`, tokenHash).
			Scan(&r.TokenHash, &r.TenantID, &r.UserID, &r.ExpiresAt, &r.SessionVersion)
		if errors.Is(e, pgx.ErrNoRows) {
			return authkit.ErrNotFound
		}
		return e
	})
	return r, storeErr("Take", err)
}

func (s *Store) RevokeAll(ctx context.Context, tenantID, userID string) error {
	return storeErr("RevokeAll", s.tx.InTenant(ctx, tenantID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `DELETE FROM refresh_tokens WHERE user_id=$1::uuid`, userID)
		return err
	}))
}

// storeErr añade "auth.store.<op>" al error. Devuelve nil si no hay error.
func storeErr(op string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("auth.store.%s: %w", op, err)
}
