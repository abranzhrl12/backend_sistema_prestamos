// Command migrate aplica o muestra las migraciones de la BD de desarrollo y otorga permisos al rol de la app.
//
//	go run ./cmd/migrate up|status
//
// Variables: DATABASE_URL; MIGRATE_DATABASE_URL es una sobrescritura opcional para desarrollo;
// APP_DB_USER (rol de la aplicación, def. app_user).
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"

	"prestamos/internal/platform/db"
	"prestamos/internal/platform/migrate"
)

const (
	migrationsDir  = "migrations"
	defaultAppUser = "app_user"
)

var errUsage = errors.New("uso: go run ./cmd/migrate up|status")

func main() {
	if err := run(context.Background(), os.Args[1:], os.Getenv, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, getenv func(string) string, out io.Writer) error {
	if len(args) != 1 || (args[0] != "up" && args[0] != "status") {
		return errUsage
	}
	url := getenv("MIGRATE_DATABASE_URL")
	if url == "" {
		url = getenv("DATABASE_URL")
	}
	if url == "" {
		return errors.New("falta DATABASE_URL")
	}
	pool, err := db.NewPool(ctx, url, db.PoolOptions{MaxConns: 2, MinConns: 1})
	if err != nil {
		return fmt.Errorf("no se pudo conectar a la base de datos: %w", err)
	}
	defer pool.Close()
	if args[0] == "status" {
		return status(ctx, pool, out)
	}
	return up(ctx, pool, appUser(getenv), out)
}

func appUser(getenv func(string) string) string {
	if u := getenv("APP_DB_USER"); u != "" {
		return u
	}
	return defaultAppUser
}

func up(ctx context.Context, pool *pgxpool.Pool, role string, out io.Writer) error {
	applied, err := migrate.UpAndGrant(ctx, pool, migrationsDir, role)
	if err != nil {
		return err
	}
	if len(applied) == 0 {
		fmt.Fprintln(out, "No hay migraciones pendientes.")
	}
	for _, v := range applied {
		fmt.Fprintf(out, "Aplicada: %s\n", v)
	}
	fmt.Fprintf(out, "Permisos de DML otorgados al rol %q.\n", role)
	return nil
}

func status(ctx context.Context, pool *pgxpool.Pool, out io.Writer) error {
	states, err := migrate.Status(ctx, pool, migrationsDir)
	if err != nil {
		return err
	}
	for _, s := range states {
		mark := "[ ]"
		if s.Applied {
			mark = "[x]"
		}
		fmt.Fprintf(out, "%s %s\n", mark, s.Version)
	}
	return nil
}
