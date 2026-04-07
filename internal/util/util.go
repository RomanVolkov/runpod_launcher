// Package util provides utility functions for runpod-launcher.
package util

import (
	"crypto/rand"
	"encoding/base64"
)

// GenerateAPIKey generates a random, strong API key suitable for vLLM.
// It returns a 32-byte random string encoded in base64, approximately 43 characters.
func GenerateAPIKey() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(bytes), nil
}
