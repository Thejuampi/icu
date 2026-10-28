package icu

import (
	"encoding/json"
	"fmt"
)

// Write bodies for the intervals.icu API must use the snake_case field names
// from the OpenAPI spec (docs/api/intervals-icu-openapi.json); camelCase keys
// are silently ignored. The Go structs keep their camelCase tags for CLI
// output, so each request type marshals through a snake_case wire struct, and
// response types decode both casings.

func marshalWire(name string, wire any) ([]byte, error) {
	data, err := json.Marshal(wire)
	if err != nil {
		return nil, fmt.Errorf("marshal %s: %w", name, err)
	}

	return data, nil
}

// AthleteUpdateDTO.
//
//nolint:tagliatelle // wire format matches external API snake_case contract
type athleteUpdateWire struct {
	Weight      *float64 `json:"weight,omitempty"`
	Height      *float64 `json:"height,omitempty"`
	HeightUnits *string  `json:"height_units,omitempty"`
	ICUWeight   *float64 `json:"icu_weight,omitempty"`
	RestingHR   *int     `json:"icu_resting_hr,omitempty"`
	Name        *string  `json:"name,omitempty"`
	Sex         *string  `json:"sex,omitempty"`
	Timezone    *string  `json:"timezone,omitempty"`
	Bio         *string  `json:"bio,omitempty"`
}

func (update AthleteUpdate) MarshalJSON() ([]byte, error) {
	return marshalWire("AthleteUpdate", athleteUpdateWire(update))
}

// WorkoutEx (spec schema WorkoutEx).
//
//nolint:tagliatelle // wire format matches external API snake_case contract
type workoutExWire struct {
	Name            string   `json:"name,omitempty"`
	Description     string   `json:"description,omitempty"`
	Type            string   `json:"type,omitempty"`
	FolderID        int      `json:"folder_id,omitempty"`
	TrainingLoad    int      `json:"icu_training_load,omitempty"`
	MovingTime      int      `json:"moving_time,omitempty"`
	Indoor          bool     `json:"indoor,omitempty"`
	Target          string   `json:"target,omitempty"`
	Tags            []string `json:"tags,omitempty"`
	FileContents    string   `json:"file_contents,omitempty"`
	FileContentsB64 string   `json:"file_contents_base64,omitempty"`
	Filename        string   `json:"filename,omitempty"`
}

func (workout WorkoutEx) MarshalJSON() ([]byte, error) {
	return marshalWire("WorkoutEx", workoutExWire(workout))
}

// FolderCreate (spec schema CreateFolderDTO).
//
//nolint:tagliatelle // wire format matches external API snake_case contract
type folderCreateWire struct {
	Name          string   `json:"name"`
	Description   string   `json:"description,omitempty"`
	Type          string   `json:"type,omitempty"`
	Visibility    string   `json:"visibility,omitempty"`
	ActivityTypes []string `json:"activity_types,omitempty"`
}

func (folder FolderCreate) MarshalJSON() ([]byte, error) {
	return marshalWire("FolderCreate", folderCreateWire(folder))
}

// NewMessage (spec schema NewMessage).
//
//nolint:tagliatelle // wire format matches external API snake_case contract
type newMessageWire struct {
	ChatID      int    `json:"chat_id,omitempty"`
	ToAthleteID string `json:"to_athlete_id,omitempty"`
	Content     string `json:"content"`
}

func (msg NewMessage) MarshalJSON() ([]byte, error) {
	return marshalWire("NewMessage", newMessageWire(msg))
}

// GearWrite is the gear create/update request body (spec schema Gear). Gear
// itself stays camelCase for CLI output.
type GearWrite struct {
	AthleteID string
	Type      string
	Name      string
	Distance  float64
	Time      float64
	Retired   string
}

//nolint:tagliatelle // wire format matches external API snake_case contract
type gearWire struct {
	AthleteID string  `json:"athlete_id,omitempty"`
	Type      string  `json:"type,omitempty"`
	Name      string  `json:"name,omitempty"`
	Distance  float64 `json:"distance,omitempty"`
	Time      float64 `json:"time,omitempty"`
	Retired   string  `json:"retired,omitempty"`
}

func (gear GearWrite) MarshalJSON() ([]byte, error) {
	return marshalWire("GearWrite", gearWire(gear))
}

// CustomItemWrite is the custom-item create/update request body (spec schemas
// NewCustomItem / CustomItem). CustomItem itself stays camelCase for output.
type CustomItemWrite struct {
	AthleteID   string
	Type        string
	Visibility  string
	Name        string
	Description string
	Content     any
	Index       int
}

//nolint:tagliatelle // wire format matches external API snake_case contract
type customItemWire struct {
	AthleteID   string `json:"athlete_id,omitempty"`
	Type        string `json:"type,omitempty"`
	Visibility  string `json:"visibility,omitempty"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Content     any    `json:"content,omitempty"`
	Index       int    `json:"index,omitempty"`
}

func (item CustomItemWrite) MarshalJSON() ([]byte, error) {
	return marshalWire("CustomItemWrite", customItemWire(item))
}

// Response decoders: accept the API's snake_case alongside camelCase.

type workoutAlias Workout

func (workout *Workout) UnmarshalJSON(data []byte) error {
	var alias workoutAlias

	return unmarshalWithSnakeFields(data, workout, &alias,
		func(a workoutAlias) Workout { return Workout(a) },
		func(w *Workout, raw map[string]json.RawMessage) {
			copyRawIfSet(&w.TrainingLoad, raw, "icu_training_load")
			copyRawIfSet(&w.MovingTime, raw, "moving_time")
			copyRawIfSet(&w.FolderID, raw, "folder_id")
			copyRawIfSet(&w.Intensity, raw, "icu_intensity")
			copyRawAnyIfSet(&w.WorkoutDoc, raw, "workout_doc")
		}, nil)
}

type folderAlias Folder

func (folder *Folder) UnmarshalJSON(data []byte) error {
	var alias folderAlias

	return unmarshalWithSnakeFields(data, folder, &alias,
		func(a folderAlias) Folder { return Folder(a) },
		func(f *Folder, raw map[string]json.RawMessage) {
			copyRawIfSet(&f.StartDate, raw, "start_date_local")
			copyRawValueIfSet(&f.ActivityTypes, raw, "activity_types")
		}, nil)
}

type gearAlias Gear

func (gear *Gear) UnmarshalJSON(data []byte) error {
	var alias gearAlias

	return unmarshalWithSnakeFields(data, gear, &alias,
		func(a gearAlias) Gear { return Gear(a) },
		func(g *Gear, raw map[string]json.RawMessage) {
			copyRawIfSet(&g.AthleteID, raw, "athlete_id")
		}, nil)
}

type gearReminderAlias GearReminder

func (reminder *GearReminder) UnmarshalJSON(data []byte) error {
	var alias gearReminderAlias

	return unmarshalWithSnakeFields(data, reminder, &alias,
		func(a gearReminderAlias) GearReminder { return GearReminder(a) },
		func(r *GearReminder, raw map[string]json.RawMessage) {
			copyRawIfSet(&r.LastReset, raw, "last_reset")
			copyRawIfSet(&r.PctUsed, raw, "percent_used")
		}, nil)
}

type chatAlias Chat

func (chat *Chat) UnmarshalJSON(data []byte) error {
	var alias chatAlias

	return unmarshalWithSnakeFields(data, chat, &alias,
		func(a chatAlias) Chat { return Chat(a) },
		func(c *Chat, raw map[string]json.RawMessage) {
			copyRawIfSet(&c.ActivityID, raw, "activity_id")
			copyRawIfSet(&c.AthleteID, raw, "athlete_id")
			copyRawIfSet(&c.OtherAthleteID, raw, "other_athlete_id")
			copyRawIfSet(&c.NewMsgCount, raw, "new_message_count")
		}, nil)
}

type messageAlias Message

func (msg *Message) UnmarshalJSON(data []byte) error {
	var alias messageAlias

	return unmarshalWithSnakeFields(data, msg, &alias,
		func(a messageAlias) Message { return Message(a) },
		func(m *Message, raw map[string]json.RawMessage) {
			copyRawIfSet(&m.AthleteID, raw, "athlete_id")
		}, nil)
}
