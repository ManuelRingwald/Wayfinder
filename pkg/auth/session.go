package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
	"time"
)

var (
	// ErrSessionInvalid means the token is malformed or its signature does not
	// verify (tampering or wrong key).
	ErrSessionInvalid = errors.New("auth: invalid session token")
	// ErrSessionExpired means the signature is valid but the token has expired.
	ErrSessionExpired = errors.New("auth: session token expired")
)

var b64 = base64.RawURLEncoding

// MintSession returns a signed, tamper-evident session token carrying subject
// and an expiry ttl from now. Format: `b64(subject).exp.signature`, where
// signature = HMAC-SHA256(key, "b64(subject).exp"). The token is signed, not
// encrypted — do not place secrets in subject.
func MintSession(subject string, ttl time.Duration, key []byte) string {
	exp := time.Now().Add(ttl).Unix()
	payload := b64.EncodeToString([]byte(subject)) + "." + strconv.FormatInt(exp, 10)
	return payload + "." + sign(payload, key)
}

// ParseSession verifies a token's signature and expiry and returns its subject.
// Tampering/malformed input yields ErrSessionInvalid; a valid but past-expiry
// token yields ErrSessionExpired.
func ParseSession(token string, key []byte) (string, error) {
	i := strings.LastIndex(token, ".")
	if i < 0 {
		return "", ErrSessionInvalid
	}
	payload, sig := token[:i], token[i+1:]

	// Constant-time signature check before trusting any field.
	if subtle.ConstantTimeCompare([]byte(sig), []byte(sign(payload, key))) != 1 {
		return "", ErrSessionInvalid
	}

	j := strings.LastIndex(payload, ".")
	if j < 0 {
		return "", ErrSessionInvalid
	}
	subjB64, expStr := payload[:j], payload[j+1:]

	exp, err := strconv.ParseInt(expStr, 10, 64)
	if err != nil {
		return "", ErrSessionInvalid
	}
	if time.Now().Unix() > exp {
		return "", ErrSessionExpired
	}

	subject, err := b64.DecodeString(subjB64)
	if err != nil {
		return "", ErrSessionInvalid
	}
	return string(subject), nil
}

func sign(payload string, key []byte) string {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(payload))
	return b64.EncodeToString(mac.Sum(nil))
}
