package icu

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

const (
	// CyclingAnalysisSchemaVersion marks the wire shape of CyclingAnalysis
	// (D-7): a consumer parsing analysis output needs a version marker to
	// detect a future meaning change, the same reasoning that already backs
	// RebalanceSchemaVersion and PlanSchemaVersion.
	CyclingAnalysisSchemaVersion = "icu.analysis.cycling.v1"

	analysisDateLayout       = "2006-01-02"
	acuteLoadDays            = 7
	chronicLoadDays          = 28
	longEnduranceSeconds     = 7200
	secondsPerMinute         = 60
	secondsPerHour           = 3600
	metersPerKilometer       = 1000
	percentScale             = 100
	thousandScale            = 1000
	unknownState             = "unknown"
	stateLabelLoadPressure   = "Load Pressure"
	stateOperationalRecovery = "recovery_priority"
	stateLoadPressure        = "load_pressure"
	stateDurabilityWatch     = "watch"
	highIntensityThreshold   = 0.85
	highDecouplingPct        = 5
	hotTempThreshold         = 30
	hotFeelsLikeThreshold    = 32
	windSpeedThreshold       = 15
	headwindThreshold        = 40
	climbingGradientPct      = 3
	wPrimeRepeatablePct      = 50
	wPrimeModeratePct        = 30
	isdmBaseScore            = 100
	isdmDecouplingPenalty    = 5
	isdmHighDriftPenalty     = 10
	lowEfficiencyThreshold   = 1.2
	highEfficiencyThreshold  = 1.4

	// DefaultAnalysisTimezone is the timezone used for computed analysis date ranges.
	DefaultAnalysisTimezone = "UTC"
	// DefaultAnalysisTimezoneSource documents how the analysis timezone was selected.
	DefaultAnalysisTimezoneSource = "default_utc"
)

type AnalysisOptions struct {
	StartDate      string `json:"startDate,omitempty"`
	EndDate        string `json:"endDate,omitempty"`
	Timezone       string `json:"timezone,omitempty"`
	TimezoneSource string `json:"timezoneSource,omitempty"`
	// MinIncludedActivities is D-4's optional structural-refusal threshold,
	// wired to --min-included-activities: zero means disabled (the flag has
	// no default), any positive value adds a named warning when
	// Validity.Included falls below it. Input-only: never part of the
	// analysis wire contract.
	MinIncludedActivities int `json:"-"`
	// Thresholds are the athlete's profile overrides for baseline-derived
	// readiness levels (nil derives everything from history).
	Thresholds *TrainingThresholds `json:"-"`
}

type CyclingAnalysis struct {
	SchemaVersion string                         `json:"schemaVersion"`
	Scope         CyclingScope                   `json:"scope"`
	Validity      CyclingValidity                `json:"validity"`
	State         CyclingState                   `json:"state"`
	Volume        CyclingVolume                  `json:"volume"`
	PowerAnchors  CyclingPowerAnchors            `json:"powerAnchors"`
	Environment   CyclingEnvironmentContext      `json:"environment"`
	Load          CyclingLoad                    `json:"load"`
	Intensity     CyclingIntensity               `json:"intensity"`
	Durability    CyclingDurability              `json:"durability"`
	Anaerobic     CyclingAnaerobic               `json:"anaerobic"`
	Performance   CyclingPerformanceIntelligence `json:"performance"`
	// FTPDiscriminatorConflicts is T1b.5/R1's read-path mirror of
	// rebalance.go's FTPDiscriminatorConflicts, at analysis level only — no
	// per-session key (the wave-2 golden's Sessions rows must stay
	// untouched). Top-level and omitempty: absent when there is no conflict
	// to report, not a false zero (D-3/D-5). See
	// CyclingFTPDiscriminatorConflicts for why it is currently always absent.
	FTPDiscriminatorConflicts *CyclingFTPDiscriminatorConflicts `json:"ftpDiscriminatorConflicts,omitempty"`
	Sessions                  []CyclingSession                  `json:"sessions,omitempty"`
	Warnings                  []string                          `json:"warnings,omitempty"`
}

// cyclingValidityZeroSemantics documents D-5's absence-vs-zero rule for the
// new tri-load channels (T2.5): a channel total of 0 with that channel's
// NIncluded also 0 means no session in scope reported the field at all, not
// a measured zero-load session.
var cyclingValidityZeroSemantics = []string{ //nolint:gochecknoglobals // read-only lookup table, same shape as existing type-set constants in this package
	"load.byChannel.hr/power/pace total 0 with nIncluded 0 means no session in scope reported that channel, not a measured zero-load session",
}

// CyclingValidity is D-2/D-3/D-24's per-analysis accounting of which
// activities fed the aggregates below and why any did not (F-19: stub
// contamination was the live defect; icu_ignore_* filtering is declared
// prevention that is measured false on every real activity fetched so far).
type CyclingValidity struct {
	// Evaluated/Included/Excluded describe only the cycling-typed, non-stub
	// population (Unavailable.StravaStubs is the separate whole-scope stub
	// count; a stub never reaches this classification because it carries no
	// activity type and is dropped before cycling-type filtering).
	Evaluated       int                        `json:"evaluated"`
	Included        int                        `json:"included"`
	Excluded        int                        `json:"excluded"`
	ExclusionRate   float64                    `json:"exclusionRate"`
	Unavailable     CyclingValidityUnavailable `json:"unavailable"`
	ExcludedByFlag  map[string]int             `json:"excludedByFlag,omitempty"`
	ExcludedByScope map[string]int             `json:"excludedByScope,omitempty"`
	// ExcludedActivityIDs lists the flag-excluded (not stub) activity IDs,
	// so a caller can inspect exactly which sessions were dropped from which
	// aggregates without re-deriving D-2's scoping rule.
	ExcludedActivityIDs []string `json:"excludedActivityIds,omitempty"`
	ZeroSemantics       []string `json:"zeroSemantics,omitempty"`
	Source              string   `json:"source,omitempty"`
	// FieldsGuaranteed records that D-8's single choke point fetches the
	// fields this validity block depends on regardless of any caller-supplied
	// --fields/--activity-fields override.
	FieldsGuaranteed bool `json:"fieldsGuaranteed"`
}

// CyclingValidityUnavailable is D-24's stub accounting: activities present
// in the raw fetch that are not sessions at all (STRAVA withheld the record).
type CyclingValidityUnavailable struct {
	StravaStubs int `json:"stravaStubs"`
}

type CyclingPowerAnchors struct {
	FTP           int    `json:"ftp,omitempty"`
	CriticalPower int    `json:"criticalPower,omitempty"`
	WPrime        int    `json:"wPrime,omitempty"`
	PMax          int    `json:"pMax,omitempty"`
	RollingFTP    int    `json:"rollingFtp,omitempty"`
	Source        string `json:"source,omitempty"`
}

type CyclingEnvironmentContext struct {
	Samples              int     `json:"samples"`
	AverageTemp          float64 `json:"averageTemp,omitempty"`
	MaxTemp              float64 `json:"maxTemp,omitempty"`
	AverageWeatherTemp   float64 `json:"averageWeatherTemp,omitempty"`
	AverageFeelsLike     float64 `json:"averageFeelsLike,omitempty"`
	AverageWindSpeed     float64 `json:"averageWindSpeed,omitempty"`
	MaxWindGust          float64 `json:"maxWindGust,omitempty"`
	HeadwindPercent      float64 `json:"headwindPercent,omitempty"`
	TailwindPercent      float64 `json:"tailwindPercent,omitempty"`
	AverageAltitude      float64 `json:"averageAltitude,omitempty"`
	MaxAltitude          float64 `json:"maxAltitude,omitempty"`
	AverageGradient      float64 `json:"averageGradient,omitempty"`
	AverageYaw           float64 `json:"averageYaw,omitempty"`
	AverageStrainScore   float64 `json:"averageStrainScore,omitempty"`
	HotSessions          int     `json:"hotSessions,omitempty"`
	WindAffectedSessions int     `json:"windAffectedSessions,omitempty"`
	ClimbingSessions     int     `json:"climbingSessions,omitempty"`
	Source               string  `json:"source,omitempty"`
}

type CyclingState struct {
	StateLabel        string  `json:"stateLabel,omitempty"`
	OperationalState  string  `json:"operationalState,omitempty"`
	LoadRecoveryState string  `json:"loadRecoveryState,omitempty"`
	LoadPressure      float64 `json:"loadPressure,omitempty"`
	Directive         string  `json:"directive,omitempty"`
	Source            string  `json:"source,omitempty"`
}

type CyclingScope struct {
	Activities        int    `json:"activities"`
	CyclingActivities int    `json:"cyclingActivities"`
	StartDate         string `json:"startDate,omitempty"`
	EndDate           string `json:"endDate,omitempty"`
	Timezone          string `json:"timezone,omitempty"`
	TimezoneSource    string `json:"timezoneSource,omitempty"`
}

type CyclingVolume struct {
	MovingTimeSecs      int     `json:"movingTimeSecs"`
	MovingTimeHours     float64 `json:"movingTimeHours"`
	DistanceMeters      float64 `json:"distanceMeters"`
	DistanceKilometers  float64 `json:"distanceKilometers"`
	ElevationGainMeters float64 `json:"elevationGainMeters"`
}

type CyclingLoad struct {
	Total                 int     `json:"total"`
	AveragePerActivity    float64 `json:"averagePerActivity"`
	PerHour               float64 `json:"perHour"`
	Monotony              float64 `json:"monotony"`
	Strain                float64 `json:"strain"`
	Acute7                int     `json:"acute7"`
	Chronic28             int     `json:"chronic28"`
	AcuteChronicWorkRatio float64 `json:"acuteChronicWorkRatio"`
	LatestCTL             float64 `json:"latestCtl,omitempty"`
	LatestATL             float64 `json:"latestAtl,omitempty"`
	LatestForm            float64 `json:"latestForm,omitempty"`
	// LatestLoadDate and LatestLoadSource are set only by
	// ApplyWellnessFitness ("wellness": the daily Intervals.icu fitness row
	// for LatestLoadDate). When absent, LatestCTL/ATL/Form are the values
	// recorded on the newest activity, with no decay over later rest days.
	LatestLoadDate   string `json:"latestLoadDate,omitempty"`
	LatestLoadSource string `json:"latestLoadSource,omitempty"`
	// NIncluded is D-3's per-section inclusion count (T2.3): AveragePerActivity
	// and PerHour divide by this, not by Scope.CyclingActivities, so an
	// icu_ignore_power/icu_ignore_time exclusion never silently dilutes the
	// average by counting an activity that contributed zero to Total.
	NIncluded int `json:"nIncluded"`
	// ByChannel is T2.5's tri-load breakdown: TrainingLoad is the pre-existing
	// composite total (unchanged, D-6), HR/Power/Pace are the newly-fetched
	// per-channel loads, each with its own NIncluded since a channel is only
	// "included" when the field is actually present on the activity.
	ByChannel CyclingLoadByChannel `json:"byChannel"`
	// Divergence is the mean |hr-power|/max(hr,power) across sessions
	// carrying both channels (0 when no session has both): a large value
	// flags sessions where the HR-based and power-based load estimates
	// disagree, independent of which channel wave-1's Total represents.
	Divergence float64             `json:"divergence,omitempty"`
	Daily      []DailyTrainingLoad `json:"daily"`
}

// CyclingLoadByChannel is T2.5's tri-load breakdown (D-23's precursor:
// wave-3 enforces a capacity bound per channel, this wave only surfaces the
// numbers safely, never conflating a channel's absence with a measured zero).
type CyclingLoadByChannel struct {
	TrainingLoad CyclingLoadChannel `json:"trainingLoad"`
	HR           CyclingLoadChannel `json:"hr"`
	Power        CyclingLoadChannel `json:"power"`
	Pace         CyclingLoadChannel `json:"pace"`
}

// CyclingLoadChannel pairs a channel's summed value with the count of
// sessions that actually carried it, so a 0 total with 0 NIncluded ("never
// reported") never reads the same as a 0 total with a positive NIncluded
// ("measured, genuinely zero").
type CyclingLoadChannel struct {
	Total     int `json:"total"`
	NIncluded int `json:"nIncluded"`
}

type DailyTrainingLoad struct {
	Date string `json:"date"`
	Load int    `json:"load"`
}

type CyclingIntensity struct {
	WeightedAverageIntensity float64            `json:"weightedAverageIntensity"`
	HighIntensityActivities  int                `json:"highIntensityActivities"`
	ZoneSeconds              map[string]int     `json:"zoneSeconds,omitempty"`
	ZonePercent              map[string]float64 `json:"zonePercent,omitempty"`
	HRZoneSeconds            []int              `json:"hrZoneSeconds,omitempty"`
	// NIncluded is D-3's per-section inclusion count (T2.3): counts
	// activities contributing to the power-derived fields above
	// (icu_ignore_power/icu_ignore_time excludes; HRZoneSeconds has its own
	// implicit hr-scope population and is not covered by this count).
	NIncluded int `json:"nIncluded"`
}

type CyclingDurability struct {
	AverageDecoupling float64 `json:"averageDecoupling"`
	// AverageDecouplingAPI / AverageDecouplingStreams average each source
	// separately (B15): the API value of every ride that has one, and the
	// stream value of every measured ride. Emitted only when streams
	// enrichment ran (--decoupling-source streams); nil otherwise.
	AverageDecouplingAPI     *float64 `json:"averageDecouplingApi,omitempty"`
	AverageDecouplingStreams *float64 `json:"averageDecouplingStreams,omitempty"`
	HighDecouplingActivities int      `json:"highDecouplingActivities"`
	// DecouplingSources counts contributing activities by decoupling source
	// (B15): "api" or "streams" (omitted when not enriched).
	DecouplingSources       map[string]int `json:"decouplingSources,omitempty"`
	AverageEfficiencyFactor float64        `json:"averageEfficiencyFactor"`
	AverageVariabilityIndex float64        `json:"averageVariabilityIndex"`
	// NIncluded is D-3's per-section inclusion count (T2.3): decoupling and
	// efficiency both compare power against heart rate, so either
	// icu_ignore_power or icu_ignore_hr (or icu_ignore_time) excludes an
	// activity from this section — only a fully-included activity counts.
	NIncluded int `json:"nIncluded"`
}

type CyclingAnaerobic struct {
	TotalJoulesAboveFTP      int     `json:"totalJoulesAboveFtp"`
	MaxWBalDepletion         int     `json:"maxWbalDepletion"`
	WBalActivities           int     `json:"wbalActivities"`
	WPrimeCapacity           int     `json:"wPrimeCapacity,omitempty"`
	MaxWBalDepletionPercent  float64 `json:"maxWbalDepletionPercent,omitempty"`
	MeanWBalDepletionPercent float64 `json:"meanWbalDepletionPercent,omitempty"`
}

// CyclingFTPDiscriminatorConflicts mirrors D-10b's asymmetric flag-trust
// counter (FTPDiscriminatorConflicts, rebalance.go/plan.go) for the
// analysis/read path's activity partition (T1b.5/R1).
//
// AmbiguousTypeFlagAssertedIndoor counts activities of an ambiguous cycling
// type (D-10 class 3) whose environment flag was explicitly asserted indoor
// (ResolveFTP Source: flag_asserted) — the activity-partition twin of
// FTPDiscriminatorConflicts.EventFlagAssertedIndoor.
//
// It is currently and structurally always 0, unlike its event-side sibling:
// Activity.Trainer is not yet an exposed Go field (rebalance.go's own
// rebalanceFTPDiscriminatorConflicts documents the identical gap; wiring
// Trainer was explicitly deferred past wave-2 and remains out of this
// remediation's declared scope), so every activity-side ResolveFTP call in
// this file passes Trainer: nil and can never classify Source as
// flag_asserted. The counter is wired now, honestly inert, so it starts
// working the day Trainer is exposed instead of silently never having
// existed — the same "mechanism now, disclosed inertness" shape already
// used for icu_ignore_* filtering (CyclingValidity) and wave-4's
// carbsIngested.
type CyclingFTPDiscriminatorConflicts struct {
	AmbiguousTypeFlagAssertedIndoor int `json:"ambiguousTypeFlagAssertedIndoor"`
}

type CyclingSession struct {
	ID                 string  `json:"id,omitempty"`
	Date               string  `json:"date,omitempty"`
	Name               string  `json:"name,omitempty"`
	Type               string  `json:"type,omitempty"`
	MovingTimeSecs     int     `json:"movingTimeSecs"`
	MovingTimeMinutes  float64 `json:"movingTimeMinutes"`
	DistanceMeters     float64 `json:"distanceMeters"`
	DistanceKilometers float64 `json:"distanceKilometers"`
	TrainingLoad       int     `json:"trainingLoad"`
	Intensity          float64 `json:"intensity"`
	WeightedAvgPower   int     `json:"weightedAvgPower,omitempty"`
	AverageHeartRate   int     `json:"averageHeartRate,omitempty"`
	// Decoupling is the consumed value; nil when absent, a real 0.0 is kept.
	Decoupling              *float64   `json:"decoupling,omitempty"`
	DecouplingSource        string     `json:"decouplingSource,omitempty"`
	DecouplingAPI           *float64   `json:"decouplingApi,omitempty"`
	DecouplingStreams       *float64   `json:"decouplingStreams,omitempty"`
	DecouplingStreamsStatus string     `json:"decouplingStreamsStatus,omitempty"`
	DecouplingExperimental  bool       `json:"decouplingExperimental,omitempty"`
	EfficiencyFactor        float64    `json:"efficiencyFactor,omitempty"`
	VariabilityIndex        float64    `json:"variabilityIndex,omitempty"`
	JoulesAboveFTP          int        `json:"joulesAboveFtp,omitempty"`
	MaxWBalDepletion        int        `json:"maxWbalDepletion,omitempty"`
	WPrime                  int        `json:"wPrime,omitempty"`
	WBalDepletionPct        float64    `json:"wbalDepletionPct,omitempty"`
	CriticalPower           int        `json:"criticalPower,omitempty"`
	PMax                    int        `json:"pMax,omitempty"`
	FTP                     int        `json:"ftp,omitempty"`
	RollingFTP              int        `json:"rollingFtp,omitempty"`
	AverageTemp             float64    `json:"averageTemp,omitempty"`
	AverageFeelsLike        float64    `json:"averageFeelsLike,omitempty"`
	HeadwindPercent         float64    `json:"headwindPercent,omitempty"`
	StrainScore             float64    `json:"strainScore,omitempty"`
	ZoneTimes               []ZoneTime `json:"zoneTimes,omitempty"`
	HRZoneTimes             []int      `json:"hrZoneTimes,omitempty"`
	MaxHeartRate            int        `json:"maxHeartRate,omitempty"`
}

type CyclingPerformanceIntelligence struct {
	Repeatability CyclingRepeatability    `json:"repeatability"`
	Durability    CyclingDurabilitySignal `json:"durability"`
	NeuralDensity CyclingNeuralDensity    `json:"neuralDensity"`
	Efficiency    CyclingEfficiencySignal `json:"efficiency"`
}

type CyclingRepeatability struct {
	TotalWorkAboveFTP        int     `json:"totalWorkAboveFtp"`
	MaxWBalDepletion         int     `json:"maxWbalDepletion,omitempty"`
	MeanMaxWBalDepletion     float64 `json:"meanMaxWbalDepletion,omitempty"`
	MaxWBalDepletionPercent  float64 `json:"maxWbalDepletionPercent,omitempty"`
	MeanWBalDepletionPercent float64 `json:"meanWbalDepletionPercent,omitempty"`
	WPrimeCapacity           int     `json:"wPrimeCapacity,omitempty"`
	WorkAboveFTPPerHour      float64 `json:"workAboveFtpPerHour,omitempty"`
	SessionsWithWBalData     int     `json:"sessionsWithWbalData"`
	Pattern                  string  `json:"pattern,omitempty"`
	Classification           string  `json:"classification,omitempty"`
	ClassificationContext    string  `json:"classificationContext,omitempty"`
}

type CyclingDurabilitySignal struct {
	State                 string  `json:"state,omitempty"`
	MeanDecoupling        float64 `json:"meanDecoupling"`
	MaxDecoupling         float64 `json:"maxDecoupling"`
	HighDriftSessions     int     `json:"highDriftSessions"`
	LongEnduranceSessions int     `json:"longEnduranceSessions"`
	ISDMState             string  `json:"isdmState,omitempty"`
	ISDMScore             float64 `json:"isdmScore,omitempty"`
	Classification        string  `json:"classification,omitempty"`
}

type CyclingNeuralDensity struct {
	RollingWorkAboveFTP  int     `json:"rollingWorkAboveFtp"`
	HighIntensityDays    int     `json:"highIntensityDays"`
	MeanIntensity        float64 `json:"meanIntensity"`
	MeanEfficiencyFactor float64 `json:"meanEfficiencyFactor"`
	MeanVariabilityIndex float64 `json:"meanVariabilityIndex"`
	Classification       string  `json:"classification,omitempty"`
}

type CyclingEfficiencySignal struct {
	State                  string  `json:"state,omitempty"`
	MeanEfficiencyFactor   float64 `json:"meanEfficiencyFactor,omitempty"`
	MeanVariabilityIndex   float64 `json:"meanVariabilityIndex,omitempty"`
	SessionsWithEfficiency int     `json:"sessionsWithEfficiency"`
	Classification         string  `json:"classification,omitempty"`
}

type cyclingAnalysisAccumulator struct {
	analysis                       CyclingAnalysis
	dailyLoads                     map[string]int
	intensityWeightedSum           float64
	powerMovingTimeSecs            int
	intensity                      numericAccumulator
	decoupling                     numericAccumulator
	decouplingAPI                  numericAccumulator
	decouplingStreams              numericAccumulator
	decouplingEnriched             bool
	efficiency                     numericAccumulator
	variability                    numericAccumulator
	wbalDepletion                  numericAccumulator
	wbalDepletionPercent           numericAccumulator
	maxWbalDepletionPercent        float64
	loadDivergence                 numericAccumulator
	temp                           numericAccumulator
	weatherTemp                    numericAccumulator
	feelsLike                      numericAccumulator
	windSpeed                      numericAccumulator
	headwind                       numericAccumulator
	tailwind                       numericAccumulator
	altitude                       numericAccumulator
	gradient                       numericAccumulator
	yaw                            numericAccumulator
	strain                         numericAccumulator
	environmentSamples             int
	maxTemp                        float64
	maxWindGust                    float64
	maxAltitude                    float64
	hotSessions                    int
	windAffectedSessions           int
	climbingSessions               int
	maxDecoupling                  float64
	longEndurance                  int
	latestLoadDate                 string
	latestAnchorDate               string
	ftpAmbiguousFlagAssertedIndoor int
}

type numericAccumulator struct {
	sum   float64
	count int
}

func AnalyzeCyclingActivities(activities []Activity, options AnalysisOptions) CyclingAnalysis {
	stubCount := countStubActivities(activities)
	accumulator := newCyclingAnalysisAccumulator(len(activities) - stubCount)
	accumulator.analysis.Validity.Unavailable.StravaStubs = stubCount

	for index := range activities {
		activity := &activities[index]
		if activity.StubShape || !IsCyclingActivityType(activity.Type) {
			continue
		}

		accumulator.add(activity)
	}

	return accumulator.finish(options)
}

// countStubActivities is F-19/D-24's fix at the source: a D-24 STRAVA stub
// carries no activity type, so it never reaches cycling-type filtering, but
// it must still be subtracted from Scope.Activities (the whole-fetch count,
// not the cycling-only one) and reported in Validity.Unavailable.StravaStubs
// — otherwise every consumer dividing by Scope.Activities inherits the
// ~one-third inflation this wave exists to close.
func countStubActivities(activities []Activity) int {
	count := 0

	for index := range activities {
		if activities[index].StubShape {
			count++
		}
	}

	return count
}

func newCyclingAnalysisAccumulator(totalActivities int) cyclingAnalysisAccumulator {
	var accumulator cyclingAnalysisAccumulator

	accumulator.analysis.SchemaVersion = CyclingAnalysisSchemaVersion
	accumulator.analysis.Scope.Activities = totalActivities
	accumulator.analysis.Intensity.ZoneSeconds = map[string]int{}
	accumulator.analysis.Intensity.ZonePercent = map[string]float64{}
	accumulator.dailyLoads = map[string]int{}

	return accumulator
}

// add applies D-2's per-flag scoping (T2.3): icu_ignore_time (or two-or-more
// flags together) drops an activity from every aggregate including volume;
// icu_ignore_power/icu_ignore_hr each drop only their own derived aggregates.
// Durability (decoupling, efficiency, variability index) compares power
// against heart rate, so it requires full inclusion — either flag alone
// disqualifies it. Sessions are raw per-activity rows, not aggregates, and
// are recorded for every non-stub cycling activity regardless of scope.
func (accumulator *cyclingAnalysisAccumulator) add(activity *Activity) {
	validity := ClassifyActivityValidity(activity)
	accumulator.recordValidity(activity, validity)
	accumulator.analysis.Scope.CyclingActivities++

	includeVolume := validity.Scope != ValidityScopeAll
	includePower := validity.Scope != ValidityScopePower && validity.Scope != ValidityScopeAll
	includeHR := validity.Scope != ValidityScopeHR && validity.Scope != ValidityScopeAll
	includeDurability := validity.Class == ActivityValidityIncluded

	if includeVolume {
		accumulator.addVolume(activity)
		accumulator.addPaceLoad(activity)
		accumulator.addEnvironment(activity)
	}

	if includePower {
		accumulator.addLoad(activity)
		accumulator.addPowerAnchors(activity)
		accumulator.addAnaerobic(activity)
		accumulator.addPowerIntensity(activity)
		accumulator.analysis.Load.NIncluded++
		accumulator.analysis.Intensity.NIncluded++
	}

	if includeHR {
		accumulator.addHRIntensity(activity)
		accumulator.addHRLoad(activity)
	}

	if includePower && includeHR {
		accumulator.addLoadDivergence(activity)
	}

	if includeDurability {
		accumulator.addDurability(activity)
		accumulator.analysis.Durability.NIncluded++
	}

	accumulator.addDate(activity, includePower)
	accumulator.addSession(activity)
	accumulator.addFTPDiscriminator(activity)
}

// addFTPDiscriminator counts CyclingFTPDiscriminatorConflicts'
// AmbiguousTypeFlagAssertedIndoor signal (T1b.5/R1). See that type's doc
// comment for why this can never currently increment.
func (accumulator *cyclingAnalysisAccumulator) addFTPDiscriminator(activity *Activity) {
	selection := ResolveFTP(FTPRequest{Type: activity.Type, Entity: FTPEntityActivity})

	if selection.Class == FTPClassAmbiguous && selection.Source == FTPSourceFlagAsserted &&
		selection.Indoor != nil && *selection.Indoor {
		accumulator.ftpAmbiguousFlagAssertedIndoor++
	}
}

// recordValidity is D-3's single source of the validity block's counters:
// no other code path increments Evaluated/Included/Excluded so every
// consumer of CyclingAnalysis reads the same accounting this wave computes.
func (accumulator *cyclingAnalysisAccumulator) recordValidity(activity *Activity, validity ActivityValidity) {
	accumulator.analysis.Validity.Evaluated++

	if validity.Class == ActivityValidityIncluded {
		accumulator.analysis.Validity.Included++

		return
	}

	accumulator.analysis.Validity.Excluded++

	if accumulator.analysis.Validity.ExcludedByScope == nil {
		accumulator.analysis.Validity.ExcludedByScope = map[string]int{}
	}

	accumulator.analysis.Validity.ExcludedByScope[validity.Scope]++
	accumulator.recordExcludedFlags(activity)
	accumulator.analysis.Validity.ExcludedActivityIDs = append(accumulator.analysis.Validity.ExcludedActivityIDs, activity.ID)
}

func (accumulator *cyclingAnalysisAccumulator) recordExcludedFlags(activity *Activity) {
	if accumulator.analysis.Validity.ExcludedByFlag == nil {
		accumulator.analysis.Validity.ExcludedByFlag = map[string]int{}
	}

	if activity.IgnorePower {
		accumulator.analysis.Validity.ExcludedByFlag["icu_ignore_power"]++
	}

	if activity.IgnoreHR {
		accumulator.analysis.Validity.ExcludedByFlag["icu_ignore_hr"]++
	}

	if activity.IgnoreTime {
		accumulator.analysis.Validity.ExcludedByFlag["icu_ignore_time"]++
	}
}

func (accumulator *cyclingAnalysisAccumulator) addVolume(activity *Activity) {
	accumulator.analysis.Volume.MovingTimeSecs += activity.MovingTime
	accumulator.analysis.Volume.DistanceMeters += activity.Distance
	accumulator.analysis.Volume.ElevationGainMeters += activity.TotalElevationGain

	if activity.MovingTime >= longEnduranceSeconds {
		accumulator.longEndurance++
	}
}

func (accumulator *cyclingAnalysisAccumulator) addLoad(activity *Activity) {
	accumulator.analysis.Load.Total += activity.TrainingLoad
	accumulator.analysis.Load.ByChannel.TrainingLoad.Total += activity.TrainingLoad
	accumulator.analysis.Load.ByChannel.TrainingLoad.NIncluded++
	accumulator.powerMovingTimeSecs += activity.MovingTime

	if activity.PowerLoad != nil {
		accumulator.analysis.Load.ByChannel.Power.Total += *activity.PowerLoad
		accumulator.analysis.Load.ByChannel.Power.NIncluded++
	}
}

func (accumulator *cyclingAnalysisAccumulator) addHRLoad(activity *Activity) {
	if activity.HRLoad != nil {
		accumulator.analysis.Load.ByChannel.HR.Total += *activity.HRLoad
		accumulator.analysis.Load.ByChannel.HR.NIncluded++
	}
}

func (accumulator *cyclingAnalysisAccumulator) addPaceLoad(activity *Activity) {
	if activity.PaceLoad != nil {
		accumulator.analysis.Load.ByChannel.Pace.Total += *activity.PaceLoad
		accumulator.analysis.Load.ByChannel.Pace.NIncluded++
	}
}

// addLoadDivergence is T2.5's disagreement signal: only meaningful when a
// session reports both channels, so it is computed independently of
// addLoad/addHRLoad's own per-channel NIncluded counts.
func (accumulator *cyclingAnalysisAccumulator) addLoadDivergence(activity *Activity) {
	if activity.HRLoad == nil || activity.PowerLoad == nil {
		return
	}

	hrLoad := float64(*activity.HRLoad)
	powerLoad := float64(*activity.PowerLoad)
	maxLoad := math.Max(hrLoad, powerLoad)

	if maxLoad == 0 {
		return
	}

	accumulator.loadDivergence.add(math.Abs(hrLoad-powerLoad) / maxLoad)
}

func (accumulator *cyclingAnalysisAccumulator) addPowerIntensity(activity *Activity) {
	accumulator.intensityWeightedSum += activity.Intensity * float64(activity.MovingTime)

	if activity.Intensity != 0 {
		accumulator.intensity.add(activity.Intensity)
	}

	if activity.Intensity >= highIntensityThreshold {
		accumulator.analysis.Intensity.HighIntensityActivities++
	}

	for _, zone := range activity.ZoneTimes {
		accumulator.analysis.Intensity.ZoneSeconds[zone.ID] += zone.Secs
	}
}

// addHRIntensity is HR-derived (D-2): icu_ignore_hr excludes it while
// leaving power-derived intensity fields untouched, the mirror of
// addPowerIntensity.
func (accumulator *cyclingAnalysisAccumulator) addHRIntensity(activity *Activity) {
	mergeHRZoneSeconds(&accumulator.analysis.Intensity.HRZoneSeconds, activity.HRZoneTimes)
}

func (accumulator *cyclingAnalysisAccumulator) addDurability(activity *Activity) {
	if activity.HasDecoupling() {
		accumulator.addDecouplingSource(activity.DecouplingSource)
		accumulator.decoupling.add(activity.Decoupling)
		accumulator.updateMaxDecoupling(activity.Decoupling)
		accumulator.addHighDecoupling(activity.Decoupling)
	}
	if activity.DecouplingStreamsStatus != "" {
		accumulator.decouplingEnriched = true
	}
	if activity.DecouplingAPI != nil {
		accumulator.decouplingAPI.add(*activity.DecouplingAPI)
	} else if activity.HasDecoupling() && activity.DecouplingSource != DecouplingSourceStreams {
		accumulator.decouplingAPI.add(activity.Decoupling)
	}
	if activity.DecouplingStreams != nil {
		accumulator.decouplingStreams.add(*activity.DecouplingStreams)
	}

	if activity.EfficiencyFactor != 0 {
		accumulator.efficiency.add(activity.EfficiencyFactor)
	}

	if activity.VariabilityIndex != 0 {
		accumulator.variability.add(activity.VariabilityIndex)
	}
}

func (accumulator *cyclingAnalysisAccumulator) addDecouplingSource(source string) {
	durability := &accumulator.analysis.Durability
	durability.DecouplingSources = addDecouplingSourceCount(durability.DecouplingSources, source)
}

// addDecouplingSourceCount counts enriched sources only; raw API values
// from library callers that never enriched (source "") are not labeled, so
// existing outputs keep their shape.
func addDecouplingSourceCount(counts map[string]int, source string) map[string]int {
	if source == "" {
		return counts
	}
	if counts == nil {
		counts = map[string]int{}
	}
	counts[source]++

	return counts
}

func (accumulator *cyclingAnalysisAccumulator) updateMaxDecoupling(decoupling float64) {
	absoluteDecoupling := math.Abs(decoupling)

	if absoluteDecoupling > accumulator.maxDecoupling {
		accumulator.maxDecoupling = absoluteDecoupling
	}
}

func (accumulator *cyclingAnalysisAccumulator) addHighDecoupling(decoupling float64) {
	if math.Abs(decoupling) >= highDecouplingPct {
		accumulator.analysis.Durability.HighDecouplingActivities++
	}
}

func (accumulator *cyclingAnalysisAccumulator) addAnaerobic(activity *Activity) {
	accumulator.analysis.Anaerobic.TotalJoulesAboveFTP += activity.JoulesAboveFTP

	if activity.MaxWbalDepletion > 0 {
		accumulator.analysis.Anaerobic.WBalActivities++
		accumulator.wbalDepletion.add(float64(activity.MaxWbalDepletion))
	}

	if activity.MaxWbalDepletion > accumulator.analysis.Anaerobic.MaxWBalDepletion {
		accumulator.analysis.Anaerobic.MaxWBalDepletion = activity.MaxWbalDepletion
	}

	if activity.WPrime > 0 && activity.MaxWbalDepletion > 0 {
		depletionPercent := round2(float64(activity.MaxWbalDepletion) * percentScale / float64(activity.WPrime))
		accumulator.wbalDepletionPercent.add(depletionPercent)

		if depletionPercent > accumulator.maxWbalDepletionPercent {
			accumulator.maxWbalDepletionPercent = depletionPercent
		}
	}
}

func (accumulator *cyclingAnalysisAccumulator) addPowerAnchors(activity *Activity) {
	if !activityHasPowerAnchors(activity) {
		return
	}

	date := activityDate(activity)
	if date != "" && date < accumulator.latestAnchorDate {
		return
	}

	accumulator.latestAnchorDate = date
	accumulator.analysis.PowerAnchors.FTP = activity.FTP
	accumulator.analysis.PowerAnchors.CriticalPower = activity.CriticalPower
	accumulator.analysis.PowerAnchors.WPrime = activity.WPrime
	accumulator.analysis.PowerAnchors.PMax = activity.PMax
	accumulator.analysis.PowerAnchors.RollingFTP = activity.RollingFTP
	accumulator.analysis.PowerAnchors.Source = "latest_activity_model_fields"
}

func activityHasPowerAnchors(activity *Activity) bool {
	return activity.FTP != 0 || activity.CriticalPower != 0 || activity.WPrime != 0 ||
		activity.PMax != 0 || activity.RollingFTP != 0
}

func (accumulator *cyclingAnalysisAccumulator) addEnvironment(activity *Activity) {
	if !activityHasEnvironment(activity) {
		return
	}

	accumulator.environmentSamples++
	accumulator.temp.addIfNonZero(activity.AverageTemp)
	accumulator.weatherTemp.addIfNonZero(activity.AverageWeatherTemp)
	accumulator.feelsLike.addIfNonZero(activity.AverageFeelsLike)
	accumulator.windSpeed.addIfNonZero(activity.AverageWindSpeed)
	accumulator.headwind.addIfNonZero(activity.HeadwindPercent)
	accumulator.tailwind.addIfNonZero(activity.TailwindPercent)
	accumulator.altitude.addIfNonZero(activity.AverageAltitude)
	accumulator.gradient.addIfNonZero(activity.AverageGradient)
	accumulator.yaw.addIfNonZero(activity.AverageYaw)
	accumulator.strain.addIfNonZero(activity.StrainScore)
	accumulator.maxTemp = maxFloat(accumulator.maxTemp, activity.MaxTemp)
	accumulator.maxWindGust = maxFloat(accumulator.maxWindGust, activity.AverageWindGust)
	accumulator.maxAltitude = maxFloat(accumulator.maxAltitude, activity.MaxAltitude)

	if activity.AverageTemp >= hotTempThreshold || activity.AverageFeelsLike >= hotFeelsLikeThreshold {
		accumulator.hotSessions++
	}

	if activity.AverageWindSpeed >= windSpeedThreshold || activity.HeadwindPercent >= headwindThreshold {
		accumulator.windAffectedSessions++
	}

	if activity.AverageGradient >= climbingGradientPct {
		accumulator.climbingSessions++
	}
}

func activityHasEnvironment(activity *Activity) bool {
	return activity.AverageTemp != 0 || activity.AverageWeatherTemp != 0 || activity.AverageFeelsLike != 0 ||
		activity.AverageWindSpeed != 0 || activity.HeadwindPercent != 0 || activity.AverageAltitude != 0 ||
		activity.AverageGradient != 0 || activity.StrainScore != 0
}

// addDate always tracks the scope's date range (informational, not an
// aggregate a flag can distort), but the daily-load bucket that feeds
// Acute7/Chronic28/Monotony/Strain is TrainingLoad-based (power-derived), so
// it only accumulates when includeLoadDay (icu_ignore_power/icu_ignore_time
// did not exclude this activity from the load channel).
func (accumulator *cyclingAnalysisAccumulator) addDate(activity *Activity, includeLoadDay bool) {
	date := activityDate(activity)
	if date == "" {
		return
	}

	if includeLoadDay {
		accumulator.dailyLoads[date] += activity.TrainingLoad
	}

	accumulator.analysis.Scope.StartDate = minDateString(accumulator.analysis.Scope.StartDate, date)
	accumulator.analysis.Scope.EndDate = maxDateString(accumulator.analysis.Scope.EndDate, date)
	accumulator.updateLatestLoad(activity, date)
}

func (accumulator *cyclingAnalysisAccumulator) addSession(activity *Activity) {
	accumulator.analysis.Sessions = append(accumulator.analysis.Sessions, cyclingSessionFromActivity(activity))
}

func (accumulator *cyclingAnalysisAccumulator) updateLatestLoad(activity *Activity, date string) {
	if date < accumulator.latestLoadDate {
		return
	}

	// Skip activities that omit CTL/ATL (both zero). A later session without
	// fitness-model fields must not wipe a previously recorded form state.
	if activity.CTL == 0 && activity.ATL == 0 {
		if accumulator.latestLoadDate == "" {
			accumulator.latestLoadDate = date
		}
		return
	}

	accumulator.latestLoadDate = date
	accumulator.analysis.Load.LatestCTL = activity.CTL
	accumulator.analysis.Load.LatestATL = activity.ATL
	accumulator.analysis.Load.LatestForm = round2(activity.CTL - activity.ATL)
}

const (
	latestLoadSourceActivity = "last_activity"
	latestLoadSourceWellness = "wellness"
)

// ApplyWellnessFitness replaces activity-derived LatestCTL/ATL/Form with the
// newest daily wellness fitness row dated on or before endDate. Activity
// values are frozen at the last workout and ignore decay on later rest days
// and load from other sports, so the daily row is the current state. Rows
// after endDate (Intervals.icu projects future CTL/ATL) and rows without
// CTL/ATL are ignored; when none qualify the load is left unchanged.
func ApplyWellnessFitness(load *CyclingLoad, records []Wellness, endDate string) {
	best := -1
	for index := range records {
		record := &records[index]
		if record.ID == "" || (endDate != "" && record.ID > endDate) {
			continue
		}
		if record.CTL == 0 && record.ATL == 0 {
			continue
		}
		if best < 0 || record.ID > records[best].ID {
			best = index
		}
	}
	if best < 0 {
		return
	}

	record := &records[best]
	load.LatestCTL = record.CTL
	load.LatestATL = record.ATL
	load.LatestForm = round2(record.CTL - record.ATL)
	load.LatestLoadDate = record.ID
	load.LatestLoadSource = latestLoadSourceWellness
}

func (accumulator *cyclingAnalysisAccumulator) finish(options AnalysisOptions) CyclingAnalysis {
	accumulator.analysis.Scope.Timezone = options.Timezone
	accumulator.analysis.Scope.TimezoneSource = options.TimezoneSource
	accumulator.finishValidity()
	accumulator.finishVolume()

	if accumulator.analysis.Scope.CyclingActivities == 0 {
		accumulator.analysis.Warnings = append(accumulator.analysis.Warnings, "no cycling activities found")

		return accumulator.analysis
	}

	accumulator.finishLoad(options)
	accumulator.finishIntensity()
	accumulator.finishDurability()
	accumulator.finishAnaerobic()
	accumulator.finishEnvironment()
	accumulator.finishState()
	accumulator.finishPerformance()
	accumulator.finishFTPDiscriminatorConflicts()
	accumulator.finishSectionRefusalWarnings(options)

	return accumulator.analysis
}

// finishFTPDiscriminatorConflicts sets CyclingAnalysis.FTPDiscriminatorConflicts
// only when there is a conflict to report (D-3/D-5: absent, not a false
// zero) — currently never, per CyclingFTPDiscriminatorConflicts' doc comment.
func (accumulator *cyclingAnalysisAccumulator) finishFTPDiscriminatorConflicts() {
	if accumulator.ftpAmbiguousFlagAssertedIndoor == 0 {
		return
	}

	accumulator.analysis.FTPDiscriminatorConflicts = &CyclingFTPDiscriminatorConflicts{
		AmbiguousTypeFlagAssertedIndoor: accumulator.ftpAmbiguousFlagAssertedIndoor,
	}
}

// finishValidity closes out D-3's validity block (T2.3): computed here
// (rather than incrementally) because ExclusionRate needs the final
// Evaluated/Excluded totals and the static fields (Source, ZeroSemantics,
// FieldsGuaranteed) do not depend on any per-activity accumulation.
func (accumulator *cyclingAnalysisAccumulator) finishValidity() {
	validity := &accumulator.analysis.Validity
	validity.Source = "activity_icu_ignore_flags_and_strava_stub_shape"
	validity.FieldsGuaranteed = true
	validity.ZeroSemantics = cyclingValidityZeroSemantics

	if validity.Evaluated > 0 {
		validity.ExclusionRate = round2(float64(validity.Excluded) / float64(validity.Evaluated))
	}
}

// finishSectionRefusalWarnings is D-4's structural refusal: rather than
// restructuring Load/Intensity/Durability into pointers to support JSON-key
// omission (D-6 declines restructuring existing output shapes), a section
// whose NIncluded is 0 keeps its already-zero-valued fields and gets a named
// warning explaining why, so a caller never mistakes "no included activity"
// for "measured zero." --min-included-activities is a separate, optional
// threshold (no default) that never blocks output, only annotates it.
func (accumulator *cyclingAnalysisAccumulator) finishSectionRefusalWarnings(options AnalysisOptions) {
	analysis := &accumulator.analysis

	if analysis.Load.NIncluded == 0 {
		analysis.Warnings = appendUniqueWarning(analysis.Warnings,
			"load section refused: 0 included activities (all excluded by icu_ignore_power/icu_ignore_time)")
	}

	if analysis.Intensity.NIncluded == 0 {
		analysis.Warnings = appendUniqueWarning(analysis.Warnings,
			"intensity section refused: 0 included activities (all excluded by icu_ignore_power/icu_ignore_time)")
	}

	if analysis.Durability.NIncluded == 0 {
		analysis.Warnings = appendUniqueWarning(analysis.Warnings,
			"durability section refused: 0 included activities (all excluded by icu_ignore_power/icu_ignore_hr/icu_ignore_time)")
	}

	if options.MinIncludedActivities > 0 && analysis.Validity.Included < options.MinIncludedActivities {
		analysis.Warnings = appendUniqueWarning(analysis.Warnings, fmt.Sprintf(
			"included activity count %d is below the configured minimum %d (--min-included-activities)",
			analysis.Validity.Included, options.MinIncludedActivities,
		))
	}
}

func (accumulator *cyclingAnalysisAccumulator) finishAnaerobic() {
	accumulator.analysis.Anaerobic.WPrimeCapacity = accumulator.analysis.PowerAnchors.WPrime
	accumulator.analysis.Anaerobic.MaxWBalDepletionPercent = round2(accumulator.maxWbalDepletionPercent)
	accumulator.analysis.Anaerobic.MeanWBalDepletionPercent = round2(accumulator.wbalDepletionPercent.average())
}

func (accumulator *cyclingAnalysisAccumulator) finishEnvironment() {
	if accumulator.environmentSamples == 0 {
		return
	}

	accumulator.analysis.Environment = CyclingEnvironmentContext{
		Samples:              accumulator.environmentSamples,
		AverageTemp:          round2(accumulator.temp.average()),
		MaxTemp:              round2(accumulator.maxTemp),
		AverageWeatherTemp:   round2(accumulator.weatherTemp.average()),
		AverageFeelsLike:     round2(accumulator.feelsLike.average()),
		AverageWindSpeed:     round2(accumulator.windSpeed.average()),
		MaxWindGust:          round2(accumulator.maxWindGust),
		HeadwindPercent:      round2(accumulator.headwind.average()),
		TailwindPercent:      round2(accumulator.tailwind.average()),
		AverageAltitude:      round2(accumulator.altitude.average()),
		MaxAltitude:          round2(accumulator.maxAltitude),
		AverageGradient:      round2(accumulator.gradient.average()),
		AverageYaw:           round2(accumulator.yaw.average()),
		AverageStrainScore:   round2(accumulator.strain.average()),
		HotSessions:          accumulator.hotSessions,
		WindAffectedSessions: accumulator.windAffectedSessions,
		ClimbingSessions:     accumulator.climbingSessions,
		Source:               "activity_environment_fields",
	}
}

func (accumulator *cyclingAnalysisAccumulator) finishVolume() {
	accumulator.analysis.Volume.MovingTimeHours = round2(float64(accumulator.analysis.Volume.MovingTimeSecs) / secondsPerHour)
	accumulator.analysis.Volume.DistanceKilometers = round2(accumulator.analysis.Volume.DistanceMeters / metersPerKilometer)
}

func (accumulator *cyclingAnalysisAccumulator) finishLoad(options AnalysisOptions) {
	// D-4/T2.3: divide by NIncluded, not Scope.CyclingActivities — an
	// icu_ignore_power/icu_ignore_time exclusion contributes 0 to Total but
	// must not also dilute the average by inflating the denominator.
	if accumulator.analysis.Load.NIncluded > 0 {
		averagePerActivity := float64(accumulator.analysis.Load.Total) / float64(accumulator.analysis.Load.NIncluded)
		accumulator.analysis.Load.AveragePerActivity = round2(averagePerActivity)
	}

	accumulator.analysis.Load.Daily = buildDailyTrainingLoads(accumulator.dailyLoads, options)
	accumulator.analysis.Load.Acute7 = sumLastLoads(accumulator.analysis.Load.Daily, acuteLoadDays)
	accumulator.analysis.Load.Chronic28 = sumLastLoads(accumulator.analysis.Load.Daily, chronicLoadDays)
	accumulator.analysis.Load.AcuteChronicWorkRatio = acuteChronicWorkRatio(
		accumulator.analysis.Load.Acute7,
		accumulator.analysis.Load.Chronic28,
	)
	accumulator.analysis.Load.Monotony = round2(trainingMonotony(accumulator.analysis.Load.Daily))
	accumulator.analysis.Load.Strain = round2(float64(accumulator.analysis.Load.Total) * accumulator.analysis.Load.Monotony)
	accumulator.analysis.Load.Divergence = round2(accumulator.loadDivergence.average())
}

func (accumulator *cyclingAnalysisAccumulator) finishIntensity() {
	// D-4/T2.3: the power-included population's own moving time is the
	// correct denominator for load-per-hour and weighted intensity — using
	// Volume.MovingTimeSecs (which includes icu_ignore_power-excluded
	// activities' time, since volume retains them) would dilute both.
	if accumulator.powerMovingTimeSecs > 0 {
		movingHours := float64(accumulator.powerMovingTimeSecs) / secondsPerHour
		loadPerHour := float64(accumulator.analysis.Load.Total) / movingHours
		weightedIntensity := accumulator.intensityWeightedSum / float64(accumulator.powerMovingTimeSecs)

		accumulator.analysis.Load.PerHour = round2(loadPerHour)
		accumulator.analysis.Intensity.WeightedAverageIntensity = round3(weightedIntensity)
	}

	accumulator.analysis.Intensity.ZonePercent = zonePercentages(accumulator.analysis.Intensity.ZoneSeconds)
}

func (accumulator *cyclingAnalysisAccumulator) finishDurability() {
	accumulator.analysis.Durability.AverageDecoupling = round2(accumulator.decoupling.average())
	// Only with stream enrichment: in API mode averageDecoupling already is
	// the API average, and the default output shape is unchanged.
	if accumulator.decouplingEnriched && accumulator.decouplingAPI.count > 0 {
		accumulator.analysis.Durability.AverageDecouplingAPI = new(round2(accumulator.decouplingAPI.average()))
	}
	if accumulator.decouplingStreams.count > 0 {
		accumulator.analysis.Durability.AverageDecouplingStreams = new(round2(accumulator.decouplingStreams.average()))
	}
	accumulator.analysis.Durability.AverageEfficiencyFactor = round2(accumulator.efficiency.average())
	accumulator.analysis.Durability.AverageVariabilityIndex = round2(accumulator.variability.average())
}

func (accumulator *cyclingAnalysisAccumulator) finishState() {
	if accumulator.analysis.Load.LatestCTL == 0 && accumulator.analysis.Load.LatestATL == 0 {
		return
	}

	loadPressure := round2(accumulator.analysis.Load.LatestATL - accumulator.analysis.Load.LatestCTL)
	accumulator.analysis.State.LoadPressure = loadPressure
	accumulator.analysis.State.Source = "local_ctl_atl_heuristic"

	if loadPressure > 0 {
		accumulator.analysis.State.StateLabel = stateLabelLoadPressure
		accumulator.analysis.State.OperationalState = stateOperationalRecovery
		accumulator.analysis.State.LoadRecoveryState = stateLoadPressure
		accumulator.analysis.State.Directive = "prioritise aerobic work and recovery spacing"

		return
	}

	accumulator.analysis.State.StateLabel = "Load Accepting"
	accumulator.analysis.State.OperationalState = "load_accepting"
	accumulator.analysis.State.LoadRecoveryState = "load_accepting"
	accumulator.analysis.State.Directive = "continue planned training with normal recovery checks"
}

func (accumulator *cyclingAnalysisAccumulator) finishPerformance() {
	accumulator.analysis.Performance = CyclingPerformanceIntelligence{
		Repeatability: accumulator.repeatabilitySignal(),
		Durability:    accumulator.durabilitySignal(),
		NeuralDensity: accumulator.neuralDensitySignal(),
		Efficiency:    accumulator.efficiencySignal(),
	}
}

func (accumulator *cyclingAnalysisAccumulator) repeatabilitySignal() CyclingRepeatability {
	return CyclingRepeatability{
		TotalWorkAboveFTP:        accumulator.analysis.Anaerobic.TotalJoulesAboveFTP,
		MaxWBalDepletion:         accumulator.analysis.Anaerobic.MaxWBalDepletion,
		MeanMaxWBalDepletion:     round2(accumulator.wbalDepletion.average()),
		MaxWBalDepletionPercent:  accumulator.analysis.Anaerobic.MaxWBalDepletionPercent,
		MeanWBalDepletionPercent: accumulator.analysis.Anaerobic.MeanWBalDepletionPercent,
		WPrimeCapacity:           accumulator.analysis.Anaerobic.WPrimeCapacity,
		WorkAboveFTPPerHour:      accumulator.workAboveFTPPerHour(),
		SessionsWithWBalData:     accumulator.analysis.Anaerobic.WBalActivities,
		Pattern:                  wPrimePattern(accumulator.maxWbalDepletionPercent),
		Classification:           "informational",
		ClassificationContext:    "local_activity_fields",
	}
}

func (accumulator *cyclingAnalysisAccumulator) durabilitySignal() CyclingDurabilitySignal {
	return CyclingDurabilitySignal{
		State:                 durabilityState(accumulator.maxDecoupling),
		MeanDecoupling:        accumulator.analysis.Durability.APIAverageDecoupling(),
		MaxDecoupling:         round2(accumulator.maxDecoupling),
		HighDriftSessions:     accumulator.analysis.Durability.HighDecouplingActivities,
		LongEnduranceSessions: accumulator.longEndurance,
		ISDMState:             isdmState(accumulator.maxDecoupling, accumulator.longEndurance),
		ISDMScore:             isdmScore(accumulator.maxDecoupling, accumulator.analysis.Durability.HighDecouplingActivities),
		Classification:        "local_heuristic",
	}
}

func (accumulator *cyclingAnalysisAccumulator) neuralDensitySignal() CyclingNeuralDensity {
	return CyclingNeuralDensity{
		RollingWorkAboveFTP:  accumulator.analysis.Anaerobic.TotalJoulesAboveFTP,
		HighIntensityDays:    accumulator.analysis.Intensity.HighIntensityActivities,
		MeanIntensity:        round3(accumulator.intensity.average()),
		MeanEfficiencyFactor: accumulator.analysis.Durability.AverageEfficiencyFactor,
		MeanVariabilityIndex: accumulator.analysis.Durability.AverageVariabilityIndex,
		Classification:       "local_heuristic",
	}
}

func (accumulator *cyclingAnalysisAccumulator) efficiencySignal() CyclingEfficiencySignal {
	meanEfficiency := accumulator.analysis.Durability.AverageEfficiencyFactor
	meanVariability := accumulator.analysis.Durability.AverageVariabilityIndex

	return CyclingEfficiencySignal{
		State:                  efficiencyState(meanEfficiency, meanVariability),
		MeanEfficiencyFactor:   meanEfficiency,
		MeanVariabilityIndex:   meanVariability,
		SessionsWithEfficiency: accumulator.efficiency.count,
		Classification:         "local_efficiency_heuristic",
	}
}

func (accumulator *cyclingAnalysisAccumulator) workAboveFTPPerHour() float64 {
	if accumulator.analysis.Volume.MovingTimeSecs == 0 {
		return 0
	}

	movingHours := float64(accumulator.analysis.Volume.MovingTimeSecs) / secondsPerHour

	return round2(float64(accumulator.analysis.Anaerobic.TotalJoulesAboveFTP) / movingHours)
}

func wPrimePattern(maxDepletionPercent float64) string {
	if maxDepletionPercent == 0 {
		return ""
	}

	if maxDepletionPercent >= wPrimeRepeatablePct {
		return "repeatable_w_prime_depletion"
	}

	if maxDepletionPercent >= wPrimeModeratePct {
		return "moderate_w_prime_depletion"
	}

	return "low_w_prime_depletion"
}

func isdmState(maxDecoupling float64, longEnduranceSessions int) string {
	if maxDecoupling == 0 && longEnduranceSessions == 0 {
		return unknownState
	}

	if maxDecoupling >= highDecouplingPct {
		return "durability_limiter"
	}

	if longEnduranceSessions > 0 {
		return "durability_supported"
	}

	return "durability_unproven"
}

func isdmScore(maxDecoupling float64, highDriftSessions int) float64 {
	if maxDecoupling == 0 && highDriftSessions == 0 {
		return 0
	}

	score := isdmBaseScore - maxDecoupling*isdmDecouplingPenalty -
		float64(highDriftSessions*isdmHighDriftPenalty)

	if score < 0 {
		return 0
	}

	return round2(score)
}

func efficiencyState(meanEfficiency, meanVariability float64) string {
	if meanEfficiency == 0 {
		return unknownState
	}

	if meanEfficiency < lowEfficiencyThreshold {
		return "low_efficiency"
	}

	if meanEfficiency >= highEfficiencyThreshold && meanVariability >= lowEfficiencyThreshold {
		return "efficient_variable"
	}

	if meanEfficiency >= highEfficiencyThreshold {
		return "efficient_stable"
	}

	return "steady"
}

func durabilityState(maxDecoupling float64) string {
	if maxDecoupling == 0 {
		return unknownState
	}

	if maxDecoupling >= highDecouplingPct {
		return stateDurabilityWatch
	}

	return "stable"
}

func (accumulator *numericAccumulator) add(value float64) {
	accumulator.sum += value
	accumulator.count++
}

func (accumulator *numericAccumulator) addIfNonZero(value float64) {
	if value == 0 {
		return
	}

	accumulator.add(value)
}

func (accumulator *numericAccumulator) average() float64 {
	if accumulator.count == 0 {
		return 0
	}

	return accumulator.sum / float64(accumulator.count)
}

func IsCyclingActivityType(activityType string) bool {
	normalized := strings.ToLower(activityType)

	switch normalized {
	case "ride", "virtualride", "ebikeride", "emountainbikeride", "gravelride", "trackride",
		"mountainbikeride", "cyclocross", "handcycle", "velomobile":
		return true
	default:
		return false
	}
}

func acuteChronicWorkRatio(acuteLoad, chronicLoad int) float64 {
	if chronicLoad == 0 {
		return 0
	}

	acuteDaily := float64(acuteLoad) / acuteLoadDays
	chronicDaily := float64(chronicLoad) / chronicLoadDays

	return round2(acuteDaily / chronicDaily)
}

func activityDate(activity *Activity) string {
	if len(activity.StartDateLocal) >= len(analysisDateLayout) {
		return activity.StartDateLocal[:len(analysisDateLayout)]
	}

	if len(activity.StartDate) >= len(analysisDateLayout) {
		return activity.StartDate[:len(analysisDateLayout)]
	}

	return ""
}

func cyclingSessionFromActivity(activity *Activity) CyclingSession {
	wbalDepletionPct := float64(0)
	if activity.WPrime > 0 && activity.MaxWbalDepletion > 0 {
		wbalDepletionPct = round2(float64(activity.MaxWbalDepletion) * percentScale / float64(activity.WPrime))
	}

	return CyclingSession{
		ID:                      activity.ID,
		Date:                    activityDate(activity),
		Name:                    activity.Name,
		Type:                    activity.Type,
		MovingTimeSecs:          activity.MovingTime,
		MovingTimeMinutes:       round2(float64(activity.MovingTime) / secondsPerMinute),
		DistanceMeters:          activity.Distance,
		DistanceKilometers:      round2(activity.Distance / metersPerKilometer),
		TrainingLoad:            activity.TrainingLoad,
		Intensity:               activity.Intensity,
		WeightedAvgPower:        activity.WeightedAvgPower,
		AverageHeartRate:        activity.AverageHeartRate,
		Decoupling:              sessionDecoupling(activity),
		DecouplingSource:        activity.DecouplingSource,
		DecouplingAPI:           activity.DecouplingAPI,
		DecouplingStreams:       activity.DecouplingStreams,
		DecouplingStreamsStatus: activity.DecouplingStreamsStatus,
		DecouplingExperimental:  activity.DecouplingExperimental,
		EfficiencyFactor:        activity.EfficiencyFactor,
		VariabilityIndex:        activity.VariabilityIndex,
		JoulesAboveFTP:          activity.JoulesAboveFTP,
		MaxWBalDepletion:        activity.MaxWbalDepletion,
		WPrime:                  activity.WPrime,
		WBalDepletionPct:        wbalDepletionPct,
		CriticalPower:           activity.CriticalPower,
		PMax:                    activity.PMax,
		FTP:                     activity.FTP,
		RollingFTP:              activity.RollingFTP,
		AverageTemp:             activity.AverageTemp,
		AverageFeelsLike:        activity.AverageFeelsLike,
		HeadwindPercent:         activity.HeadwindPercent,
		StrainScore:             activity.StrainScore,
		ZoneTimes:               activity.ZoneTimes,
		HRZoneTimes:             activity.HRZoneTimes,
		MaxHeartRate:            activity.MaxHeartRate,
	}
}

func maxFloat(current, candidate float64) float64 {
	if candidate > current {
		return candidate
	}

	return current
}

func minDateString(current, candidate string) string {
	if current == "" || candidate < current {
		return candidate
	}

	return current
}

func maxDateString(current, candidate string) string {
	if current == "" || candidate > current {
		return candidate
	}

	return current
}

func mergeHRZoneSeconds(total *[]int, next []int) {
	if len(next) == 0 {
		return
	}

	for len(*total) < len(next) {
		*total = append(*total, 0)
	}

	for index, seconds := range next {
		(*total)[index] += seconds
	}
}

func buildDailyTrainingLoads(loads map[string]int, options AnalysisOptions) []DailyTrainingLoad {
	start := options.StartDate
	end := options.EndDate

	if start == "" || end == "" {
		return dailyTrainingLoadsFromMap(loads)
	}

	startDate, startErr := time.Parse(analysisDateLayout, start)
	endDate, endErr := time.Parse(analysisDateLayout, end)
	invalidRange := startErr != nil || endErr != nil || endDate.Before(startDate)

	if invalidRange {
		return dailyTrainingLoadsFromMap(loads)
	}

	var days []DailyTrainingLoad

	for current := startDate; !current.After(endDate); current = current.AddDate(0, 0, 1) {
		key := current.Format(analysisDateLayout)
		days = append(days, DailyTrainingLoad{Date: key, Load: loads[key]})
	}

	return days
}

func dailyTrainingLoadsFromMap(loads map[string]int) []DailyTrainingLoad {
	if len(loads) == 0 {
		return nil
	}

	dates := make([]string, 0, len(loads))
	for date := range loads {
		dates = append(dates, date)
	}
	sort.Strings(dates)

	startDate, startErr := time.Parse(analysisDateLayout, dates[0])
	endDate, endErr := time.Parse(analysisDateLayout, dates[len(dates)-1])
	// Fall back to sparse ride-only series when keys are not calendar dates.
	if startErr != nil || endErr != nil || endDate.Before(startDate) {
		days := make([]DailyTrainingLoad, 0, len(dates))
		for _, date := range dates {
			days = append(days, DailyTrainingLoad{Date: date, Load: loads[date]})
		}
		return days
	}

	// Expand min→max inclusive so rest days contribute zero load for monotony,
	// strain, and acute/chronic windows (Foster-style consecutive days).
	var days []DailyTrainingLoad
	for current := startDate; !current.After(endDate); current = current.AddDate(0, 0, 1) {
		key := current.Format(analysisDateLayout)
		days = append(days, DailyTrainingLoad{Date: key, Load: loads[key]})
	}

	return days
}

func sumLastLoads(days []DailyTrainingLoad, count int) int {
	if len(days) == 0 {
		return 0
	}

	start := len(days) - count

	if start < 0 {
		start = 0
	}

	var total int

	for _, day := range days[start:] {
		total += day.Load
	}

	return total
}

func trainingMonotony(days []DailyTrainingLoad) float64 {
	if len(days) == 0 {
		return 0
	}

	var mean float64

	for _, day := range days {
		mean += float64(day.Load)
	}

	mean /= float64(len(days))

	var variance float64

	for _, day := range days {
		delta := float64(day.Load) - mean
		variance += delta * delta
	}

	standardDeviation := math.Sqrt(variance / float64(len(days)))

	if standardDeviation == 0 {
		return 0
	}

	return mean / standardDeviation
}

func zonePercentages(zoneSeconds map[string]int) map[string]float64 {
	var total int

	for _, seconds := range zoneSeconds {
		total += seconds
	}

	if total == 0 {
		return map[string]float64{}
	}

	percentages := map[string]float64{}

	for zone, seconds := range zoneSeconds {
		percentages[zone] = round2(float64(seconds) * percentScale / float64(total))
	}

	return percentages
}

func round2(value float64) float64 {
	return math.Round(value*percentScale) / percentScale
}

func round3(value float64) float64 {
	return math.Round(value*thousandScale) / thousandScale
}

// sessionDecoupling returns the consumed decoupling, nil when absent.
func sessionDecoupling(activity *Activity) *float64 {
	if !activity.HasDecoupling() {
		return nil
	}
	value := activity.Decoupling

	return &value
}

// APIAverageDecoupling is the intervals.icu-API average decoupling: the
// separate API average in streams mode, otherwise AverageDecoupling (which is
// the API average when not enriched). Downstream signals use it so a mixed
// API/streams average never drives state.
func (durability CyclingDurability) APIAverageDecoupling() float64 {
	if durability.AverageDecouplingAPI != nil {
		return *durability.AverageDecouplingAPI
	}

	return durability.AverageDecoupling
}
