package secrets

import (
	"bytes"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
)

func TestCipherRoundTripUsesRandomNonce(t *testing.T) {
	cipher, err := NewCipher(bytes.Repeat([]byte{0x11}, 32))
	if err != nil {
		t.Fatalf("NewCipher() error = %v", err)
	}
	const plaintext = "super-secret-password"

	first, err := cipher.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	second, err := cipher.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt() second error = %v", err)
	}
	if first == second {
		t.Fatal("Encrypt() returned identical ciphertexts for the same plaintext")
	}
	if strings.Contains(first, plaintext) {
		t.Fatal("ciphertext contains plaintext")
	}
	decrypted, err := cipher.Decrypt(first)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}
	if decrypted != plaintext {
		t.Fatalf("Decrypt() = %q, want %q", decrypted, plaintext)
	}
}

func TestCipherRejectsWrongKey(t *testing.T) {
	first, _ := NewCipher(bytes.Repeat([]byte{0x11}, 32))
	second, _ := NewCipher(bytes.Repeat([]byte{0x22}, 32))
	ciphertext, err := first.Encrypt("secret")
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	if _, err := second.Decrypt(ciphertext); !errors.Is(err, ErrInvalidCiphertext) {
		t.Fatalf("Decrypt() error = %v, want ErrInvalidCiphertext", err)
	}
}

func TestDecodeMasterKey(t *testing.T) {
	key := bytes.Repeat([]byte{0x33}, 32)
	decoded, err := DecodeMasterKey("  " + base64.StdEncoding.EncodeToString(key) + "\n")
	if err != nil {
		t.Fatalf("DecodeMasterKey() error = %v", err)
	}
	if !bytes.Equal(decoded, key) {
		t.Fatal("DecodeMasterKey() returned a different key")
	}
	if _, err := DecodeMasterKey("not-base64"); err == nil {
		t.Fatal("DecodeMasterKey() accepted invalid base64")
	}
}

func TestRedact(t *testing.T) {
	input := `redis://alice:password123@127.0.0.1:6379 {"password":"json-secret","token":"api-token"} secret=plain-secret`
	redacted := Redact(input)
	for _, secret := range []string{"password123", "json-secret", "api-token", "plain-secret"} {
		if strings.Contains(redacted, secret) {
			t.Fatalf("Redact() leaked %q in %q", secret, redacted)
		}
	}
	if !strings.Contains(redacted, "alice") || !strings.Contains(redacted, "127.0.0.1:6379") {
		t.Fatalf("Redact() removed non-secret context: %q", redacted)
	}
}
