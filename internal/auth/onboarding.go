package auth

import (
	"context"
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5"
	"prestamos/authkit"
	"prestamos/internal/platform/httpauth"
	"prestamos/internal/platform/httpx"
)

// ResolveEmail reads each tenant under its own RLS context and rejects ambiguity.
func (s *Store) ResolveEmail(ctx context.Context, email string) (string, error) {
	var ids []string
	err := s.tx.InTx(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT id::text FROM tenants WHERE active AND deleted_at IS NULL`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				return err
			}
			ids = append(ids, id)
		}
		return rows.Err()
	})
	if err != nil {
		return "", err
	}
	var found string
	for _, id := range ids {
		_, err := s.FindByLogin(ctx, id, strings.TrimSpace(email))
		if errors.Is(err, authkit.ErrNotFound) {
			continue
		}
		if err != nil {
			return "", err
		}
		if found != "" {
			return "", authkit.ErrNotFound
		}
		found = id
	}
	if found == "" {
		return "", authkit.ErrNotFound
	}
	return found, nil
}

func (s *Store) ChangePassword(ctx context.Context, tenantID, userID, current, next string) error {
	if current == next {
		return authkit.ErrInvalidCredentials
	}
	h := authkit.NewPasswordHasher(authkit.DefaultPasswordParams)
	return s.tx.InTenant(ctx, tenantID, func(tx pgx.Tx) error {
		var hash string
		err := tx.QueryRow(ctx, `SELECT password_hash FROM users WHERE id=$1::uuid AND active`+liveAccount+` FOR UPDATE`, userID).Scan(&hash)
		if err != nil {
			return authkit.ErrInvalidCredentials
		}
		ok, err := h.Verify(current, hash)
		if err != nil || !ok {
			return authkit.ErrInvalidCredentials
		}
		hash, err = h.Hash(next)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE users SET password_hash=$1, must_change_password=false, session_version=session_version+1, updated_at=now() WHERE id=$2::uuid`, hash, userID)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `DELETE FROM refresh_tokens WHERE user_id=$1::uuid`, userID)
		return err
	})
}

func validPassword(password string) bool {
	n := utf8.RuneCountInString(password)
	if n < 10 || n > 128 {
		return false
	}
	var letter, digit bool
	for _, r := range password {
		letter = letter || unicode.IsLetter(r)
		digit = digit || unicode.IsDigit(r)
	}
	return letter && digit
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// ChangePassword godoc
// @Summary Cambiar la contraseña propia y cerrar las sesiones
// @Tags auth
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body ChangePasswordRequest true "contraseñas actual y nueva"
// @Success 204
// @Failure 400 {object} httpx.ErrorResponse
// @Failure 401 {object} httpx.ErrorResponse
// @Router /auth/change-password [post]
func (h *Handler) ChangePassword(c fiber.Ctx) error {
	var req ChangePasswordRequest
	if err := c.Bind().Body(&req); err != nil {
		return httpx.InvalidJSON()
	}
	var issues []httpx.FieldError
	if req.CurrentPassword == "" {
		issues = append(issues, httpx.FieldError{Field: "current_password", Message: "La contraseña actual es obligatoria"})
	}
	if !validPassword(req.NewPassword) {
		issues = append(issues, httpx.FieldError{Field: "new_password", Message: "La contraseña debe tener entre 10 y 128 caracteres, con letra y número"})
	}
	if req.NewPassword == req.CurrentPassword && req.NewPassword != "" {
		issues = append(issues, httpx.FieldError{Field: "new_password", Message: "La contraseña nueva debe ser diferente a la actual"})
	}
	if len(issues) > 0 {
		return httpx.Validation(issues...)
	}
	cl := httpauth.Claims(c)
	if err := h.tenants.ChangePassword(c.Context(), cl.TenantID, cl.UserID(), req.CurrentPassword, req.NewPassword); err != nil {
		if errors.Is(err, authkit.ErrInvalidCredentials) {
			return httpx.Validation(httpx.FieldError{Field: "current_password", Message: "La contraseña actual es incorrecta"})
		}
		return err
	}
	h.cookies.Clear(c)
	return c.SendStatus(fiber.StatusNoContent)
}
