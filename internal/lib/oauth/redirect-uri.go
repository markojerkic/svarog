package oauth

import (
	"net/netip"
	"net/url"
	"strings"
)

// validRedirectURI accepts https URLs and http URLs on the loopback interface,
// as the MCP authorization spec requires.
func validRedirectURI(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || !u.IsAbs() || u.Opaque != "" || u.User != nil || u.Hostname() == "" || strings.Contains(raw, "#") {
		return false
	}
	switch u.Scheme {
	case "https":
		return strings.HasPrefix(raw, "https://")
	case "http":
		return strings.HasPrefix(raw, "http://") && isLoopbackHost(u.Hostname())
	default:
		return false
	}
}

// redirectURIRegistered compares redirect URIs as exact strings. A loopback
// redirect URI may use any port, because native clients listen on a port
// chosen at runtime (RFC 8252, section 7.3).
func redirectURIRegistered(registered []string, requested string) bool {
	if !validRedirectURI(requested) {
		return false
	}
	for _, candidate := range registered {
		if candidate == requested || sameLoopbackRedirect(candidate, requested) {
			return true
		}
	}
	return false
}

func sameLoopbackRedirect(registered, requested string) bool {
	a, errA := url.Parse(registered)
	b, errB := url.Parse(requested)
	if errA != nil || errB != nil || a.Scheme != "http" || b.Scheme != "http" {
		return false
	}
	if !isLoopbackHost(a.Hostname()) || a.Hostname() != b.Hostname() {
		return false
	}
	return a.EscapedPath() == b.EscapedPath() && a.RawQuery == b.RawQuery && a.ForceQuery == b.ForceQuery
}

func isLoopbackHost(host string) bool {
	if host == "localhost" {
		return true
	}
	ip, err := netip.ParseAddr(host)
	return err == nil && ip.IsLoopback()
}
