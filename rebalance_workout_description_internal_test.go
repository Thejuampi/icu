package icu

import "testing"

func TestAppendFTPNoteAppendsWhenApplicable(t *testing.T) {
	t.Parallel()

	got := appendFTPNote("- 60m 65-70%", 270)

	want := "- 60m 65-70%\n(FTP 270W)"
	if got != want {
		t.Fatalf("appendFTPNote = %q, want %q", got, want)
	}
}

func TestAppendFTPNoteLeavesDescriptionUntouchedWhenFTPNotApplicable(t *testing.T) {
	t.Parallel()

	// A class-4 session (D-10 not_applicable) has no watts to attribute; a
	// zero-watts note would fabricate a number rather than honestly omit it.
	got := appendFTPNote("- 60m 65-70%", 0)

	if got != "- 60m 65-70%" {
		t.Fatalf("appendFTPNote with ftpWatts=0 = %q, want description unchanged", got)
	}
}

func TestAppendFTPNoteLeavesEmptyDescriptionEmpty(t *testing.T) {
	t.Parallel()

	if got := appendFTPNote("", 270); got != "" {
		t.Fatalf("appendFTPNote(\"\", 270) = %q, want empty (nothing to annotate)", got)
	}
}
