package icu_test

import (
	"math"
	"testing"

	icu "github.com/Thejuampi/icu"
)

func TestBacktestPhysicsConsistentRecoversHighCorrelation(t *testing.T) {
	t.Parallel()

	// Generate a long ride whose power is exactly the Martin model. Mask the
	// second half and require the estimator to recover near 1:1 correlation.
	const n = 2400
	params := baseParams()
	plantedCdA := 0.30
	params.CdA = icu.LabeledParam{Value: plantedCdA, Source: "user"}

	watts := make([]float64, n)
	cad := make([]float64, n)
	cadPresent := make([]bool, n)
	speed := make([]float64, n)
	dist := make([]float64, n)
	alt := make([]float64, n)
	times := make([]float64, n)

	mass := params.RiderMassKg.Value + params.BikeMassKg.Value
	g := 9.80665
	rho := params.AirDensity.Value
	eta := params.DrivetrainEff.Value
	crr := params.Crr.Value

	var distance float64
	var altitude float64
	for i := range n {
		times[i] = float64(i)
		// Varied speed and mild grade waves so cal + grade physics are exercised.
		speed[i] = 6 + 2*math.Sin(float64(i)/80) + 0.5*math.Sin(float64(i)/17)
		if speed[i] < 3 {
			speed[i] = 3
		}
		grade := 0.02 * math.Sin(float64(i)/120) // ±2%
		// Integrate distance/altitude for stream-derived grade.
		if i > 0 {
			distance += speed[i]
			altitude += grade * speed[i]
		}
		dist[i] = distance
		alt[i] = 100 + altitude

		den := math.Sqrt(1 + grade*grade)
		pWheel := mass*g*speed[i]*(grade/den) +
			crr*mass*g*speed[i]/den +
			0.5*rho*plantedCdA*speed[i]*speed[i]*speed[i]
		if pWheel < 0 {
			watts[i] = 0
			cad[i] = 0
			cadPresent[i] = true
			continue
		}
		watts[i] = pWheel / eta
		cad[i] = 85
		cadPresent[i] = true
	}

	streams := icu.NullableStreamData{
		"watts":           seriesFrom(watts, nil),
		"cadence":         seriesFrom(cad, cadPresent),
		"velocity_smooth": seriesFrom(speed, nil),
		"distance":        seriesFrom(dist, nil),
		"altitude":        seriesFrom(alt, nil),
		"time":            seriesFrom(times, nil),
	}

	// Calibrate without telling the planted CdA.
	params.CdA = icu.LabeledParam{}
	got := icu.BacktestPowerEstimate(icu.PowerBacktestRequest{
		Streams:               streams,
		Params:                params,
		CalibrateFromMeasured: true,
		Mode:                  icu.PowerBacktestMaskSecondHalf,
	})
	if got.BlockingError != "" {
		t.Fatalf("blocking: %s warnings=%v", got.BlockingError, got.Warnings)
	}
	if got.ComparedSeconds < 200 {
		t.Fatalf("compared=%d", got.ComparedSeconds)
	}
	// Physics-consistent data must recover near 1:1 on multiple scores.
	if got.PearsonR < 0.95 {
		t.Fatalf("pearsonR=%.4f want >= 0.95 (meanAct=%.1f meanEst=%.1f rmse=%.1f)",
			got.PearsonR, got.MeanActual, got.MeanEstimated, got.RMSE)
	}
	if got.SpearmanRho < 0.95 {
		t.Fatalf("spearmanRho=%.4f want >= 0.95", got.SpearmanRho)
	}
	if got.Scores.ZScorePearsonR < 0.95 {
		t.Fatalf("zScorePearsonR=%.4f want >= 0.95", got.Scores.ZScorePearsonR)
	}
	if got.Scores.ResidualZMedianAbs > 1.0 {
		t.Fatalf("residualZMedianAbs=%.3f want <= 1.0 on physics-consistent data", got.Scores.ResidualZMedianAbs)
	}
	if got.RMSE > 40 {
		t.Fatalf("rmse=%.1f too high for physics-consistent series", got.RMSE)
	}
}

func TestScorePowerBacktestOutlierRobustness(t *testing.T) {
	t.Parallel()

	// Actual and estimated mostly agree; one huge spike should not dominate robust scores.
	n := 100
	actual := make([]float64, n)
	estimated := make([]float64, n)
	for i := range n {
		actual[i] = 200
		estimated[i] = 200 + float64(i%3-1) // small noise -1,0,1
	}
	estimated[50] = 2000 // outlier spike
	scores := icu.ScorePowerBacktestForTest(actual, estimated)
	if scores.OutlierSeconds < 1 {
		t.Fatalf("expected outlier detection, got %+v", scores)
	}
	if scores.RobustRMSE > scores.RMSE {
		t.Fatalf("robustRmse=%v should be <= rmse=%v", scores.RobustRMSE, scores.RMSE)
	}
	if scores.SpearmanRho < 0.5 {
		// rank correlation should survive one spike better than raw level in some cases;
		// at least residual z median should stay moderate.
		t.Logf("spearman=%.3f residualZMed=%.3f", scores.SpearmanRho, scores.ResidualZMedianAbs)
	}
	if scores.ResidualZMedianAbs > 1.5 {
		t.Fatalf("residualZMedianAbs=%.3f inflated by single outlier", scores.ResidualZMedianAbs)
	}
}

func TestBacktestMaskCreatesMissingGaps(t *testing.T) {
	t.Parallel()

	const n = 200
	watts := make([]float64, n)
	cad := make([]float64, n)
	present := make([]bool, n)
	speed := make([]float64, n)
	dist := make([]float64, n)
	alt := make([]float64, n)
	times := make([]float64, n)
	for i := range n {
		watts[i] = 200
		cad[i] = 90
		present[i] = true
		speed[i] = 8
		dist[i] = 8 * float64(i)
		alt[i] = 100
		times[i] = float64(i)
	}
	streams := icu.NullableStreamData{
		"watts":           seriesFrom(watts, nil),
		"cadence":         seriesFrom(cad, present),
		"velocity_smooth": seriesFrom(speed, nil),
		"distance":        seriesFrom(dist, nil),
		"altitude":        seriesFrom(alt, nil),
		"time":            seriesFrom(times, nil),
	}
	params := baseParams()
	got := icu.BacktestPowerEstimate(icu.PowerBacktestRequest{
		Streams:               streams,
		Params:                params,
		CalibrateFromMeasured: true,
		Mode:                  icu.PowerBacktestMaskAfterFraction,
		MaskAfterFraction:     0.4,
	})
	if got.BlockingError != "" {
		t.Fatalf("blocking: %s", got.BlockingError)
	}
	if got.MaskStartIndex != 80 {
		t.Fatalf("maskStart=%d want 80", got.MaskStartIndex)
	}
	if got.Fill.Classification.MissingSeconds < 50 {
		t.Fatalf("missing=%d, mask did not create gaps", got.Fill.Classification.MissingSeconds)
	}
}

func TestBacktestRequiresWatts(t *testing.T) {
	t.Parallel()

	got := icu.BacktestPowerEstimate(icu.PowerBacktestRequest{
		Streams: icu.NullableStreamData{},
		Params:  baseParams(),
	})
	if got.BlockingError == "" {
		t.Fatal("expected blocking error")
	}
}

func TestBacktestMaskStartModes(t *testing.T) {
	t.Parallel()

	// Exercise mask_after_fraction edge and default second half via full backtest.
	const n = 100
	watts := make([]float64, n)
	cad := make([]float64, n)
	present := make([]bool, n)
	speed := make([]float64, n)
	dist := make([]float64, n)
	alt := make([]float64, n)
	times := make([]float64, n)
	for i := range n {
		watts[i] = 200
		cad[i] = 90
		present[i] = true
		speed[i] = 8
		dist[i] = 8 * float64(i)
		alt[i] = 0
		times[i] = float64(i)
	}
	streams := icu.NullableStreamData{
		"watts":           seriesFrom(watts, nil),
		"cadence":         seriesFrom(cad, present),
		"velocity_smooth": seriesFrom(speed, nil),
		"distance":        seriesFrom(dist, nil),
		"altitude":        seriesFrom(alt, nil),
		"time":            seriesFrom(times, nil),
	}
	// Default mode (empty) → second half
	got := icu.BacktestPowerEstimate(icu.PowerBacktestRequest{
		Streams:               streams,
		Params:                baseParams(),
		CalibrateFromMeasured: true,
		ActivityType:          "Ride",
		DistanceMeters:        10000,
	})
	if got.BlockingError != "" {
		t.Fatalf("blocking: %s", got.BlockingError)
	}
	if got.MaskStartIndex != 50 {
		t.Fatalf("maskStart=%d want 50", got.MaskStartIndex)
	}
}

func TestBacktestMaskScatter(t *testing.T) {
	t.Parallel()

	const n = 400
	watts := make([]float64, n)
	cad := make([]float64, n)
	present := make([]bool, n)
	speed := make([]float64, n)
	dist := make([]float64, n)
	alt := make([]float64, n)
	times := make([]float64, n)
	for i := range n {
		watts[i] = 180 + float64(i%20)
		cad[i] = 90
		present[i] = true
		speed[i] = 8
		dist[i] = 8 * float64(i)
		alt[i] = 50
		times[i] = float64(i)
	}
	streams := icu.NullableStreamData{
		"watts":           seriesFrom(watts, nil),
		"cadence":         seriesFrom(cad, present),
		"velocity_smooth": seriesFrom(speed, nil),
		"distance":        seriesFrom(dist, nil),
		"altitude":        seriesFrom(alt, nil),
		"time":            seriesFrom(times, nil),
	}
	params := baseParams()
	got := icu.BacktestPowerEstimate(icu.PowerBacktestRequest{
		Streams:               streams,
		Params:                params,
		CalibrateFromMeasured: true,
		Mode:                  icu.PowerBacktestMaskScatter,
		MaskAfterFraction:     0.3,
		ActivityType:          "Ride",
		DistanceMeters:        20000,
	})
	if got.BlockingError != "" {
		t.Fatalf("blocking: %s", got.BlockingError)
	}
	if got.MaskedSeconds < 50 {
		t.Fatalf("masked=%d", got.MaskedSeconds)
	}
	if got.ComparedSeconds < 20 {
		t.Fatalf("compared=%d", got.ComparedSeconds)
	}
}

func TestBacktestSkipsIndoorVirtual(t *testing.T) {
	t.Parallel()

	got := icu.BacktestPowerEstimate(icu.PowerBacktestRequest{
		Streams: icu.NullableStreamData{
			"watts": seriesFrom([]float64{100, 100, 100}, nil),
		},
		Params:       baseParams(),
		ActivityType: "VirtualRide",
	})
	if got.BlockingError == "" {
		t.Fatal("expected indoor skip")
	}
	if !icu.IsOutdoorCyclingActivity("Ride") {
		t.Fatal("Ride should be outdoor")
	}
	if !icu.IsOutdoorCyclingActivity("GravelRide") {
		t.Fatal("GravelRide should be outdoor")
	}
	if icu.IsOutdoorCyclingActivity("VirtualRide") {
		t.Fatal("VirtualRide should not be outdoor")
	}
	if icu.IsOutdoorCyclingActivity("IndoorCycling") {
		t.Fatal("IndoorCycling should not be outdoor")
	}
}

func TestBacktestSkipsShortDistance(t *testing.T) {
	t.Parallel()

	got := icu.BacktestPowerEstimate(icu.PowerBacktestRequest{
		Streams: icu.NullableStreamData{
			"watts": seriesFrom(make([]float64, 100), nil),
		},
		Params:         baseParams(),
		ActivityType:   "Ride",
		DistanceMeters: 1000,
	})
	if got.BlockingError == "" {
		t.Fatal("expected short-distance skip")
	}
}

func TestScorePowerBacktestEdgeCases(t *testing.T) {
	t.Parallel()

	// Empty / length mismatch returns zero scores (exported scorer path).
	empty := icu.ScorePowerBacktestForTest(nil, nil)
	if empty.PearsonR != 0 || empty.RMSE != 0 {
		t.Fatalf("empty scores=%+v", empty)
	}
	mismatch := icu.ScorePowerBacktestForTest([]float64{1, 2}, []float64{1})
	if mismatch.PearsonR != 0 {
		t.Fatalf("mismatch scores=%+v", mismatch)
	}
	// Rank correlation needs ≥2 points.
	single := icu.ScorePowerBacktestForTest([]float64{10}, []float64{12})
	if single.SpearmanRho != 0 {
		t.Fatalf("single spearman=%v", single.SpearmanRho)
	}
}

func TestBacktestUnknownModeAndScatterEdges(t *testing.T) {
	t.Parallel()

	const sampleCount = 120
	watts := make([]float64, sampleCount)
	cad := make([]float64, sampleCount)
	present := make([]bool, sampleCount)
	speed := make([]float64, sampleCount)
	dist := make([]float64, sampleCount)
	alt := make([]float64, sampleCount)
	times := make([]float64, sampleCount)
	for i := range sampleCount {
		watts[i] = 200
		cad[i] = 90
		present[i] = true
		speed[i] = 8
		dist[i] = 8 * float64(i)
		alt[i] = 100
		times[i] = float64(i)
	}
	streams := icu.NullableStreamData{
		"watts":           seriesFrom(watts, nil),
		"cadence":         seriesFrom(cad, present),
		"velocity_smooth": seriesFrom(speed, nil),
		"distance":        seriesFrom(dist, nil),
		"altitude":        seriesFrom(alt, nil),
		"time":            seriesFrom(times, nil),
	}
	unknown := icu.BacktestPowerEstimate(icu.PowerBacktestRequest{
		Streams: streams, Params: baseParams(), Mode: "not_a_mode",
		CalibrateFromMeasured: true, ActivityType: "Ride", DistanceMeters: 10000,
	})
	if unknown.BlockingError == "" {
		t.Fatal("expected unknown mode error")
	}
	// Invalid scatter fraction falls back to default 0.35 inside the switch when
	// MaskAfterFraction is out of range — exercise the path.
	scatter := icu.BacktestPowerEstimate(icu.PowerBacktestRequest{
		Streams:               streams,
		Params:                baseParams(),
		Mode:                  icu.PowerBacktestMaskScatter,
		MaskAfterFraction:     0, // invalid → default inside scatter branch
		CalibrateFromMeasured: true,
		ActivityType:          "Ride",
		DistanceMeters:        10000,
	})
	if scatter.BlockingError != "" {
		t.Fatalf("scatter default fraction: %s", scatter.BlockingError)
	}
	if scatter.ComparedSeconds < 10 {
		t.Fatalf("compared=%d", scatter.ComparedSeconds)
	}
}

// outdoorBacktestFixture is a synthetic outdoor ride shaped like real GPS PM data:
// Martin free-air watts (optional planted headwind/rho), mild GPS speed/altitude noise,
// and measured power noise — long enough for mask_second_half (~20 min half).
type outdoorBacktestFixture struct {
	streams          icu.NullableStreamData
	params           icu.PowerModelParams
	headwindMSSeries []float64
	rhoSeries        []float64
	distanceMeters   float64
}

// outdoorFixtureOpts controls planted aero for outdoor backtest fixtures.
type outdoorFixtureOpts struct {
	// PlantHeadwind includes a non-zero headwind series in true watts and returns it.
	PlantHeadwind bool
	// PhaseWind: large oscillating along-path wind with the same distribution in
	// both halves. Still-air can absorb mean aero into CdA but cannot track phase,
	// so correlation/robust error worsen vs providing the series. Avoids regime
	// shift that would trip measured-half instant-cap on the held-out half.
	PhaseWind bool
	// LowNoise reduces GPS/power noise (physics-leaning outdoor shape).
	LowNoise bool
}

// buildOutdoorRealisticBacktestFixture builds outdoor-shaped streams for PM-death replay.
// GPS noise and measurement noise make the fixture harder than pure physics-consistent.
func buildOutdoorRealisticBacktestFixture(n int, plantHeadwind bool) outdoorBacktestFixture {
	return buildOutdoorBacktestFixture(n, outdoorFixtureOpts{PlantHeadwind: plantHeadwind})
}

func buildOutdoorBacktestFixture(n int, opts outdoorFixtureOpts) outdoorBacktestFixture {
	if n < 1800 {
		n = 1800 // ≥ ~15 min half at 1 Hz after second-half mask
	}
	params := baseParams()
	plantedCdA := 0.32
	params.CdA = icu.LabeledParam{Value: plantedCdA, Source: "user"}
	params.RiderMassKg = icu.LabeledParam{Value: 81, Source: "user"}
	params.BikeMassKg = icu.LabeledParam{Value: 7.8, Source: "user"}
	params.Crr = icu.LabeledParam{Value: 0.005, Source: "user"}
	params.AirDensity = icu.LabeledParam{Value: 1.19, Source: "user"}

	mass := params.RiderMassKg.Value + params.BikeMassKg.Value
	g := 9.80665
	eta := params.DrivetrainEff.Value
	crr := params.Crr.Value
	rho0 := params.AirDensity.Value

	trueSpeed := make([]float64, n)
	gpsSpeed := make([]float64, n)
	dist := make([]float64, n)
	alt := make([]float64, n)
	times := make([]float64, n)
	watts := make([]float64, n)
	cad := make([]float64, n)
	cadPresent := make([]bool, n)
	hr := make([]float64, n)
	hw := make([]float64, n)
	rho := make([]float64, n)

	speedNoiseAmp := 0.12
	altNoiseAmp := 0.6
	powerNoiseAmp := 4.0
	if opts.LowNoise {
		speedNoiseAmp = 0.04
		altNoiseAmp = 0.2
		powerNoiseAmp = 1.5
	}

	var distance float64
	var altitude float64
	for i := range n {
		times[i] = float64(i)
		// Varied outdoor pace: climbs/descents + surges (not pure sine laboratory).
		trueSpeed[i] = 6.5 + 1.8*math.Sin(float64(i)/95) + 0.9*math.Sin(float64(i)/23) + 0.4*math.Sin(float64(i)/7)
		if trueSpeed[i] < 2.5 {
			trueSpeed[i] = 2.5
		}
		// Mild GPS speed noise (deterministic).
		gpsSpeed[i] = trueSpeed[i] + speedNoiseAmp*math.Sin(float64(i)*1.7) + 0.08*math.Sin(float64(i)*0.31)
		if gpsSpeed[i] < 1.5 {
			gpsSpeed[i] = 1.5
		}
		grade := 0.035*math.Sin(float64(i)/140) + 0.012*math.Sin(float64(i)/41)
		// Occasional short steeper pitch (GPS-like altitude steps after integrate).
		if i%400 > 350 {
			grade += 0.02
		}
		if i > 0 {
			distance += trueSpeed[i]
			altitude += grade * trueSpeed[i]
		}
		dist[i] = distance
		// Altitude stream includes small GPS wander on top of true integration.
		alt[i] = 220 + altitude + altNoiseAmp*math.Sin(float64(i)/11) + 0.3*math.Sin(float64(i)*0.9)

		rho[i] = rho0 + 0.015*math.Sin(float64(i)/300) // slow density drift
		switch {
		case opts.PhaseWind:
			// Large phase-varying headwind (same stats both halves). Still-air CdA
			// cannot track wind phase; the series can.
			hw[i] = 2.8*math.Sin(float64(i)/48) + 1.1*math.Sin(float64(i)/17)
		case opts.PlantHeadwind:
			// Non-zero varying along-path wind throughout.
			hw[i] = 1.8 + 1.4*math.Sin(float64(i)/180) + 0.5*math.Sin(float64(i)/55)
		default:
			hw[i] = 0
		}

		den := math.Sqrt(1 + grade*grade)
		airSpeed := trueSpeed[i] + hw[i]
		pWheel := mass*g*trueSpeed[i]*(grade/den) +
			crr*mass*g*trueSpeed[i]/den +
			0.5*rho[i]*plantedCdA*airSpeed*math.Abs(airSpeed)*trueSpeed[i]
		w := 0.0
		if pWheel > 0 {
			w = pWheel / eta
		}
		// Measurement noise on power (PM). Coast-ish moments stay near zero.
		noise := powerNoiseAmp*math.Sin(float64(i)*2.1) + 2.5*math.Sin(float64(i)/13)
		w += noise
		if w < 0 {
			w = 0
		}
		// Sparse true coasts on measured half will be classified via cadence 0.
		if i%97 == 0 && grade < -0.01 {
			w = 0
			cad[i] = 0
			cadPresent[i] = true
		} else if w < 15 {
			cad[i] = 0
			cadPresent[i] = true
			w = 0
		} else {
			cad[i] = 78 + 8*math.Sin(float64(i)/50)
			cadPresent[i] = true
		}
		watts[i] = w
		// HR drifts slowly and is only weakly related to instant watts so the
		// outdoor ensemble cannot recover power from HR alone (aero path matters).
		hr[i] = 142 + 6*math.Sin(float64(i)/320) + 3*math.Sin(float64(i)/80)
	}

	streams := icu.NullableStreamData{
		"watts":           seriesFrom(watts, nil),
		"cadence":         seriesFrom(cad, cadPresent),
		"velocity_smooth": seriesFrom(gpsSpeed, nil),
		"distance":        seriesFrom(dist, nil),
		"altitude":        seriesFrom(alt, nil),
		"time":            seriesFrom(times, nil),
		"heartrate":       seriesFrom(hr, nil),
	}

	var hwOut []float64
	if opts.PlantHeadwind || opts.PhaseWind {
		hwOut = hw
	}

	return outdoorBacktestFixture{
		streams:          streams,
		params:           params,
		headwindMSSeries: hwOut,
		rhoSeries:        rho,
		distanceMeters:   distance,
	}
}

func assertOutdoorPMDeathGate(t *testing.T, got icu.PowerBacktestResult) {
	t.Helper()
	if got.BlockingError != "" {
		t.Fatalf("blocking: %s warnings=%v", got.BlockingError, got.Warnings)
	}
	// Compared region must be long enough that scores are not toy (~15–20 min half).
	if got.ComparedSeconds < 900 {
		t.Fatalf("comparedSeconds=%d want >= 900 (mask half too short)", got.ComparedSeconds)
	}
	if got.PearsonR < 0.80 {
		t.Fatalf("pearsonR=%.4f want >= 0.80 (meanAct=%.1f meanEst=%.1f rmse=%.1f)",
			got.PearsonR, got.MeanActual, got.MeanEstimated, got.RMSE)
	}
	if got.SpearmanRho < 0.80 {
		t.Fatalf("spearmanRho=%.4f want >= 0.80", got.SpearmanRho)
	}
	meanAct := got.MeanActual
	if meanAct < 1 {
		meanAct = 1
	}
	biasOK := math.Abs(got.Bias) <= 20 || math.Abs(got.Bias)/meanAct <= 0.10
	if !biasOK {
		t.Fatalf("|bias|=%.2f (meanAct=%.1f) want |bias|<=20 or relative <=0.10", got.Bias, got.MeanActual)
	}
	if got.Scores.ResidualZMedianAbs > 1.5 {
		t.Fatalf("residualZMedianAbs=%.3f want <= 1.5", got.Scores.ResidualZMedianAbs)
	}
	if got.Scores.RobustRMSE > got.RMSE+1e-9 {
		t.Fatalf("robustRmse=%.2f should be <= rmse=%.2f", got.Scores.RobustRMSE, got.RMSE)
	}
}

func TestBacktestOutdoorRealisticMaskSecondHalfMeetsScoreGate(t *testing.T) {
	t.Parallel()

	// Outdoor-shaped fixture (GPS noise, density series, mild wind) — not pure lab physics.
	// Acceptance: multi-metric PM-death replay scores on held-out second half.
	fix := buildOutdoorRealisticBacktestFixture(2400, true)
	params := fix.params
	params.CdA = icu.LabeledParam{} // force calibration from measured first half

	got := icu.BacktestPowerEstimate(icu.PowerBacktestRequest{
		ActivityID:            "fixture-outdoor-realistic",
		Streams:               fix.streams,
		Params:                params,
		HeadwindMSSeries:      fix.headwindMSSeries,
		RhoSeries:             fix.rhoSeries,
		CalibrateFromMeasured: true,
		Mode:                  icu.PowerBacktestMaskSecondHalf,
		ActivityType:          "Ride",
		DistanceMeters:        fix.distanceMeters,
	})
	assertOutdoorPMDeathGate(t, got)
}

func TestBacktestKnownWindImprovesScoresVsStillAir(t *testing.T) {
	t.Parallel()

	// Phase-varying headwind with the same distribution both halves. Still-air
	// absorbs mean aero into CdA but cannot track wind phase; the planted series
	// recovers correlation/error — proves aero inputs move scores without CFD.
	fix := buildOutdoorBacktestFixture(2400, outdoorFixtureOpts{
		PhaseWind: true,
		LowNoise:  true,
	})
	params := fix.params
	params.CdA = icu.LabeledParam{}

	withWind := icu.BacktestPowerEstimate(icu.PowerBacktestRequest{
		ActivityID:            "fixture-known-wind",
		Streams:               fix.streams,
		Params:                params,
		HeadwindMSSeries:      fix.headwindMSSeries,
		RhoSeries:             fix.rhoSeries,
		CalibrateFromMeasured: true,
		Mode:                  icu.PowerBacktestMaskSecondHalf,
		ActivityType:          "Ride",
		DistanceMeters:        fix.distanceMeters,
	})
	if withWind.BlockingError != "" {
		t.Fatalf("with wind blocking: %s", withWind.BlockingError)
	}

	// Same streams/params/rho; no headwind series (still-air aero only).
	stillAir := icu.BacktestPowerEstimate(icu.PowerBacktestRequest{
		ActivityID:            "fixture-still-air",
		Streams:               fix.streams,
		Params:                params,
		RhoSeries:             fix.rhoSeries,
		CalibrateFromMeasured: true,
		Mode:                  icu.PowerBacktestMaskSecondHalf,
		ActivityType:          "Ride",
		DistanceMeters:        fix.distanceMeters,
	})
	if stillAir.BlockingError != "" {
		t.Fatalf("still air blocking: %s", stillAir.BlockingError)
	}

	betterPearson := withWind.PearsonR > stillAir.PearsonR
	betterRobust := withWind.Scores.RobustRMSE < stillAir.Scores.RobustRMSE
	if !betterPearson && !betterRobust {
		t.Fatalf(
			"known wind should improve pearsonR or robustRmse: withWind pearson=%.4f robustRmse=%.2f; stillAir pearson=%.4f robustRmse=%.2f",
			withWind.PearsonR, withWind.Scores.RobustRMSE,
			stillAir.PearsonR, stillAir.Scores.RobustRMSE,
		)
	}
	// Wind case must itself clear the outdoor PM-death gate.
	assertOutdoorPMDeathGate(t, withWind)
}
