package payments

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

func extensionHash(in ExtensionInput) string {
	value, _ := json.Marshal(in)
	hash := sha256.Sum256(value)
	return hex.EncodeToString(hash[:])
}
