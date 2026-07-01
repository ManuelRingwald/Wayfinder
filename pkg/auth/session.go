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

// Claims are the verified fields carried by a session token.
type Claims struct {
	Subject string
	// IssuedAt is the unix second of first login, carried unchanged across
	// sliding-session renews so an absolute maximum lifetime can be enforced. It
	// is 0 for legacy tokens minted before the field existed.
	IssuedAt  int64
	ExpiresAt int64 // unix second at which the token expires
}

// MintSession returns a signed, tamper-evident session token carrying subject,
// an issued-at stamp of now and an expiry ttl from now. Format:
// `b64(subject).iat.exp.signature`, where signature = HMAC-SHA256(key,
// "b64(subject).iat.exp"). The token is signed, not encrypted — do not place
// secrets in subject.
func MintSession(subject string, ttl time.Duration, key []byte) string {
	now := time.Now()
	return MintSessionAt(subject, now, now.Add(ttl), key)
}

// MintSessionAt mints a token with an explicit issued-at and expiry. The
// sliding-session renew uses it to preserve the ORIGINAL first-login time
// (issuedAt) while setting a fresh — and possibly max-lifetime-capped — expiry,
// so an absolute session maximum can be enforced independently of activity.
func MintSessionAt(subject string, issuedAt, expiresAt time.Time, key []byte) string {
	payload := b64.EncodeToString([]byte(subject)) + "." +
		strconv.FormatInt(issuedAt.Unix(), 10) + "." +
		strconv.FormatInt(expiresAt.Unix(), 10)
	return payload + "." + sign(payload, key)
}

// ParseSession verifies a token's signature and expiry and returns its subject.
// Tampering/malformed input yields ErrSessionInvalid; a valid but past-expiry
// token yields ErrSessionExpired.
func ParseSession(token string, key []byte) (string, error) {
	c, err := ParseSessionClaims(token, key)
	if err != nil {
		return "", err
	}
	return c.Subject, nil
}

// ParseSessionClaims verifies a token's signature and expiry and returns its
// claims. It accepts both the current `subject.iat.exp` layout and the legacy
// `subject.exp` layout (tokens minted before the issued-at field existed); the
// latter parse with IssuedAt == 0, so cookies already held by browsers stay
// valid across the upgrade (no mass logout).
func ParseSessionClaims(token string, key []byte) (Claims, error) {
	i := strings.LastIndex(token, ".")
	if i < 0 {
		return Claims{}, ErrSessionInvalid
	}
	payload, sig := token[:i], token[i+1:]

	// Constant-time signature check before trusting any field.
	if subtle.ConstantTimeCompare([]byte(sig), []byte(sign(payload, key))) != 1 {
		return Claims{}, ErrSessionInvalid
	}

	// The subject is base64 (RawURLEncoding, dot-free), so the payload splits
	// cleanly on ".": two fields = legacy (subject.exp), three = subject.iat.exp.
	parts := strings.Split(payload, ".")
	var subjB64, iatStr, expStr string
	switch len(parts) {
	case 2:
		subjB64, expStr = parts[0], parts[1]
	case 3:
		subjB64, iatStr, expStr = parts[0], parts[1], parts[2]
	default:
		return Claims{}, ErrSessionInvalid
	}

	exp, err := strconv.ParseInt(expStr, 10, 64)
	if err != nil {
		return Claims{}, ErrSessionInvalid
	}
	var iat int64
	if iatStr != "" {
		if iat, err = strconv.ParseInt(iatStr, 10, 64); err != nil {
			return Claims{}, ErrSessionInvalid
		}
	}
	if time.Now().Unix() > exp {
		return Claims{}, ErrSessionExpired
	}
	subject, err := b64.DecodeString(subjB64)
	if err != nil {
		return Claims{}, ErrSessionInvalid
	}
	return Claims{Subject: string(subject), IssuedAt: iat, ExpiresAt: exp}, nil
}

// MintSessionID wraps an opaque registry session token in a signed cookie value
// (AP7, ADR 0009 §5). Format: `token.signature`, where signature =
// HMAC-SHA256(key, token). The token is a dot-free base64url string, so the value
// is structurally distinct from a stateless `subject.iat.exp` cookie (which
// carries at least one dot in its payload) — ParseSessionID and ParseSessionClaims
// therefore never confuse the two. The signature lets the edge reject a forged
// token without a database round-trip; the registry lookup then authorises it.
func MintSessionID(token string, key []byte) string {
	return token + "." + sign(token, key)
}

// ParseSessionID verifies a registry cookie's signature and returns its token.
// It accepts only the single-field `token.signature` layout: a payload that still
// contains a dot is a legacy stateless cookie, not a session id, and yields
// ErrSessionInvalid so the caller can fall back to ParseSession. Unlike the
// stateless parse there is no expiry check here — expiry lives in the registry.
func ParseSessionID(cookie string, key []byte) (string, error) {
	i := strings.LastIndex(cookie, ".")
	if i < 0 {
		return "", ErrSessionInvalid
	}
	token, sig := cookie[:i], cookie[i+1:]
	// A dot in the token means this is not a session-id cookie (e.g. a legacy
	// subject.iat.exp value); reject before the constant-time compare so the
	// caller falls through to the legacy path.
	if strings.Contains(token, ".") {
		return "", ErrSessionInvalid
	}
	if subtle.ConstantTimeCompare([]byte(sig), []byte(sign(token, key))) != 1 {
		return "", ErrSessionInvalid
	}
	return token, nil
}

func sign(payload string, key []byte) string {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(payload))
	return b64.EncodeToString(mac.Sum(nil))
}
