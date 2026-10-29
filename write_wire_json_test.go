package icu_test

import (
	"encoding/json"
	"sort"
	"strings"
	"testing"

	icu "github.com/Thejuampi/icu"
)

func jsonKeys(t *testing.T, v any) string {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	return strings.Join(keys, ",")
}

func TestWriteBodiesUseSnakeCaseWireKeys(t *testing.T) {
	t.Parallel()
	f, i, s := 81.0, 52, "cm"
	cases := []struct {
		name string
		body any
		want string
	}{
		{"AthleteUpdate", icu.AthleteUpdate{
			Weight: &f, Height: &f, HeightUnits: &s, ICUWeight: &f, RestingHR: &i,
			Name: &s, Sex: &s, Timezone: &s, Bio: &s,
		}, "bio,height,height_units,icu_resting_hr,icu_weight,name,sex,timezone,weight"},
		{"WorkoutEx", icu.WorkoutEx{
			Name: "n", Description: "d", Type: "Ride", FolderID: 1, TrainingLoad: 2, MovingTime: 3,
			Indoor: true, Target: "POWER", Tags: []string{"t"}, FileContents: "x", FileContentsB64: "eA==", Filename: "f.zwo",
		}, "description,file_contents,file_contents_base64,filename,folder_id,icu_training_load,indoor,moving_time,name,tags,target,type"},
		{"FolderCreate", icu.FolderCreate{
			Name: "n", Description: "d", Type: "FOLDER", Visibility: "PRIVATE", ActivityTypes: []string{"Ride"},
		}, "activity_types,description,name,type,visibility"},
		{"NewMessage", icu.NewMessage{ChatID: 1, ToAthleteID: "i2", Content: "hi"}, "chat_id,content,to_athlete_id"},
		{
			"GearWrite",
			icu.GearWrite{AthleteID: "i1", Type: "Bike", Name: "n", Distance: 1, Time: 2, Retired: "2026-01-01"},
			"athlete_id,distance,name,retired,time,type",
		},
		{"CustomItemWrite", icu.CustomItemWrite{
			AthleteID: "i1", Type: "FITNESS_CHART", Visibility: "PRIVATE", Name: "n", Description: "d", Content: map[string]int{"a": 1}, Index: 2,
		}, "athlete_id,content,description,index,name,type,visibility"},
	}
	for _, tc := range cases {
		if got := jsonKeys(t, tc.body); got != tc.want {
			t.Errorf("%s keys = %s, want %s", tc.name, got, tc.want)
		}
	}
}

func TestWriteBodiesOmitUnsetFields(t *testing.T) {
	t.Parallel()
	f := 80.5
	if got := jsonKeys(t, icu.AthleteUpdate{ICUWeight: &f}); got != "icu_weight" {
		t.Fatalf("keys = %s", got)
	}
	if got := jsonKeys(t, icu.NewMessage{Content: "x"}); got != "content" {
		t.Fatalf("keys = %s", got)
	}
}

func TestResponseTypesDecodeSnakeAndCamel(t *testing.T) {
	t.Parallel()
	var w icu.Workout
	if err := json.Unmarshal([]byte(`{"id":1,"icu_training_load":50,"moving_time":3600,"folder_id":7,"icu_intensity":0.7,"workout_doc":{"a":1}}`), &w); err != nil {
		t.Fatal(err)
	}
	if w.TrainingLoad != 50 || w.MovingTime != 3600 || w.FolderID != 7 || w.Intensity != 0.7 || w.WorkoutDoc == nil {
		t.Fatalf("workout = %+v", w)
	}
	if err := json.Unmarshal([]byte(`{"folderId":8,"movingTime":60}`), &w); err != nil || w.FolderID != 8 || w.MovingTime != 60 {
		t.Fatalf("camel workout = %+v err=%v", w, err)
	}

	var f icu.Folder
	if err := json.Unmarshal([]byte(`{"start_date_local":"2026-01-01","activity_types":["Ride"]}`), &f); err != nil ||
		f.StartDate != "2026-01-01" || len(f.ActivityTypes) != 1 {
		t.Fatalf("folder = %+v err=%v", f, err)
	}

	var g icu.Gear
	if err := json.Unmarshal([]byte(`{"athlete_id":"i1","reminders":[{"last_reset":"2026-01-01","percent_used":12.5}]}`), &g); err != nil ||
		g.AthleteID != "i1" || g.Reminders[0].LastReset != "2026-01-01" || g.Reminders[0].PctUsed != 12.5 {
		t.Fatalf("gear = %+v err=%v", g, err)
	}

	var c icu.Chat
	if err := json.Unmarshal([]byte(`{"activity_id":"a1","athlete_id":"i1","other_athlete_id":"i2","new_message_count":3}`), &c); err != nil ||
		c.ActivityID != "a1" || c.AthleteID != "i1" || c.OtherAthleteID != "i2" || c.NewMsgCount != 3 {
		t.Fatalf("chat = %+v err=%v", c, err)
	}

	var m icu.Message
	if err := json.Unmarshal([]byte(`{"athlete_id":"i1","content":"x"}`), &m); err != nil || m.AthleteID != "i1" {
		t.Fatalf("message = %+v err=%v", m, err)
	}
}

func TestResponseDecodersRejectInvalidJSON(t *testing.T) {
	t.Parallel()
	for _, target := range []any{&icu.Workout{}, &icu.Folder{}, &icu.Gear{}, &icu.GearReminder{}, &icu.Chat{}, &icu.Message{}} {
		if err := json.Unmarshal([]byte(`{"id":"x"`), target); err == nil {
			t.Fatalf("%T: want error", target)
		}
	}
}

func TestWellnessWireKeysLiteral(t *testing.T) {
	t.Parallel()
	w := icu.Wellness{ID: "2026-10-27", RestingHR: 48, HRVSDNN: 70, AvgSleepHR: 50, BaevskySI: 9, CTL: 40}
	raw, err := json.Marshal(icu.NewWellnessWrite(&w))
	if err != nil {
		t.Fatal(err)
	}
	want := `{"id":"2026-10-27","restingHR":48,"hrvSDNN":70,"avgSleepingHR":50,"baevskySI":9}`
	if string(raw) != want {
		t.Fatalf("got %s want %s", raw, want)
	}
	var back icu.Wellness
	if err := json.Unmarshal([]byte(`{"restingHR":48,"hrvSDNN":70,"avgSleepingHR":50,"baevskySI":9}`), &back); err != nil {
		t.Fatal(err)
	}
	if back.RestingHR != 48 || back.HRVSDNN != 70 || back.AvgSleepHR != 50 || back.BaevskySI != 9 {
		t.Fatalf("read = %+v", back)
	}
}

func TestActivityManualWireKeysLiteral(t *testing.T) {
	t.Parallel()
	raw, err := json.Marshal(icu.ActivityManualWrite{
		Type: "Ride", Name: "R", Description: "d", StartDateLocal: "2026-10-27T08:00:00",
		MovingTime: 60, Distance: 1, TrainingLoad: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"type":"Ride","name":"R","description":"d","start_date_local":"2026-10-27T08:00:00","moving_time":60,"distance":1,"icu_training_load":2}`
	if string(raw) != want {
		t.Fatalf("got %s", raw)
	}
}
