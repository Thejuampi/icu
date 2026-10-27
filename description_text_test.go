package icu

import (
	"errors"
	"reflect"
	"testing"
)

func TestValidateDescriptionText(t *testing.T) {
	t.Parallel()
	cases := map[string]bool{
		"Warmup\n- 10m 60%": false,
		`Warmup\n- 10m 60%`: true,
		`a\tb`:              true,
		`C:\notes`:          true,
		"":                  false,
	}
	for input, wantErr := range cases {
		err := ValidateDescriptionText(input)
		if wantErr != errors.Is(err, ErrEscapedDescription) {
			t.Fatalf("ValidateDescriptionText(%q) = %v, wantErr %v", input, err, wantErr)
		}
	}
}

func TestPreviewShowsRenderedDescriptionAndRejectsEscapes(t *testing.T) {
	t.Parallel()
	ops := []CalendarOperation{
		{ID: "a", Action: CalendarActionCreate, Body: EventEx{Name: "W", Description: "Warmup\r\n- 10m 60%"}},
		{ID: "b", Action: CalendarActionCreate, Body: EventEx{Name: "W", Description: `Warmup\n- 10m 60%`}},
	}

	preview := PreviewCalendarOperations(ops)
	errs := ValidateCalendarOperations(ops)

	if !reflect.DeepEqual(preview.Operations[0].Description, []string{"Warmup", "- 10m 60%"}) {
		t.Fatalf("rendered = %#v", preview.Operations[0].Description)
	}
	if len(errs) != 1 || errs[0][:2] != "b:" {
		t.Fatalf("errors = %v, want one for op b", errs)
	}
}

func TestPlanSessionRejectsEscapedDescription(t *testing.T) {
	t.Parallel()

	_, _, issue := estimatePlanSessionLoad(`- 10m 60%\n- 20m 80%`, FTPSelectionFromWatts(250), "WORKOUT")

	if issue.errMsg != ErrEscapedDescription.Error() {
		t.Fatalf("issue = %+v, want escaped-description error", issue)
	}
}
