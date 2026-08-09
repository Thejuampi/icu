package icu

import "math"

// ActivityTypeVirtualRide and ActivityTypeIndoorCycling are the two
// intervals.icu types that are inherently indoor cycling: this type cannot be
// otherwise, regardless of any indoor/trainer flag (D-10 class 1,
// type_inherent, Confidence: high). Declared once here and reused by
// IsOutdoorCyclingActivity (power_estimate_backtest.go) so the set is never
// re-declared (F-15).
const (
	ActivityTypeVirtualRide   = "VirtualRide"
	ActivityTypeIndoorCycling = "IndoorCycling"
)

// IndoorCyclingTypes is the D-10 class-1 indoor type set.
var IndoorCyclingTypes = []string{ActivityTypeVirtualRide, ActivityTypeIndoorCycling} //nolint:gochecknoglobals // read-only lookup table, same shape as existing type-set constants in this package

// IsIndoorCyclingType reports whether t is inherently indoor cycling
// (D-10 class 1), regardless of any indoor/trainer flag.
func IsIndoorCyclingType(t string) bool {
	for _, indoorType := range IndoorCyclingTypes {
		if t == indoorType {
			return true
		}
	}

	return false
}

// activityNotApplicableTypes is D-10's activity-partition class 4: types for
// which no FTP applies at all.
var activityNotApplicableTypes = []string{"Walk", "Run", "VirtualRun", "WeightTraining", "Workout"} //nolint:gochecknoglobals // read-only lookup table

// eventNotApplicableTypes is D-10's event-partition class 4. It adds NOTE,
// which has no activity-partition counterpart.
var eventNotApplicableTypes = []string{"Walk", "Run", "VirtualRun", "WeightTraining", "Workout", "NOTE"} //nolint:gochecknoglobals // read-only lookup table

func containsType(types []string, t string) bool {
	for _, candidate := range types {
		if t == candidate {
			return true
		}
	}

	return false
}

// FTPEntity partitions D-10's two resolution tables. Ride resolves
// differently by entity (class 2 for an activity, class 3 for an event) —
// deliberate, evidenced, and not collapsible into one table.
type FTPEntity string

const (
	FTPEntityActivity FTPEntity = "activity"
	FTPEntityEvent    FTPEntity = "event"
)

// FTPClass is total over five values (D-9). not_a_record (D-24) is a fifth
// class alongside the four described by D-10, not a variant of
// not_applicable: a STRAVA stub is not a session at all.
type FTPClass string

const (
	FTPClassIndoor        FTPClass = "indoor"
	FTPClassOutdoor       FTPClass = "outdoor"
	FTPClassAmbiguous     FTPClass = "ambiguous"
	FTPClassNotApplicable FTPClass = "not_applicable"
	FTPClassNotARecord    FTPClass = "not_a_record"
)

// FTP selection sources (D-9).
const (
	FTPSourceTypeInherent   = "type_inherent"
	FTPSourceTypeDefault    = "type_default"
	FTPSourceFlagAsserted   = "flag_asserted"
	FTPSourceExplicit       = "explicit"
	FTPSourceAthleteDefault = "athlete_default"
	FTPSourceUnknown        = "unknown"
)

// Confidence tiers (D-10). high = type_inherent only: this type cannot be
// otherwise. medium = type_default and flag_asserted: usually is, or
// asserted by a flag whose positive direction has n=2. low = unknown.
// type_default is deliberately not high: it covers Ride activities with a
// null flag, and reporting high there would be fabricated certainty in the
// field built to be the audit trail.
const (
	FTPConfidenceHigh   = "high"
	FTPConfidenceMedium = "medium"
	FTPConfidenceLow    = "low"
)

const ftpZoneSourceSportSettings = "sport_settings_power_zones"

// ftpZonePercentDivisor converts SportSettings.PowerZones (percentages,
// F-3) into a fraction of the selected FTP.
const ftpZonePercentDivisor = 100.0

// FTPRequest is the single input to ResolveFTP (D-9). Type is required and
// is the first test the classifier runs.
type FTPRequest struct {
	// Type is the intervals.icu activity/event type string. Required.
	Type string
	// Entity selects which of D-10's two partitions applies.
	Entity FTPEntity
	// Trainer is the activity-side environment flag. Evidence, never the
	// sole discriminator for a type-inherent class (D-10).
	Trainer *bool
	// EventIndoor is the event-side environment flag. Same trust rule as
	// Trainer, scoped to the event partition.
	EventIndoor *bool
	// StubShape marks a STRAVA stub (D-24): five keys, "_note" present,
	// source == "STRAVA". Discriminated on shape, never on absence of type,
	// so a genuinely typeless-but-real record does not fall in here.
	StubShape bool
	Settings  *SportSettings
	// Explicit carries --session-environment ("indoor" | "outdoor"). Wired
	// by wave-1b (D-10c); ResolveFTP does not yet consume it — declared here
	// because FTPRequest is D-9's stable, forward-compatible shape.
	Explicit *string
}

// FTPSelection is ResolveFTP's total result (D-9).
type FTPSelection struct {
	// Applicable is false for not_applicable and not_a_record: no FTP-backed
	// value is meaningful for either (D-10c).
	Applicable bool
	Watts      int
	// Source names how Watts was chosen: type_inherent | type_default |
	// flag_asserted | explicit | athlete_default | unknown.
	Source     string
	Class      FTPClass
	Confidence string
	// Indoor is the resolved environment; nil when Applicable is false.
	Indoor *bool
	// Disagreement is true exactly when the flag contradicts the class's own
	// default direction (D-10): a false flag on a type_inherent class, or a
	// true flag on a type_default class. Ambiguous classes have no default
	// to contradict, so this is always false there.
	Disagreement bool
	ZoneWatts    []int
	ZoneSource   string
	// Warning is set when Confidence is low: analysis/read sites proceed and
	// warn (D-10c); it carries the human-readable reason.
	Warning string
}

// ResolveFTP is D-9's single FTP selection rule: pure, zero IO, total over
// five classes and both entity partitions (D-10). It replaces the three
// divergent rules named in F-1.
func ResolveFTP(request FTPRequest) FTPSelection {
	if request.StubShape {
		return FTPSelection{Class: FTPClassNotARecord}
	}

	var classified environmentClassification
	if request.Entity == FTPEntityEvent {
		classified = classifyEventEnvironment(request)
	} else {
		classified = classifyActivityEnvironment(request)
	}

	if classified.class == FTPClassNotApplicable {
		return FTPSelection{Class: FTPClassNotApplicable}
	}

	watts, inert := selectFTPWatts(request.Settings, classified.indoor)

	selection := FTPSelection{
		Applicable:   true,
		Watts:        watts,
		Source:       classified.source,
		Class:        classified.class,
		Confidence:   classified.confidence,
		Indoor:       &classified.indoor,
		Disagreement: classified.disagreement,
	}

	switch {
	case inert:
		// Inertness guard (D-10): IndoorFTP is unset or equals FTP, so there
		// is no real number to discriminate between. No gate, no warning.
		selection.Source = FTPSourceAthleteDefault
		selection.Disagreement = false
	case classified.confidence == FTPConfidenceLow:
		selection.Warning = "ftp environment could not be determined: ambiguous type with no discriminating flag"
	}

	selection.ZoneWatts, selection.ZoneSource = zoneWattsFor(request.Settings, watts)

	return selection
}

// environmentClassification is the intermediate result of applying D-10's
// per-entity class table, before the inertness guard and zone lookup run.
type environmentClassification struct {
	class        FTPClass
	indoor       bool
	source       string
	confidence   string
	disagreement bool
}

// classifyActivityEnvironment implements D-10's activity partition (classes
// 1-3; not_applicable and not_a_record are handled by the caller/above).
func classifyActivityEnvironment(request FTPRequest) environmentClassification {
	switch {
	case IsIndoorCyclingType(request.Type):
		return environmentClassification{
			class: FTPClassIndoor, indoor: true,
			source: FTPSourceTypeInherent, confidence: FTPConfidenceHigh,
			disagreement: request.Trainer != nil && !*request.Trainer,
		}
	case request.Type == "Ride":
		if request.Trainer != nil && *request.Trainer {
			return environmentClassification{
				class: FTPClassOutdoor, indoor: true,
				source: FTPSourceFlagAsserted, confidence: FTPConfidenceMedium,
				disagreement: true,
			}
		}

		return environmentClassification{
			class: FTPClassOutdoor, indoor: false,
			source: FTPSourceTypeDefault, confidence: FTPConfidenceMedium,
		}
	case request.Type == "" || containsType(activityNotApplicableTypes, request.Type):
		return environmentClassification{class: FTPClassNotApplicable}
	default:
		return classifyAmbiguousActivity(request)
	}
}

func classifyAmbiguousActivity(request FTPRequest) environmentClassification {
	if request.Trainer == nil {
		return environmentClassification{
			class:  FTPClassAmbiguous,
			source: FTPSourceUnknown, confidence: FTPConfidenceLow,
		}
	}

	return environmentClassification{
		class: FTPClassAmbiguous, indoor: *request.Trainer,
		source: FTPSourceFlagAsserted, confidence: FTPConfidenceMedium,
	}
}

// classifyEventEnvironment implements D-10's event partition. Ride and every
// other non-inherent cycling type share one ambiguous class here — unlike
// the activity partition, where Ride has its own default-outdoor class 2.
func classifyEventEnvironment(request FTPRequest) environmentClassification {
	switch {
	case IsIndoorCyclingType(request.Type):
		return environmentClassification{
			class: FTPClassIndoor, indoor: true,
			source: FTPSourceTypeInherent, confidence: FTPConfidenceHigh,
			disagreement: request.EventIndoor != nil && !*request.EventIndoor,
		}
	case request.Type == "" || containsType(eventNotApplicableTypes, request.Type):
		return environmentClassification{class: FTPClassNotApplicable}
	default:
		if request.EventIndoor != nil && *request.EventIndoor {
			return environmentClassification{
				class: FTPClassAmbiguous, indoor: true,
				source: FTPSourceFlagAsserted, confidence: FTPConfidenceMedium,
			}
		}

		return environmentClassification{
			class: FTPClassAmbiguous, indoor: false,
			source: FTPSourceTypeDefault, confidence: FTPConfidenceMedium,
		}
	}
}

// selectFTPWatts applies the D-10 inertness guard: when IndoorFTP is unset or
// equals FTP, there is no real number to discriminate between.
func selectFTPWatts(settings *SportSettings, indoor bool) (int, bool) {
	if settings == nil {
		return 0, true
	}

	if settings.IndoorFTP == 0 || settings.IndoorFTP == settings.FTP {
		return settings.FTP, true
	}

	if indoor {
		return settings.IndoorFTP, false
	}

	return settings.FTP, false
}

// zoneWattsFor derives zone watt boundaries from SportSettings.PowerZones
// (percentages, F-3) and the selected watts. It is the same call that
// yields Watts — a second independent zone lookup is not expressible,
// because the percentages are shared and only the watts differ.
func zoneWattsFor(settings *SportSettings, watts int) ([]int, string) {
	if settings == nil || len(settings.PowerZones) == 0 || watts <= 0 {
		return nil, ""
	}

	zones := make([]int, len(settings.PowerZones))
	for i, percent := range settings.PowerZones {
		zones[i] = int(math.Round(float64(percent) / ftpZonePercentDivisor * float64(watts)))
	}

	return zones, ftpZoneSourceSportSettings
}
