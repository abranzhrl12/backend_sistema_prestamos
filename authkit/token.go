package authkit

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrInvalidToken = errors.New("authkit: token inválido o expirado")
	// ErrTokenExpired distingue un access token vencido (renovable con refresh) de uno
	// manipulado o malformado (no renovable).
	ErrTokenExpired = errors.New("authkit: access token vencido")
)

// Claims viaja dentro del access token (JWT de vida corta).
type Claims struct {
	TenantID       string   `json:"tid"`
	Roles          []string `json:"roles,omitempty"`
	SessionVersion int64    `json:"sv"`
	jwt.RegisteredClaims
}

func (c *Claims) UserID() string { return c.Subject }

// TokenIssuer emite y valida access tokens HS256.
type TokenIssuer struct {
	secret []byte
	issuer string
	ttl    time.Duration
	clock  Clock
}

func NewTokenIssuer(secret, issuer string, ttl time.Duration, clock Clock) (*TokenIssuer, error) {
	if len(secret) < 32 {
		return nil, errors.New("authkit: el secreto JWT debe tener al menos 32 bytes")
	}
	return &TokenIssuer{secret: []byte(secret), issuer: issuer, ttl: ttl, clock: clock}, nil
}

func (t *TokenIssuer) Issue(a *Account) (string, time.Time, error) {
	now := t.clock.Now()
	exp := now.Add(t.ttl)
	c := Claims{TenantID: a.TenantID, Roles: a.Roles, SessionVersion: a.SessionVersion, RegisteredClaims: jwt.RegisteredClaims{
		Subject: a.UserID, Issuer: t.issuer, IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(exp),
	}}
	s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString(t.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("authkit: firmar token: %w", err)
	}
	return s, exp, nil
}

func (t *TokenIssuer) Parse(raw string) (*Claims, error) {
	c := &Claims{}
	_, err := jwt.ParseWithClaims(raw, c, func(*jwt.Token) (any, error) { return t.secret, nil },
		jwt.WithValidMethods([]string{"HS256"}), jwt.WithIssuer(t.issuer), jwt.WithExpirationRequired(), jwt.WithTimeFunc(t.clock.Now))
	if errors.Is(err, jwt.ErrTokenExpired) {
		return nil, ErrTokenExpired
	}
	if err != nil || c.TenantID == "" || c.Subject == "" {
		return nil, ErrInvalidToken
	}
	return c, nil
}
