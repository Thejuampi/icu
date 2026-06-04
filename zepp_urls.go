package icu

import (
	"fmt"
	"net/http"
)

// Real Zepp/Amazfit cloud API endpoints. These are reverse-engineered from
// huami-token, bentasker/zepp_to_influxdb, rolandsz/Mi-Fit-and-Zepp-workout-exporter,
// and the Zepp mobile app traffic. Endpoints may change without notice.

const (
	// Auth endpoints (v2, host: api-user-us2.zepp.com / api-mifit-us2.zepp.com).
	// These are handled in zepp_auth.go directly.

	// Data hosts (v1, host: api-mifit[/-cn/-de].huami.com).
	// The right host is selected by the country code returned in the auth flow.
	zeppDataHostGlobal = "https://api-mifit.huami.com"
	zeppDataHostCN     = "https://api-mifit-cn.huami.com"
	zeppDataHostEU     = "https://api-mifit-de.huami.com"
	zeppEventsHost     = "https://api-mifit.zepp.com"

	zeppUserInfoPath     = "/huami.health.getUserInfo.json"
	zeppBandDataPath     = "/v1/data/band_data.json"
	zeppSportHistoryPath = "/v1/sport/run/history.json"
	zeppSportDetailPath  = "/v1/sport/run/detail.json"
)

// zeppDataHostFor returns the regional Mi Fit data host for a given
// ISO 3166-1 alpha-2 country code. Defaults to global.
func zeppDataHostFor(countryCode string) string {
	switch countryCode {
	case "CN":
		return zeppDataHostCN
	case "DE", "FR", "IT", "ES", "GB", "NL", "PL", "RU", "TR", "SE", "NO", "FI", "DK":
		return zeppDataHostEU
	default:
		return zeppDataHostGlobal
	}
}

// BuildZeppURL composes a fully-qualified URL on the regional data host.
func BuildZeppURL(path string) string {
	return zeppDataHostGlobal + path
}

// BuildZeppURLForRegion composes a URL for a specific region.
func BuildZeppURLForRegion(countryCode, path string) string {
	return zeppDataHostFor(countryCode) + path
}

// BuildZeppEventsURL composes a Zepp events endpoint URL.
// Format: https://api-mifit.zepp.com/users/{userid}/events?...
func BuildZeppEventsURL(userID string) string {
	return fmt.Sprintf("%s/users/%s/events", zeppEventsHost, userID)
}

// Common query parameters used by /v1/data/band_data.json.
const (
	zeppApptokenHeader = "apptoken"
	zeppAppNameHeader  = "appname"
)

// ZeppCommonHeaders returns the common HTTP headers for Zepp data requests.
// The apptoken is the user's app token obtained from the auth flow.
func ZeppCommonHeaders(appToken string) http.Header {
	headers := http.Header{}
	headers.Set("apptoken", appToken)
	headers.Set("appname", "com.xiaomi.hm.health")
	headers.Set("appplatform", "web")
	headers.Set("Accept-Encoding", "gzip")

	return headers
}
