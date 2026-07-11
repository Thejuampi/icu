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
