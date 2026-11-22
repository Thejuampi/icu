package icu

import (
	"io"
	"net/http"
	"net/url"
	"strings"
)

// ZeppRideFITPreview describes the forms the canary would send without
// exposing field values, coordinates, track IDs, or account information.
type ZeppRideFITPreview struct {
	FITRecords     int            `json:"fit_records"`
	AlignedRecords int            `json:"aligned_records"`
	GPSRecords     int            `json:"gps_records"`
	SkippedRecords int            `json:"skipped_aligned_records"`
	Detail         map[string]int `json:"detail_field_bytes"`
	Summary        map[string]int `json:"summary_field_bytes"`
}

// PreviewZeppRideFIT parses and builds the canary forms without configuration,
// credentials, HTTP, or displaying any form values.
func PreviewZeppRideFIT(fit io.Reader) (ZeppRideFITPreview, error) {
	if fit == nil {
		return ZeppRideFITPreview{}, errBackfillCanaryWireInput
	}
	ride, detail, summary, err := prepareZeppRideForms(fit, backfillCanaryCommonFields("", ""))
	if err != nil {
		return ZeppRideFITPreview{}, err
	}
	if !validBackfillCanaryWireValues(http.MethodPost, backfillCanaryWireDetailPath, "", detail) ||
		!validBackfillCanaryWireValues(http.MethodPost, backfillCanaryWireSummaryPath, "", summary) {
		return ZeppRideFITPreview{}, errBackfillCanaryWireInput
	}

	aligned := strings.Count(detail.Get("time"), ";") + 1
	gps := 0
	if detail.Has("longitude_latitude") {
		gps = aligned
	}
	return ZeppRideFITPreview{
		FITRecords: len(ride.Points), AlignedRecords: aligned, GPSRecords: gps,
		SkippedRecords: len(ride.Points) - aligned,
		Detail:         zeppFormFieldSizes(detail), Summary: zeppFormFieldSizes(summary),
	}, nil
}

func prepareZeppRideForms(fit io.Reader, common url.Values) (ZeppRideFIT, url.Values, url.Values, error) {
	ride, err := DecodeZeppRideFIT(fit)
	if err != nil {
		return ZeppRideFIT{}, nil, nil, err
	}
	summary, detail, err := EncodeZeppRideFIT(ride)
	if err != nil {
		return ZeppRideFIT{}, nil, nil, err
	}
	return ride, backfillCanaryForm(detail, common), backfillCanaryForm(summary, common), nil
}

func backfillCanaryForm(fields map[string]string, common url.Values) url.Values {
	form := make(url.Values, len(fields)+len(common))
	for key := range common {
		form.Set(key, common.Get(key))
	}
	for key, value := range fields {
		form.Set(key, value)
	}
	return form
}

func zeppFormFieldSizes(form url.Values) map[string]int {
	sizes := make(map[string]int, len(form))
	for key := range form {
		sizes[key] = len(form.Get(key))
	}
	return sizes
}
