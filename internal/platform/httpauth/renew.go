package httpauth

import (
	"context"
	"crypto/sha256"
	"sync"
	"time"

	"prestamos/authkit"
)

// renewGrace es cuánto se reutiliza el resultado de una rotación. Las peticiones concurrentes
// (o casi simultáneas) que llevan la misma cookie vieja reciben el mismo par nuevo en lugar
// de que la segunda falle y cierre la sesión.
const renewGrace = 10 * time.Second

// Refresher es el puerto que rota la sesión (lo implementa authkit.Service).
type Refresher interface {
	Refresh(ctx context.Context, refreshToken string) (*authkit.TokenPair, *authkit.Account, error)
}

// Renewed es el resultado de una rotación, listo para escribir en las cookies.
type Renewed struct {
	Pair   *authkit.TokenPair
	Claims *authkit.Claims
	CSRF   string
}

// flight es una rotación en vuelo o ya completada y retenida hasta expires.
type flight struct {
	done    chan struct{} // se cierra al completar; res/err son válidos después
	res     *Renewed
	err     error
	expires time.Time // cero mientras está en vuelo
}

// Renewer rota refresh tokens con deduplicación en memoria, por hash del token viejo.
//
// Límite: el estado vive en un solo proceso. Con varias réplicas, dos peticiones paralelas
// pueden caer en instancias distintas; la segunda rotación fallaría y cerraría la sesión.
// Con réplicas hace falta sticky sessions por cookie o una ventana de gracia en la BD.
type Renewer struct {
	refresher Refresher
	parser    TokenParser
	clock     authkit.Clock
	mu        sync.Mutex
	flights   map[[sha256.Size]byte]*flight
}

// NewRenewer crea el renovador. El reloj se inyecta (no se usa time.Now directamente).
func NewRenewer(r Refresher, p TokenParser, clock authkit.Clock) *Renewer {
	return &Renewer{refresher: r, parser: p, clock: clock, flights: make(map[[sha256.Size]byte]*flight)}
}

// Renew rota el refresh token. Las llamadas concurrentes, o dentro de renewGrace, con el mismo
// token comparten una única rotación y reciben el mismo resultado. Un error significa que la
// sesión no se puede renovar.
func (r *Renewer) Renew(ctx context.Context, refreshToken string) (*Renewed, error) {
	key := sha256.Sum256([]byte(refreshToken))
	r.mu.Lock()
	r.sweepLocked()
	if f, ok := r.flights[key]; ok {
		r.mu.Unlock()
		<-f.done
		return f.res, f.err
	}
	f := &flight{done: make(chan struct{})}
	r.flights[key] = f
	r.mu.Unlock()

	r.execute(ctx, key, f, refreshToken)
	return f.res, f.err
}

// execute ejecuta la rotación y publica el resultado a quien espera. Los errores no se
// retienen: así un fallo transitorio no bloquea reintentos posteriores.
func (r *Renewer) execute(ctx context.Context, key [sha256.Size]byte, f *flight, refreshToken string) {
	defer close(f.done)
	res, err := r.rotate(ctx, refreshToken)
	r.mu.Lock()
	defer r.mu.Unlock()
	f.res, f.err = res, err
	if err != nil {
		delete(r.flights, key)
		return
	}
	f.expires = r.clock.Now().Add(renewGrace)
}

func (r *Renewer) rotate(ctx context.Context, refreshToken string) (*Renewed, error) {
	pair, _, err := r.refresher.Refresh(ctx, refreshToken)
	if err != nil {
		return nil, err
	}
	claims, err := r.parser.Parse(pair.AccessToken)
	if err != nil {
		return nil, err
	}
	csrf, err := randomToken()
	if err != nil {
		return nil, err
	}
	return &Renewed{Pair: pair, Claims: claims, CSRF: csrf}, nil
}

// sweepLocked descarta las rotaciones completadas cuya ventana de gracia ya venció.
// Debe llamarse con r.mu tomado.
func (r *Renewer) sweepLocked() {
	now := r.clock.Now()
	for k, f := range r.flights {
		if !f.expires.IsZero() && now.After(f.expires) {
			delete(r.flights, k)
		}
	}
}
