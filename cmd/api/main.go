// @title        Prestamos API
// @version      1.0
// @description  API multitenant de préstamos
// @BasePath     /api/v1
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"prestamos/internal/bootstrap"
	"prestamos/internal/platform/config"
	"prestamos/internal/platform/db"
	"prestamos/internal/platform/obs"
)

const (
	exitFatal  = 1
	exitConfig = 2
)

func main() {
	if err := config.LoadDotEnv(".env"); err != nil {
		exit(exitConfig, err)
	}
	cfg, err := config.LoadFromOS()
	if err != nil {
		exit(exitConfig, err)
	}
	log := obs.NewLogger(os.Stdout, cfg.LogLevel, cfg.LogFormat)
	slog.SetDefault(log)
	if err := run(cfg, log); err != nil {
		log.Error("fatal", "err", err)
		os.Exit(exitFatal)
	}
}

// exit imprime el error de arranque en stderr (antes de tener logger) y termina el proceso.
func exit(code int, err error) {
	_, _ = fmt.Fprintln(os.Stderr, err)
	os.Exit(code)
}

func run(cfg config.Config, log *slog.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := bootstrap.RunStartup(ctx, cfg, log, bootstrap.DefaultStartupDeps()); err != nil {
		return err
	}
	pool, err := db.NewPool(ctx, cfg.DatabaseURL, db.PoolOptions{
		MaxConns: cfg.DBMaxConns,
		MinConns: cfg.DBMinConns,
		Tracer:   &obs.SlowQueryTracer{Log: log, Threshold: cfg.SlowQueryThreshold},
	})
	if err != nil {
		return err
	}
	defer pool.Close()

	app, err := bootstrap.NewApp(cfg, pool, bootstrap.WithLogger(log))
	if err != nil {
		return err
	}
	go func() {
		<-ctx.Done()
		_ = app.Shutdown()
	}()
	log.Info("startup", "env", cfg.Env, "addr", cfg.HTTPAddr, "go_version", runtime.Version())
	return app.Listen(cfg.HTTPAddr)
}
