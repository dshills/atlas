package hash

import (
	"crypto/sha256"
	"encoding/hex"
)

// Hash returns the lowercase hex-encoded SHA-256 hash of content.
func Hash(content []byte) string {
	h := sha256.Sum256(content)
	return hex.EncodeToString(h[:])
}
