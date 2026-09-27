package icu

import "testing"

func TestResolveEventFTPUsesEventPartitionNotActivityPartition(t *testing.T) {
	t.Parallel()

	// "Ride" with no flag is class 2 (type_default, outdoor) on the activity
	// partition but class 3 (ambiguous, type_default) on the event partition
	// (D-10). A wrong Entity wiring here would silently collapse the two
	// partitions rebalance/plan rely on staying distinct.
	selection := resolveEventFTP("Ride", nil, &SportSettings{FTP: 270, IndoorFTP: 285}, "")

	if selection.Class != FTPClassAmbiguous {
		t.Fatalf("class = %q, want %q (event partition)", selection.Class, FTPClassAmbiguous)
	}
}

func TestResolveEventFTPThreadsSessionEnvironmentAsExplicitOverride(t *testing.T) {
	t.Parallel()

	selection := resolveEventFTP("Ride", nil, &SportSettings{FTP: 270, IndoorFTP: 285}, SessionEnvironmentIndoor)

	if selection.Indoor == nil || !*selection.Indoor {
		t.Fatalf("indoor = %v, want true (--session-environment=indoor overriding the ambiguous type-default)", selection.Indoor)
	}
}

func TestAthleteHasFTPFalseWithoutSettings(t *testing.T) {
	t.Parallel()

	if athleteHasFTP(nil) {
		t.Fatal("athleteHasFTP(nil) = true, want false")
	}
}

func TestAthleteHasFTPTrueFromIndoorFTPAlone(t *testing.T) {
	t.Parallel()

	// FTP and IndoorFTP are independently settable in SportSettings; a
	// configured IndoorFTP with no outdoor FTP still counts as "FTP is
	// configured" for envelope metrics that only gate on availability.
	if !athleteHasFTP(&SportSettings{IndoorFTP: 200}) {
		t.Fatal("athleteHasFTP(IndoorFTP-only) = false, want true")
	}
}

func TestAthleteHasFTPFalseWhenBothZero(t *testing.T) {
	t.Parallel()

	if athleteHasFTP(&SportSettings{}) {
		t.Fatal("athleteHasFTP(zero-value settings) = true, want false")
	}
}

func TestExplicitPtrFromSessionEnvironmentEmptyIsNil(t *testing.T) {
	t.Parallel()

	if explicitPtrFromSessionEnvironment("") != nil {
		t.Fatal("explicitPtrFromSessionEnvironment(\"\") != nil, want nil (no --session-environment flag given)")
	}
}

func TestExplicitPtrFromSessionEnvironmentCarriesValue(t *testing.T) {
	t.Parallel()

	got := explicitPtrFromSessionEnvironment(SessionEnvironmentOutdoor)

	if got == nil || *got != SessionEnvironmentOutdoor {
		t.Fatalf("explicitPtrFromSessionEnvironment(outdoor) = %v, want pointer to %q", got, SessionEnvironmentOutdoor)
	}
}

func TestFTPMixedEnvironmentWeekTrueWhenIndoorAndOutdoorBothPresent(t *testing.T) {
	t.Parallel()

	mixed := ftpMixedEnvironmentWeek(
		[]string{ActivityTypeVirtualRide, "Ride"},
		[]*bool{nil, nil},
		&SportSettings{FTP: 270, IndoorFTP: 285},
	)

	if !mixed {
		t.Fatal("mixed = false, want true (VirtualRide is type_inherent indoor, bare Ride is type_default outdoor)")
	}
}

func TestFTPMixedEnvironmentWeekFalseWhenUniform(t *testing.T) {
	t.Parallel()

	mixed := ftpMixedEnvironmentWeek(
		[]string{"Ride", "Ride"},
		[]*bool{nil, nil},
		&SportSettings{FTP: 270, IndoorFTP: 285},
	)

	if mixed {
		t.Fatal("mixed = true, want false (both sessions resolve outdoor)")
	}
}

func TestFTPMixedEnvironmentWeekToleratesShorterFlagSlice(t *testing.T) {
	t.Parallel()

	// indoorFlags may be shorter than sportTypes when a caller builds it from
	// a matched-events slice sparser than the week's session slots; the
	// bounds check must fall back to nil rather than panic or misalign.
	mixed := ftpMixedEnvironmentWeek(
		[]string{ActivityTypeVirtualRide, "Ride", "Ride"},
		[]*bool{nil},
		&SportSettings{FTP: 270, IndoorFTP: 285},
	)

	if !mixed {
		t.Fatal("mixed = false, want true (out-of-bounds indoorFlags entries must default to nil, not panic)")
	}
}
