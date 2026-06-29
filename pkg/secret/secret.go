// Package secret provides authenticated encryption for per-feed source
// credentials (ORCH-2c, ADR 0012 §6, NFR-SEC-004).
//
// Source credentials (OpenSky client secrets, FLARM/radar access) are stored
// encrypted at rest so a database leak alone never yields plaintext credentials.
// The cipher is AES-256-GCM: a 32-byte key encrypts a plaintext with a fresh
// random nonce and an authentication tag, so tampering is detected on decrypt.
// The key itself is deployment-managed (WAYFINDER_SECRET_KEY, base64) — it is
// never stored in the database; encryption defends the data-at-rest boundary, not
// a full process compromise.
package secret

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

// KeySize is the required key length: AES-256 (32 bytes).
const KeySize = 32

// Cipher seals and opens secrets with a fixed key. Safe for concurrent use (the
// underlying GCM AEAD is).
type Cipher struct {
	aead cipher.AEAD
}

// NewCipher builds a Cipher from a 32-byte key.
func NewCipher(key []byte) (*Cipher, error) {
	if len(key) != KeySize {
		return nil, fmt.Errorf("secret: key must be %d bytes (AES-256), got %d", KeySize, len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("secret: new cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("secret: new gcm: %w", err)
	}
	return &Cipher{aead: aead}, nil
}

// KeyFromBase64 decodes and validates a standard-base64-encoded 32-byte key. Use
// to parse WAYFINDER_SECRET_KEY.
func KeyFromBase64(s string) ([]byte, error) {
	key, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("secret: key is not valid base64: %w", err)
	}
	if len(key) != KeySize {
		return nil, fmt.Errorf("secret: decoded key must be %d bytes, got %d", KeySize, len(key))
	}
	return key, nil
}

// Seal encrypts plaintext and returns base64(nonce || ciphertext+tag). Each call
// uses a fresh random nonce, so sealing the same plaintext twice yields different
// blobs (no deterministic-encryption leakage).
func (c *Cipher) Seal(plaintext string) (string, error) {
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("secret: read nonce: %w", err)
	}
	sealed := c.aead.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

// ErrDecrypt is returned when a blob cannot be authenticated/decrypted (wrong
// key, tampered ciphertext, or corrupt input).
var ErrDecrypt = errors.New("secret: decryption failed")

// Open reverses Seal. It returns ErrDecrypt for any malformed, tampered or
// wrong-key input (never a partial/garbage plaintext — GCM authenticates first).
func (c *Cipher) Open(blob string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(blob)
	if err != nil {
		return "", ErrDecrypt
	}
	ns := c.aead.NonceSize()
	if len(raw) < ns {
		return "", ErrDecrypt
	}
	nonce, ct := raw[:ns], raw[ns:]
	pt, err := c.aead.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", ErrDecrypt
	}
	return string(pt), nil
}
