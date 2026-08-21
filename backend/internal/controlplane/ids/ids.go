package ids

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// New returns a random 128-bit identifier formatted as 32 lowercase hex
// characters. It avoids embedding database-specific ID generation into the
// domain model.
func New() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", fmt.Errorf("generate id: %w", err)
	}
	return hex.EncodeToString(value[:]), nil
}
