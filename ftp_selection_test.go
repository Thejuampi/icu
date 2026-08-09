package icu_test

import (
	"reflect"
	"testing"

	icu "github.com/Thejuampi/icu"
)

func trueFlag() *bool {
	v := true

	return &v
}

func falseFlag() *bool {
	v := false

	return &v
}

// W1a-P01, W1a-D01..D06, W1a-N01..N03: D-10's activity partition.
func TestResolveFTPActivityPartition(t *testing.T) {
	t.Parallel()

	settings := &icu.SportSettings{FTP: 270, IndoorFTP: 285}

	cases := []struct {
		name    string
		request icu.FTPRequest
		want    icu.FTPSelection
	}{
		{
			name: "W1a-P01 VirtualRide Trainer nil resolves indoor, type_inherent, high",
			request: icu.FTPRequest{
				Type: icu.ActivityTypeVirtualRide, Entity: icu.FTPEntityActivity, Settings: settings,
			},
			want: icu.FTPSelection{
				Applicable: true, Watts: 285, Source: icu.FTPSourceTypeInherent,
				Class: icu.FTPClassIndoor, Confidence: icu.FTPConfidenceHigh, Indoor: trueFlag(),
			},
		},
		{
			name: "W1a-D01 VirtualRide Trainer false ignored, disagreement true (46 real)",
			request: icu.FTPRequest{
				Type: icu.ActivityTypeVirtualRide, Entity: icu.FTPEntityActivity, Trainer: falseFlag(), Settings: settings,
			},
			want: icu.FTPSelection{
				Applicable: true, Watts: 285, Source: icu.FTPSourceTypeInherent,
				Class: icu.FTPClassIndoor, Confidence: icu.FTPConfidenceHigh, Indoor: trueFlag(), Disagreement: true,
			},
		},
		{
			name: "W1a-D02 Ride Trainer true asserts indoor, medium, disagreement",
			request: icu.FTPRequest{
				Type: "Ride", Entity: icu.FTPEntityActivity, Trainer: trueFlag(), Settings: settings,
			},
			want: icu.FTPSelection{
				Applicable: true, Watts: 285, Source: icu.FTPSourceFlagAsserted,
				Class: icu.FTPClassOutdoor, Confidence: icu.FTPConfidenceMedium, Indoor: trueFlag(), Disagreement: true,
			},
		},
		{
			name: "W1a-D03 Ride Trainer false, type_default, medium, no disagreement (11 real)",
			request: icu.FTPRequest{
				Type: "Ride", Entity: icu.FTPEntityActivity, Trainer: falseFlag(), Settings: settings,
			},
			want: icu.FTPSelection{
				Applicable: true, Watts: 270, Source: icu.FTPSourceTypeDefault,
				Class: icu.FTPClassOutdoor, Confidence: icu.FTPConfidenceMedium, Indoor: falseFlag(),
			},
		},
		{
			name: "W1a-D04 Ride Trainer nil, type_default, medium, no gate (45 real)",
			request: icu.FTPRequest{
				Type: "Ride", Entity: icu.FTPEntityActivity, Settings: settings,
			},
			want: icu.FTPSelection{
				Applicable: true, Watts: 270, Source: icu.FTPSourceTypeDefault,
				Class: icu.FTPClassOutdoor, Confidence: icu.FTPConfidenceMedium, Indoor: falseFlag(),
			},
		},
		{
			name: "W1a-D05 unrecognised cycling type Trainer true, ambiguous, flag_asserted, medium",
			request: icu.FTPRequest{
				Type: "GravelRide", Entity: icu.FTPEntityActivity, Trainer: trueFlag(), Settings: settings,
			},
			want: icu.FTPSelection{
				Applicable: true, Watts: 285, Source: icu.FTPSourceFlagAsserted,
				Class: icu.FTPClassAmbiguous, Confidence: icu.FTPConfidenceMedium, Indoor: trueFlag(),
			},
		},
		{
			name: "W1a-D06 unrecognised cycling type Trainer nil, unknown, low",
			request: icu.FTPRequest{
				Type: "GravelRide", Entity: icu.FTPEntityActivity, Settings: settings,
			},
			want: icu.FTPSelection{
				Applicable: true, Watts: 270, Source: icu.FTPSourceUnknown,
				Class: icu.FTPClassAmbiguous, Confidence: icu.FTPConfidenceLow, Indoor: falseFlag(),
				Warning: "ftp environment could not be determined: ambiguous type with no discriminating flag",
			},
		},
		{
			name: "W1a-N01 Walk is not applicable, no gate, Source not unknown",
			request: icu.FTPRequest{
				Type: "Walk", Entity: icu.FTPEntityActivity, Settings: settings,
			},
			want: icu.FTPSelection{Class: icu.FTPClassNotApplicable},
		},
		{
			name: "W1a-N02 typeless non-stub record is not applicable, not not_a_record",
			request: icu.FTPRequest{
				Type: "", Entity: icu.FTPEntityActivity, StubShape: false, Settings: settings,
			},
			want: icu.FTPSelection{Class: icu.FTPClassNotApplicable},
		},
		{
			name: "W1a-N03 STRAVA stub shape is not_a_record: no load, no time, no slot, no pairing",
			request: icu.FTPRequest{
				Type: "Ride", Entity: icu.FTPEntityActivity, StubShape: true, Settings: settings,
			},
			want: icu.FTPSelection{Class: icu.FTPClassNotARecord},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got := icu.ResolveFTP(testCase.request)
			if !reflect.DeepEqual(got, testCase.want) {
				t.Fatalf("ResolveFTP(%+v) = %+v, want %+v", testCase.request, got, testCase.want)
			}
		})
	}
}

// W1a-E01..E03: D-10's event partition. Ride resolves as class 3 here (D-10),
// unlike the activity partition where it is class 2 — the two tables are
// deliberately not collapsed into one.
func TestResolveFTPEventPartition(t *testing.T) {
	t.Parallel()

	settings := &icu.SportSettings{FTP: 270, IndoorFTP: 285}

	cases := []struct {
		name    string
		request icu.FTPRequest
		want    icu.FTPSelection
	}{
		{
			name: "W1a-E01 event VirtualRide Indoor nil, type_inherent, high",
			request: icu.FTPRequest{
				Type: icu.ActivityTypeVirtualRide, Entity: icu.FTPEntityEvent, Settings: settings,
			},
			want: icu.FTPSelection{
				Applicable: true, Watts: 285, Source: icu.FTPSourceTypeInherent,
				Class: icu.FTPClassIndoor, Confidence: icu.FTPConfidenceHigh, Indoor: trueFlag(),
			},
		},
		{
			name: "W1a-E02 event Ride Indoor true, flag_asserted, medium (2 real)",
			request: icu.FTPRequest{
				Type: "Ride", Entity: icu.FTPEntityEvent, EventIndoor: trueFlag(), Settings: settings,
			},
			want: icu.FTPSelection{
				Applicable: true, Watts: 285, Source: icu.FTPSourceFlagAsserted,
				Class: icu.FTPClassAmbiguous, Confidence: icu.FTPConfidenceMedium, Indoor: trueFlag(),
			},
		},
		{
			name: "W1a-E03 event VirtualRide Indoor false, type_inherent, disagreement (1 real)",
			request: icu.FTPRequest{
				Type: icu.ActivityTypeVirtualRide, Entity: icu.FTPEntityEvent, EventIndoor: falseFlag(), Settings: settings,
			},
			want: icu.FTPSelection{
				Applicable: true, Watts: 285, Source: icu.FTPSourceTypeInherent,
				Class: icu.FTPClassIndoor, Confidence: icu.FTPConfidenceHigh, Indoor: trueFlag(), Disagreement: true,
			},
		},
		{
			// Not a BDD row, but D-10's residual-risk note is explicit that this
			// is what plan.go does today (not a regression) — a Ride event with
			// no flag resolves medium, never low, unlike the activity partition.
			name: "event Ride EventIndoor nil resolves type_default medium, never low (53 real, residual risk)",
			request: icu.FTPRequest{
				Type: "Ride", Entity: icu.FTPEntityEvent, Settings: settings,
			},
			want: icu.FTPSelection{
				Applicable: true, Watts: 270, Source: icu.FTPSourceTypeDefault,
				Class: icu.FTPClassAmbiguous, Confidence: icu.FTPConfidenceMedium, Indoor: falseFlag(),
			},
		},
		{
			name: "event NOTE type is not applicable (event-only class-4 member)",
			request: icu.FTPRequest{
				Type: "NOTE", Entity: icu.FTPEntityEvent, Settings: settings,
			},
			want: icu.FTPSelection{Class: icu.FTPClassNotApplicable},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got := icu.ResolveFTP(testCase.request)
			if !reflect.DeepEqual(got, testCase.want) {
				t.Fatalf("ResolveFTP(%+v) = %+v, want %+v", testCase.request, got, testCase.want)
			}
		})
	}
}

// W1a-R01: the inertness guard fires for any type/partition once IndoorFTP
// carries no real discrimination — no gate, no warning.
func TestResolveFTPInertnessGuardIgnoresType(t *testing.T) {
	t.Parallel()

	settings := &icu.SportSettings{FTP: 285, IndoorFTP: 285}

	cases := []struct {
		name    string
		request icu.FTPRequest
	}{
		{
			name:    "type_inherent class collapses to athlete_default",
			request: icu.FTPRequest{Type: icu.ActivityTypeVirtualRide, Entity: icu.FTPEntityActivity, Settings: settings},
		},
		{
			name:    "type_default class collapses to athlete_default",
			request: icu.FTPRequest{Type: "Ride", Entity: icu.FTPEntityActivity, Settings: settings},
		},
		{
			name: "flag_asserted class collapses to athlete_default",
			request: icu.FTPRequest{
				Type: "GravelRide", Entity: icu.FTPEntityActivity, Trainer: trueFlag(), Settings: settings,
			},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got := icu.ResolveFTP(testCase.request)
			isInert := got.Source == icu.FTPSourceAthleteDefault && !got.Disagreement && got.Warning == "" && got.Watts == 285

			if !isInert {
				t.Fatalf("ResolveFTP(%+v) = %+v, want Source=athlete_default, Disagreement=false, Warning=\"\", Watts=285",
					testCase.request, got)
			}
		})
	}
}

// ZoneWatts is returned by the same call as Watts (D-9): a second independent
// zone lookup is not expressible.
func TestResolveFTPZoneWattsDerivedFromSameCall(t *testing.T) {
	t.Parallel()

	settings := &icu.SportSettings{FTP: 270, IndoorFTP: 285, PowerZones: []int{55, 75, 90, 105, 120}}
	request := icu.FTPRequest{Type: icu.ActivityTypeVirtualRide, Entity: icu.FTPEntityActivity, Settings: settings}
	want := []int{157, 214, 257, 299, 342}

	got := icu.ResolveFTP(request)

	matches := reflect.DeepEqual(got.ZoneWatts, want) && got.ZoneSource != ""
	if !matches {
		t.Fatalf("ResolveFTP(%+v).ZoneWatts = %v (source %q), want %v with a non-empty source",
			request, got.ZoneWatts, got.ZoneSource, want)
	}
}

// D-24: a STRAVA stub blocks classification before any entity partition
// runs, regardless of what Type happens to carry.
func TestResolveFTPStubShapeShortCircuitsEntityPartition(t *testing.T) {
	t.Parallel()

	request := icu.FTPRequest{
		Type: icu.ActivityTypeVirtualRide, Entity: icu.FTPEntityEvent, StubShape: true,
		Settings: &icu.SportSettings{FTP: 270, IndoorFTP: 285},
	}
	want := icu.FTPSelection{Class: icu.FTPClassNotARecord}

	got := icu.ResolveFTP(request)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ResolveFTP(%+v) = %+v, want %+v", request, got, want)
	}
}
