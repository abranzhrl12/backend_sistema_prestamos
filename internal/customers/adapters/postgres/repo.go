// Package postgres implementa domain.Repository sobre pgx con aislamiento por tenant (RLS).
// Toda consulta excluye los clientes eliminados (deleted_at IS NULL); nunca se borra físicamente.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"prestamos/internal/customers/domain"
	"prestamos/internal/platform/db"
)

const uniqueViolation = "23505"

// live es el predicado de "cliente no eliminado". Los índices parciales lo usan: debe repetirse tal cual.
const live = "deleted_at IS NULL"

const cols = `id::text, tenant_id::text, full_name, document_id, coalesce(phone,''), coalesce(phone2,''),
	coalesce(whatsapp,''), coalesce(email,''), coalesce(occupation,''), coalesce(address,''),
	coalesce(address_reference,''), coalesce(notes,''), status, assigned_collector_id::text, created_at, updated_at`

const softDeleteSQL = `UPDATE customers SET deleted_at=now(), deleted_by=nullif($2,'')::uuid, updated_at=now()
	WHERE id=$1::uuid AND deleted_at IS NULL`

const auditSQL = `INSERT INTO audit_log (id, tenant_id, user_id, action, entity, entity_id, data)
	VALUES ($1::uuid,$2::uuid,nullif($3,'')::uuid,'customer.delete','customer',$4::uuid,'{"changed":["deleted_at"]}'::jsonb)`

type Repo struct{ tx db.TxRunner }

func NewRepo(tx db.TxRunner) *Repo { return &Repo{tx: tx} }

func scan(row pgx.Row) (*domain.Customer, error) {
	c := &domain.Customer{}
	var st string
	err := row.Scan(&c.ID, &c.TenantID, &c.FullName, &c.DocumentID, &c.Phone, &c.Phone2, &c.WhatsApp,
		&c.Email, &c.Occupation, &c.Address, &c.AddressRef, &c.Notes, &st, &c.CollectorID, &c.CreatedAt, &c.UpdatedAt)
	c.Status = domain.Status(st)
	return c, err
}

func (r *Repo) Create(ctx context.Context, c *domain.Customer) error {
	return opErr("Create", mapErr(r.tx.InTenant(ctx, c.TenantID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO customers
			(id, tenant_id, full_name, document_id, phone, phone2, whatsapp, email, occupation,
			 address, address_reference, notes, status, assigned_collector_id)
			VALUES ($1::uuid,$2::uuid,$3,$4,nullif($5,''),nullif($6,''),nullif($7,''),nullif($8,''),nullif($9,''),
			        nullif($10,''),nullif($11,''),nullif($12,''),$13,$14::uuid)`,
			c.ID, c.TenantID, c.FullName, c.DocumentID, c.Phone, c.Phone2, c.WhatsApp, c.Email, c.Occupation,
			c.Address, c.AddressRef, c.Notes, string(c.Status), c.CollectorID)
		return err
	})))
}

func (r *Repo) Update(ctx context.Context, c *domain.Customer) error {
	return opErr("Update", mapErr(r.tx.InTenant(ctx, c.TenantID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE customers SET full_name=$2, document_id=$3, phone=nullif($4,''),
			phone2=nullif($5,''), whatsapp=nullif($6,''), email=nullif($7,''), occupation=nullif($8,''),
			address=nullif($9,''), address_reference=nullif($10,''), notes=nullif($11,''), status=$12,
			assigned_collector_id=$13::uuid, updated_at=now() WHERE id=$1::uuid AND `+live,
			c.ID, c.FullName, c.DocumentID, c.Phone, c.Phone2, c.WhatsApp, c.Email, c.Occupation,
			c.Address, c.AddressRef, c.Notes, string(c.Status), c.CollectorID)
		if err == nil && tag.RowsAffected() == 0 {
			return domain.ErrNotFound
		}
		return err
	})))
}

// SoftDelete marca el cliente como eliminado y escribe su auditoría en la misma transacción.
func (r *Repo) SoftDelete(ctx context.Context, d domain.Deletion) error {
	return opErr("SoftDelete", mapErr(r.tx.InTenant(ctx, d.TenantID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, softDeleteSQL, d.ID, d.ActorID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return domain.ErrNotFound
		}
		_, err = tx.Exec(ctx, auditSQL, d.AuditID, d.TenantID, d.ActorID, d.ID)
		return err
	})))
}

func (r *Repo) Get(ctx context.Context, tenantID, id string) (c *domain.Customer, err error) {
	err = r.tx.InTenant(ctx, tenantID, func(tx pgx.Tx) error {
		var e error
		c, e = scan(tx.QueryRow(ctx, `SELECT `+cols+` FROM customers WHERE id=$1::uuid AND `+live, id))
		return e
	})
	return c, opErr("Get", mapErr(err))
}

func (r *Repo) List(ctx context.Context, tenantID string, f domain.ListFilter) (res domain.ListResult, err error) {
	where, args := buildWhere(f)
	err = r.tx.InTenant(ctx, tenantID, func(tx pgx.Tx) error {
		// Pide limit+1 para saber si hay otra página sin contar la tabla.
		// El orden coincide con el índice (tenant_id, lower(full_name), id): LIMIT corta sin ordenar todo.
		n := len(args)
		page := fmt.Sprintf(`SELECT %s FROM customers%s ORDER BY lower(full_name), customers.id LIMIT $%d OFFSET $%d`, cols, where, n+1, n+2)
		rows, e := tx.Query(ctx, page, append(args[:n:n], f.Limit+1, f.Offset)...)
		if e != nil {
			return e
		}
		defer rows.Close()
		for rows.Next() {
			c, e := scan(rows)
			if e != nil {
				return e
			}
			res.Items = append(res.Items, *c)
		}
		if e := rows.Err(); e != nil {
			return e
		}
		if res.HasMore = len(res.Items) > f.Limit; res.HasMore {
			res.Items = res.Items[:f.Limit]
		}
		if !f.WithTotal {
			return nil
		}
		total, e := r.total(ctx, tx, f, where, args, len(res.Items), res.HasMore)
		res.Total = &total
		return e
	})
	return res, opErr("List", err)
}

// total evita el COUNT cuando la página incompleta ya lo determina.
func (r *Repo) total(ctx context.Context, tx pgx.Tx, f domain.ListFilter, where string, args []any, got int, hasMore bool) (int, error) {
	if !hasMore && (got > 0 || f.Offset == 0) {
		return f.Offset + got, nil
	}
	var total int
	err := tx.QueryRow(ctx, `SELECT count(*) FROM customers`+where, args...).Scan(&total)
	return total, err
}

// searchCondition elige columnas según lo escrito: solo dígitos => documento/teléfono; si no => nombre/documento (pasaportes alfanuméricos).
// Menos columnas por fila = búsqueda más barata (ILIKE no usa índices bajo RLS, ver migración 00003).
func searchCondition(q string, n int) string {
	if isDigits(q) {
		return fmt.Sprintf("(document_id ILIKE $%[1]d OR phone ILIKE $%[1]d)", n)
	}
	return fmt.Sprintf("(full_name ILIKE $%[1]d OR document_id ILIKE $%[1]d)", n)
}

func isDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// buildWhere agrega solo los filtros presentes; siempre excluye los clientes eliminados.
func buildWhere(f domain.ListFilter) (string, []any) {
	conds := []string{live}
	var args []any
	if f.Status != "" {
		args = append(args, string(f.Status))
		conds = append(conds, fmt.Sprintf("status = $%d", len(args)))
	}
	if q := strings.TrimSpace(f.Query); q != "" {
		args = append(args, "%"+escapeLike(q)+"%")
		conds = append(conds, searchCondition(q, len(args)))
	}
	return " WHERE " + strings.Join(conds, " AND "), args
}

func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(strings.TrimSpace(s))
}

func mapErr(err error) error {
	var pg *pgconn.PgError
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return domain.ErrNotFound
	case errors.As(err, &pg) && pg.Code == uniqueViolation:
		return domain.ErrDuplicateDocument
	}
	return err
}

// opErr añade el contexto de módulo y capa al error (p. ej. "customers.repo.Create: ...").
// Envuelve con %w: los centinelas del dominio siguen funcionando con errors.Is/As.
func opErr(op string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("customers.repo.%s: %w", op, err)
}
