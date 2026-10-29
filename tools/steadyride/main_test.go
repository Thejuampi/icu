package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// The committed fixture is exactly the generator's output.
func TestGeneratorReproducesCommittedFixture(t *testing.T) {
	t.Parallel()
	activity, streams, err := Generate()
	if err != nil {
		t.Fatal(err)
	}
	base := filepath.Join("..", "..", "testdata", "fixtures", "api", "v1", "activity")
	for path, want := range map[string][]byte{
		filepath.Join(base, activityID+".json"):         activity,
		filepath.Join(base, activityID, "streams.json"): streams,
	} {
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("%s differs from the generator output: run go run ./tools/steadyride", path)
		}
	}
}

func TestRunWritesFixtures(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if code := run([]string{"-fixtures", dir}, io.Discard); code != 0 {
		t.Fatalf("exit %d", code)
	}
	if _, err := os.Stat(filepath.Join(dir, "api", "v1", "activity", activityID, "streams.json")); err != nil {
		t.Fatal(err)
	}
	if code := run([]string{"-bogus"}, io.Discard); code != 2 {
		t.Fatalf("bad flag exit %d, want 2", code)
	}
	blocker := filepath.Join(dir, "file")
	if err := os.WriteFile(blocker, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if code := run([]string{"-fixtures", blocker}, io.Discard); code != 1 {
		t.Fatalf("unwritable dir exit %d, want 1", code)
	}
}

// The generated ride has the shape the fixture promises: warm-up and
// cool-down ramps, no zero-watt samples, cadence and speed in road range.
func TestSimulateShape(t *testing.T) {
	t.Parallel()
	ride := Simulate()
	if len(ride.Watts) != durationSec || ride.Watts[0] > 130 || ride.Watts[durationSec-1] > 130 {
		t.Fatalf("ramps: first %d last %d", ride.Watts[0], ride.Watts[durationSec-1])
	}
	for i, watts := range ride.Watts {
		if watts <= 0 || ride.Cadence[i] < 75 || ride.Cadence[i] > 105 || ride.Velocity[i] < 5 || ride.Velocity[i] > 12 {
			t.Fatalf("sample %d: %d W cadence %d speed %.3f", i, watts, ride.Cadence[i], ride.Velocity[i])
		}
	}
}
