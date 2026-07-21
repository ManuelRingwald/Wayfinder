package mapconfig

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// ValidateFetchURL screens an admin-supplied URL that the SERVER will fetch
// (DWD WMS/WFS, a BKG style mirror, …). Admin is a trusted platform role, but a
// server-side fetch of an operator-typed URL is an SSRF surface, so we apply
// defence-in-depth before the value is ever stored:
//
//   - scheme must be http or https (no file://, gopher://, …);
//   - a host must be present;
//   - a literal IP host in a private / loopback / link-local / unspecified /
//     unique-local range is rejected (no fetch of internal services);
//   - obvious internal hostnames (localhost, *.local, *.internal) are rejected;
//   - when allowHosts is non-empty the host must match one of them (suffix match
//     on a leading-dot entry, else exact) — an optional hard allowlist.
//
// RESIDUAL RISK (documented, ADR 0033): a PUBLIC hostname that RESOLVES to a
// private IP (DNS rebinding) is not caught here — that needs a resolve-time check
// at fetch. Given the trusted-admin threat model this is an accepted limitation;
// tightening it (resolve + re-check, or a strict allowlist) is a follow-up. The
// per-fetch size/timeout caps live in the fetching services (e.g. pkg/basemap).
func ValidateFetchURL(raw string, allowHosts []string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fmt.Errorf("mapconfig: empty URL")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("mapconfig: invalid URL: %w", err)
	}
	switch u.Scheme {
	case "http", "https":
	default:
		return fmt.Errorf("mapconfig: URL scheme %q not allowed (http/https only)", u.Scheme)
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("mapconfig: URL has no host")
	}

	lower := strings.ToLower(host)
	if lower == "localhost" || strings.HasSuffix(lower, ".localhost") ||
		strings.HasSuffix(lower, ".local") || strings.HasSuffix(lower, ".internal") {
		return fmt.Errorf("mapconfig: host %q is internal", host)
	}

	if ip := net.ParseIP(host); ip != nil {
		if isDisallowedIP(ip) {
			return fmt.Errorf("mapconfig: host IP %s is in a private/loopback range", host)
		}
	}

	if len(allowHosts) > 0 && !hostAllowed(lower, allowHosts) {
		return fmt.Errorf("mapconfig: host %q is not in the allowlist", host)
	}
	return nil
}

// isDisallowedIP reports whether ip is in a range a server-side fetch must never
// reach (loopback, private, link-local, unspecified, or IPv6 unique-local).
func isDisallowedIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
		return true
	}
	// IPv6 unique-local (fc00::/7) — net.IP.IsPrivate covers this for Go ≥1.17,
	// kept explicit for clarity/robustness.
	if v6 := ip.To16(); v6 != nil && ip.To4() == nil && (v6[0]&0xfe) == 0xfc {
		return true
	}
	return false
}

// hostAllowed matches a lowercased host against the allowlist. An entry starting
// with "." is a suffix match (".example.com" allows "a.example.com"); otherwise
// it is an exact match.
func hostAllowed(host string, allow []string) bool {
	for _, a := range allow {
		a = strings.ToLower(strings.TrimSpace(a))
		if a == "" {
			continue
		}
		if strings.HasPrefix(a, ".") {
			if strings.HasSuffix(host, a) || host == strings.TrimPrefix(a, ".") {
				return true
			}
		} else if host == a {
			return true
		}
	}
	return false
}
