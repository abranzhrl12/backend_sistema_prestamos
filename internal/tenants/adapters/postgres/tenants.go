package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"prestamos/internal/tenants/domain"
)

const tenantCols = `tenants.id::text, tenants.slug, tenants.name, tenants.active, tenants.created_at, tenants.is_system`

func scanTenant(row pgx.Row, t *domain.Tenant, extra ...any) error {
	return row.Scan(append([]any{&t.ID, &t.Slug, &t.Name, &t.Active, &t.CreatedAt, &t.System}, extra...)...)
}

// liveTenant es el predicado de "empresa no eliminada"; las consultas de lectura y escritura lo incluyen.
const liveTenant = `tenants.deleted_at IS NULL`

func (r *Repo) InsertTenant(ctx context.Context, t *domain.Tenant) error {
	err := r.unscoped(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO tenants (id, slug, name, active, is_system) VALUES ($1::uuid,$2,$3,$4,$5)`,
			t.ID, t.Slug, t.Name, t.Active, t.System)
		return uniqueTo(err, domain.ErrSlugTaken)
	})
	return wrap("InsertTenant", err)
}

// GetTenant devuelve la empresa viva con user_count (cuenta usuarios vivos bajo RLS de esa empresa).
func (r *Repo) GetTenant(ctx context.Context, id string) (*domain.Tenant, error) {
	t := &domain.Tenant{}
	err := r.scoped(ctx, id, func(tx pgx.Tx) error {
		return orNotFound(scanTenant(tx.QueryRow(ctx,
			`SELECT `+tenantCols+`, (SELECT count(*)::int FROM users WHERE users.deleted_at IS NULL)
			FROM tenants WHERE tenants.id = $1::uuid AND `+liveTenant, id),
			t, &t.UserCount), domain.ErrTenantNotFound)
	})
	if err != nil {
		return nil, wrap("GetTenant", err)
	}
	return t, nil
}

func (r *Repo) FindTenantBySlug(ctx context.Context, slug string) (*domain.Tenant, error) {
	t := &domain.Tenant{}
	err := r.unscoped(ctx, func(tx pgx.Tx) error {
		return orNotFound(scanTenant(tx.QueryRow(ctx,
			`SELECT `+tenantCols+` FROM tenants WHERE tenants.slug = $1 AND `+liveTenant, slug), t), domain.ErrTenantNotFound)
	})
	if err != nil {
		return nil, wrap("FindTenantBySlug", err)
	}
	return t, nil
}

func (r *Repo) UpdateTenant(ctx context.Context, t *domain.Tenant) error {
	err := r.unscoped(ctx, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE tenants SET name = $2, active = $3 WHERE id = $1::uuid AND `+liveTenant,
			t.ID, t.Name, t.Active)
		if err == nil && tag.RowsAffected() == 0 {
			return domain.ErrTenantNotFound
		}
		return err
	})
	return wrap("UpdateTenant", err)
}

// SoftDeleteTenant marca la empresa como eliminada (nunca borra la fila).
func (r *Repo) SoftDeleteTenant(ctx context.Context, id, actorID string) error {
	err := r.unscoped(ctx, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE tenants SET deleted_at = now(), deleted_by = nullif($2, '')::uuid
			WHERE id = $1::uuid AND `+liveTenant, id, actorID)
		if err == nil && tag.RowsAffected() == 0 {
			return domain.ErrTenantNotFound
		}
		return err
	})
	return wrap("SoftDeleteTenant", err)
}

// RestoreTenant quita la marca de eliminada. Solo afecta a empresas eliminadas.
func (r *Repo) RestoreTenant(ctx context.Context, id string) error {
	err := r.unscoped(ctx, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE tenants SET deleted_at = NULL, deleted_by = NULL
			WHERE id = $1::uuid AND tenants.deleted_at IS NOT NULL`, id)
		if err == nil && tag.RowsAffected() == 0 {
			return domain.ErrTenantNotFound
		}
		return err
	})
	return wrap("RestoreTenant", err)
}

// MarkSystem marca la empresa como de sistema (solo la usa el seed de la plataforma).
func (r *Repo) MarkSystem(ctx context.Context, id string) error {
	err := r.unscoped(ctx, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE tenants SET is_system = true WHERE id = $1::uuid AND `+liveTenant, id)
		if err == nil && tag.RowsAffected() == 0 {
			return domain.ErrTenantNotFound
		}
		return err
	})
	return wrap("MarkSystem", err)
}

func (r *Repo) ListTenants(ctx context.Context, f domain.TenantFilter) (domain.TenantPage, error) {
	where, args := tenantWhere(f)
	var res domain.TenantPage
	err := r.unscoped(ctx, func(tx pgx.Tx) error {
		n := len(args)
		q := fmt.Sprintf(`SELECT %s FROM tenants%s ORDER BY tenants.id DESC LIMIT $%d OFFSET $%d`,
			tenantCols, where, n+1, n+2)
		// limit+1 filas: detecta si hay otra página sin contar la tabla.
		rows, err := tx.Query(ctx, q, append(args, f.Limit+1, f.Offset)...)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var t domain.Tenant
			if err := scanTenant(rows, &t); err != nil {
				return err
			}
			res.Items = append(res.Items, t)
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
		total, err := totalOf(ctx, tx, "SELECT count(*) FROM tenants"+where, args, f.Offset, len(res.Items), res.HasMore)
		res.Total = &total
		return err
	})
	return res, wrap("ListTenants", err)
}

// tenantWhere arma los filtros de listado; siempre excluye las empresas eliminadas.
func tenantWhere(f domain.TenantFilter) (string, []any) {
	conds := []string{liveTenant}
	var args []any
	if q := strings.TrimSpace(f.Query); q != "" {
		args = append(args, likeArg(q))
		conds = append(conds, fmt.Sprintf("(tenants.slug ILIKE $%[1]d OR tenants.name ILIKE $%[1]d)", len(args)))
	}
	if f.Active != nil {
		args = append(args, *f.Active)
		conds = append(conds, fmt.Sprintf("tenants.active = $%d", len(args)))
	}
	return whereClause(conds), args
}
