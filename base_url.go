package icu

import (
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

	host := strings.ToLower(parsed.Hostname())

	return host == "intervals.icu" || strings.HasSuffix(host, ".intervals.icu")
}
