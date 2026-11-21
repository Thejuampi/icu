package icu

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

var (
	errBackfillCanaryOverlap   = errors.New("workout in the Zepp history feed overlaps this FIT")
	errBackfillCanaryUncertain = errors.New("canary outcome could not be verified")
)

// TestRideBackfill tries one local cycling FIT and removes it after exact
// attribution in the Zepp history feed. A nonzero track ID with an error means
// a write was attempted; it does not establish that the workout exists or that
// it is safe to retry or delete. It never signs in or reads Intervals.icu.
// Feed coverage across Zepp sport types has not been established.
func (c *ZeppClient) TestRideBackfill(ctx context.Context, fit io.Reader) (int64, error) {
	if c == nil || fit == nil || c.httpClient == nil || !allowedBackfillCanaryOrigin(c.dataHost) {
		return 0, errBackfillCanaryWireInput
	}

	ride, err := DecodeZeppRideFIT(fit)
	if err != nil {
		return 0, err
	}
	summaryFields, detailFields, err := EncodeZeppRideFIT(ride)
	if err != nil {
		return 0, err
	}

	summary := backfillCanaryForm(summaryFields)
	detail := backfillCanaryForm(detailFields)
	if !validBackfillCanaryWireValues(http.MethodPost, backfillCanaryWireSummaryPath, c.userID, summary) ||
		!validBackfillCanaryWireValues(http.MethodPost, backfillCanaryWireDetailPath, c.userID, detail) {
		return 0, errBackfillCanaryWireInput
	}

	target := zeppBackfillInterval{Start: ride.Start, End: ride.End, Source: summaryFields[backfillCanarySourceKey]}
	history, err := c.readZeppBackfillHistory(ctx)
	if err != nil {
		return 0, err
	}
	if zeppBackfillOverlap(target, history) {
		return 0, errBackfillCanaryOverlap
	}

	if err := c.postBackfillCanaryDetail(ctx, detail); err != nil {
		if errors.Is(err, errBackfillCanaryWireInput) {
			return 0, errBackfillCanaryWireInput
		}
		return target.Start, backfillCanaryFailure("writing workout detail", err)
	}
	if err := c.postBackfillCanarySummary(ctx, summary); err != nil {
		return target.Start, backfillCanaryFailure("writing workout summary", err)
	}

	if err := c.verifyBackfillCanary(ctx, target, summaryFields[backfillCanaryTrackKey]); err != nil {
		return target.Start, err
	}
	return target.Start, nil
}

func (c *ZeppClient) verifyBackfillCanary(ctx context.Context, target zeppBackfillInterval, trackID string) error {
	history, err := c.readZeppBackfillHistory(ctx)
	if err != nil {
		return backfillCanaryFailure("reading back the workout", err)
	}
	if !backfillCanaryAttributed(target, history) {
		return fmt.Errorf("%w: workout not attributable; no deletion attempted", errBackfillCanaryUncertain)
	}

	identity := url.Values{backfillCanaryTrackKey: {trackID}, backfillCanarySourceKey: {target.Source}}
	if err := c.deleteBackfillCanarySummary(ctx, identity); err != nil {
		return backfillCanaryFailure("deleting the attributed workout", err)
	}
	history, err = c.readZeppBackfillHistory(ctx)
	if err != nil {
		return backfillCanaryFailure("verifying deletion", err)
	}
	for _, workout := range history {
		if workout.Start == target.Start {
			return fmt.Errorf("%w: track still present after deletion", errBackfillCanaryUncertain)
		}
	}
	return nil
}

func backfillCanaryAttributed(target zeppBackfillInterval, history []zeppBackfillInterval) bool {
	matched := false
	for _, workout := range history {
		if workout == target && !matched {
			matched = true
			continue
		}
		if workout.Start <= target.End && target.Start <= workout.End {
			return false
		}
	}
	return matched
}

func backfillCanaryForm(fields map[string]string) url.Values {
	form := make(url.Values, len(fields))
	for key, value := range fields {
		form.Set(key, value)
	}
	return form
}

func backfillCanaryFailure(stage string, err error) error {
	if errors.Is(err, errBackfillHistoryUnauthorized) || errors.Is(err, ErrZeppUnauthorized) {
		return fmt.Errorf("%w: authorization rejected while %s; no further requests made: %w", errBackfillCanaryUncertain, stage, errBackfillHistoryUnauthorized)
	}
	return fmt.Errorf("%w: stopped while %s; no further requests made", errBackfillCanaryUncertain, stage)
}
