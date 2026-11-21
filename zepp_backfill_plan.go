package icu

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"time"
)

const zeppBackfillHistoryPageLimit = 1000

var (
	errBackfillHistoryIncomplete   = errors.New("Zepp history feed is incomplete")
	errBackfillHistoryUnauthorized = errors.New("Zepp rejected the saved session; stopped without signing in")
)

type zeppBackfillInterval struct {
	Start int64
	End   int64
}

type zeppBackfillHistoryPage struct {
	Code *int `json:"code"`
	Data *struct {
		Next    *int64 `json:"next"`
		Summary *[]struct {
			TrackID string `json:"trackid"`
			EndTime string `json:"end_time"`
		} `json:"summary"`
	} `json:"data"`
}

// zeppBackfillOverlap treats shared timestamps and exact keys as conflicts even
// across sport types. It intentionally does not assume similar distance implies identity.
func zeppBackfillOverlap(candidate zeppBackfillInterval, existing []zeppBackfillInterval) bool {
	for _, r := range existing {
		if candidate.Start <= r.End && r.Start <= candidate.End {
			return true
		}
	}
	return false
}

// readZeppBackfillHistory enumerates the run history feed. The feed's
// coverage across sport types is unverified; no result authorizes writes.
// No early cutoff relies on the order of entries across pages.
func (c *ZeppClient) readZeppBackfillHistory(ctx context.Context) ([]zeppBackfillInterval, error) {
	var records []zeppBackfillInterval
	seenCursor := make(map[int64]bool)
	seenTrack := make(map[int64]int64)
	for cursor, page := int64(0), 0; page < zeppBackfillHistoryPageLimit; page++ {
		if seenCursor[cursor] {
			return nil, errBackfillHistoryIncomplete
		}
		seenCursor[cursor] = true
		p, err := c.readZeppBackfillHistoryPage(ctx, cursor)
		if err != nil {
			return nil, err
		}
		for _, item := range *p.Data.Summary {
			start, e1 := strconv.ParseInt(item.TrackID, 10, 64)
			end, e2 := strconv.ParseInt(item.EndTime, 10, 64)
			if e1 != nil || e2 != nil || start <= 0 || end < start {
				return nil, errBackfillHistoryIncomplete
			}
			if oldEnd, seen := seenTrack[start]; seen {
				if oldEnd != end {
					return nil, errBackfillHistoryIncomplete
				}
				continue
			}
			seenTrack[start] = end
			records = append(records, zeppBackfillInterval{Start: start, End: end})
		}
		next := *p.Data.Next
		if next == -1 {
			return records, nil
		}
		if next <= 0 {
			return nil, errBackfillHistoryIncomplete
		}
		cursor = next
	}
	return nil, errBackfillHistoryIncomplete
}

func (c *ZeppClient) readZeppBackfillHistoryPage(ctx context.Context, cursor int64) (zeppBackfillHistoryPage, error) {
	var page zeppBackfillHistoryPage
	q := url.Values{}
	if cursor != 0 {
		q.Set("trackid", strconv.FormatInt(cursor, 10))
	}
	// Reject redirects, even when the supplied client would follow them.
	httpClient := *c.httpClient
	httpClient.CheckRedirect = stopZeppAuthRedirect
	readOnly := *c
	readOnly.httpClient = &httpClient
	resp, err := readOnly.doGet(ctx, SportHistoryURL("", "run"), q)
	if err != nil {
		return page, errBackfillHistoryIncomplete
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return page, errBackfillHistoryUnauthorized
	}
	if resp.StatusCode != http.StatusOK {
		return page, fmt.Errorf("%w: HTTP %d", errBackfillHistoryIncomplete, resp.StatusCode)
	}
	lr := &io.LimitedReader{R: resp.Body, N: 16 << 20}
	dec := json.NewDecoder(lr)
	if err := dec.Decode(&page); err != nil {
		return page, errBackfillHistoryIncomplete
	}
	if err := dec.Decode(new(json.RawMessage)); err != io.EOF || lr.N == 0 {
		return page, errBackfillHistoryIncomplete
	}
	if page.Code != nil && (*page.Code == 401 || *page.Code == 403) {
		return page, errBackfillHistoryUnauthorized
	}
	if page.Code == nil || (*page.Code != 0 && *page.Code != 1) || page.Data == nil || page.Data.Next == nil || page.Data.Summary == nil {
		return page, errBackfillHistoryIncomplete
	}
	return page, nil
}

// BackfillHistoryPreflight reports the number of intervals in the fully
// enumerated run history feed. It uses only the configured client and does not
// modify workouts or initiate any authentication exchange.
func (c *ZeppClient) BackfillHistoryPreflight(ctx context.Context) (int, error) {
	records, err := c.readZeppBackfillHistory(ctx)
	return len(records), err
}

// ZeppBackfillPlanItem describes a local candidate; "not-in-feed" is a preview
// against a single read-only history snapshot, never a write authorization.
type ZeppBackfillPlanItem struct {
	ActivityID string `json:"activityId"`
	TrackID    int64  `json:"trackId,omitempty"`
	EndTime    int64  `json:"endTime,omitempty"`
	Status     string `json:"status"`
}

// PlanRideBackfill classifies cycling activities against the Zepp run history feed.
// It makes GET requests only and never signs in, renews a session, or writes.
func (c *ZeppClient) PlanRideBackfill(ctx context.Context, activities []Activity) ([]ZeppBackfillPlanItem, error) {
	history, err := c.readZeppBackfillHistory(ctx)
	if err != nil {
		return nil, err
	}
	return planRideBackfill(activities, history), nil
}

func planRideBackfill(activities []Activity, history []zeppBackfillInterval) []ZeppBackfillPlanItem {
	items := make([]ZeppBackfillPlanItem, 0, len(activities))
	for _, a := range activities {
		switch a.Type {
		case "Ride", "VirtualRide", "MountainBikeRide", "GravelRide", "EBikeRide", "EMountainBikeRide":
		default:
			continue
		}
		item := ZeppBackfillPlanItem{ActivityID: a.ID, Status: "invalid-source"}
		at, err := time.Parse(time.RFC3339, a.StartDate)
		if err == nil && a.ID != "" && at.Unix() > 0 && a.ElapsedTime > 0 && a.ElapsedTime <= 7*86400 {
			item.TrackID = at.Unix()
			item.EndTime = item.TrackID + int64(a.ElapsedTime)
			item.Status = "not-in-feed"
			if zeppBackfillOverlap(zeppBackfillInterval{item.TrackID, item.EndTime}, history) {
				item.Status = "overlap"
			}
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].TrackID != items[j].TrackID {
			return items[i].TrackID < items[j].TrackID
		}
		return items[i].ActivityID < items[j].ActivityID
	})
	// Do not nominate either side of a conflicting source pair. Status is
	// checked separately from timestamp validity because a source conflict can
	// exist even when only one of the source rides already overlaps Zepp.
	var active []int
	for i := range items {
		cur := &items[i]
		if cur.TrackID == 0 {
			continue
		}
		kept := active[:0]
		for _, j := range active {
			if items[j].EndTime < cur.TrackID {
				continue
			}
			kept = append(kept, j)
			if items[j].Status == "not-in-feed" {
				items[j].Status = "source-conflict"
			}
			if cur.Status == "not-in-feed" {
				cur.Status = "source-conflict"
			}
		}
		active = append(kept, i)
	}
	return items
}
