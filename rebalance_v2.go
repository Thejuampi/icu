package icu

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"math/big"
	"sort"
	"strconv"
	"strings"
)

const (
	RebalanceStrategyAdaptiveBidirectional = "adaptive-bidirectional"

	RebalanceModePreserveStructure = "preserve-structure"
	RebalanceModePreserveSchedule  = "preserve-schedule"
	RebalanceModeRedistribute      = "redistribute"

	RebalanceCurveMethodPCHIP = "pchip_monotone_c1"
	RebalancePolicySource     = "adaptive_bidirectional_level_envelope"
	rebalancePercentScale     = 100
)

// RebalanceWeeklyLoadSeries aggregates cycling activities before the scope week
// into per-week training-load totals, ordered oldest to newest. The scope week
// itself is excluded so the series describes prior regimes, not the week under
// rebalance. It is a pure function over its inputs.
func RebalanceWeeklyLoadSeries(activities []Activity, scopeStart string) []float64 {
	loads := map[string]float64{}
	scopeWeek := isoWeekKeyFromDate(scopeStart)
	for index := range activities {
		activity := &activities[index]
		if !IsCyclingActivityType(activity.Type) || activity.TrainingLoad <= 0 {
			continue
		}
		key := isoWeekKeyFromDate(activityDate(activity))
		if key == "" || key >= scopeWeek {
			continue
		}
		loads[key] += float64(activity.TrainingLoad)
	}
	keys := make([]string, 0, len(loads))
	for key := range loads {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]float64, 0, len(keys))
	for _, key := range keys {
		out = append(out, loads[key])
	}
	return out
}

type RebalancePolicy struct {
	Strategy    string `json:"strategy,omitempty"`
	Level       string `json:"level,omitempty"`
	Mode        string `json:"mode,omitempty"`
	CurveMethod string `json:"curveMethod,omitempty"`
	Source      string `json:"source,omitempty"`
}

type RebalanceHistoryReport struct {
	RangeStart        string  `json:"rangeStart,omitempty"`
	RangeEnd          string  `json:"rangeEnd,omitempty"`
	RegimeStartIndex  int     `json:"regimeStartIndex"`
	RegimeEndIndex    int     `json:"regimeEndIndex"`
	ChangePoints      []int   `json:"changePoints,omitempty"`
	RecentDisturbance bool    `json:"recentDisturbance"`
	Coverage          float64 `json:"coverage"`
	Confidence        float64 `json:"confidence"`
	Source            string  `json:"source,omitempty"`
}

type RebalanceProjectionReport struct {
	ExactTotalLoad   string                    `json:"exactTotalLoad,omitempty"`
	AppliedTotalLoad int                       `json:"appliedTotalLoad"`
	Residual         string                    `json:"residual,omitempty"`
	MaxSessionError  string                    `json:"maxSessionError,omitempty"`
	Steps            []RebalanceProjectionStep `json:"steps,omitempty"`
	Errors           []string                  `json:"errors,omitempty"`
}

type RebalanceProjectionStep struct {
	ID             string       `json:"id"`
	ExactLoad      string       `json:"exactLoad,omitempty"`
	ExactLoadRat   RebalanceRat `json:"-"`
	AppliedLoad    int          `json:"appliedLoad"`
	ExactIF        string       `json:"exactIf,omitempty"`
	AppliedIF      string       `json:"appliedIf,omitempty"`
	ExactSeconds   string       `json:"exactSeconds,omitempty"`
	AppliedSeconds int          `json:"appliedSeconds"`
	Residual       string       `json:"residual,omitempty"`
	Blocked        bool         `json:"blocked,omitempty"`
	Reason         string       `json:"reason,omitempty"`
}

// HasErrors reports whether any projection step was blocked or carries a
// non-zero projection error.
func (p *RebalanceProjectionReport) HasErrors() bool {
	if len(p.Errors) > 0 {
		return true
	}
	for index := range p.Steps {
		if p.Steps[index].Blocked {
			return true
		}
	}
	return false
}

type RebalanceApprove struct {
	Reason         string `json:"reason,omitempty"`
	ProvidedLimits bool   `json:"providedLimits"`
	TargetLoad     int    `json:"targetLoad,omitempty"`
	Level          string `json:"level,omitempty"`
	Mode           string `json:"mode,omitempty"`
	RecalcHash     string `json:"recalcHash,omitempty"`
	LimitsHash     string `json:"limitsHash,omitempty"`
	Verified       bool   `json:"verified"`
}

// ValidRebalanceLevel reports whether a decimal level string parses to a value
// in the closed interval [0,1]; the level is a slider coordinate, not a
// physiological threshold.
func ValidRebalanceLevel(level string) bool {
	value, err := ParseRebalanceRat(level)
	if err != nil {
		return false
	}
	if value.Cmp(NewRebalanceRatFromInt(0, 1)) < 0 || value.Cmp(NewRebalanceRatFromInt(1, 1)) > 0 {
		return false
	}
	return true
}

func validRebalanceMode(mode string) bool {
	switch mode {
	case RebalanceModePreserveStructure, RebalanceModePreserveSchedule, RebalanceModeRedistribute:
		return true
	default:
		return false
	}
}

// rebalanceRatFromInt builds a RebalanceRat from a plain int load.
func rebalanceRatFromInt(value int) RebalanceRat {
	return NewRebalanceRatFromInt(value, 1)
}

// rebalanceRatFloorInt returns the integer floor of a RebalanceRat.
func rebalanceRatFloorInt(value RebalanceRat) int {
	num := new(big.Int)
	den := new(big.Int)
	num.Set(value.raw().Num())
	den.Set(value.raw().Denom())
	q := new(big.Int).Quo(num, den)
	if q.IsInt64() {
		return int(q.Int64())
	}
	text := q.String()
	n, _ := strconv.Atoi(text)
	return n
}

func buildRebalanceProposalV2(input *RebalanceInput) RebalanceProposalFile {
	policy := rebalanceV2Policy(input)
	proposal := RebalanceProposalFile{
		SchemaVersion: RebalanceSchemaVersion,
		ProposalID:    rebalanceProposalID(input),
		GeneratedAt:   input.GeneratedAt,
		AthleteID:     input.AthleteID,
		Scope:         input.Scope,
		Request:       input.Request,
		Constraints:   input.Constraints,
		Policy:        &policy,
	}
	notes := []string{
		"adaptive-bidirectional v2: level maps weekly load onto a data-backed envelope via monotone PCHIP.",
		"0.5 is an exact no-op; below 0.5 reduces toward the data-backed low; above 0.5 increases toward the data-backed high.",
	}

	validation := RebalanceValidation{Feasible: true}
	rebalanceV2ValidateLevel(input, &validation)
	rebalanceV2ValidateMode(input, &validation)
	regime, notes, ok := rebalanceV2HistoryContext(input, notes, &validation, &proposal)
	if !ok {
		return rebalanceV2FinalizeProposal(&proposal, notes, &validation)
	}
	envelope, ok := rebalanceV2EnvelopeContext(input, &proposal.Baseline, &regime, notes, &validation, &proposal)
	if !ok {
		return rebalanceV2FinalizeProposal(&proposal, notes, &validation)
	}
	level, _ := ParseRebalanceRat(input.Constraints.Level)
	target := rebalanceV2Target(input, &envelope, level)
	effectiveMax := rebalanceV2EffectiveMax(input)
	proposal.Constraints.MaxIntensity = round3(effectiveMax)

	outside := envelope.OutsideEnvelope && !input.Constraints.Approved
	baseline := proposal.Baseline
	inflight := rebalanceV2Plan(input, &baseline, &envelope, target, level, outside)
	proposal.Operations = inflight.operations
	proposal.Projection = &inflight.projection
	proposal.Options = []RebalanceOption{{
		ID:         "adaptive_bidirectional",
		Label:      "Adaptive bidirectional level-driven rebalance",
		Evaluation: rebalanceV2Evaluation(&baseline, target, &inflight),
		Sessions:   inflight.sessions,
		Warnings:   inflight.warnings,
	}}
	proposal.SelectedOptionID = "adaptive_bidirectional"
	targets := DynamicRebalanceTargets(input)
	proposal.Context = rebalanceContext(input, &targets, inflight.warnings)

	if outside {
		validation.Warnings = append(
			validation.Warnings,
			"baseline outside data-backed envelope: no applicable operations; provide explicit limits via `icu rebalance approve --file --reason` then recalculate",
		)
		notes = append(notes, "baseline outside envelope: proposal is diagnostic; accept requires explicit limits and a recalculation reason")
	}

	return rebalanceV2FinalizeProposal(&proposal, notes, &validation)
}

type rebalanceV2Inflight struct {
	operations   []RebalanceOperation
	sessions     []RebalanceSession
	projection   RebalanceProjectionReport
	warnings     []string
	targetRat    RebalanceRat
	appliedTotal int
}

type rebalanceMutableLoads struct {
	targetMutable  RebalanceRat
	currentMutable RebalanceRat
	ok             bool
}

func rebalanceV2Evaluation(baseline *RebalanceEvaluation, target RebalanceRat, inflight *rebalanceV2Inflight) RebalanceEvaluation {
	applied := inflight.appliedTotal
	total := baseline.CompletedLoad + baseline.LockedPlannedLoad + applied
	return RebalanceEvaluation{
		WeeklyLoad:         total,
		CompletedLoad:      baseline.CompletedLoad,
		LockedPlannedLoad:  baseline.LockedPlannedLoad,
		MutablePlannedLoad: applied,
		TargetLoad:         rebalanceRatFloorInt(target),
		TargetDelta:        total - rebalanceRatFloorInt(target),
		MovingTimeSecs:     rebalanceV2MovingTime(inflight),
		MovingTimeHours:    round2(float64(rebalanceV2MovingTime(inflight)) / rebalanceSecondsPerHour),
		Feasible:           !inflight.projection.HasErrors(),
		Source:             "rebalance_v2_operations",
	}
}

func rebalanceV2FinalizeValidation(validation *RebalanceValidation) RebalanceValidation {
	if len(validation.Errors) > 0 {
		validation.Blocking = true
		validation.Feasible = false
	}
	return *validation
}

func rebalanceV2ValidateLevel(input *RebalanceInput, validation *RebalanceValidation) {
	if input.Constraints.Level == "" {
		validation.Errors = append(validation.Errors, "adaptive-bidirectional requires --level in [0,1]")
		return
	}
	if !ValidRebalanceLevel(input.Constraints.Level) {
		validation.Errors = append(validation.Errors, "adaptive-bidirectional level must be a decimal in [0,1]")
	}
}

func rebalanceV2ValidateMode(input *RebalanceInput, validation *RebalanceValidation) {
	mode := input.Constraints.Mode
	if mode == "" {
		return
	}
	if !validRebalanceMode(mode) {
		validation.Errors = append(validation.Errors, "unsupported rebalance mode: "+mode)
	}
}

func rebalanceV2History(input *RebalanceInput, series []float64, regime *RebalanceRegime) RebalanceHistoryReport {
	report := RebalanceHistoryReport{
		RangeEnd:   input.Scope.StartDate,
		Source:     regime.Source,
		Coverage:   regime.Coverage,
		Confidence: regime.Confidence,
	}
	if len(series) > 0 {
		firstWeek := ""
		for index := range input.Activities {
			week := isoWeekKeyFromDate(activityDate(&input.Activities[index]))
			if week != "" && week < isoWeekKeyFromDate(input.Scope.StartDate) {
				if firstWeek == "" || week < firstWeek {
					firstWeek = week
				}
			}
		}
		report.RangeStart = firstWeek
	}
	report.RegimeStartIndex = regime.StartIndex
	report.RegimeEndIndex = regime.EndIndex
	report.ChangePoints = regime.ChangePoints
	report.RecentDisturbance = regime.RecentDisturbance
	return report
}

func rebalanceV2HistoryContext(
	input *RebalanceInput,
	notes []string,
	validation *RebalanceValidation,
	proposal *RebalanceProposalFile,
) (RebalanceRegime, []string, bool) {
	series := RebalanceWeeklyLoadSeries(input.Activities, input.Scope.StartDate)
	regime, regimeOK := DetectRebalanceRegime(series)
	history := rebalanceV2History(input, series, &regime)
	proposal.History = &history
	proposal.Baseline = RebalanceBaseline(input)
	if !regimeOK {
		validation.Errors = append(validation.Errors, "insufficient prior history to derive a vigent regime; onboarding/calibration is separate from rebalance")
	}

	return regime, notes, len(validation.Errors) == 0
}

func rebalanceV2EnvelopeContext(
	input *RebalanceInput,
	baseline *RebalanceEvaluation,
	regime *RebalanceRegime,
	notes []string,
	validation *RebalanceValidation,
	proposal *RebalanceProposalFile,
) (RebalanceEnvelopeReport, bool) {
	envelope, envOK := BuildRebalanceEnvelope(regime, input, float64(baseline.WeeklyLoad))
	proposal.Envelope = &envelope
	if envOK {
		return envelope, true
	}
	validation.Errors = append(validation.Errors, "envelope could not be derived from available metrics")
	_ = notes

	return RebalanceEnvelopeReport{}, false
}

func rebalanceV2FinalizeProposal(
	proposal *RebalanceProposalFile,
	notes []string,
	validation *RebalanceValidation,
) RebalanceProposalFile {
	proposal.Notes = notes
	proposal.Validation = rebalanceV2FinalizeValidation(validation)

	return *proposal
}

func rebalanceV2Target(input *RebalanceInput, envelope *RebalanceEnvelopeReport, level RebalanceRat) RebalanceRat {
	half := NewRebalanceRatFromInt(1, 2)
	curve := NewRebalancePCHIP(envelope.Envelope)
	target := curve.Evaluate(level)
	if input.Constraints.TargetLoad > 0 {
		target = rebalanceV2ClampTarget(target, rebalanceRatFromInt(input.Constraints.TargetLoad), level, half)
	}
	if level.Cmp(half) == 0 {
		return envelope.Envelope.Current.clone()
	}

	return target
}

func rebalanceV2ClampTarget(target, explicitTarget, level, half RebalanceRat) RebalanceRat {
	if level.Cmp(half) < 0 && target.Cmp(explicitTarget) < 0 {
		return explicitTarget.clone()
	}
	if level.Cmp(half) > 0 && target.Cmp(explicitTarget) > 0 {
		return explicitTarget.clone()
	}

	return target
}

func rebalanceV2Plan(input *RebalanceInput, baseline *RebalanceEvaluation, envelope *RebalanceEnvelopeReport, target, level RebalanceRat, outside bool) rebalanceV2Inflight {
	inflight := rebalanceV2Inflight{targetRat: target}
	mode := input.Constraints.Mode
	if mode == "" {
		mode = RebalanceModePreserveStructure
	}
	if early, ok := rebalanceV2EarlyInflight(outside, envelope, target, level); ok {
		return early
	}
	slots := rebalanceV2ModeSlots(input, mode)
	if len(slots) == 0 {
		inflight.warnings = append(inflight.warnings, "no mutable cycling POWER sessions for the selected mode")
		return inflight
	}
	loads := rebalanceV2MutableLoads(baseline, target, &inflight)
	if !loads.ok {
		return inflight
	}
	shares := rebalanceV2Distribute(input, slots, loads.targetMutable, loads.currentMutable, mode)
	inflight.projection = rebalanceV2Projection(input, slots, shares, loads.targetMutable, level, mode, &inflight)

	return inflight
}

func rebalanceV2EarlyInflight(outside bool, envelope *RebalanceEnvelopeReport, target, level RebalanceRat) (rebalanceV2Inflight, bool) {
	inflight := rebalanceV2Inflight{targetRat: target}
	if outside {
		inflight.warnings = append(inflight.warnings, "baseline outside envelope: no operations generated")
		return inflight, true
	}
	half := NewRebalanceRatFromInt(1, 2)
	if level.Cmp(half) == 0 || target.Cmp(envelope.Envelope.Current) == 0 {
		inflight.warnings = append(inflight.warnings, "level 0.5 / target equals current plan: no operations generated (exact no-op)")
		return inflight, true
	}

	return rebalanceV2Inflight{}, false
}

func rebalanceV2MutableLoads(
	baseline *RebalanceEvaluation,
	target RebalanceRat,
	inflight *rebalanceV2Inflight,
) rebalanceMutableLoads {
	completedLocked := rebalanceRatFromInt(baseline.CompletedLoad + baseline.LockedPlannedLoad)
	targetMutable := target.Sub(completedLocked)
	if targetMutable.Sign() < 0 {
		targetMutable = ZeroRebalanceRat()
	}
	currentMutable := rebalanceRatFromInt(baseline.MutablePlannedLoad)
	if currentMutable.IsZero() {
		inflight.warnings = append(inflight.warnings, "no mutable planned load to redistribute")
		return rebalanceMutableLoads{}
	}

	return rebalanceMutableLoads{targetMutable: targetMutable, currentMutable: currentMutable, ok: true}
}

func rebalanceV2Projection(
	input *RebalanceInput,
	slots []rebalanceV2Slot,
	shares []RebalanceRat,
	targetMutable RebalanceRat,
	level RebalanceRat,
	mode string,
	inflight *rebalanceV2Inflight,
) RebalanceProjectionReport {
	projection := RebalanceProjectionReport{ExactTotalLoad: targetMutable.DecimalString()}
	residual := ZeroRebalanceRat()
	maxError := ZeroRebalanceRat()
	for index := range slots {
		slot := &slots[index]
		share := shares[index]
		// Compensated distribution: carry forward accumulated residue
		compensatedShare := share.Add(residual)
		session, step, blocked := rebalanceV2BuildSession(input, slot, compensatedShare, level, mode)
		inflight.sessions = append(inflight.sessions, session)
		if blocked {
			projection.Steps = append(projection.Steps, step)
			continue
		}
		// Update residual: difference between compensated share and applied load
		actualApplied := rebalanceRatFromInt(step.AppliedLoad)
		residual = compensatedShare.Sub(actualApplied)
		errStep := actualApplied.Sub(step.ExactLoadRat)
		if errStep.Sign() < 0 {
			errStep = errStep.Sub(ZeroRebalanceRat())
		}
		if errStep.Cmp(maxError) > 0 {
			maxError = errStep
		}
		inflight.appliedTotal += step.AppliedLoad
		projection.Steps = append(projection.Steps, step)
		operation := buildRebalanceOperation(&session)
		if operation.Action != "" {
			inflight.operations = append(inflight.operations, operation)
		}
	}
	projection.AppliedTotalLoad = inflight.appliedTotal
	projection.Residual = residual.DecimalString()
	projection.MaxSessionError = maxError.DecimalString()

	return projection
}

type rebalanceV2Slot struct {
	slot          rebalanceSlot
	originalLoad  int
	originalIF    float64
	transformable bool
	weight        float64
}

func rebalanceV2ModeSlots(input *RebalanceInput, mode string) []rebalanceV2Slot {
	split := splitRebalanceEvents(input)
	var slots []rebalanceV2Slot
	switch mode {
	case RebalanceModeRedistribute:
		base := rebalanceSlots(input, split.mutable)
		for index := range base {
			slots = append(slots, rebalanceV2SlotFromEvent(input, &base[index]))
		}
	default:
		for index := range split.mutable {
			event := &split.mutable[index]
			if !rebalanceV2CyclingPowerEvent(event) {
				continue
			}
			slots = append(slots, rebalanceV2SlotFromEvent(input, &rebalanceSlot{date: eventDateOnly(event.StartDateLocal), event: event}))
		}
	}
	return slots
}

func rebalanceV2SlotFromEvent(input *RebalanceInput, base *rebalanceSlot) rebalanceV2Slot {
	slot := rebalanceV2Slot{
		slot:          *base,
		originalLoad:  0,
		originalIF:    input.Constraints.Z2IF,
		transformable: true,
		weight:        1,
	}
	if base.event != nil {
		slot.originalLoad = rebalanceEventLoad(base.event, input)
		if base.event.Intensity > 0 {
			slot.originalIF = base.event.Intensity
		}
		slot.transformable = rebalanceV2Transformable(base.event)
		slot.weight = float64(maxInt(1, slot.originalLoad))
	}
	return slot
}

func rebalanceV2CyclingPowerEvent(event *Event) bool {
	return isWorkoutEvent(event) && IsCyclingActivityType(event.Type) && strings.EqualFold(event.Target, rebalanceWorkoutTargetPower)
}

func rebalanceV2Transformable(event *Event) bool {
	if event == nil {
		return false
	}
	if event.TrainingLoad > 0 && event.MovingTime > 0 {
		return true
	}
	if event.WorkoutDoc != nil {
		return true
	}
	if event.Description != "" {
		doc, err := ParseWorkoutDescription(event.Description)
		if err == nil && doc != nil && len(doc.Steps) > 0 {
			return true
		}
	}
	return false
}

func rebalanceV2Distribute(input *RebalanceInput, slots []rebalanceV2Slot, target, current RebalanceRat, mode string) []RebalanceRat {
	shares := make([]RebalanceRat, len(slots))
	if len(slots) == 0 {
		return shares
	}
	if mode == RebalanceModeRedistribute {
		weights := rebalanceV2RedistributeWeights(input, slots)
		residual := ZeroRebalanceRat()
		for index := range slots {
			share := target.Mul(weights[index])
			shares[index] = share
			residual = residual.Add(share)
		}
		// adjust last share to absorb rounding-free sum is already exact; nothing to do
		return shares
	}
	if current.IsZero() {
		return shares
	}
	scale := target.Quo(current)
	residual := ZeroRebalanceRat()
	for index := range slots {
		old := rebalanceRatFromInt(slots[index].originalLoad)
		share := old.Mul(scale)
		shares[index] = share
		residual = residual.Add(share)
	}
	// exact sum equals target by construction; compensate nothing more
	return shares
}

func rebalanceV2RedistributeWeights(input *RebalanceInput, slots []rebalanceV2Slot) []RebalanceRat {
	weights := make([]RebalanceRat, len(slots))
	if len(slots) == 0 {
		return weights
	}
	basis := rebalanceSlotWeights(input, v2SlotsToPlain(slots))
	if len(basis) == 0 {
		basis = equalRebalanceWeights(len(slots))
	}
	for index := range slots {
		w, ok := NewRebalanceRatFromDyadic(basis[index])
		if !ok {
			w = NewRebalanceRatFromInt(1, len(slots))
		}
		weights[index] = w
	}
	// renormalize exact weights so they sum to 1
	total := ZeroRebalanceRat()
	for index := range weights {
		total = total.Add(weights[index])
	}
	if total.IsZero() {
		return weights
	}
	for index := range weights {
		weights[index] = weights[index].Quo(total)
	}
	return weights
}

func v2SlotsToPlain(slots []rebalanceV2Slot) []rebalanceSlot {
	out := make([]rebalanceSlot, len(slots))
	for index := range slots {
		out[index] = slots[index].slot
	}
	return out
}

func rebalanceV2BuildSession(input *RebalanceInput, slot *rebalanceV2Slot, share, level RebalanceRat, mode string) (RebalanceSession, RebalanceProjectionStep, bool) {
	effectiveMax := rebalanceV2EffectiveMax(input)
	step := RebalanceProjectionStep{ID: slot.slot.date, ExactLoad: share.DecimalString(), ExactLoadRat: share}
	zero := NewRebalanceRatFromInt(0, 1)
	if share.Cmp(zero) <= 0 {
		step.AppliedLoad = 0
		step.AppliedSeconds = 0
		return RebalanceSession{}, step, true
	}
	if mode == RebalanceModePreserveStructure && !slot.transformable {
		step.Blocked = true
		step.Reason = "session not transformable without loss: no power WorkoutDoc or magnitude basis"
		return RebalanceSession{}, step, true
	}

	z1IF := input.Constraints.Z1IF
	if z1IF <= 0 {
		z1IF = DynamicRebalanceTargets(input).Z1IF
	}
	origIF := slot.originalIF
	if origIF <= 0 {
		origIF = input.Constraints.Z2IF
	}
	if origIF <= 0 {
		origIF = DynamicRebalanceTargets(input).Z2IF
	}

	// Continuous piece-wise linear IF interpolation
	half := NewRebalanceRatFromInt(1, 2)
	var targetIF float64
	if level.Cmp(half) < 0 {
		levelFrac := level.Float64() * 2 // 0→0, 0.5→1
		targetIF = z1IF + (origIF-z1IF)*levelFrac
	} else {
		levelFrac := (level.Float64() - 0.5) * 2 // 0.5→0, 1→1
		targetIF = origIF + (effectiveMax-origIF)*levelFrac
	}

	// CP/W'/PMax duration-power limit at level=1
	if level.Cmp(NewRebalanceRatFromInt(1, 1)) == 0 {
		ftp := rebalanceFTP(input)
		if ftp > 0 {
			cp := float64(ftp)
			wPrime := 0
			pMax := 0
			if input.SportSettings != nil {
				if input.SportSettings.WPrime > 0 {
					wPrime = input.SportSettings.WPrime
				}
				if input.SportSettings.PMax > 0 {
					pMax = input.SportSettings.PMax
				}
			}
			tSec := rebalanceV2SecondsEstimate(share, targetIF)
			if tSec > 0 {
				maxWatts := cp
				if wPrime > 0 {
					maxWatts = cp + float64(wPrime)/tSec
				}
				if pMax > 0 && float64(pMax) < maxWatts {
					maxWatts = float64(pMax)
				}
				maxIF := maxWatts / float64(ftp)
				if targetIF > maxIF {
					targetIF = maxIF
				}
			}
		}
	}

	if targetIF <= 0 {
		step.Blocked = true
		step.Reason = "no usable intensity target after interpolation"
		return RebalanceSession{}, step, true
	}

	exactSeconds := rebalanceV2ExactSeconds(share, targetIF)
	appliedSeconds := rebalanceV2RoundSeconds(input, exactSeconds)
	if input.Constraints.MaxSessionMinutes > 0 {
		maxSeconds := input.Constraints.MaxSessionMinutes * rebalanceSecondsPerMinute
		if appliedSeconds > maxSeconds {
			appliedSeconds = maxSeconds
		}
	}
	appliedLoadFloat := estimateLoadFromDurationIF(appliedSeconds, targetIF)
	appliedLoad := maxInt(0, appliedLoadFloat)
	step.AppliedLoad = appliedLoad
	step.AppliedSeconds = appliedSeconds
	step.ExactIF = rebalanceV2ExactIF(share, exactSeconds).DecimalString()
	step.AppliedIF = strconv.FormatFloat(targetIF, 'f', 3, 64)
	step.Residual = rebalanceRatFromInt(appliedLoad).Sub(share).DecimalString()
	step.ExactSeconds = exactSeconds.DecimalString()

	session := buildRebalanceV2Session(input, slot, appliedLoad, appliedSeconds, targetIF)

	// preserve-structure: scale workout steps and generate description
	if mode == RebalanceModePreserveStructure && slot.slot.event != nil && math.Abs(targetIF-origIF) > 0.0001 {
		desc := RebalanceScaleWorkoutDescription(slot.slot.event, origIF, targetIF)
		if desc != "" {
			session.Description = desc
		}
	}

	return session, step, false
}

// rebalanceV2SecondsEstimate estimates the duration in seconds for a given
// load and intensity factor, using floating-point approximation.
func rebalanceV2SecondsEstimate(load RebalanceRat, ifloat float64) float64 {
	if ifloat <= 0 {
		return 0
	}
	// load = hours * 100 * IF^2
	// hours = load / (100 * IF^2)
	hours := load.Float64() / (rebalanceSportZoneDivisor * ifloat * ifloat)
	return hours * rebalanceSecondsPerHour
}

func rebalanceV2ExactSeconds(load RebalanceRat, ifloat float64) RebalanceRat {
	hundred := NewRebalanceRatFromInt(rebalancePercentScale, 1)
	ifRat, ok := NewRebalanceRatFromDyadic(ifloat)
	if !ok {
		ifRat = NewRebalanceRatFromInt(int(ifloat*rebalancePercentScale), rebalancePercentScale)
	}
	ifSquared := ifRat.Mul(ifRat)
	perHour := hundred.Mul(ifSquared)
	hours := load.Quo(perHour)
	secondsPerHour := NewRebalanceRatFromInt(rebalanceSecondsPerHour, 1)
	return hours.Mul(secondsPerHour)
}

func rebalanceV2ExactIF(load, seconds RebalanceRat) RebalanceRat {
	// IF = sqrt(load / (hours * 100)) ; compute rationally as load/(seconds/3600*100)
	hundred := NewRebalanceRatFromInt(rebalancePercentScale, 1)
	secondsPerHour := NewRebalanceRatFromInt(rebalanceSecondsPerHour, 1)
	hours := seconds.Quo(secondsPerHour)
	denom := hours.Mul(hundred)
	if denom.IsZero() {
		return ZeroRebalanceRat()
	}
	return load.Quo(denom)
}

func rebalanceV2RoundSeconds(input *RebalanceInput, exact RebalanceRat) int {
	minutes := rebalanceRatFloorInt(exact.Quo(NewRebalanceRatFromInt(rebalanceSecondsPerMinute, 1)))
	minMinutes := input.Constraints.MinSessionMinutes
	stepMinutes := input.Constraints.DurationStepMinutes
	if minMinutes > 0 && minutes < minMinutes {
		minutes = minMinutes
	}
	if stepMinutes > 0 {
		minutes = roundMinutesToStep(minutes, stepMinutes)
	}
	return maxInt(0, minutes) * rebalanceSecondsPerMinute
}

func buildRebalanceV2Session(input *RebalanceInput, slot *rebalanceV2Slot, load, seconds int, ifloat float64) RebalanceSession {
	intensity := round3(ifloat)
	action := RebalanceActionCreate
	eventID := 0
	var sourceHash string
	name := ""
	if slot.slot.event != nil {
		action = RebalanceActionUpdate
		eventID = slot.slot.event.ID
		sourceHash = RebalanceEventHash(slot.slot.event)
		name = slot.slot.event.Name
	}
	startTime := rebalanceSessionStartTime(input, &slot.slot)
	minutes := int(round2(float64(seconds) / rebalanceSecondsPerMinute))
	if name == "" {
		name = fmt.Sprintf("Rebalance %s %d", slot.slot.date, load)
	}
	classification := classifyGeneratedRebalanceSession(input, seconds, intensity)
	return RebalanceSession{
		ID:                   action + "-" + slot.slot.date,
		Date:                 slot.slot.date,
		Day:                  rebalanceWeekday(slot.slot.date),
		StartTime:            startTime,
		StartTimeSource:      rebalanceSessionStartTimeSource(input, &slot.slot),
		EventID:              eventID,
		Action:               action,
		SportType:            rebalanceSessionSportType(input, &slot.slot),
		SportTypeSource:      rebalanceSessionSportTypeSource(input, &slot.slot),
		WorkoutTarget:        rebalanceSessionWorkoutTarget(input, &slot.slot),
		WorkoutTargetSource:  rebalanceSessionWorkoutTargetSource(input, &slot.slot),
		Name:                 name,
		Classification:       classification.label,
		ClassificationSource: classification.source,
		AllocationSource:     rebalanceAllocationSource(input, &slot.slot),
		TargetLoad:           load,
		DurationSeconds:      seconds,
		DurationMinutes:      round2(float64(seconds) / rebalanceSecondsPerMinute),
		DurationSource:       rebalanceSessionDurationSource(input),
		IntensityFactor:      intensity,
		IntensitySource:      rebalanceSessionIntensitySource(input),
		WattsRange:           wattsRange(input, intensity),
		Description:          rebalanceWorkoutDescription(minutes, intensity),
		SourceHash:           sourceHash,
		Reason:               "adaptive-bidirectional level-driven magnitude transform",
	}
}

func rebalanceV2MovingTime(inflight *rebalanceV2Inflight) int {
	var total int
	for index := range inflight.sessions {
		total += inflight.sessions[index].DurationSeconds
	}
	return total
}

func rebalanceV2Policy(input *RebalanceInput) RebalancePolicy {
	mode := input.Constraints.Mode
	if mode == "" {
		mode = RebalanceModePreserveStructure
	}
	return RebalancePolicy{
		Strategy:    RebalanceStrategyAdaptiveBidirectional,
		Level:       input.Constraints.Level,
		Mode:        mode,
		CurveMethod: RebalanceCurveMethodPCHIP,
		Source:      RebalancePolicySource,
	}
}

func rebalanceV2EffectiveMax(input *RebalanceInput) float64 {
	if input.Constraints.MaxIntensity > 0 {
		return input.Constraints.MaxIntensity
	}
	return DynamicRebalanceTargets(input).MaxIntensity
}

const rebalanceApproveHashLength = 12

// ComputeRebalanceApprove binds a proposal to an approval reason and explicit
// limits and hashes the resolved policy/envelope so accept can detect drift.
// Verified is true when the proposal is not an outside-envelope diagnostic or
// when the user supplied explicit target-load and level limits alongside the
// reason. The returned hashes are short SHA-256 prefixes.
func ComputeRebalanceApprove(proposal *RebalanceProposalFile, reason string, providedLimits bool) RebalanceApprove {
	approve := RebalanceApprove{
		Reason:         reason,
		ProvidedLimits: providedLimits,
		TargetLoad:     proposal.Constraints.TargetLoad,
		Level:          proposal.Constraints.Level,
		Mode:           proposal.Constraints.Mode,
	}
	outside := proposal.Envelope != nil && proposal.Envelope.OutsideEnvelope
	approve.Verified = (reason != "" && (!outside || providedLimits))
	approve.RecalcHash = rebalanceApproveFingerprint(rebalanceApprovePolicyFingerprint(proposal), rebalanceApproveEnvelopeFingerprint(proposal))
	approve.LimitsHash = rebalanceApproveFingerprint(approve.Level + "|" + strconv.Itoa(approve.TargetLoad) + "|" + approve.Mode)
	return approve
}

func rebalanceApprovePolicyFingerprint(proposal *RebalanceProposalFile) string {
	if proposal.Policy == nil {
		return ""
	}
	return proposal.Policy.Strategy + "|" + proposal.Policy.Level + "|" + proposal.Policy.Mode + "|" + proposal.Policy.CurveMethod
}

func rebalanceApproveEnvelopeFingerprint(proposal *RebalanceProposalFile) string {
	if proposal.Envelope == nil {
		return ""
	}
	return proposal.Envelope.Envelope.Low.DecimalString() + "|" +
		proposal.Envelope.Envelope.Current.DecimalString() + "|" +
		proposal.Envelope.Envelope.High.DecimalString() + "|" +
		proposal.Envelope.LowSource + "|" + proposal.Envelope.HighSource
}

func rebalanceApproveFingerprint(parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "::")))
	return hex.EncodeToString(sum[:])[:rebalanceApproveHashLength]
}
