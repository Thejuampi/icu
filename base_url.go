package icu

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
)

// BaseURLEnv is the environment variable that overrides the API base URL.
const BaseURLEnv = "ICU_BASE_URL"

// ResolveBaseURL returns the API base URL: the --base-url flag wins over the
// ICU_BASE_URL environment variable, and the default is the real API.
// A trailing slash is removed so request paths join cleanly.
func ResolveBaseURL(flags map[string]string) string {
	value := strings.TrimSpace(flags["base-url"])
	if value == "" {
		value = strings.TrimSpace(os.Getenv(BaseURLEnv))
	}

	if value == "" {
		return BaseURL
	}

	return strings.TrimRight(value, "/")
}

// IsRealAPIBaseURL reports whether baseURL points at the production
// intervals.icu API (intervals.icu or any of its subdomains).
func IsRealAPIBaseURL(baseURL string) bool {
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Host == "" {
		return false
	}

	host := strings.TrimSuffix(strings.ToLower(parsed.Hostname()), ".")

	return host == "intervals.icu" || strings.HasSuffix(host, ".intervals.icu")
}

// ErrCrossOriginRedirect is returned when a redirect would leave the
// configured base URL's scheme and host (e.g. from a mock to the real API).
var ErrCrossOriginRedirect = errors.New("refusing redirect to a different scheme or host")

const maxRedirects = 10

// SameOriginRedirects is an http.Client CheckRedirect policy that follows a
// redirect only when it keeps the original request's scheme and host, so a
// redirect can never move a request off the --base-url target.
func SameOriginRedirects(req *http.Request, via []*http.Request) error {
	if len(via) >= maxRedirects {
		return fmt.Errorf("stopped after %d redirects", maxRedirects)
	}

	origin := via[0].URL
	if !strings.EqualFold(req.URL.Scheme, origin.Scheme) || !strings.EqualFold(req.URL.Host, origin.Host) {
		return fmt.Errorf("%w: %s://%s -> %s://%s", ErrCrossOriginRedirect, origin.Scheme, origin.Host, req.URL.Scheme, req.URL.Host)
	}

	return nil
}
