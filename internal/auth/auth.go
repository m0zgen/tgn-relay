package auth

import (
	"crypto/subtle"
	"net"
	"net/http"
	"net/netip"
	"strings"
)

func CheckKey(r *http.Request, keys []string) bool {
	got := r.Header.Get("X-Relay-Key")
	if got == "" {
		got = r.URL.Query().Get("relay_key")
	}
	if got == "" {
		return false
	}
	for _, key := range keys {
		if subtle.ConstantTimeCompare([]byte(got), []byte(key)) == 1 {
			return true
		}
	}
	return false
}

func CheckIP(r *http.Request, cidrs []string) bool {
	if len(cidrs) == 0 {
		return true
	}
	ipStr := clientIP(r)
	addr, err := netip.ParseAddr(ipStr)
	if err != nil {
		return false
	}
	for _, cidr := range cidrs {
		p, err := netip.ParsePrefix(cidr)
		if err == nil && p.Contains(addr) {
			return true
		}
	}
	return false
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		part := strings.TrimSpace(strings.Split(xff, ",")[0])
		if part != "" {
			return part
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}
