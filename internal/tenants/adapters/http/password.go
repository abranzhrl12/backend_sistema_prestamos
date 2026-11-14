package http

import (
	"crypto/rand"
	"math/big"

	"github.com/gofiber/fiber/v3"
	"prestamos/internal/platform/httpauth"
	"prestamos/internal/tenants/domain"
)

func temporaryPassword() (string, error) {
	const alphabet = "abcdefghjkmnpqrstuvwxyz23456789"
	limit := big.NewInt(int64(len(alphabet)))
	for {
		password := make([]byte, 8)
		letter, digit := false, false
		for i := range password {
			n, err := rand.Int(rand.Reader, limit)
			if err != nil {
				return "", err
			}
			password[i] = alphabet[n.Int64()]
			letter = letter || password[i] >= 'a' && password[i] <= 'z'
			digit = digit || password[i] >= '2' && password[i] <= '9'
		}
		if letter && digit {
			return string(password), nil
		}
	}
}

type TemporaryPasswordResponse struct {
	TemporaryPassword string `json:"temporary_password"`
}

// ResetPassword godoc
// @Summary Restablecer la contraseña de un usuario
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Param id path string true "ID de empresa"
// @Param userId path string true "ID de usuario"
// @Success 200 {object} TemporaryPasswordResponse
// @Failure 403 {object} httpx.ErrorResponse
// @Failure 404 {object} httpx.ErrorResponse
// @Router /admin/tenants/{id}/users/{userId}/reset-password [post]
func (h *Handler) ResetPassword(c fiber.Ctx) error {
	tenantID, userID, err := userParams(c)
	if err != nil {
		return err
	}
	password, err := temporaryPassword()
	if err != nil {
		return err
	}
	_, err = h.svc.UpdateUser(c.Context(), httpauth.Claims(c).UserID(), tenantID, userID, domain.UserPatch{Password: &password})
	if err != nil {
		return mapErr(err)
	}
	return c.JSON(TemporaryPasswordResponse{password})
}
