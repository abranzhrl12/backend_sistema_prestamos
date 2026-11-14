package http

import (
	"strings"
	"testing"
)

func TestTemporaryPassword_IsShortAndReadable(t *testing.T) {
	seen := map[string]bool{}
	for range 100 {
		password, err := temporaryPassword()
		if err != nil {
			t.Fatal(err)
		}
		if len(password) != 8 {
			t.Fatalf("temporary password must contain 8 characters, got %d", len(password))
		}
		letter, digit := false, false
		for _, c := range password {
			if !strings.ContainsRune("abcdefghjkmnpqrstuvwxyz23456789", c) {
				t.Fatal("temporary password contains an ambiguous character")
			}
			letter = letter || c >= 'a' && c <= 'z'
			digit = digit || c >= '2' && c <= '9'
		}
		if !letter || !digit {
			t.Fatal("temporary password must contain letters and numbers")
		}
		if seen[password] {
			t.Fatal("temporary passwords must be generated independently")
		}
		seen[password] = true
	}
}
