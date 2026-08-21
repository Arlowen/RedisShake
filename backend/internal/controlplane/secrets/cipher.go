package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
)

const ciphertextVersion = "v1"

var ErrInvalidCiphertext = errors.New("invalid ciphertext")

type Cipher struct {
	aead cipher.AEAD
}

func DecodeMasterKey(value string) ([]byte, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}

	key, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return nil, fmt.Errorf("REDISSHAKE_MASTER_KEY must be base64 encoded: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("REDISSHAKE_MASTER_KEY must decode to exactly 32 bytes, got %d", len(key))
	}
	return key, nil
}

func NewCipher(key []byte) (*Cipher, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("master key must be exactly 32 bytes, got %d", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create secret cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create secret GCM: %w", err)
	}
	return &Cipher{aead: aead}, nil
}

func (c *Cipher) Encrypt(plaintext string) (string, error) {
	if c == nil || c.aead == nil {
		return "", errors.New("secret cipher is not configured")
	}
	if plaintext == "" {
		return "", nil
	}

	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate secret nonce: %w", err)
	}
	sealed := c.aead.Seal(nil, nonce, []byte(plaintext), nil)
	payload := append(nonce, sealed...)
	return ciphertextVersion + ":" + base64.RawStdEncoding.EncodeToString(payload), nil
}

func (c *Cipher) Decrypt(ciphertext string) (string, error) {
	if c == nil || c.aead == nil {
		return "", errors.New("secret cipher is not configured")
	}
	if ciphertext == "" {
		return "", nil
	}

	parts := strings.SplitN(ciphertext, ":", 2)
	if len(parts) != 2 || parts[0] != ciphertextVersion {
		return "", ErrInvalidCiphertext
	}
	payload, err := base64.RawStdEncoding.DecodeString(parts[1])
	if err != nil || len(payload) < c.aead.NonceSize()+c.aead.Overhead() {
		return "", ErrInvalidCiphertext
	}
	nonce := payload[:c.aead.NonceSize()]
	sealed := payload[c.aead.NonceSize():]
	plaintext, err := c.aead.Open(nil, nonce, sealed, nil)
	if err != nil {
		return "", ErrInvalidCiphertext
	}
	return string(plaintext), nil
}
