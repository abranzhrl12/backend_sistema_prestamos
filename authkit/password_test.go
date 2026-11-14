package authkit

import "testing"

func TestPasswordRoundTrip(t *testing.T) {
	h := NewPasswordHasher(PasswordParams{Memory: 8 * 1024, Time: 1, Threads: 1, KeyLen: 32, SaltLen: 16})
	enc, err := h.Hash("secreto")
	if err != nil {
		t.Fatal(err)
	}
	if ok, _ := h.Verify("secreto", enc); !ok {
		t.Fatal("debería verificar")
	}
	if ok, _ := h.Verify("otro", enc); ok {
		t.Fatal("no debería verificar")
	}
	if h.NeedsRehash(enc) {
		t.Fatal("no necesita rehash")
	}
}
