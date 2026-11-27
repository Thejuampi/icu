package icu

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// Request forms for the series that the real backend rejected with the
// earlier forms. Sources for each form are cited at the call sites.
const (
	zeppWatchStatisticsLimit = "900"
	zeppBloodPressureSources = "com.huami.midong.associated,com.huami.midong"
	zeppLegacyEventsPageSize = 200
	zeppLegacyEventsMaxPages = 100
	zeppWorkoutHistoryPages  = 50
	zeppParamUserIDLower     = "userid"
	// zeppDayMillis is one day in milliseconds.
	zeppDayMillis = int64(24 * time.Hour / time.Millisecond)

	// ZeppManualDataDefaultType is the manualData record type used when the
	// caller does not pick one (zepp-health-cli default).
	ZeppManualDataDefaultType = "sleep"
	// ZeppWorkoutDetailSegment is the sport segment that serves the detail
	// of every sport; other segments 404 on the real backend.
	ZeppWorkoutDetailSegment = "run"
	// ZeppWorkoutDefaultSource is the detail `source` used when the history
	// summary carries none (amazfit-mcp fallback).
	ZeppWorkoutDefaultSource = "run.mi.com"
)

var (
	// ErrZeppDateRange reports an inverted date range.
	ErrZeppDateRange = errors.New("invalid zepp date range")
	// ErrZeppSeriesRejected reports that the backend rejected a series
	// request (4xx/5xx other than 401/403). The message names the series,
	// the endpoint and the likely cause.
	ErrZeppSeriesRejected = errors.New("zepp rejected the series request")
	// ErrZeppWorkoutEmpty reports a workout detail without data.
	ErrZeppWorkoutEmpty = errors.New("zepp workout detail is empty")
	// ErrZeppWorkoutNotFound reports a trackid missing from the history.
	ErrZeppWorkoutNotFound = errors.New("zepp workout not found in history")
)

// zeppSeriesStatusError turns a non-200 status into an error that names the
// series, the endpoint and the likely cause. 401/403 keep ErrZeppUnauthorized.
func zeppSeriesStatusError(series, endpoint string, status int, cause string) error {
	base := zeppStatusError(status)
	if errors.Is(base, ErrZeppUnauthorized) {
		return fmt.Errorf("zepp %s: %w", series, base)
	}

	return fmt.Errorf("%w: zepp %s: %s returned HTTP %d; likely cause: %s (%w)",
		ErrZeppSeriesRejected, series, endpoint, status, cause, base)
}

// getWithAppHeaders sends a GET with the app headers the working V2 events
// calls use and returns the status and the body.
func (c *ZeppClient) getWithAppHeaders(ctx context.Context, endpoint string) (int, []byte, error) {
	if err := c.ensureAuthenticated(); err != nil {
		return 0, nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, http.NoBody)
	if err != nil {
		return 0, nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Apptoken", c.appToken)
	req.Header.Set("Appname", "com.xiaomi.hm.health")
	req.Header.Set("Appplatform", "web")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	body, err := readZeppBody(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, fmt.Errorf("read response: %w", err)
	}

	return resp.StatusCode, body, nil
}

// fetchSeries GETs a series endpoint and maps a non-200 status to a
// series error.
func (c *ZeppClient) fetchSeries(ctx context.Context, series, label, cause, endpoint string) ([]byte, error) {
	status, body, err := c.getWithAppHeaders(ctx, endpoint)
	if err != nil {
		return nil, err
	}

	if status != http.StatusOK {
		return nil, zeppSeriesStatusError(series, label, status, cause)
	}

	return body, nil
}

// legacyEvents reads /users/{id}/events for one eventType between two
// epoch-millisecond bounds and returns `{"items":[...]}` with every page.
//
// Request form (ha-zepp async_fetch_user_events, zepp-health-cli events_user,
// amazfit-mcp events): `eventType`, `from`/`to` in ms, `limit`, `userId`,
// with the app headers; pages follow the response's `next.timestamp`.
// The earlier form sent no app headers, no `userId` and `limit=1000`, and
// got HTTP 500 from the real backend.
func (c *ZeppClient) legacyEvents(ctx context.Context, series, eventType string, from, to int64) ([]byte, error) {
	label := "/users/{id}/events?eventType=" + eventType
	cause := "the legacy events request form (app headers, userId, limit, from/to in ms) " +
		"is not confirmed for web-login tokens"

	items := []json.RawMessage{}
	seen := map[string]bool{}
	cursor := from

	for range zeppLegacyEventsMaxPages {
		query := url.Values{
			zeppParamEventType: {eventType},
			"from":             {strconv.FormatInt(cursor, 10)},
			"to":               {strconv.FormatInt(to, 10)},
			zeppParamLimit:     {strconv.Itoa(zeppLegacyEventsPageSize)},
			"userId":           {c.userID},
		}

		endpoint := c.eventsBase + "/users/" + url.PathEscape(c.userID) + "/events?" + query.Encode()

		body, err := c.fetchSeries(ctx, series, label, cause, endpoint)
		if err != nil {
			return nil, err
		}

		var page struct {
			Items []json.RawMessage `json:"items"`
			Next  *struct {
				Timestamp int64 `json:"timestamp"`
			} `json:"next"`
		}

		if err := json.Unmarshal(body, &page); err != nil {
			return nil, fmt.Errorf("decode response: %w", err)
		}

		added := 0

		for _, item := range page.Items {
			key := string(bytes.TrimSpace(item))
			if seen[key] {
				continue
			}

			seen[key] = true
			items = append(items, item)
			added++
		}

		if page.Next == nil || page.Next.Timestamp <= cursor || added == 0 {
			break
		}

		cursor = page.Next.Timestamp
	}

	out, err := json.Marshal(map[string][]json.RawMessage{"items": items})
	if err != nil {
		return nil, fmt.Errorf("encode events: %w", err)
	}

	return out, nil
}

// filterManualDataByDate keeps the entries whose timestamp falls in the
// inclusive UTC date range.
func filterManualDataByDate(entries []ZeppManualDataEntry, oldest, newest string) ([]ZeppManualDataEntry, error) {
	from, err := parseDateToMillis(oldest)
	if err != nil {
		return nil, fmt.Errorf("parse oldest date: %w", err)
	}

	to, err := parseDateEndOfDayMillis(newest)
	if err != nil {
		return nil, fmt.Errorf("parse newest date: %w", err)
	}

	out := make([]ZeppManualDataEntry, 0, len(entries))

	for _, entry := range entries {
		if entry.Timestamp >= from && entry.Timestamp <= to {
			out = append(out, entry)
		}
	}

	return out, nil
}

// WorkoutSource returns the history `source` of a workout, paging back
// through /v1/sport/run/history.json (which lists every sport) until the
// trackid is found or the history is older than it (amazfit-mcp
// workout_summary).
func (c *ZeppClient) WorkoutSource(ctx context.Context, trackID string) (string, error) {
	if err := c.ensureAuthenticated(); err != nil {
		return "", err
	}

	want, err := strconv.ParseInt(trackID, 10, 64)
	if err != nil {
		return "", fmt.Errorf("parse trackid %q: %w", trackID, err)
	}

	cursor := int64(0)

	for range zeppWorkoutHistoryPages {
		page, next, err := c.workoutsPage(ctx, ZeppWorkoutDetailSegment, cursor)
		if err != nil {
			return "", err
		}

		for i := range page {
			if page[i].TrackID != trackID {
				continue
			}

			if page[i].Source == "" {
				return ZeppWorkoutDefaultSource, nil
			}

			return page[i].Source, nil
		}

		if next <= 0 || len(page) == 0 {
			break
		}

		if last, err := strconv.ParseInt(page[len(page)-1].TrackID, 10, 64); err == nil && last < want {
			break
		}

		cursor = next
	}

	return "", fmt.Errorf("%w: trackid %s", ErrZeppWorkoutNotFound, trackID)
}

// zeppTrackIDString accepts a trackid encoded as a JSON string or number.
type zeppTrackIDString string

func (t *zeppTrackIDString) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) == 0 || string(b) == "null" {
		*t = ""

		return nil
	}

	if b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return fmt.Errorf("decode trackid: %w", err)
		}

		*t = zeppTrackIDString(s)

		return nil
	}

	var n json.Number
	if err := json.Unmarshal(b, &n); err != nil {
		return fmt.Errorf("decode trackid: %w", err)
	}

	*t = zeppTrackIDString(n.String())

	return nil
}
