package icu

import (
	"errors"
	"io"
	"slices"
	"testing"
)

// The rules overlap on purpose (defense in depth), so deleting a single
// entry may not change behavior; this pins every entry the review named.
func TestZeppRedactRuleListsPinned(t *testing.T) {
	t.Parallel()

	exact := zeppDeniedKeys()
	for _, k := range []string{"uid", "code", "imei", "id", "ids", "sn", "key", "url", "timezone", "ip", "lat", "lon"} {
		if !exact[k] {
			t.Errorf("exact list lost %q", k)
		}
	}
	frags := zeppDeniedFragments()
	for _, f := range []string{"session", "apikey", "nickname", "imei", "token", "secret", "passw", "email", "uuid"} {
		if !slices.Contains(frags, f) {
			t.Errorf("fragment list lost %q", f)
		}
	}
	sufs := zeppDeniedSuffixes()
	for _, s := range []string{"id", "ids", "url", "key", "token", "name"} {
		if !slices.Contains(sufs, s) {
			t.Errorf("suffix list lost %q", s)
		}
	}
	safe := zeppSafeStringKeys()
	for _, k := range []string{"date", "timestamp", "starttime", "eventtype", "subtype", "type", "value"} {
		if safe[k] == zeppClassNone {
			t.Errorf("safe list lost %q", k)
		}
	}
}

func TestReadZeppBodyLimit(t *testing.T) {
	t.Parallel()

	if _, err := readZeppBody(io.LimitReader(zeroReader{}, zeppMaxBody+1)); !errors.Is(err, errZeppBodyTooLarge) {
		t.Fatalf("over limit: %v", err)
	}
	body, err := readZeppBody(io.LimitReader(zeroReader{}, zeppMaxBody))
	if err != nil || len(body) != zeppMaxBody {
		t.Fatalf("at limit: %d %v", len(body), err)
	}
}

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	clear(p)

	return len(p), nil
}
