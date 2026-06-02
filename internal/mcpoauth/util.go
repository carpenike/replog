package mcpoauth

import (
	"crypto/rand"
	"encoding/base64"
)

// randString returns a URL-safe random string with nBytes of entropy. Used for
// the AS↔PocketID state/nonce and the client-facing authorization code.
func randString(nBytes int) (string, error) {
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
