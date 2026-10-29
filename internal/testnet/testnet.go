// Package testnet makes tests unable to reach anything but loopback.
package testnet

import (
	"fmt"
	"net"
	"net/http"
	"strings"
)

// LoopbackOnly wraps an http.RoundTripper and refuses every request whose
// host is not loopback, so no test can ever reach the real API.
type LoopbackOnly struct {
	Base http.RoundTripper
}

// RoundTrip implements http.RoundTripper.
func (l LoopbackOnly) RoundTrip(req *http.Request) (*http.Response, error) {
	if !IsLoopback(req.URL.Hostname()) {
		return nil, fmt.Errorf("testnet: refusing non-loopback request to %s (tests must use httptest or the mock)", req.URL.Host)
	}

	resp, err := l.Base.RoundTrip(req)
	if err != nil {
		return nil, fmt.Errorf("testnet: %w", err)
	}

	return resp, nil
}

// IsLoopback reports whether host is localhost or a loopback IP.
func IsLoopback(host string) bool {
	if strings.EqualFold(strings.TrimSuffix(host, "."), "localhost") {
		return true
	}

	ip := net.ParseIP(host)

	return ip != nil && ip.IsLoopback()
}

// Install replaces http.DefaultTransport with a loopback-only wrapper.
// Call it from TestMain before running tests.
func Install() {
	http.DefaultTransport = LoopbackOnly{Base: http.DefaultTransport}
}
