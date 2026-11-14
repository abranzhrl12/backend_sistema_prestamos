// Package authkit es un SDK de autenticación independiente del framework HTTP y de la BD.
// Se integra implementando los puertos AccountRepository y RefreshStore.
package authkit

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"time"
)

var (
	ErrInvalidCredentials = errors.New("authkit: credenciales inválidas")
	ErrInvalidRefresh     = errors.New("authkit: refresh token inválido")
	ErrNotFound           = errors.New("authkit: no encontrado")
)

// Account es la vista mínima de un usuario que necesita el SDK.
type Account struct {
	UserID             string
	TenantID           string
	Roles              []string
	PasswordHash       string
	Active             bool
	SessionVersion     int64
	MustChangePassword bool
}

type RefreshRecord struct {
	SessionVersion int64
	TokenHash      string
	TenantID       string
	UserID         string
	ExpiresAt      time.Time
}

// AccountRepository es un puerto que implementa la aplicación.
type AccountRepository interface {
	FindByLogin(ctx context.Context, tenantID, login string) (*Account, error) // ErrNotFound si no existe
	FindByID(ctx context.Context, tenantID, userID string) (*Account, error)
	UpdatePasswordHash(ctx context.Context, tenantID, userID, hash string) error
}

// RefreshStore es un puerto que implementa la aplicación.
type RefreshStore interface {
	Save(ctx context.Context, r RefreshRecord) error
	// Take consume el registro de forma atómica (rotación). ErrNotFound si no existe o expiró.
	Take(ctx context.Context, tenantID, tokenHash string) (*RefreshRecord, error)
	RevokeAll(ctx context.Context, tenantID, userID string) error
}

type TokenPair struct {
	AccessToken      string
	AccessExpiresAt  time.Time
	RefreshToken     string
	RefreshExpiresAt time.Time
}

type Service struct {
	accounts   AccountRepository
	refresh    RefreshStore
	hasher     *PasswordHasher
	issuer     *TokenIssuer
	refreshTTL time.Duration
	clock      Clock
	dummyHash  string
}

func NewService(a AccountRepository, r RefreshStore, h *PasswordHasher, i *TokenIssuer, refreshTTL time.Duration, clock Clock) (*Service, error) {
	dummy, err := h.Hash("dummy-password")
	if err != nil {
		return nil, err
	}
	return &Service{accounts: a, refresh: r, hasher: h, issuer: i, refreshTTL: refreshTTL, clock: clock, dummyHash: dummy}, nil
}

func (s *Service) Issuer() *TokenIssuer { return s.issuer }

func (s *Service) Hasher() *PasswordHasher { return s.hasher }

func (s *Service) Login(ctx context.Context, tenantID, login, password string) (*TokenPair, *Account, error) {
	acc, err := s.accounts.FindByLogin(ctx, tenantID, login)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			_, _ = s.hasher.Verify(password, s.dummyHash) // mitiga enumeración por timing
			return nil, nil, ErrInvalidCredentials
		}
		return nil, nil, err
	}
	ok, err := s.hasher.Verify(password, acc.PasswordHash)
	if err != nil || !ok || !acc.Active {
		return nil, nil, ErrInvalidCredentials
	}
	if s.hasher.NeedsRehash(acc.PasswordHash) {
		if h, err := s.hasher.Hash(password); err == nil {
			_ = s.accounts.UpdatePasswordHash(ctx, acc.TenantID, acc.UserID, h)
		}
	}
	pair, err := s.newPair(ctx, acc)
	return pair, acc, err
}

// Refresh rota el refresh token: el anterior queda inutilizable.
func (s *Service) Refresh(ctx context.Context, refreshToken string) (*TokenPair, *Account, error) {
	tenantID, secret, ok := strings.Cut(refreshToken, ".")
	if !ok || tenantID == "" || secret == "" {
		return nil, nil, ErrInvalidRefresh
	}
	rec, err := s.refresh.Take(ctx, tenantID, hashToken(refreshToken))
	if err != nil {
		return nil, nil, ErrInvalidRefresh
	}
	acc, err := s.accounts.FindByID(ctx, rec.TenantID, rec.UserID)
	if err != nil || !acc.Active || acc.SessionVersion != rec.SessionVersion {
		return nil, nil, ErrInvalidRefresh
	}
	pair, err := s.newPair(ctx, acc)
	return pair, acc, err
}

// Logout revoca todas las sesiones del usuario.
func (s *Service) Logout(ctx context.Context, tenantID, userID string) error {
	return s.refresh.RevokeAll(ctx, tenantID, userID)
}

func (s *Service) newPair(ctx context.Context, acc *Account) (*TokenPair, error) {
	access, aexp, err := s.issuer.Issue(acc)
	if err != nil {
		return nil, err
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return nil, err
	}
	raw := acc.TenantID + "." + base64.RawURLEncoding.EncodeToString(buf)
	rexp := s.clock.Now().Add(s.refreshTTL)
	rec := RefreshRecord{TokenHash: hashToken(raw), TenantID: acc.TenantID, UserID: acc.UserID, ExpiresAt: rexp, SessionVersion: acc.SessionVersion}
	if err := s.refresh.Save(ctx, rec); err != nil {
		return nil, err
	}
	return &TokenPair{AccessToken: access, AccessExpiresAt: aexp, RefreshToken: raw, RefreshExpiresAt: rexp}, nil
}

func hashToken(t string) string { h := sha256.Sum256([]byte(t)); return hex.EncodeToString(h[:]) }
