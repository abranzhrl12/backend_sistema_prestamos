// Package bootstrap es la raíz de composición: el ÚNICO lugar que conoce todas las
// implementaciones concretas y las inyecta donde se necesitan.
package bootstrap

import (
	"log/slog"

	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5/pgxpool"

	"prestamos/authkit"
	"prestamos/internal/auth"
	customershttp "prestamos/internal/customers/adapters/http"
	customerspg "prestamos/internal/customers/adapters/postgres"
	customersapp "prestamos/internal/customers/app"
	"prestamos/internal/platform/config"
	"prestamos/internal/platform/db"
	"prestamos/internal/platform/httpauth"
	"prestamos/internal/platform/ids"
)

type options struct{ log *slog.Logger }

// Option configura NewApp sin cambiar su firma básica.
type Option func(*options)

// WithLogger fija el logger de la aplicación (por defecto slog.Default()).
func WithLogger(l *slog.Logger) Option {
	return func(o *options) { o.log = l }
}

// NewApp construye la aplicación HTTP con todas sus dependencias.
func NewApp(cfg config.Config, pool *pgxpool.Pool, opts ...Option) (*fiber.App, error) {
	o := options{log: slog.Default()}
	for _, opt := range opts {
		opt(&o)
	}
	tx := db.NewPgRunner(pool)
	authn, authHandler, err := wireAuth(cfg, tx)
	if err != nil {
		return nil, err
	}
	idGen := ids.UUIDv7{}
	customersSvc := customersapp.NewService(customerspg.NewRepo(tx), idGen)

	registerSettings, settingsSvc := newSettingsRegistrar(tx, idGen, authn)
	registrars := []func(fiber.Router){
		registerSettings,
		newLoanProductsRegistrar(tx, idGen, authn),
		newLoansRegistrar(tx, idGen, authn),
		newCapitalRegistrar(tx, idGen, authn, settingsSvc), // capital consulta allow_negative_capital a settings
		adminGroup(newTenantsRegistrar(tx, idGen, authkit.NewPasswordHasher(authkit.DefaultPasswordParams), authn)),
	}

	app := newFiberApp(o.log)
	registerRoutes(app, authHandler, customershttp.NewHandler(customersSvc), authn, registrars...)
	return app, nil
}

// wireAuth cablea authkit con su almacén Postgres y devuelve el middleware y el handler HTTP.
func wireAuth(cfg config.Config, tx db.TxRunner) (fiber.Handler, *auth.Handler, error) {
	clock := authkit.SystemClock{}
	issuer, err := authkit.NewTokenIssuer(cfg.JWTSecret, cfg.JWTIssuer, cfg.AccessTTL, clock)
	if err != nil {
		return nil, nil, err
	}
	store := auth.NewStore(tx)
	svc, err := authkit.NewService(store, store,
		authkit.NewPasswordHasher(authkit.DefaultPasswordParams), issuer, cfg.RefreshTTL, clock)
	if err != nil {
		return nil, nil, err
	}
	cookies := httpauth.CookieSettings{Secure: cfg.CookieSecure, Domain: cfg.CookieDomain, RefreshPath: httpauth.RefreshPath}
	authn := httpauth.Authenticate(issuer, httpauth.NewRenewer(svc, issuer, clock), cookies, store)
	return authn, auth.NewHandler(svc, store, authn, cookies), nil
}
