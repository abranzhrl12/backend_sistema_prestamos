// Package migrate aplica las migraciones SQL (formato goose: bloque "-- +goose Up") sobre Postgres
// y otorga permisos al rol de la aplicación. Lleva el registro en schema_migrations.
package migrate

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	upMarker   = "-- +goose Up"
	downMarker = "-- +goose Down"
)

type file struct{ version, sql string }

// State indica si una migración ya está aplicada.
type State struct {
	Version string
	Applied bool
}

// Up aplica en orden las migraciones pendientes de dir, cada una en su propia transacción.
// Devuelve las versiones aplicadas en esta llamada.
func Up(ctx context.Context, pool *pgxpool.Pool, dir string) ([]string, error) {
	if err := ensureTable(ctx, pool); err != nil {
		return nil, err
	}
	files, err := readFiles(dir)
	if err != nil {
		return nil, err
	}
	applied, err := appliedSet(ctx, pool)
	if err != nil {
		return nil, err
	}
	var done []string
	for _, f := range files {
		if applied[f.version] {
			continue
		}
		if err := applyOne(ctx, pool, f); err != nil {
			return done, fmt.Errorf("migración %s: %w", f.version, err)
		}
		done = append(done, f.version)
	}
	return done, nil
}

// UpAndGrant aplica las migraciones pendientes de dir y después otorga DML al rol de la aplicación.
// Es el paso que comparten cmd/migrate y el arranque automático de la API.
func UpAndGrant(ctx context.Context, pool *pgxpool.Pool, dir, role string) ([]string, error) {
	applied, err := Up(ctx, pool, dir)
	if err != nil {
		return applied, err
	}
	if err := GrantApp(ctx, pool, role); err != nil {
		return applied, fmt.Errorf("no se pudieron otorgar permisos al rol %q: %w", role, err)
	}
	return applied, nil
}

// Status lista todas las migraciones de dir con su estado (aplicada o pendiente).
func Status(ctx context.Context, pool *pgxpool.Pool, dir string) ([]State, error) {
	if err := ensureTable(ctx, pool); err != nil {
		return nil, err
	}
	files, err := readFiles(dir)
	if err != nil {
		return nil, err
	}
	applied, err := appliedSet(ctx, pool)
	if err != nil {
		return nil, err
	}
	out := make([]State, 0, len(files))
	for _, f := range files {
		out = append(out, State{Version: f.version, Applied: applied[f.version]})
	}
	return out, nil
}

// GrantApp da USAGE en el esquema public y DML (SELECT, INSERT, UPDATE, DELETE) sobre todas las tablas al rol.
func GrantApp(ctx context.Context, pool *pgxpool.Pool, role string) error {
	if !ValidRole(role) {
		return fmt.Errorf("nombre de rol inválido %q (solo minúsculas, dígitos y guion bajo)", role)
	}
	ident := pgx.Identifier{role}.Sanitize()
	stmts := []string{
		"GRANT USAGE ON SCHEMA public TO " + ident,
		"GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO " + ident,
	}
	return pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		for _, s := range stmts {
			if _, err := tx.Exec(ctx, s); err != nil {
				return err
			}
		}
		return nil
	})
}

// UpSection devuelve solo el bloque "-- +goose Up" de una migración (sin el bloque Down).
func UpSection(sql string) string {
	if i := strings.Index(sql, upMarker); i >= 0 {
		sql = sql[i+len(upMarker):]
	}
	if j := strings.Index(sql, downMarker); j >= 0 {
		sql = sql[:j]
	}
	return sql
}

func ensureTable(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version text PRIMARY KEY,
		applied_at timestamptz NOT NULL DEFAULT now())`)
	return err
}

func appliedSet(ctx context.Context, pool *pgxpool.Pool) (map[string]bool, error) {
	rows, err := pool.Query(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	set := map[string]bool{}
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		set[v] = true
	}
	return set, rows.Err()
}

func applyOne(ctx context.Context, pool *pgxpool.Pool, f file) error {
	return pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, UpSection(f.sql)); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, f.version)
		return err
	})
}

// readFiles lee dir/*.sql ordenados por nombre (los prefijos numéricos fijan el orden).
func readFiles(dir string) ([]file, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*.sql"))
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	files := make([]file, 0, len(paths))
	for _, p := range paths {
		raw, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		files = append(files, file{version: filepath.Base(p), sql: string(raw)})
	}
	return files, nil
}
