package icu_test

import (
	"math"
	"strings"
	"testing"

	icu "github.com/Thejuampi/icu"
)

func TestVirtualPowerFlatSteadyIsAeroPlusRoll(t *testing.T) {
	t.Parallel()

	params := baseParams()
	speedMS := 10.0
	massKg := params.RiderMassKg.Value + params.BikeMassKg.Value
	gravity := 9.80665
	wantWheel := params.Crr.Value*massKg*gravity*speedMS + 0.5*params.AirDensity.Value*params.CdA.Value*speedMS*speedMS*speedMS
	want := wantWheel / params.DrivetrainEff.Value

	streams := icu.NullableStreamData{
		"watts":           seriesFrom([]float64{0}, []bool{true}),
		"cadence":         seriesFrom([]float64{0}, []bool{false}),
		"velocity_smooth": seriesFrom([]float64{speedMS}, nil),
		"time":            seriesFrom([]float64{0}, nil),
		"distance":        seriesFrom([]float64{0}, nil),
		// Sea-level altitude so stream density matches params.AirDensity (1.225).
		"altitude": seriesFrom([]float64{0}, nil),
	}
	got := icu.EstimateAndFillPower(icu.PowerFillRequest{
		Streams:        streams,
		Params:         params,
		IncludeStreams: true,
	})
	if got.BlockingError != "" {
		t.Fatalf("blocking: %s", got.BlockingError)
	}
	if got.Fill.EstimatedSeconds != 1 {
		t.Fatalf("estimated=%d labels=%v", got.Fill.EstimatedSeconds, got.Classification.Labels)
	}
	if math.Abs(got.FilledWatts[0]-want) > 0.5 {
		t.Fatalf("est=%.2f want=%.2f", got.FilledWatts[0], want)
	}
}

func TestEstimatePreservesMeasuredAndTrueZero(t *testing.T) {
	t.Parallel()

	params := baseParams()
	streams := icu.NullableStreamData{
		"watts":           seriesFrom([]float64{200, 0, 0}, nil),
		"cadence":         seriesFrom([]float64{90, 0, 0}, []bool{true, true, false}),
		"velocity_smooth": seriesFrom([]float64{8, 8, 8}, nil),
		"time":            seriesFrom([]float64{0, 1, 2}, nil),
		"distance":        seriesFrom([]float64{0, 8, 16}, nil),
		"altitude":        seriesFrom([]float64{0, 0, 0}, nil),
	}
	got := icu.EstimateAndFillPower(icu.PowerFillRequest{
		Streams:        streams,
		Params:         params,
		IncludeStreams: true,
	})
	if got.SampleSource[0] != icu.PowerSampleMeasured {
		t.Fatalf("src0=%s", got.SampleSource[0])
	}
	if got.FilledWatts[0] != 200 {
		t.Fatalf("measured overwritten: %v", got.FilledWatts[0])
	}
	if got.SampleSource[1] != icu.PowerSampleTrueZero || got.FilledWatts[1] != 0 {
		t.Fatalf("coasting not true_zero: src=%s w=%v", got.SampleSource[1], got.FilledWatts[1])
	}
	if !strings.HasPrefix(got.SampleSource[2], "estimated") {
		t.Fatalf("gap src=%s want estimated*", got.SampleSource[2])
	}
	if got.FilledWatts[2] <= 0 {
		t.Fatalf("gap estimate should be >0, got %v", got.FilledWatts[2])
	}
}

func TestCalibrateCrrFromLowSpeedClimbs(t *testing.T) {
	t.Parallel()

	params := baseParams()
	params.Crr = icu.LabeledParam{} // force Crr calibration
	// Keep CdA known so calibration focuses on rolling residual.
	const n = 240
	watts := make([]float64, n)
	cad := make([]float64, n)
	speed := make([]float64, n)
	dist := make([]float64, n)
	alt := make([]float64, n)
	times := make([]float64, n)
	// Mix: low-speed climbs for Crr + flat for CdA path stability.
	mass := params.RiderMassKg.Value + params.BikeMassKg.Value
	g := 9.80665
	crrPlant := 0.005
	cdaPlant := params.CdA.Value
	rho := params.AirDensity.Value
	eta := params.DrivetrainEff.Value
	cadPresent := make([]bool, n)
	wattsPresent := make([]bool, n)
	for i := range n {
		times[i] = float64(i)
		cad[i] = 75
		cadPresent[i] = true
		wattsPresent[i] = true
		var speedMS, grade float64
		if i < 120 {
			speedMS = 3.0
			grade = 0.08
		} else {
			speedMS = 8.0
			grade = 0.0
		}
		speed[i] = speedMS
		if i == 0 {
			dist[i] = 0
			alt[i] = 0
		} else {
			dist[i] = dist[i-1] + speedMS
			alt[i] = alt[i-1] + grade*speedMS
		}
		denom := math.Sqrt(1 + grade*grade)
		pGrav := mass * g * speedMS * (grade / denom)
		pRoll := crrPlant * mass * g * speedMS / denom
		pAero := 0.5 * rho * cdaPlant * speedMS * speedMS * speedMS
		watts[i] = (pGrav + pRoll + pAero) / eta
	}
	// Gap at end.
	for i := n - 30; i < n; i++ {
		watts[i] = 0
		wattsPresent[i] = false
		cad[i] = 0
		cadPresent[i] = false
	}
	streams := icu.NullableStreamData{
		"watts":           seriesFrom(watts, wattsPresent),
		"cadence":         seriesFrom(cad, cadPresent),
		"velocity_smooth": seriesFrom(speed, nil),
		"distance":        seriesFrom(dist, nil),
		"altitude":        seriesFrom(alt, nil),
		"time":            seriesFrom(times, nil),
	}
	got := icu.EstimateAndFillPower(icu.PowerFillRequest{
		Streams:               streams,
		Params:                params,
		CalibrateFromMeasured: true,
		IncludeStreams:        true,
	})
	if got.BlockingError != "" {
		t.Fatalf("blocking: %s", got.BlockingError)
	}
	if got.Model.Parameters.Crr.Value <= 0 {
		t.Fatalf("Crr not set: %+v", got.Model.Parameters.Crr)
	}
}

func TestEstimateWithAeroHeadwindAndDensity(t *testing.T) {
	t.Parallel()

	params := baseParams()
	streams := singleMissingStream(10, 10, 0)
	got := icu.EstimateAndFillPower(icu.PowerFillRequest{
		Streams: streams,
		Params:  params,
		Aero: icu.PowerAeroInputs{
			MeanAltitudeM:   100,
			MeanTempC:       20,
			WindSpeed:       18,
			WindSpeedIsKmh:  true,
			HeadwindPercent: 80,
			TailwindPercent: 10,
		},
		IncludeStreams: true,
	})
	if got.BlockingError != "" {
		t.Fatalf("blocking: %s", got.BlockingError)
	}
	// Headwind should be derived into params.
	if got.Model.Parameters.HeadwindMS.Value == 0 && got.Model.Parameters.AirDensity.Value <= 0 {
		// At least density or headwind should appear in warnings/params.
		found := false
		for _, w := range got.Warnings {
			if strings.Contains(w, "headwind") || strings.Contains(w, "density") || strings.Contains(w, "ρ") {
				found = true
			}
		}
		if !found {
			t.Fatalf("expected aero weather warnings, got %+v params=%+v", got.Warnings, got.Model.Parameters)
		}
	}
}

func TestCalibrateCdARecoversPlantedValue(t *testing.T) {
	t.Parallel()

	planted := 0.32
	params := baseParams()
	params.CdA = icu.LabeledParam{}
	const measuredCount = 120
	const gapCount = 40
	total := measuredCount + gapCount
	watts := make([]float64, total)
	cad := make([]float64, total)
	speed := make([]float64, total)
	dist := make([]float64, total)
	alt := make([]float64, total)
	times := make([]float64, total)
	cadPresent := make([]bool, total)
	speedMS := 8.0
	massKg := params.RiderMassKg.Value + params.BikeMassKg.Value
	gravity := 9.80665
	for index := range total {
		times[index] = float64(index)
		speed[index] = speedMS
		dist[index] = speedMS * float64(index)
		alt[index] = 100
		if index < measuredCount {
			cad[index] = 85
			cadPresent[index] = true
			pWheel := params.Crr.Value*massKg*gravity*speedMS + 0.5*params.AirDensity.Value*planted*speedMS*speedMS*speedMS
			watts[index] = pWheel / params.DrivetrainEff.Value
			continue
		}
		watts[index] = 0
		cadPresent[index] = false
	}

	streams := icu.NullableStreamData{
		"watts":           seriesFrom(watts, nil),
		"cadence":         seriesFrom(cad, cadPresent),
		"velocity_smooth": seriesFrom(speed, nil),
		"time":            seriesFrom(times, nil),
		"distance":        seriesFrom(dist, nil),
		"altitude":        seriesFrom(alt, nil),
	}
	got := icu.EstimateAndFillPower(icu.PowerFillRequest{
		Streams:               streams,
		Params:                params,
		CalibrateFromMeasured: true,
		IncludeStreams:        true,
		FTP:                   285,
		FTPSource:             "test",
	})
	if got.BlockingError != "" {
		t.Fatalf("blocking: %s", got.BlockingError)
	}
	if math.Abs(got.Model.Parameters.CdA.Value-planted) > 0.02 {
		t.Fatalf("CdA=%.4f want ~%.2f", got.Model.Parameters.CdA.Value, planted)
	}
	if got.Model.Parameters.CdA.Source != "calibrated" {
		t.Fatalf("source=%s", got.Model.Parameters.CdA.Source)
	}
	if got.Fill.EstimatedSeconds < 30 {
		t.Fatalf("estimated=%d", got.Fill.EstimatedSeconds)
	}
	if got.Metrics.After.EstimatedTrainingLoad <= 0 {
		t.Fatalf("expected TSS after fill, got %v", got.Metrics.After.EstimatedTrainingLoad)
	}
}

func TestEstimateRequiresMass(t *testing.T) {
	t.Parallel()

	params := baseParams()
	params.RiderMassKg.Value = 0
	got := icu.EstimateAndFillPower(icu.PowerFillRequest{
		Streams: icu.NullableStreamData{
			"watts": seriesFrom([]float64{0}, nil),
		},
		Params: params,
	})
	if got.BlockingError == "" {
		t.Fatal("expected blocking error for missing mass")
	}
}

func TestEstimateRequiresCdAWithoutCalibration(t *testing.T) {
	t.Parallel()

	params := baseParams()
	params.CdA = icu.LabeledParam{}
	got := icu.EstimateAndFillPower(icu.PowerFillRequest{
		Streams: icu.NullableStreamData{
			"watts":           seriesFrom([]float64{0}, nil),
			"cadence":         seriesFrom([]float64{0}, []bool{false}),
			"velocity_smooth": seriesFrom([]float64{8}, nil),
		},
		Params: params,
	})
	if got.BlockingError == "" {
		t.Fatal("expected CdA blocking error")
	}
}

func TestClimbIncreasesEstimatedPower(t *testing.T) {
	t.Parallel()

	params := baseParams()
	flat := icu.EstimateAndFillPower(icu.PowerFillRequest{
		Streams: singleMissingStream(10, 0, 0),
		Params:  params,
	})
	climb := icu.EstimateAndFillPower(icu.PowerFillRequest{
		Streams: singleMissingStream(10, 10, 0.5),
		Params:  params,
	})
	if climb.FilledWatts[1] <= flat.FilledWatts[1] {
		t.Fatalf("climb est=%.1f should exceed flat=%.1f", climb.FilledWatts[1], flat.FilledWatts[1])
	}
}

func TestBuildAcceptWattsStream(t *testing.T) {
	t.Parallel()

	if icu.BuildAcceptWattsStream(nil) != nil {
		t.Fatal("nil result should yield nil")
	}
	result := &icu.PowerFillResult{FilledWatts: []float64{1, 2, 3}}
	out := icu.BuildAcceptWattsStream(result)
	if len(out) != 3 || out[1] != 2 {
		t.Fatalf("out=%v", out)
	}
}

func TestHRCrossCheckWarnsOnDivergence(t *testing.T) {
	t.Parallel()

	// Measured: very low W/HR. Gap: high HR with physics that still exceeds HR-implied
	// after residual scale / envelope (use mild speed so envelope does not crush estimates).
	const measured = 80
	const gap = 80
	total := measured + gap
	watts := make([]float64, total)
	cad := make([]float64, total)
	cadPresent := make([]bool, total)
	speed := make([]float64, total)
	dist := make([]float64, total)
	alt := make([]float64, total)
	times := make([]float64, total)
	hr := make([]float64, total)
	for index := range total {
		times[index] = float64(index)
		alt[index] = 100
		speed[index] = 8
		dist[index] = 8 * float64(index)
		if index < measured {
			watts[index] = 100
			cad[index] = 90
			cadPresent[index] = true
			hr[index] = 160 // low W/HR
			continue
		}
		watts[index] = 0
		cadPresent[index] = false
		hr[index] = 110 // lower HR → low implied watts vs physics
	}
	params := baseParams()
	params.CdA = icu.LabeledParam{Value: 0.40, Source: "user"}
	got := icu.EstimateAndFillPower(icu.PowerFillRequest{
		Streams: icu.NullableStreamData{
			"watts":           seriesFrom(watts, nil),
			"cadence":         seriesFrom(cad, cadPresent),
			"velocity_smooth": seriesFrom(speed, nil),
			"distance":        seriesFrom(dist, nil),
			"altitude":        seriesFrom(alt, nil),
			"time":            seriesFrom(times, nil),
			"heartrate":       seriesFrom(hr, nil),
		},
		Params:         params,
		IncludeStreams: true,
	})
	found := false
	for _, warning := range got.Warnings {
		if strings.Contains(warning, "HR-implied") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected HR-implied warning, got %v meanEst=%.1f", got.Warnings, got.Fill.MeanEstimatedWatts)
	}
}

func TestEstimateDecelClampsNegativePower(t *testing.T) {
	t.Parallel()

	// Hard decel should not produce negative filled watts.
	params := baseParams()
	got := icu.EstimateAndFillPower(icu.PowerFillRequest{
		Streams: icu.NullableStreamData{
			"watts":           seriesFrom([]float64{0, 0}, nil),
			"cadence":         seriesFrom([]float64{0, 0}, []bool{false, false}),
			"velocity_smooth": seriesFrom([]float64{15, 1}, nil),
			"time":            seriesFrom([]float64{0, 1}, nil),
			"distance":        seriesFrom([]float64{0, 8}, nil),
			"altitude":        seriesFrom([]float64{0, 0}, nil),
		},
		Params:         params,
		IncludeStreams: true,
	})
	for index, value := range got.FilledWatts {
		if value < 0 {
			t.Fatalf("index %d negative watts %v", index, value)
		}
	}
}

func TestEstimateMissingParamErrors(t *testing.T) {
	t.Parallel()

	streams := icu.NullableStreamData{
		"watts": seriesFrom([]float64{100}, nil),
	}
	labeled := func(value float64) icu.LabeledParam {
		return icu.LabeledParam{Value: value, Source: "user"}
	}
	cases := []icu.PowerModelParams{
		{BikeMassKg: labeled(9), Crr: labeled(0.004), DrivetrainEff: labeled(0.97), CdA: labeled(0.3)},
		{RiderMassKg: labeled(75), Crr: labeled(0.004), DrivetrainEff: labeled(0.97), CdA: labeled(0.3)},
		{RiderMassKg: labeled(75), BikeMassKg: labeled(9), DrivetrainEff: labeled(0.97), CdA: labeled(0.3)},
		{RiderMassKg: labeled(75), BikeMassKg: labeled(9), Crr: labeled(0.004), CdA: labeled(0.3)},
	}
	for _, params := range cases {
		got := icu.EstimateAndFillPower(icu.PowerFillRequest{Streams: streams, Params: params})
		if got.BlockingError == "" {
			t.Fatalf("expected blocking error for params %+v", params)
		}
	}
}

func TestEstimateMinGapWarningAndDefaultAirDensity(t *testing.T) {
	t.Parallel()

	params := baseParams()
	params.AirDensity = icu.LabeledParam{} // trigger ISA default
	got := icu.EstimateAndFillPower(icu.PowerFillRequest{
		Streams: icu.NullableStreamData{
			"watts":           seriesFrom([]float64{0, 0, 0}, nil),
			"cadence":         seriesFrom([]float64{0, 0, 0}, []bool{false, false, false}),
			"velocity_smooth": seriesFrom([]float64{8, 8, 8}, nil),
			"time":            seriesFrom([]float64{0, 1, 2}, nil),
			"distance":        seriesFrom([]float64{0, 8, 16}, nil),
			"altitude":        seriesFrom([]float64{0, 0, 0}, nil),
		},
		Params:        params,
		MinGapSeconds: 100,
	})
	if got.Model.Parameters.AirDensity.Source != "isa_sea_level" {
		t.Fatalf("air density source=%s", got.Model.Parameters.AirDensity.Source)
	}
	found := false
	for _, warning := range got.Warnings {
		if strings.Contains(warning, "min-gap-seconds") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected min-gap warning, got %v", got.Warnings)
	}
}

func TestCalibrateFailsWithTooFewSamples(t *testing.T) {
	t.Parallel()

	params := baseParams()
	params.CdA = icu.LabeledParam{}
	got := icu.EstimateAndFillPower(icu.PowerFillRequest{
		Streams: icu.NullableStreamData{
			"watts":           seriesFrom([]float64{200, 0}, nil),
			"cadence":         seriesFrom([]float64{90, 0}, []bool{true, false}),
			"velocity_smooth": seriesFrom([]float64{8, 8}, nil),
			"time":            seriesFrom([]float64{0, 1}, nil),
			"distance":        seriesFrom([]float64{0, 8}, nil),
			"altitude":        seriesFrom([]float64{0, 0}, nil),
		},
		Params:                params,
		CalibrateFromMeasured: true,
	})
	if got.BlockingError == "" {
		t.Fatal("expected calibration/CdA blocking error")
	}
}

func TestEstimateEmptyStreamsBlocking(t *testing.T) {
	t.Parallel()

	got := icu.EstimateAndFillPower(icu.PowerFillRequest{
		Params: baseParams(),
	})
	if got.BlockingError == "" {
		t.Fatal("expected empty streams blocking error")
	}
}

func TestOutdoorEnsembleWithHRAndVaryingTerrain(t *testing.T) {
	t.Parallel()

	// Long enough measured half with HR variation so multi-linear + blend train.
	const n = 600
	params := baseParams()
	params.CdA = icu.LabeledParam{}
	watts := make([]float64, n)
	cad := make([]float64, n)
	speed := make([]float64, n)
	dist := make([]float64, n)
	alt := make([]float64, n)
	times := make([]float64, n)
	hr := make([]float64, n)
	cadPresent := make([]bool, n)
	wattsPresent := make([]bool, n)
	mass := params.RiderMassKg.Value + params.BikeMassKg.Value
	g := 9.80665
	cda := 0.32
	crr := params.Crr.Value
	rho := params.AirDensity.Value
	eta := params.DrivetrainEff.Value
	for i := range n {
		times[i] = float64(i)
		// Vary speed/grade/HR across the ride.
		speed[i] = 6 + 3*math.Sin(float64(i)/40)
		grade := 0.02 * math.Sin(float64(i)/55)
		if i == 0 {
			dist[i] = 0
			alt[i] = 0
		} else {
			dist[i] = dist[i-1] + speed[i]
			alt[i] = alt[i-1] + grade*speed[i]
		}
		hr[i] = 130 + 25*math.Sin(float64(i)/70)
		denom := math.Sqrt(1 + grade*grade)
		pGrav := mass * g * speed[i] * (grade / denom)
		pRoll := crr * mass * g * speed[i] / denom
		pAero := 0.5 * rho * cda * speed[i] * speed[i] * speed[i]
		// Small HR-correlated residual so multi-linear has signal.
		extra := 0.15 * (hr[i] - 140)
		watts[i] = (pGrav+pRoll+pAero)/eta + extra
		if watts[i] < 0 {
			watts[i] = 0
		}
		cad[i] = 80
		cadPresent[i] = true
		wattsPresent[i] = true
		if i >= n/2 {
			// PM death second half.
			watts[i] = 0
			wattsPresent[i] = false
			cad[i] = 0
			cadPresent[i] = false
		}
	}
	// A few true zeros in measured half.
	for i := 20; i < 30; i++ {
		watts[i] = 0
		cad[i] = 0
	}
	streams := icu.NullableStreamData{
		"watts":           seriesFrom(watts, wattsPresent),
		"cadence":         seriesFrom(cad, cadPresent),
		"velocity_smooth": seriesFrom(speed, nil),
		"distance":        seriesFrom(dist, nil),
		"altitude":        seriesFrom(alt, nil),
		"time":            seriesFrom(times, nil),
		"heartrate":       seriesFrom(hr, nil),
	}
	// Second half headwind series.
	hw := make([]float64, n)
	for i := range n {
		hw[i] = 1.5 * math.Sin(float64(i)/90)
	}
	got := icu.EstimateAndFillPower(icu.PowerFillRequest{
		Streams:               streams,
		Params:                params,
		HeadwindMSSeries:      hw,
		CalibrateFromMeasured: true,
		IncludeStreams:        true,
		FTP:                   280,
	})
	if got.BlockingError != "" {
		t.Fatalf("blocking: %s", got.BlockingError)
	}
	if got.Fill.EstimatedSeconds < 100 {
		t.Fatalf("estimated=%d", got.Fill.EstimatedSeconds)
	}
	// Expect outdoor model engaged.
	if got.Model.Name == "" {
		t.Fatal("empty model name")
	}
}

func TestEstimateUsesVelocityAlias(t *testing.T) {
	t.Parallel()

	got := icu.EstimateAndFillPower(icu.PowerFillRequest{
		Streams: icu.NullableStreamData{
			"watts":    seriesFrom([]float64{0}, nil),
			"cadence":  seriesFrom([]float64{0}, []bool{false}),
			"velocity": seriesFrom([]float64{9}, nil),
			"time":     seriesFrom([]float64{0}, nil),
			"distance": seriesFrom([]float64{0}, nil),
			"altitude": seriesFrom([]float64{0}, nil),
		},
		Params:         baseParams(),
		IncludeStreams: true,
	})
	if got.Fill.EstimatedSeconds != 1 || got.FilledWatts[0] <= 0 {
		t.Fatalf("fill=%+v watts=%v", got.Fill, got.FilledWatts)
	}
}

func TestEstimateInvalidDrivetrainEff(t *testing.T) {
	t.Parallel()

	params := baseParams()
	params.DrivetrainEff.Value = 1.5
	got := icu.EstimateAndFillPower(icu.PowerFillRequest{
		Streams: icu.NullableStreamData{"watts": seriesFrom([]float64{1}, nil)},
		Params:  params,
	})
	if got.BlockingError == "" {
		t.Fatal("expected drivetrain error")
	}
}

func TestEstimateUserAirDensityAndSources(t *testing.T) {
	t.Parallel()

	params := baseParams()
	params.AirDensity = icu.LabeledParam{Value: 1.1} // source empty → user
	params.RiderMassKg.Source = ""
	params.BikeMassKg.Source = ""
	params.Crr.Source = ""
	params.DrivetrainEff.Source = ""
	params.CdA.Source = ""
	got := icu.EstimateAndFillPower(icu.PowerFillRequest{
		Streams: icu.NullableStreamData{
			"watts":           seriesFrom([]float64{0}, nil),
			"cadence":         seriesFrom([]float64{0}, []bool{false}),
			"velocity_smooth": seriesFrom([]float64{7}, nil),
			"time":            seriesFrom([]float64{0}, nil),
			"distance":        seriesFrom([]float64{0}, nil),
			"altitude":        seriesFrom([]float64{10}, nil),
		},
		Params: params,
	})
	if got.BlockingError != "" {
		t.Fatalf("blocking: %s", got.BlockingError)
	}
	if got.Model.Parameters.AirDensity.Source != "user" {
		t.Fatalf("air source=%s", got.Model.Parameters.AirDensity.Source)
	}
	if got.Model.Parameters.CdA.Source != "user" {
		t.Fatalf("cda source=%s", got.Model.Parameters.CdA.Source)
	}
}

func TestEstimateNegativeSpeedTreatedAsZero(t *testing.T) {
	t.Parallel()

	got := icu.EstimateAndFillPower(icu.PowerFillRequest{
		Streams: icu.NullableStreamData{
			"watts":           seriesFrom([]float64{0}, nil),
			"cadence":         seriesFrom([]float64{0}, []bool{false}),
			"velocity_smooth": seriesFrom([]float64{-3}, nil),
			"time":            seriesFrom([]float64{0}, nil),
			"distance":        seriesFrom([]float64{0}, nil),
			"altitude":        seriesFrom([]float64{0}, nil),
		},
		Params:         baseParams(),
		IncludeStreams: true,
	})
	if got.FilledWatts[0] < 0 {
		t.Fatalf("negative filled %v", got.FilledWatts[0])
	}
}

func TestEstimateEnvelopeCapsLongEfforts(t *testing.T) {
	t.Parallel()

	// Measured half: steady 220 W. Gap: high speed flat physics would invent ~400 W+.
	// Envelope must keep long estimated averages near measured MMP.
	const measuredN = 800
	const gapN = 800
	total := measuredN + gapN
	watts := make([]float64, total)
	cad := make([]float64, total)
	cadPresent := make([]bool, total)
	speed := make([]float64, total)
	dist := make([]float64, total)
	alt := make([]float64, total)
	times := make([]float64, total)
	for index := range total {
		times[index] = float64(index)
		alt[index] = 100
		if index < measuredN {
			watts[index] = 220
			cad[index] = 85
			cadPresent[index] = true
			speed[index] = 7
			dist[index] = 7 * float64(index)
			continue
		}
		watts[index] = 0
		cadPresent[index] = false
		speed[index] = 12 // fast gap → high aero without envelope
		dist[index] = 7*float64(measuredN) + 12*float64(index-measuredN)
	}
	params := baseParams()
	params.CdA = icu.LabeledParam{Value: 0.45, Source: "user"}
	got := icu.EstimateAndFillPower(icu.PowerFillRequest{
		Streams: icu.NullableStreamData{
			"watts":           seriesFrom(watts, nil),
			"cadence":         seriesFrom(cad, cadPresent),
			"velocity_smooth": seriesFrom(speed, nil),
			"distance":        seriesFrom(dist, nil),
			"altitude":        seriesFrom(alt, nil),
			"time":            seriesFrom(times, nil),
		},
		Params:         params,
		IncludeStreams: true,
	})
	if got.BlockingError != "" {
		t.Fatalf("blocking: %s", got.BlockingError)
	}
	// Best ~12 min mean in gap must stay near measured 220, not 350+.
	best := bestRollingMean(got.FilledWatts, times, 720)
	if best > 250 {
		t.Fatalf("best 12min mean=%.1f still overestimated (want <=250 near measured 220)", best)
	}
	if got.Fill.MeanEstimatedWatts > 280 {
		t.Fatalf("mean estimated=%.1f still too high", got.Fill.MeanEstimatedWatts)
	}
	if !got.Model.Calibration.EnvelopeApplied && best > 230 {
		// Envelope should fire when physics alone invents high long efforts.
		t.Fatalf("expected measured MMP envelope to apply, cal=%+v best=%.1f", got.Model.Calibration, best)
	}
}

func bestRollingMean(values, times []float64, duration float64) float64 {
	left := 0
	sum := 0.0
	best := 0.0
	for right := range values {
		sum += values[right]
		for left < right && times[right]-times[left] > duration {
			sum -= values[left]
			left++
		}
		if times[right]-times[left] >= duration*0.95 {
			avg := sum / float64(right-left+1)
			if avg > best {
				best = avg
			}
		}
	}

	return best
}

func TestEstimateCalibrationReportsGuards(t *testing.T) {
	t.Parallel()

	// Enough measured samples for residual scale + cap metadata.
	const measuredN = 120
	const gapN = 40
	total := measuredN + gapN
	watts := make([]float64, total)
	cad := make([]float64, total)
	cadPresent := make([]bool, total)
	speed := make([]float64, total)
	dist := make([]float64, total)
	alt := make([]float64, total)
	times := make([]float64, total)
	for index := range total {
		times[index] = float64(index)
		speed[index] = 8
		dist[index] = 8 * float64(index)
		alt[index] = 100
		if index < measuredN {
			watts[index] = 200
			cad[index] = 85
			cadPresent[index] = true
			continue
		}
		watts[index] = 0
		cadPresent[index] = false
	}
	params := baseParams()
	params.CdA = icu.LabeledParam{}
	got := icu.EstimateAndFillPower(icu.PowerFillRequest{
		Streams: icu.NullableStreamData{
			"watts":           seriesFrom(watts, nil),
			"cadence":         seriesFrom(cad, cadPresent),
			"velocity_smooth": seriesFrom(speed, nil),
			"distance":        seriesFrom(dist, nil),
			"altitude":        seriesFrom(alt, nil),
			"time":            seriesFrom(times, nil),
		},
		Params:                params,
		CalibrateFromMeasured: true,
		IncludeStreams:        true,
	})
	if got.BlockingError != "" {
		t.Fatalf("blocking: %s", got.BlockingError)
	}
	if got.Model.Calibration.InstantCapWatts <= 0 {
		t.Fatalf("expected instant cap, got %+v", got.Model.Calibration)
	}
	// ResidualScale / linear gains are optional when multi-linear fit is weak;
	// calibrated CdA + filled estimates are the hard requirements.
	if got.Model.Parameters.CdA.Value <= 0 {
		t.Fatalf("expected CdA, got %+v", got.Model.Parameters.CdA)
	}
	// No estimated sample above measured cap.
	for index, source := range got.SampleSource {
		if source == "estimated" && got.FilledWatts[index] > got.Model.Calibration.InstantCapWatts+1 {
			t.Fatalf("estimated sample %d = %.1f above cap %.1f", index, got.FilledWatts[index], got.Model.Calibration.InstantCapWatts)
		}
	}
}

func TestPreserveNullableAliases(t *testing.T) {
	t.Parallel()

	streams := icu.NullableStreamData{
		"power":      seriesFrom([]float64{100}, nil),
		"heart_rate": seriesFrom([]float64{140}, nil),
		"velocity":   seriesFrom([]float64{7}, nil),
	}
	if v, ok := icu.NullableStream(streams, "watts").At(0); !ok || v != 100 {
		t.Fatalf("watts alias failed")
	}
	if v, ok := icu.NullableStream(streams, "heartrate").At(0); !ok || v != 140 {
		t.Fatalf("hr alias failed")
	}
	if v, ok := icu.NullableStream(streams, "velocity_smooth").At(0); !ok || v != 7 {
		t.Fatalf("velocity alias failed")
	}
}

func baseParams() icu.PowerModelParams {
	return icu.PowerModelParams{
		RiderMassKg:   icu.LabeledParam{Value: 75, Source: "user"},
		BikeMassKg:    icu.LabeledParam{Value: 9, Source: "user"},
		CdA:           icu.LabeledParam{Value: 0.35, Source: "user"},
		Crr:           icu.LabeledParam{Value: 0.0045, Source: "user"},
		DrivetrainEff: icu.LabeledParam{Value: 0.975, Source: "user"},
		AirDensity:    icu.LabeledParam{Value: 1.2, Source: "user"},
	}
}

func singleMissingStream(speedMS, dist1, alt1 float64) icu.NullableStreamData {
	return icu.NullableStreamData{
		"watts":           seriesFrom([]float64{0, 0}, nil),
		"cadence":         seriesFrom([]float64{0, 0}, []bool{false, false}),
		"velocity_smooth": seriesFrom([]float64{speedMS, speedMS}, nil),
		"time":            seriesFrom([]float64{0, 1}, nil),
		"distance":        seriesFrom([]float64{0, dist1}, nil),
		"altitude":        seriesFrom([]float64{0, alt1}, nil),
	}
}

func TestEstimateWithExplicitRhoAndHeadwindSeries(t *testing.T) {
	t.Parallel()

	params := baseParams()
	const n = 40
	watts := make([]float64, n)
	cad := make([]float64, n)
	speed := make([]float64, n)
	dist := make([]float64, n)
	alt := make([]float64, n)
	times := make([]float64, n)
	wp := make([]bool, n)
	cp := make([]bool, n)
	rho := make([]float64, n)
	hw := make([]float64, n)
	for i := range n {
		times[i] = float64(i)
		speed[i] = 9
		dist[i] = 9 * float64(i)
		alt[i] = 100
		rho[i] = 1.15
		hw[i] = 2
		if i < 25 {
			watts[i] = 220
			cad[i] = 90
			wp[i] = true
			cp[i] = true
		} else {
			wp[i] = false
			cp[i] = false
		}
	}
	streams := icu.NullableStreamData{
		"watts":           seriesFrom(watts, wp),
		"cadence":         seriesFrom(cad, cp),
		"velocity_smooth": seriesFrom(speed, nil),
		"distance":        seriesFrom(dist, nil),
		"altitude":        seriesFrom(alt, nil),
		"time":            seriesFrom(times, nil),
	}
	got := icu.EstimateAndFillPower(icu.PowerFillRequest{
		Streams:               streams,
		Params:                params,
		RhoSeries:             rho,
		HeadwindMSSeries:      hw,
		CalibrateFromMeasured: true,
		IncludeStreams:        true,
		FTP:                   280,
		FTPSource:             "test",
		MinGapSeconds:         1,
	})
	if got.BlockingError != "" {
		t.Fatalf("blocking: %s", got.BlockingError)
	}
	if got.Fill.EstimatedSeconds < 5 {
		t.Fatalf("estimated=%d", got.Fill.EstimatedSeconds)
	}
	if got.Metrics.After.FTP != 280 {
		t.Fatalf("ftp=%d", got.Metrics.After.FTP)
	}
}
