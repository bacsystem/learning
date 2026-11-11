// Package netutil holds network helpers with no dependency on the HTTP layer
// or the configuration, so both config and middleware can use them.
package netutil

import (
	"fmt"
	"net"
	"strings"
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
