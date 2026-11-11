package middleware

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
)

// Trusted proxy configuration shared by every component that needs the real
// client IP (auth, audit, rate limiter). Configure once at startup with
// SetTrustedProxies(cfg.TrustedProxies).
var (
	trustedMu   sync.RWMutex
	trustedNets []*net.IPNet
)

// ParseTrustedProxies parses a comma-separated list of CIDRs or IPs (an IP
// without a mask is a single host: /32 for IPv4, /128 for IPv6). Empty
// entries are skipped, so "" or a trailing comma mean no proxy. Any other
// invalid entry is an error naming it, so config.Load can fail at startup
// instead of silently trusting fewer proxies than configured.
func ParseTrustedProxies(list string) ([]*net.IPNet, error) {
	var nets []*net.IPNet
	for _, entry := range strings.Split(list, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		cidr := entry
		if !strings.Contains(cidr, "/") {
			if ip := net.ParseIP(cidr); ip != nil && ip.To4() == nil {
				cidr += "/128"
			} else {
				cidr += "/32"
			}
		}
		_, ipNet, err := net.ParseCIDR(cidr)
		if err != nil {
			return nil, fmt.Errorf("invalid trusted proxy %q: want an IP or a CIDR", entry)
		}
		nets = append(nets, ipNet)
	}
	return nets, nil
}

// SetTrustedProxies configures the trusted proxies from a list already
// validated by config.Load. An invalid list fails closed: no proxy is
// trusted, so X-Forwarded-For is ignored.
func SetTrustedProxies(list string) {
	nets, err := ParseTrustedProxies(list)
	if err != nil {
		nets = nil
	}
	trustedMu.Lock()
	trustedNets = nets
	trustedMu.Unlock()
}

func isTrusted(ip net.IP) bool {
	trustedMu.RLock()
	defer trustedMu.RUnlock()
	for _, cidr := range trustedNets {
		if cidr.Contains(ip) {
			return true
		}
	}
	return false
}

func peerIP(remoteAddr string) net.IP {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	return net.ParseIP(strings.TrimSpace(host))
}

// ClientIP returns the real client IP as a canonical string, or "" when no
// valid IP can be determined.
//
// X-Forwarded-For is only honoured when the direct peer is a trusted proxy.
// The header is walked right to left, skipping trusted proxies, and the first
// untrusted address is returned: the left-most entries are client-controlled
// and must not be believed. Every value is validated with net.ParseIP.
func ClientIP(r *http.Request) string {
	peer := peerIP(r.RemoteAddr)
	if peer == nil {
		return ""
	}
	if !isTrusted(peer) {
		return peer.String()
	}
	hops := strings.Split(strings.Join(r.Header.Values("X-Forwarded-For"), ","), ",")
	for i := len(hops) - 1; i >= 0; i-- {
		ip := net.ParseIP(strings.TrimSpace(hops[i]))
		if ip == nil {
			// Malformed hop: stop trusting the chain here.
			break
		}
		if !isTrusted(ip) {
			return ip.String()
		}
	}
	return peer.String()
}

// LimiterIP is the ONE normalization of a client IP used to key rate
// limiters (in-memory RateLimiter and the persisted DBLimiter counters): an
// IPv4 address (also when IPv4-mapped IPv6) is kept as is, an IPv6 address
// is reduced to its /64 ("2001:db8:1:2::/64"), the smallest block an end
// site normally gets, so rotating addresses inside it does not buy more
// quota. Values that are not an IP are returned unchanged; an already
// normalized /64 is returned as is (idempotent). Audit logs must keep the
// real IP (ClientIP): this is only for limiter keys.
func LimiterIP(ip string) string {
	if _, cidr, err := net.ParseCIDR(ip); err == nil && cidr.IP.To4() == nil {
		if ones, _ := cidr.Mask.Size(); ones == 64 {
			return cidr.String()
		}
	}
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return ip
	}
	if v4 := parsed.To4(); v4 != nil {
		return v4.String()
	}
	return (&net.IPNet{IP: parsed.Mask(net.CIDRMask(64, 128)), Mask: net.CIDRMask(64, 128)}).String()
}
