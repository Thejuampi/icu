package icu

import (
	"reflect"
	"testing"
)

// TestEventWireV1FieldsMatchEvent guards against silent field drift.
// eventWireV1 is a hand-mirrored copy of Event (needed to preserve wire field
// order under Go's encoding/json shadowing rules — see event_wire_compat.go).
// If Event ever gains, loses, or reorders a field without a matching edit
// here, MarshalJSON would silently drop that field from every read response
// instead of failing loudly. Indoor is the one deliberate exception: it is
// *bool on Event and bool on eventWireV1 by design (D-25).
func TestEventWireV1FieldsMatchEvent(t *testing.T) {
	t.Parallel()

	eventType := reflect.TypeOf(Event{})
	wireType := reflect.TypeOf(eventWireV1{})

	mismatch := eventType.NumField() != wireType.NumField()
	if !mismatch {
		for i := range eventType.NumField() {
			eventField := eventType.Field(i)
			wireField := wireType.Field(i)

			sameName := eventField.Name == wireField.Name
			sameTag := eventField.Tag.Get("json") == wireField.Tag.Get("json")
			isDeliberateIndoorException := eventField.Name == "Indoor" && wireField.Name == "Indoor"

			if !sameName || (!sameTag && !isDeliberateIndoorException) {
				mismatch = true

				break
			}
		}
	}

	if mismatch {
		t.Fatalf("Event and eventWireV1 have drifted out of lockstep (Event: %+v, eventWireV1: %+v) — "+
			"update eventWireV1 in event_wire_compat.go to mirror Event field-for-field, keeping Indoor "+
			"as the sole bool-vs-*bool exception (D-25)", eventType, wireType)
	}
}
