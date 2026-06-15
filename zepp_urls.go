package icu

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
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

	zeppUserInfoPath = "/huami.health.getUserInfo.json"
	zeppBandDataPath = "/v1/data/band_data.json"
)

// SportHistoryURL composes the /v1/sport/{sport}/history.json URL.
func SportHistoryURL(base, sport string) string {
	return base + "/v1/sport/" + sport + "/history.json"
}

// SportDetailURL composes the /v1/sport/{sport}/detail.json URL.
func SportDetailURL(base, sport string) string {
	return base + "/v1/sport/" + sport + "/detail.json"
}

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

// V2EventsURL composes a fully-qualified /v2/users/me/events URL for a preset
// and a date range. The base URL is typically the regional data host returned
// by zeppDataHostFor.
func V2EventsURL(base string, preset V2EventPreset, oldest, newest string) (string, error) {
	from, err := parseDateToMillis(oldest)
	if err != nil {
		return "", fmt.Errorf("parse oldest date: %w", err)
	}

	to, err := parseDateEndOfDayMillis(newest)
	if err != nil {
		return "", fmt.Errorf("parse newest date: %w", err)
	}

	query := url.Values{
		"eventType": {preset.EventType},
		"subType":   {preset.SubType},
		"from":      {strconv.FormatInt(from, 10)},
		"to":        {strconv.FormatInt(to, 10)},
		"limit":     {"1000"},
	}

	return base + "/v2/users/me/events?" + query.Encode(), nil
}

// WatchSportStatisticsURL composes a /v2/watch/users/{userID}/WatchSportStatistics/{statType}
// URL for a date range. The base URL is the regional data host.
func WatchSportStatisticsURL(base, userID, statType, oldest, newest string) (string, error) {
	from, err := parseDateToMillis(oldest)
	if err != nil {
		return "", fmt.Errorf("parse oldest date: %w", err)
	}

	to, err := parseDateEndOfDayMillis(newest)
	if err != nil {
		return "", fmt.Errorf("parse newest date: %w", err)
	}

	query := url.Values{
		"from": {strconv.FormatInt(from, 10)},
		"to":   {strconv.FormatInt(to, 10)},
	}

	return base + "/v2/watch/users/" + userID + "/WatchSportStatistics/" + statType + "?" + query.Encode(), nil
}

// UserHeartRateURL composes the /users/{userID}/heartRate URL.
func UserHeartRateURL(base, userID, oldest, newest string) (string, error) {
	from, err := parseDateToSecondsUTC(oldest)
	if err != nil {
		return "", fmt.Errorf("parse oldest date: %w", err)
	}

	to, err := parseDateEndOfDaySecondsUTC(newest)
	if err != nil {
		return "", fmt.Errorf("parse newest date: %w", err)
	}

	query := url.Values{
		"startTime": {strconv.FormatInt(from, 10)},
		"endTime":   {strconv.FormatInt(to, 10)},
		"limit":     {"1000"},
		"type":      {"2"},
	}

	return base + "/users/" + userID + "/heartRate?" + query.Encode(), nil
}

// WeightRecordsURL composes the /users/{userID}/members/-1/weightRecords URL.
func WeightRecordsURL(base, userID, oldest, newest string) (string, error) {
	from, err := parseDateToSecondsUTC(oldest)
	if err != nil {
		return "", fmt.Errorf("parse oldest date: %w", err)
	}

	to, err := parseDateEndOfDaySecondsUTC(newest)
	if err != nil {
		return "", fmt.Errorf("parse newest date: %w", err)
	}

	query := url.Values{
		"fromTime":  {strconv.FormatInt(from, 10)},
		"toTime":    {strconv.FormatInt(to, 10)},
		"limit":     {"300"},
		"isForward": {"0"},
	}

	return base + "/users/" + userID + "/members/-1/weightRecords?" + query.Encode(), nil
}

// BloodPressureUserURL composes the /users/me/bloodPressure URL.
func BloodPressureUserURL(base, oldest, newest string) (string, error) {
	from, err := parseDateToMillis(oldest)
	if err != nil {
		return "", fmt.Errorf("parse oldest date: %w", err)
	}

	to, err := parseDateEndOfDayMillis(newest)
	if err != nil {
		return "", fmt.Errorf("parse newest date: %w", err)
	}

	query := url.Values{
		"from": {strconv.FormatInt(from, 10)},
		"to":   {strconv.FormatInt(to, 10)},
	}

	return base + "/users/me/bloodPressure?" + query.Encode(), nil
}

// SecondHeartRateFilesURL composes the /users/me/fileInfo/events URL used to
// list per-second heart-rate COS file indices.
func SecondHeartRateFilesURL(base, oldest, newest string) (string, error) {
	from, err := parseDateToMillis(oldest)
	if err != nil {
		return "", fmt.Errorf("parse oldest date: %w", err)
	}

	to, err := parseDateEndOfDayMillis(newest)
	if err != nil {
		return "", fmt.Errorf("parse newest date: %w", err)
	}

	query := url.Values{
		"eventType": {"second_heart_rate"},
		"subType":   {"real_data"},
		"from":      {strconv.FormatInt(from, 10)},
		"to":        {strconv.FormatInt(to, 10)},
		"limit":     {"200"},
	}

	return base + "/users/me/fileInfo/events?" + query.Encode(), nil
}

// SpO2WindowsURL composes the /users/{userID}/events/dateString URL used for
// SpO2 ODI/OSA windows. The from/to bounds are ISO-8601 datetimes in the
// requested timezone.
func SpO2WindowsURL(base, userID, date, tz string) (string, error) {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return "", fmt.Errorf("parse timezone: %w", err)
	}

	day, err := time.ParseInLocation("2006-01-02", date, loc)
	if err != nil {
		return "", fmt.Errorf("parse date: %w", err)
	}

	from := day.Format(time.RFC3339)
	to := day.Add(24*time.Hour - time.Second).Format(time.RFC3339)

	query := url.Values{
		"eventType": {"blood_oxygen"},
		"subType":   {"odi"},
		"from":      {from},
		"to":        {to},
		"timeZone":  {tz},
		"limit":     {"999"},
		"reverse":   {"0"},
		"userId":    {userID},
	}

	return base + "/users/" + userID + "/events/dateString?" + query.Encode(), nil
}

// ManualDataURL composes the /v1/user/manualData.json URL.
func ManualDataURL(base, oldest, newest string) (string, error) {
	from, err := parseDateToMillis(oldest)
	if err != nil {
		return "", fmt.Errorf("parse oldest date: %w", err)
	}

	to, err := parseDateEndOfDayMillis(newest)
	if err != nil {
		return "", fmt.Errorf("parse newest date: %w", err)
	}

	query := url.Values{
		"from": {strconv.FormatInt(from, 10)},
		"to":   {strconv.FormatInt(to, 10)},
	}

	return base + "/v1/user/manualData.json?" + query.Encode(), nil
}

func parseDateToMillis(dateStr string) (int64, error) {
	t, err := time.ParseInLocation("2006-01-02", dateStr, time.UTC)
	if err != nil {
		return 0, fmt.Errorf("parse date %s to millis: %w", dateStr, err)
	}

	return t.UnixMilli(), nil
}

func parseDateToSecondsUTC(dateStr string) (int64, error) {
	t, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return 0, fmt.Errorf("parse date %s to seconds: %w", dateStr, err)
	}

	return t.Unix(), nil
}

func parseDateEndOfDaySecondsUTC(dateStr string) (int64, error) {
	t, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return 0, fmt.Errorf("parse date %s end of day: %w", dateStr, err)
	}

	return t.Add(24*time.Hour - time.Second).Unix(), nil
}

// ParseZeppDateToMillisForTest is a test-friendly alias of parseDateToMillis.
// It exists so that the public test package can verify date parsing.
func ParseZeppDateToMillisForTest(dateStr string) (int64, error) {
	return parseDateToMillis(dateStr)
}

func parseDateEndOfDayMillis(dateStr string) (int64, error) {
	t, err := time.ParseInLocation("2006-01-02", dateStr, time.UTC)
	if err != nil {
		return 0, fmt.Errorf("parse date %s end of day millis: %w", dateStr, err)
	}

	return t.Add(24*time.Hour - time.Millisecond).UnixMilli(), nil
}

// ZeppCommonHeaders returns the common HTTP headers for Zepp data requests.
// The apptoken is the user's app token obtained from the auth flow.
func ZeppCommonHeaders(appToken string) http.Header {
	headers := http.Header{}
	headers.Set("Apptoken", appToken)
	headers.Set("Appname", "com.xiaomi.hm.health")
	headers.Set("Appplatform", "web")
	headers.Set("Accept-Encoding", "gzip")

	return headers
}
