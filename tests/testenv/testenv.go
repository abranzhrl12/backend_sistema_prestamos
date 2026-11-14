// Package testenv levanta una BD Postgres temporal con Docker, aplica las migraciones,
// crea un rol de aplicación SIN superusuario (para que RLS se aplique de verdad) y la
// destruye al terminar. Solo para tests e2e y de rendimiento.
package testenv

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"prestamos/internal/platform/db"
	"prestamos/internal/platform/migrate"
)

const (
	adminURL = "postgres://postgres:test@127.0.0.1:55432/prestamos_test?sslmode=disable"
	appURL   = "postgres://app_user:app@127.0.0.1:55432/prestamos_test?sslmode=disable"
)

// Env es el entorno de pruebas listo para usar.
type Env struct {
	Admin *pgxpool.Pool // superusuario: sembrar datos saltándose RLS
	App   *pgxpool.Pool // rol de la app: sujeto a RLS
	root  string
}

// Start levanta el contenedor (si KEEP_TEST_DB=1 no se borra al cerrar).
func Start(ctx context.Context) (*Env, error) {
	root, err := projectRoot()
	if err != nil {
		return nil, err
	}
	e := &Env{root: root}
	port := os.Getenv("TEST_DB_PORT")
	if port == "" {
		port = "55432"
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1024 || n > 65535 {
		return nil, fmt.Errorf("TEST_DB_PORT debe ser un puerto entre 1024 y 65535")
	}
	adminConn := strings.Replace(adminURL, ":55432/", ":"+port+"/", 1)
	appConn := strings.Replace(appURL, ":55432/", ":"+port+"/", 1)
	if err := e.compose(ctx, "up", "-d", "--wait", "--force-recreate"); err != nil {
		return nil, fmt.Errorf("docker compose up: %w (¿Docker está corriendo?)", err)
	}
	if e.Admin, err = connect(ctx, adminConn); err != nil {
		e.Close()
		return nil, err
	}
	if err := e.migrate(ctx); err != nil {
		e.Close()
		return nil, err
	}
	if e.App, err = connect(ctx, appConn); err != nil {
		e.Close()
		return nil, err
	}
	return e, nil
}

// Close cierra conexiones y destruye el contenedor y sus datos.
func (e *Env) Close() {
	if e.App != nil {
		e.App.Close()
	}
	if e.Admin != nil {
		e.Admin.Close()
	}
	if os.Getenv("KEEP_TEST_DB") == "1" {
		return
	}
	cctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	_ = e.compose(cctx, "down", "-v", "--remove-orphans")
}

func (e *Env) compose(ctx context.Context, args ...string) error {
	full := append([]string{"compose", "-f", filepath.Join(e.root, "docker-compose.test.yml")}, args...)
	cmd := exec.CommandContext(ctx, "docker", full...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// migrate aplica las migraciones del proyecto y crea el rol app_user (sin superusuario, para que RLS aplique).
func (e *Env) migrate(ctx context.Context) error {
	if _, err := migrate.Up(ctx, e.Admin, filepath.Join(e.root, "migrations")); err != nil {
		return err
	}
	if _, err := e.Admin.Exec(ctx, `CREATE ROLE app_user LOGIN PASSWORD 'app' NOSUPERUSER NOBYPASSRLS`); err != nil {
		return err
	}
	return migrate.GrantApp(ctx, e.Admin, "app_user")
}

func connect(ctx context.Context, url string) (*pgxpool.Pool, error) {
	var lastErr error
	for range 30 { // la BD puede tardar un instante en aceptar el rol nuevo
		pool, err := db.NewPool(ctx, url, db.PoolOptions{MaxConns: 40, MinConns: 1})
		if err == nil {
			return pool, nil
		}
		lastErr = err
		time.Sleep(200 * time.Millisecond)
	}
	return nil, lastErr
}

func projectRoot() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("no se pudo ubicar la raíz del proyecto")
	}
	return filepath.Join(filepath.Dir(file), "..", ".."), nil
}
