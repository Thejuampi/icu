package icu_test

import (
	"encoding/json"
	"testing"

	icu "github.com/Thejuampi/icu"
)

// TestActivityUnmarshalEmitOnlyFieldsPopulated is wave-4/A2's happy path:
// all nine emit-only fields (D-1) present and non-null, typed correctly
// from their raw snake_case keys. icu_hrr uses the corpus's real sample
// shape (F-24) except for AverageWatts, which the live corpus never once
// populates -- see TestActivityUnmarshalHRRAverageWattsOnlyExercisedSynthetically
// for why a non-null value here is itself a synthetic case, not a live one.
func TestActivityUnmarshalEmitOnlyFieldsPopulated(t *testing.T) {
	t.Parallel()

	var activity icu.Activity

	data := []byte(`{"polarization_index":1.17,"ss_cp":62.551357,"ss_w_prime":0.447127,` +
		`"ss_p_max":0.092711866,"carbs_used":215,"carbs_ingested":90,"icu_rolling_ftp_delta":11,` +
		`"power_field":"power","icu_hrr":{"start_index":3293,"end_index":3353,"start_time":3348,` +
		`"end_time":3408,"start_bpm":186,"end_bpm":162,"average_watts":180,"hrr":24}}`)

	if err := json.Unmarshal(data, &activity); err != nil {
		t.Fatalf("json.Unmarshal Activity: %v", err)
	}

	mismatch := activity.HRR == nil ||
		derefFloat(activity.PolarizationIndex) != 1.17 ||
		derefFloat(activity.SSCP) != 62.551357 ||
		derefFloat(activity.SSWPrime) != 0.447127 ||
		derefFloat(activity.SSPMax) != 0.092711866 ||
		derefInt(activity.CarbsUsed) != 215 ||
		derefInt(activity.CarbsIngested) != 90 ||
		derefInt(activity.RollingFTPDelta) != 11 ||
		derefString(activity.PowerField) != "power" ||
		activity.HRR.StartIndex != 3293 || activity.HRR.EndIndex != 3353 ||
		activity.HRR.StartTime != 3348 || activity.HRR.EndTime != 3408 ||
		activity.HRR.StartBPM != 186 || activity.HRR.EndBPM != 162 ||
		activity.HRR.HRR != 24 || derefInt(activity.HRR.AverageWatts) != 180

	if mismatch {
		t.Fatalf("Activity emit-only fields = %+v (HRR=%+v), want the fixture's typed values", activity, activity.HRR)
	}
}

// TestActivityUnmarshalEmitOnlyFieldsAbsentStayNil is D-5's absent-field
// fixture: none of the nine keys present anywhere in the payload must leave
// every pointer nil, not a measured zero.
func TestActivityUnmarshalEmitOnlyFieldsAbsentStayNil(t *testing.T) {
	t.Parallel()

	var activity icu.Activity

	if err := json.Unmarshal([]byte(`{"name":"no emit-only fields here"}`), &activity); err != nil {
		t.Fatalf("json.Unmarshal Activity: %v", err)
	}

	allNil := activity.PolarizationIndex == nil && activity.SSCP == nil && activity.SSWPrime == nil &&
		activity.SSPMax == nil && activity.CarbsUsed == nil && activity.CarbsIngested == nil &&
		activity.RollingFTPDelta == nil && activity.PowerField == nil && activity.HRR == nil

	if !allNil {
		t.Fatalf("Activity = %+v, want every emit-only field nil when absent from the payload", activity)
	}
}

// TestActivityUnmarshalEmitOnlyFieldsExplicitZeroAreNotNil is D-5's
// explicit-zero fixture: a measured zero (or empty string) must read as
// present, not absent -- the exact ambiguity pointer + omitempty exists to
// prevent.
func TestActivityUnmarshalEmitOnlyFieldsExplicitZeroAreNotNil(t *testing.T) {
	t.Parallel()

	var activity icu.Activity

	data := []byte(`{"polarization_index":0,"ss_cp":0,"ss_w_prime":0,"ss_p_max":0,` +
		`"carbs_used":0,"carbs_ingested":0,"icu_rolling_ftp_delta":0,"power_field":""}`)

	if err := json.Unmarshal(data, &activity); err != nil {
		t.Fatalf("json.Unmarshal Activity: %v", err)
	}

	allPresentAndZero := activity.PolarizationIndex != nil && *activity.PolarizationIndex == 0 &&
		activity.SSCP != nil && *activity.SSCP == 0 &&
		activity.SSWPrime != nil && *activity.SSWPrime == 0 &&
		activity.SSPMax != nil && *activity.SSPMax == 0 &&
		activity.CarbsUsed != nil && *activity.CarbsUsed == 0 &&
		activity.CarbsIngested != nil && *activity.CarbsIngested == 0 &&
		activity.RollingFTPDelta != nil && *activity.RollingFTPDelta == 0 &&
		activity.PowerField != nil && *activity.PowerField == ""

	if !allPresentAndZero {
		t.Fatalf("Activity = %+v, want every numeric field present with 0 and PowerField present with \"\"", activity)
	}
}

// TestActivityUnmarshalHRRNullStaysNil covers the 181/230 live records where
// icu_hrr is explicitly null (F-24): the pointer must stay nil, matching
// every other nullable object field on Activity.
func TestActivityUnmarshalHRRNullStaysNil(t *testing.T) {
	t.Parallel()

	var activity icu.Activity

	if err := json.Unmarshal([]byte(`{"icu_hrr":null}`), &activity); err != nil {
		t.Fatalf("json.Unmarshal Activity: %v", err)
	}

	if activity.HRR != nil {
		t.Fatalf("Activity.HRR = %+v, want nil for an explicit null icu_hrr", activity.HRR)
	}
}

// TestActivityUnmarshalHRRAverageWattsOnlyExercisedSynthetically documents
// wave-4's disclosed finding: icu_hrr.average_watts is null on all 49
// non-null HRR records in the live corpus (testdata/wave2_live_corpus.json)
// -- every single one. No live fixture can exercise a populated
// AverageWatts; this synthetic one is the only possible acceptance
// evidence, exactly as ignore-flag filtering's only evidence is synthetic
// (wave-2's inertness statement is the same shape of claim).
func TestActivityUnmarshalHRRAverageWattsOnlyExercisedSynthetically(t *testing.T) {
	t.Parallel()

	var activity icu.Activity

	data := []byte(`{"icu_hrr":{"start_index":1,"end_index":2,"start_time":3,"end_time":4,` +
		`"start_bpm":150,"end_bpm":130,"average_watts":180,"hrr":20}}`)

	if err := json.Unmarshal(data, &activity); err != nil {
		t.Fatalf("json.Unmarshal Activity: %v", err)
	}

	if activity.HRR == nil || derefInt(activity.HRR.AverageWatts) != 180 {
		t.Fatalf("Activity.HRR = %+v, want AverageWatts populated with 180", activity.HRR)
	}
}

// TestActivityUnmarshalStubNoteCarriesThrough is Stage 6 R5: a D-24 STRAVA
// stub's `_note` explains why the record was excluded ("STRAVA activities
// are not available via the API"); the exclusion counts were always correct
// without it, but dropping the one field that answers "why don't these
// records count?" left a caller with a bare, unexplained object. Same
// emit-only shape as wave-4's nine fields: transported verbatim, computed
// from nowhere else.
func TestActivityUnmarshalStubNoteCarriesThrough(t *testing.T) {
	t.Parallel()

	var activity icu.Activity

	data := []byte(`{"id":"19999999998","icu_athlete_id":"i445643",` +
		`"start_date_local":"2025-12-02T18:37:27","source":"STRAVA",` +
		`"_note":"STRAVA activities are not available via the API"}`)

	if err := json.Unmarshal(data, &activity); err != nil {
		t.Fatalf("json.Unmarshal Activity: %v", err)
	}

	if !activity.StubShape || derefString(activity.Note) != "STRAVA activities are not available via the API" {
		t.Fatalf("Activity.StubShape/Note = %v/%v, want stub-shaped with the upstream note carried through",
			activity.StubShape, activity.Note)
	}
}

// TestActivityUnmarshalNoteAbsentForRealActivity is D-5's absent-field
// fixture for Note: a real (non-stub) activity must leave Note nil, not a
// fabricated empty string.
func TestActivityUnmarshalNoteAbsentForRealActivity(t *testing.T) {
	t.Parallel()

	var activity icu.Activity

	if err := json.Unmarshal([]byte(`{"name":"a real ride","type":"Ride"}`), &activity); err != nil {
		t.Fatalf("json.Unmarshal Activity: %v", err)
	}

	if activity.Note != nil {
		t.Fatalf("Activity.Note = %v, want nil for a real activity with no _note key", activity.Note)
	}
}

func derefFloat(value *float64) float64 {
	if value == nil {
		return 0
	}

	return *value
}

func derefInt(value *int) int {
	if value == nil {
		return 0
	}

	return *value
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}

	return *value
}
