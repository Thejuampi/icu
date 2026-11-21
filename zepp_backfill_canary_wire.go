package icu

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const (
	backfillCanaryWireDetailPath  = "/v1/sport/run/detail.json"
	backfillCanaryWireSummaryPath = "/v1/sport/run/summary.json"
	backfillCanaryWireSource      = "run.fit.huami.com"
	backfillCanaryWireMaxReply    = 1 << 20
	backfillCanaryTrackKey        = "trackid"
	backfillCanarySourceKey       = "source"
)

var (
	errBackfillCanaryWireInput        = errors.New("invalid Zepp canary write input; no request sent")
	errBackfillCanaryWriteUnconfirmed = errors.New("zepp canary write outcome unconfirmed; do not retry")
)

// The caller owns the overlap guard, one-ride limit, and ownership checks.
// These helpers only use the already configured web read credentials.
// Successful mocks do not establish live permission to write or delete.
func (c *ZeppClient) postBackfillCanaryDetail(ctx context.Context, form url.Values) error {
	return c.doBackfillCanaryWrite(ctx, http.MethodPost, backfillCanaryWireDetailPath, form)
}

func (c *ZeppClient) postBackfillCanarySummary(ctx context.Context, form url.Values) error {
	return c.doBackfillCanaryWrite(ctx, http.MethodPost, backfillCanaryWireSummaryPath, form)
}

func (c *ZeppClient) deleteBackfillCanarySummary(ctx context.Context, identity url.Values) error {
	return c.doBackfillCanaryWrite(ctx, http.MethodDelete, backfillCanaryWireSummaryPath, identity)
}

func (c *ZeppClient) doBackfillCanaryWrite(ctx context.Context, method, path string, values url.Values) error {
	if c == nil || c.httpClient == nil || c.ensureAuthenticated() != nil ||
		!allowedBackfillCanaryOrigin(c.dataHost) || !validBackfillCanaryWireValues(method, path, c.userID, values) {
		return errBackfillCanaryWireInput
	}

	endpoint := c.dataHost + path
	var body io.Reader = http.NoBody

	if method == http.MethodDelete {
		endpoint += "?" + values.Encode()
	} else {
		body = strings.NewReader(values.Encode())
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return errBackfillCanaryWireInput
	}

	// Match the existing ZeppClient web read headers. Do not add mobile
	// session credentials, signing, or auth properties to the URL.
	req.Header.Set("Apptoken", c.appToken)
	req.Header.Set("Appname", "com.xiaomi.hm.health")
	req.Header.Set("Appplatform", "web")
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	// Do not replay the body, follow redirects, use cookies, or reuse a
	// previous connection. Use HTTP/1 on fresh standard transports to avoid
	// transport-level retries on refused HTTP/2 streams.
	req.GetBody = nil
	req.Close = true
	client := *c.httpClient
	client.CheckRedirect = stopZeppAuthRedirect
	client.Jar = nil

	if client.Transport == nil {
		transport, ok := http.DefaultTransport.(*http.Transport)
		if !ok {
			return errBackfillCanaryWireInput
		}

		client.Transport = transport
	}

	if transport, ok := client.Transport.(*http.Transport); ok {
		fresh := transport.Clone()
		fresh.Protocols = new(http.Protocols)
		fresh.Protocols.SetHTTP1(true)
		client.Transport = fresh
		defer fresh.CloseIdleConnections()
	}

	resp, err := client.Do(req)
	if err != nil {
		// Do not propagate a URL, response, or private server text in errors.
		return errBackfillCanaryWriteUnconfirmed
	}

	defer resp.Body.Close()

	return parseBackfillCanaryWriteReply(resp.StatusCode, resp.Body)
}

func allowedBackfillCanaryOrigin(origin string) bool {
	switch origin {
	case zeppEventsHost, zeppDataHostGlobal, zeppDataHostCN, zeppDataHostEU:
		return true
	default:
		return false
	}
}

func validBackfillCanaryWireValues(method, path, userID string, values url.Values) bool {
	isDetail := method == http.MethodPost && path == backfillCanaryWireDetailPath
	isSummary := method == http.MethodPost && path == backfillCanaryWireSummaryPath
	isDelete := method == http.MethodDelete && path == backfillCanaryWireSummaryPath

	if !isDetail && !isSummary && !isDelete {
		return false
	}

	trackID := values.Get(backfillCanaryTrackKey)
	id, err := strconv.ParseInt(trackID, 10, 64)
	if err != nil || id <= 0 || trackID != strconv.FormatInt(id, 10) || values.Get(backfillCanarySourceKey) != backfillCanaryWireSource {
		return false
	}

	for key, items := range values {
		if len(items) != 1 {
			return false
		}

		switch key {
		case backfillCanaryTrackKey, backfillCanarySourceKey, "v", "appid", "lang", "country", "timezone", "channel", "device", "bind_device", "cv":
		case "userid":
			if items[0] != userID {
				return false
			}
		case "version":
			if isDelete || items[0] != "16" {
				return false
			}
		case "time", "longitude_latitude", "altitude", "speed", "pace", "heart_rate", "distance", "currentDistance",
			"cadence", "power_meter", "lap", "pause", "provider":
			if !isDetail {
				return false
			}
		case "dis", "calorie", "end_time", "run_time", "avg_heart_rate", "avg_pace", "type", "sport_mode",
			"deviceid", "devicesource":
			if !isSummary {
				return false
			}
		default:
			return false
		}
	}

	switch {
	case isDetail:
		return values.Get("time") != "" && values.Get("version") == "16"
	case isSummary:
		return values.Get("end_time") != "" && (values.Get("type") == "9" || values.Get("type") == "10") &&
			values.Get("version") == "16"
	default:
		return true
	}
}

func parseBackfillCanaryWriteReply(status int, reply io.Reader) error {
	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		return errBackfillHistoryUnauthorized
	}

	data, err := io.ReadAll(io.LimitReader(reply, backfillCanaryWireMaxReply+1))
	if err != nil || len(data) > backfillCanaryWireMaxReply {
		return errBackfillCanaryWriteUnconfirmed
	}

	var envelope struct {
		Code    json.RawMessage `json:"code"`
		Message json.RawMessage `json:"message"`
	}
	if json.Unmarshal(data, &envelope) != nil {
		return errBackfillCanaryWriteUnconfirmed
	}

	code, err := parseBackfillCanaryReplyCode(envelope.Code)
	if err != nil {
		return errBackfillCanaryWriteUnconfirmed
	}

	switch code {
	case "401", "403", "108", "0108":
		return errBackfillHistoryUnauthorized
	}

	var message string
	_ = json.Unmarshal(envelope.Message, &message)
	message = strings.ToLower(message)

	for _, marker := range [...]string{
		"unauthorized", "forbidden", "invalid token", "expired token", "token expired", "invalid session",
		"session expired", "logged in on another", "signed in on another",
	} {
		if strings.Contains(message, marker) {
			return errBackfillHistoryUnauthorized
		}
	}

	if status < http.StatusOK || status >= http.StatusMultipleChoices {
		return fmt.Errorf("%w: HTTP %d", errBackfillCanaryWriteUnconfirmed, status)
	}

	if code != "1" {
		return errBackfillCanaryWriteUnconfirmed
	}

	return nil
}

func parseBackfillCanaryReplyCode(raw json.RawMessage) (string, error) {
	var word string
	if len(raw) == 0 || string(raw) == "null" {
		return "", nil
	}

	if raw[0] == '"' {
		if err := json.Unmarshal(raw, &word); err != nil {
			return "", errBackfillCanaryWriteUnconfirmed
		}
		return word, nil
	}

	var number int
	if err := json.Unmarshal(raw, &number); err != nil {
		return "", errBackfillCanaryWriteUnconfirmed
	}

	return strconv.Itoa(number), nil
}
