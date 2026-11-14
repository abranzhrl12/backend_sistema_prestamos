package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"prestamos/internal/tenants/domain"
)

const userCols = `users.id::text, users.tenant_id::text, users.email, users.full_name, users.roles, users.active, users.created_at`

// liveUser es el predicado de "usuario no eliminado"; las consultas de lectura y escritura lo incluyen.
const liveUser = `users.deleted_at IS NULL`

func scanUser(row pgx.Row) (*domain.User, error) {
	u := &domain.User{}
	err := row.Scan(&u.ID, &u.TenantID, &u.Email, &u.FullName, &u.Roles, &u.Active, &u.CreatedAt)
	return u, err
}

func (r *Repo) InsertUser(ctx context.Context, u *domain.User, passwordHash string) error {
	err := r.scoped(ctx, u.TenantID, func(tx pgx.Tx) error {
		if err := checkGlobalEmail(ctx, tx, u.TenantID, u.ID, u.Email); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `INSERT INTO users (id, tenant_id, email, full_name, password_hash, roles, active, must_change_password)
			VALUES ($1::uuid,$2::uuid,$3,$4,$5,$6,$7, NOT ($6::text[] @> ARRAY['superadmin']))`,
			u.ID, u.TenantID, u.Email, u.FullName, passwordHash, u.Roles, u.Active)
		return uniqueTo(err, domain.ErrEmailTaken)
	})
	return wrap("InsertUser", err)
}

func (r *Repo) GetUser(ctx context.Context, tenantID, id string) (*domain.User, error) {
	var u *domain.User
	err := r.scoped(ctx, tenantID, func(tx pgx.Tx) error {
		var e error
		u, e = scanUser(tx.QueryRow(ctx, `SELECT `+userCols+` FROM users WHERE users.id = $1::uuid AND `+liveUser, id))
		return orNotFound(e, domain.ErrUserNotFound)
	})
	if err != nil {
		return nil, wrap("GetUser", err)
	}
	return u, nil
}

func (r *Repo) FindUserByEmail(ctx context.Context, tenantID, email string) (*domain.User, error) {
	var u *domain.User
	err := r.scoped(ctx, tenantID, func(tx pgx.Tx) error {
		var e error
		u, e = scanUser(tx.QueryRow(ctx, `SELECT `+userCols+` FROM users
			WHERE lower(users.email) = lower($1) AND `+liveUser, email))
		return orNotFound(e, domain.ErrUserNotFound)
	})
	if err != nil {
		return nil, wrap("FindUserByEmail", err)
	}
	return u, nil
}

func (r *Repo) UpdateUser(ctx context.Context, u *domain.User) error {
	err := r.scoped(ctx, u.TenantID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE users SET full_name = $2, roles = $3, active = $4, updated_at = now()
			WHERE id = $1::uuid AND `+liveUser, u.ID, u.FullName, u.Roles, u.Active)
		if err == nil && tag.RowsAffected() == 0 {
			return domain.ErrUserNotFound
		}
		return err
	})
	return wrap("UpdateUser", err)
}

func (r *Repo) SetPasswordHash(ctx context.Context, tenantID, userID, hash string) error {
	err := r.scoped(ctx, tenantID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE users SET password_hash = $1, must_change_password = true, session_version = session_version + 1, updated_at = now()
			WHERE id = $2::uuid AND `+liveUser, hash, userID)
		if err == nil && tag.RowsAffected() == 0 {
			return domain.ErrUserNotFound
		}
		return err
	})
	return wrap("SetPasswordHash", err)
}

func (r *Repo) ListUsers(ctx context.Context, tenantID string, f domain.UserFilter) (domain.UserPage, error) {
	where, args := userWhere(f)
	var res domain.UserPage
	err := r.scoped(ctx, tenantID, func(tx pgx.Tx) error {
		n := len(args)
		q := fmt.Sprintf(`SELECT %s FROM users%s ORDER BY users.id DESC LIMIT $%d OFFSET $%d`,
			userCols, where, n+1, n+2)
		rows, err := tx.Query(ctx, q, append(args, f.Limit+1, f.Offset)...)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			u, err := scanUser(rows)
			if err != nil {
				return err
			}
			res.Items = append(res.Items, *u)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		if res.HasMore = len(res.Items) > f.Limit; res.HasMore {
			res.Items = res.Items[:f.Limit]
		}
		if !f.WithTotal {
			return nil
		}
		total, err := totalOf(ctx, tx, "SELECT count(*) FROM users"+where, args, f.Offset, len(res.Items), res.HasMore)
		res.Total = &total
		return err
	})
	return res, wrap("ListUsers", err)
}

// userWhere arma los filtros de listado; siempre excluye los usuarios eliminados.
func userWhere(f domain.UserFilter) (string, []any) {
	conds := []string{liveUser}
	var args []any
	if q := strings.TrimSpace(f.Query); q != "" {
		args = append(args, likeArg(q))
		conds = append(conds, fmt.Sprintf("(users.email ILIKE $%[1]d OR users.full_name ILIKE $%[1]d)", len(args)))
	}
	if f.Active != nil {
		args = append(args, *f.Active)
		conds = append(conds, fmt.Sprintf("users.active = $%d", len(args)))
	}
	return whereClause(conds), args
}

// SoftDeleteUser marca al usuario como eliminado (nunca borra la fila).
func (r *Repo) SoftDeleteUser(ctx context.Context, tenantID, userID, actorID string) error {
	err := r.scoped(ctx, tenantID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE users SET deleted_at = now(), deleted_by = nullif($2, '')::uuid, updated_at = now()
			WHERE id = $1::uuid AND `+liveUser, userID, actorID)
		if err == nil && tag.RowsAffected() == 0 {
			return domain.ErrUserNotFound
		}
		return err
	})
	return wrap("SoftDeleteUser", err)
}

// RestoreUser quita la marca de eliminado. Si el correo ya lo usa otro usuario vivo, ErrEmailTaken.
func (r *Repo) RestoreUser(ctx context.Context, tenantID, userID string) error {
	err := r.scoped(ctx, tenantID, func(tx pgx.Tx) error {
		var email string
		err := tx.QueryRow(ctx, `SELECT email FROM users WHERE id=$1::uuid AND deleted_at IS NOT NULL`, userID).Scan(&email)
		if err != nil {
			return orNotFound(err, domain.ErrUserNotFound)
		}
		if err := checkGlobalEmail(ctx, tx, tenantID, userID, email); err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `UPDATE users SET deleted_at = NULL, deleted_by = NULL, updated_at = now()
			WHERE id = $1::uuid AND users.deleted_at IS NOT NULL`, userID)
		if err != nil {
			return uniqueTo(err, domain.ErrEmailTaken)
		}
		if tag.RowsAffected() == 0 {
			return domain.ErrUserNotFound
		}
		return nil
	})
	return wrap("RestoreUser", err)
}

// LockActiveSuperadmins devuelve los superadmins activos y vivos de la empresa, bloqueando sus filas.
func (r *Repo) LockActiveSuperadmins(ctx context.Context, tenantID string) ([]string, error) {
	var ids []string
	err := r.scoped(ctx, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT users.id::text FROM users
			WHERE users.roles @> $1::text[] AND users.active AND `+liveUser+` FOR UPDATE`, []string{domain.RoleSuperadmin})
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				return err
			}
			ids = append(ids, id)
		}
		return rows.Err()
	})
	return ids, wrap("LockActiveSuperadmins", err)
}

// RevokeSessions borra los refresh tokens del usuario (cierra sus sesiones).
func (r *Repo) RevokeSessions(ctx context.Context, tenantID, userID string) error {
	err := r.scoped(ctx, tenantID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE users SET session_version = session_version + 1 WHERE id = $1::uuid`, userID)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `DELETE FROM refresh_tokens WHERE user_id = $1::uuid`, userID)
		return err
	})
	return wrap("RevokeSessions", err)
}

// RevokeTenantSessions borra los refresh tokens de toda la empresa.
func (r *Repo) RevokeTenantSessions(ctx context.Context, tenantID string) error {
	err := r.scoped(ctx, tenantID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE users SET session_version = session_version + 1 WHERE tenant_id = $1::uuid`, tenantID)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `DELETE FROM refresh_tokens WHERE tenant_id = $1::uuid`, tenantID)
		return err
	})
	return wrap("RevokeTenantSessions", err)
}
