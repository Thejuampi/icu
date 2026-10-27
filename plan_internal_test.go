package icu

import "testing"

func TestSanitizePlanIDCollapsesSeparators(t *testing.T) {
	tests := []struct{ name, want string }{
		{"Z2 + Tempo-lite", "Z2-Tempo-lite"},
		{"  spaced   out  ", "spaced-out"},
		{"a__b", "a__b"},
		{"+++", "session"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sanitizePlanID(tt.name); got != tt.want {
				t.Fatalf("sanitizePlanID(%q) = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}

func TestPlanOperationDoesNotWriteUIDIntoExternalID(t *testing.T) {
	t.Parallel()

	session := PlanResolvedSession{UID: "plan-uid-1", Action: CalendarActionUpdate, Date: "2026-10-27"}
	operation := planOperationFromSession(&session)
	if operation.Body.ExternalID != "" {
		t.Fatalf("ExternalID = %q, want empty", operation.Body.ExternalID)
	}
}
