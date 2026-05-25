package main

type Athlete struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	FirstName      string  `json:"firstname"`
	LastName       string  `json:"lastname"`
	Weight         float64 `json:"weight,omitempty"`
	Height         float64 `json:"height,omitempty"`
	HeightUnits    string  `json:"height_units,omitempty"`
	Email          string  `json:"email,omitempty"`
	Sex            string  `json:"sex,omitempty"`
	City           string  `json:"city,omitempty"`
	State          string  `json:"state,omitempty"`
	Country        string  `json:"country,omitempty"`
	Timezone       string  `json:"timezone,omitempty"`
	Locale         string  `json:"locale,omitempty"`
	Bio            string  `json:"bio,omitempty"`
	RestingHR      int     `json:"icu_resting_hr,omitempty"`
	ICUWeight      float64 `json:"icu_weight,omitempty"`
	FTP            int     `json:"icu_ftp,omitempty"`
	FormAsPercent  bool    `json:"icu_form_as_percent,omitempty"`
	Plan           string  `json:"plan,omitempty"`
	StravaID       int64   `json:"strava_id,omitempty"`
	StravaAuthd    bool    `json:"strava_authorized,omitempty"`
	DateOfBirth    string  `json:"icu_date_of_birth,omitempty"`
	APIKey         string  `json:"icu_api_key,omitempty"`
	SportSettings  []SportSettings `json:"sportSettings,omitempty"`
	CustomItems    []CustomItem    `json:"custom_items,omitempty"`
}

type AthleteProfile struct {
	Athlete      AthleteForProfile `json:"athlete"`
	CustomItems  []CustomItem      `json:"customItems,omitempty"`
	SharedFolder []Folder          `json:"sharedFolders,omitempty"`
}

type AthleteForProfile struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	City      string `json:"city,omitempty"`
	State     string `json:"state,omitempty"`
	Country   string `json:"country,omitempty"`
	Timezone  string `json:"timezone,omitempty"`
	Sex       string `json:"sex,omitempty"`
	Bio       string `json:"bio,omitempty"`
	IsCoach   bool   `json:"icu_coach,omitempty"`
}

type AthleteUpdate struct {
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

type AthleteTrainingPlan struct {
	AthleteID          string `json:"athlete_id,omitempty"`
	PlanID             int    `json:"training_plan_id,omitempty"`
	PlanStartDate      string `json:"training_plan_start_date,omitempty"`
	PlanLastApplied    string `json:"training_plan_last_applied,omitempty"`
}

type AthleteTrainingPlanUpdate struct {
	ID            string `json:"id"`
	PlanID        int    `json:"training_plan_id,omitempty"`
	StartDate     string `json:"training_plan_start_date,omitempty"`
	PlanAlias     string `json:"training_plan_alias,omitempty"`
}

type Activity struct {
	ID                  string  `json:"id,omitempty"`
	Name                string  `json:"name,omitempty"`
	Description         string  `json:"description,omitempty"`
	Type                string  `json:"type,omitempty"`
	StartDateLocal      string  `json:"start_date_local,omitempty"`
	StartDate           string  `json:"start_date,omitempty"`
	MovingTime          int     `json:"moving_time,omitempty"`
	ElapsedTime         int     `json:"elapsed_time,omitempty"`
	Distance            float64 `json:"distance,omitempty"`
	TotalElevationGain  float64 `json:"total_elevation_gain,omitempty"`
	AverageSpeed        float64 `json:"average_speed,omitempty"`
	MaxSpeed            float64 `json:"max_speed,omitempty"`
	HasHeartRate        bool    `json:"has_heartrate,omitempty"`
	AverageHeartRate    int     `json:"average_heartrate,omitempty"`
	MaxHeartRate        int     `json:"max_heartrate,omitempty"`
	AveragePower        int     `json:"icu_average_watts,omitempty"`
	WeightedAvgPower    int     `json:"icu_weighted_avg_watts,omitempty"`
	AverageCadence      float64 `json:"average_cadence,omitempty"`
	Calories            int     `json:"calories,omitempty"`
	TrainingLoad        int     `json:"icu_training_load,omitempty"`
	Intensity           float64 `json:"icu_intensity,omitempty"`
	FTP                 int     `json:"icu_ftp,omitempty"`
	CriticalPower       int     `json:"icu_pm_cp,omitempty"`
	WPrime              int     `json:"icu_pm_w_prime,omitempty"`
	PMax                int     `json:"icu_pm_p_max,omitempty"`
	FTPWatts            int     `json:"icu_pm_ftp,omitempty"`
	RollingFTP          int     `json:"icu_rolling_ftp,omitempty"`
	JoulesAboveFTP      int     `json:"icu_joules_above_ftp,omitempty"`
	MaxWbalDepletion    int     `json:"icu_max_wbal_depletion,omitempty"`
	Decoupling          float64 `json:"decoupling,omitempty"`
	EfficiencyFactor    float64 `json:"icu_efficiency_factor,omitempty"`
	VariabilityIndex    float64 `json:"icu_variability_index,omitempty"`
	PowerHR             float64 `json:"icu_power_hr,omitempty"`
	PowerHRZ2           float64 `json:"icu_power_hr_z2,omitempty"`
	PowerHRZ2Mins       int     `json:"icu_power_hr_z2_mins,omitempty"`
	CadenceZ2           int     `json:"icu_cadence_z2,omitempty"`
	RPE                 int     `json:"icu_rpe,omitempty"`
	Feel                int     `json:"feel,omitempty"`
	PerceivedExertion   float64 `json:"perceived_exertion,omitempty"`
	SessionRPE          int     `json:"session_rpe,omitempty"`
	Compliance          float64 `json:"compliance,omitempty"`
	AverageTemp         float64 `json:"average_temp,omitempty"`
	AverageWeatherTemp  float64 `json:"average_weather_temp,omitempty"`
	AverageFeelsLike    float64 `json:"average_feels_like,omitempty"`
	StrainScore         float64 `json:"strain_score,omitempty"`
	Source              string  `json:"source,omitempty"`
	StravaID            string  `json:"strava_id,omitempty"`
	ExternalID          string  `json:"external_id,omitempty"`
	DeviceName          string  `json:"device_name,omitempty"`
	Tags                []string `json:"tags,omitempty"`
	ZoneTimes           []ZoneTime `json:"icu_zone_times,omitempty"`
	HRZoneTimes         []int      `json:"icu_hr_zone_times,omitempty"`
	Pace                float64 `json:"pace,omitempty"`
	ThresholdPace       float64 `json:"threshold_pace,omitempty"`
	LTHR                int     `json:"lthr,omitempty"`
	AthleteMaxHR        int     `json:"athlete_max_hr,omitempty"`
	ATL                 float64 `json:"icu_atl,omitempty"`
	CTL                 float64 `json:"icu_ctl,omitempty"`
}

type ZoneTime struct {
	ID   string `json:"id"`
	Secs int    `json:"secs"`
}

type UploadResponse struct {
	ID         string       `json:"id,omitempty"`
	AthleteID  string       `json:"icu_athlete_id,omitempty"`
	Activities []ActivityID `json:"activities,omitempty"`
}

type ActivityID struct {
	ID        string `json:"id"`
	AthleteID string `json:"icu_athlete_id,omitempty"`
}

type ActivityMini struct {
	ID             string `json:"id"`
	StartDateLocal string `json:"start_date_local,omitempty"`
	Type           string `json:"type,omitempty"`
	Name           string `json:"name,omitempty"`
}

type ActivitySearchResult struct {
	ID             string   `json:"id"`
	Name           string   `json:"name,omitempty"`
	StartDateLocal string   `json:"start_date_local,omitempty"`
	Type           string   `json:"type,omitempty"`
	Distance       float64  `json:"distance,omitempty"`
	MovingTime     int      `json:"moving_time,omitempty"`
	Tags           []string `json:"tags,omitempty"`
	Description    string   `json:"description,omitempty"`
}

type Wellness struct {
	ID          string      `json:"id"`
	Weight      float64     `json:"weight,omitempty"`
	RestingHR   int         `json:"restingHR,omitempty"`
	HRV         float64     `json:"hrv,omitempty"`
	HRVSDNN     float64     `json:"hrvSDNN,omitempty"`
	SleepSecs   int         `json:"sleepSecs,omitempty"`
	SleepScore  float64     `json:"sleepScore,omitempty"`
	SleepQuality int        `json:"sleepQuality,omitempty"`
	AvgSleepHR  float64     `json:"avgSleepingHR,omitempty"`
	Readiness   float64     `json:"readiness,omitempty"`
	BaevskySI   float64     `json:"baevskySI,omitempty"`
	SpO2        float64     `json:"spO2,omitempty"`
	Systolic    int         `json:"systolic,omitempty"`
	Diastolic   int         `json:"diastolic,omitempty"`
	Kcal        int         `json:"kcalConsumed,omitempty"`
	Soreness    int         `json:"soreness,omitempty"`
	Fatigue     int         `json:"fatigue,omitempty"`
	Stress      int         `json:"stress,omitempty"`
	Mood        int         `json:"mood,omitempty"`
	Motivation  int         `json:"motivation,omitempty"`
	Injury      int         `json:"injury,omitempty"`
	Hydration   int         `json:"hydration,omitempty"`
	HydrationVol float64    `json:"hydrationVolume,omitempty"`
	BloodGlucose float64    `json:"bloodGlucose,omitempty"`
	Lactate     float64     `json:"lactate,omitempty"`
	BodyFat     float64     `json:"bodyFat,omitempty"`
	Abdomen     float64     `json:"abdomen,omitempty"`
	VO2Max      float64     `json:"vo2max,omitempty"`
	Steps       int         `json:"steps,omitempty"`
	Respiration float64     `json:"respiration,omitempty"`
	Comments    string      `json:"comments,omitempty"`
	CTL         float64     `json:"ctl,omitempty"`
	ATL         float64     `json:"atl,omitempty"`
	RampRate    float64     `json:"rampRate,omitempty"`
	SportInfo   []SportInfo `json:"sportInfo,omitempty"`
	Locked      bool        `json:"locked,omitempty"`
}

type SportInfo struct {
	Type   string  `json:"type"`
	EFTP   float64 `json:"eftp,omitempty"`
	WPrime float64 `json:"wPrime,omitempty"`
	PMax   float64 `json:"pMax,omitempty"`
}

type Event struct {
	ID                 int         `json:"id,omitempty"`
	StartDateLocal     string      `json:"start_date_local,omitempty"`
	EndDateLocal       string      `json:"end_date_local,omitempty"`
	Category           string      `json:"category,omitempty"`
	Type               string      `json:"type,omitempty"`
	Name               string      `json:"name,omitempty"`
	Description        string      `json:"description,omitempty"`
	TrainingLoad       int         `json:"icu_training_load,omitempty"`
	MovingTime         int         `json:"moving_time,omitempty"`
	Distance           float64     `json:"distance,omitempty"`
	Color              string      `json:"color,omitempty"`
	Indoor             bool        `json:"indoor,omitempty"`
	FTP                int         `json:"icu_ftp,omitempty"`
	ATL                float64     `json:"icu_atl,omitempty"`
	CTL                float64     `json:"icu_ctl,omitempty"`
	Target             string      `json:"target,omitempty"`
	UID                string      `json:"uid,omitempty"`
	CalendarID         int         `json:"calendar_id,omitempty"`
	Tags               []string    `json:"tags,omitempty"`
	ExternalID         string      `json:"external_id,omitempty"`
	HideFromAthlete    bool        `json:"hide_from_athlete,omitempty"`
	AthleteCannotEdit  bool        `json:"athlete_cannot_edit,omitempty"`
	Intensity          float64     `json:"icu_intensity,omitempty"`
	StrainScore        float64     `json:"strain_score,omitempty"`
	WorkoutDoc         any         `json:"workout_doc,omitempty"`
	WorkoutFileBase64  string      `json:"workout_file_base64,omitempty"`
	WorkoutFilename    string      `json:"workout_filename,omitempty"`
}

type EventEx struct {
	StartDateLocal    string   `json:"start_date_local"`
	EndDateLocal      string   `json:"end_date_local,omitempty"`
	Category          string   `json:"category"`
	Type              string   `json:"type,omitempty"`
	Name              string   `json:"name,omitempty"`
	Description       string   `json:"description,omitempty"`
	TrainingLoad      int      `json:"icu_training_load,omitempty"`
	MovingTime        int      `json:"moving_time,omitempty"`
	Distance          float64  `json:"distance,omitempty"`
	Color             string   `json:"color,omitempty"`
	Indoor            bool     `json:"indoor,omitempty"`
	Target            string   `json:"target,omitempty"`
	Tags              []string `json:"tags,omitempty"`
	ExternalID        string   `json:"external_id,omitempty"`
	FileContents      string   `json:"file_contents,omitempty"`
	FileContentsB64   string   `json:"file_contents_base64,omitempty"`
	Filename          string   `json:"filename,omitempty"`
}

type Workout struct {
	ID              int      `json:"id,omitempty"`
	Name            string   `json:"name,omitempty"`
	Description     string   `json:"description,omitempty"`
	Type            string   `json:"type,omitempty"`
	TrainingLoad    int      `json:"icu_training_load,omitempty"`
	MovingTime      int      `json:"moving_time,omitempty"`
	Distance        float64  `json:"distance,omitempty"`
	Indoor          bool     `json:"indoor,omitempty"`
	Target          string   `json:"target,omitempty"`
	FolderID        int      `json:"folder_id,omitempty"`
	Day             int      `json:"day,omitempty"`
	Tags            []string `json:"tags,omitempty"`
	Intensity       float64  `json:"icu_intensity,omitempty"`
	WorkoutDoc      any      `json:"workout_doc,omitempty"`
}

type WorkoutEx struct {
	Name          string   `json:"name,omitempty"`
	Description   string   `json:"description,omitempty"`
	Type          string   `json:"type,omitempty"`
	FolderID      int      `json:"folder_id,omitempty"`
	TrainingLoad  int      `json:"icu_training_load,omitempty"`
	MovingTime    int      `json:"moving_time,omitempty"`
	Indoor        bool     `json:"indoor,omitempty"`
	Target        string   `json:"target,omitempty"`
	Tags          []string `json:"tags,omitempty"`
	FileContents  string   `json:"file_contents,omitempty"`
	FileContentsB64 string `json:"file_contents_base64,omitempty"`
	Filename      string   `json:"filename,omitempty"`
}

type Folder struct {
	ID              int        `json:"id,omitempty"`
	Name            string     `json:"name,omitempty"`
	Description     string     `json:"description,omitempty"`
	Type            string     `json:"type,omitempty"`
	Visibility      string     `json:"visibility,omitempty"`
	StartDate       string     `json:"start_date_local,omitempty"`
	Children        []Workout  `json:"children,omitempty"`
	ActivityTypes   []string   `json:"activity_types,omitempty"`
}

type FolderCreate struct {
	Name          string   `json:"name"`
	Description   string   `json:"description,omitempty"`
	Type          string   `json:"type,omitempty"`
	Visibility    string   `json:"visibility,omitempty"`
	ActivityTypes []string `json:"activity_types,omitempty"`
}

type SportSettings struct {
	ID             int      `json:"id,omitempty"`
	AthleteID      string   `json:"athlete_id,omitempty"`
	Types          []string `json:"types,omitempty"`
	FTP            int      `json:"ftp,omitempty"`
	IndoorFTP      int      `json:"indoor_ftp,omitempty"`
	WPrime         int      `json:"w_prime,omitempty"`
	PMax           int      `json:"p_max,omitempty"`
	LTHR           int      `json:"lthr,omitempty"`
	MaxHR          int      `json:"max_hr,omitempty"`
	PowerZones     []int    `json:"power_zones,omitempty"`
	HRZones        []int    `json:"hr_zones,omitempty"`
	PaceZones      []float64 `json:"pace_zones,omitempty"`
	ThresholdPace  float64  `json:"threshold_pace,omitempty"`
	PaceUnits      string   `json:"pace_units,omitempty"`
	HRLoadType     string   `json:"hr_load_type,omitempty"`
	PaceLoadType   string   `json:"pace_load_type,omitempty"`
	GapModel       string   `json:"gap_model,omitempty"`
}

type Interval struct {
	StartIndex      int     `json:"start_index,omitempty"`
	EndIndex        int     `json:"end_index,omitempty"`
	Type            string  `json:"type,omitempty"`
	Distance        float64 `json:"distance,omitempty"`
	MovingTime      int     `json:"moving_time,omitempty"`
	AvgPower        int     `json:"average_watts,omitempty"`
	MaxPower        int     `json:"max_watts,omitempty"`
	AvgHR           int     `json:"average_heartrate,omitempty"`
	MaxHR           int     `json:"max_heartrate,omitempty"`
	Intensity       int     `json:"intensity,omitempty"`
	TrainingLoad    float64 `json:"training_load,omitempty"`
	Joules          int     `json:"joules,omitempty"`
	Zone            int     `json:"zone,omitempty"`
	AvgCadence      float64 `json:"average_cadence,omitempty"`
	ElevationGain   float64 `json:"total_elevation_gain,omitempty"`
	GroupID         string  `json:"group_id,omitempty"`
}

type IntervalsDTO struct {
	ID         string          `json:"id,omitempty"`
	Analyzed   string          `json:"analyzed,omitempty"`
	Intervals  []Interval      `json:"icu_intervals,omitempty"`
	Groups     []IntervalGroup `json:"icu_groups,omitempty"`
}

type IntervalGroup struct {
	StartIndex      int     `json:"start_index,omitempty"`
	EndIndex        int     `json:"end_index,omitempty"`
	Distance        float64 `json:"distance,omitempty"`
	MovingTime      int     `json:"moving_time,omitempty"`
	AvgPower        int     `json:"average_watts,omitempty"`
	MaxPower        int     `json:"max_watts,omitempty"`
	AvgHR           int     `json:"average_heartrate,omitempty"`
	MaxHR           int     `json:"max_heartrate,omitempty"`
	Intensity       int     `json:"intensity,omitempty"`
	TrainingLoad    float64 `json:"training_load,omitempty"`
	Joules          int     `json:"joules,omitempty"`
	Zone            int     `json:"zone,omitempty"`
	Count           int     `json:"count,omitempty"`
}

type ActivityStream struct {
	Type string `json:"type,omitempty"`
	Name string `json:"name,omitempty"`
	Data any    `json:"data,omitempty"`
}

type MapData struct {
	Bounds  [][]float64 `json:"bounds,omitempty"`
	LatLngs [][]float64 `json:"latlngs,omitempty"`
}

type WeatherSummary struct {
	AvgTemp        float64 `json:"average_temp,omitempty"`
	MinTemp        float64 `json:"min_weather_temp,omitempty"`
	MaxTemp        float64 `json:"max_weather_temp,omitempty"`
	AvgFeelsLike   float64 `json:"average_feels_like,omitempty"`
	AvgWindSpeed   float64 `json:"average_wind_speed,omitempty"`
	HeadwindPct    float64 `json:"headwind_percent,omitempty"`
	TailwindPct    float64 `json:"tailwind_percent,omitempty"`
	Description    string  `json:"description,omitempty"`
}

type BestEfforts struct {
	Efforts []Effort `json:"efforts,omitempty"`
}

type Effort struct {
	StartIndex int     `json:"start_index,omitempty"`
	EndIndex   int     `json:"end_index,omitempty"`
	Average    float64 `json:"average,omitempty"`
	Duration   int     `json:"duration,omitempty"`
	Distance   float64 `json:"distance,omitempty"`
}

type DataCurve struct {
	ID          string  `json:"id,omitempty"`
	Label       string  `json:"label,omitempty"`
	StartDate   string  `json:"start_date_local,omitempty"`
	EndDate     string  `json:"end_date_local,omitempty"`
	Days        int     `json:"days,omitempty"`
	MovingTime  int     `json:"moving_time,omitempty"`
	TrainingLoad int    `json:"training_load,omitempty"`
	Weight      float64 `json:"weight,omitempty"`
	Secs        []int   `json:"secs,omitempty"`
	Values      []int   `json:"values,omitempty"`
	Distance    []float64 `json:"distance,omitempty"`
}

type PowerCurve     struct{ DataCurve }
type HRCurve        struct{ DataCurve }
type PaceCurve      struct{ DataCurve }

type PowerHRCurve struct {
	AthleteID  string `json:"athleteId,omitempty"`
	MinWatts   int    `json:"minWatts,omitempty"`
	MaxWatts   int    `json:"maxWatts,omitempty"`
	BucketSize int    `json:"bucketSize,omitempty"`
	FTP        int    `json:"ftp,omitempty"`
	LTHR       int    `json:"lthr,omitempty"`
	MaxHR      int    `json:"max_hr,omitempty"`
}

type PowerModel struct {
	Type            string `json:"type,omitempty"`
	CriticalPower   int    `json:"criticalPower,omitempty"`
	WPrime          int    `json:"wPrime,omitempty"`
	PMax            int    `json:"pMax,omitempty"`
	FTP             int    `json:"ftp,omitempty"`
}

type Route struct {
	AthleteID     string    `json:"athlete_id,omitempty"`
	RouteID       int64     `json:"route_id,omitempty"`
	Name          string    `json:"name,omitempty"`
	Description   string    `json:"description,omitempty"`
	RenameActs    bool      `json:"rename_activities,omitempty"`
	Commute       bool      `json:"commute,omitempty"`
	Tags          []string  `json:"tags,omitempty"`
	Distance      float64   `json:"distance,omitempty"`
	ActivityCount int       `json:"activity_count,omitempty"`
}

type RouteSimilarity struct {
	Similarity float64 `json:"similarity,omitempty"`
}

type Gear struct {
	ID         string         `json:"id,omitempty"`
	AthleteID  string         `json:"athlete_id,omitempty"`
	Type       string         `json:"type,omitempty"`
	Name       string         `json:"name,omitempty"`
	Distance   float64        `json:"distance,omitempty"`
	Time       float64        `json:"time,omitempty"`
	Activities int            `json:"activities,omitempty"`
	Retired    string         `json:"retired,omitempty"`
	Reminders  []GearReminder `json:"reminders,omitempty"`
}

type GearReminder struct {
	ID        int     `json:"id,omitempty"`
	Name      string  `json:"name,omitempty"`
	Distance  float64 `json:"distance,omitempty"`
	Time      float64 `json:"time,omitempty"`
	Activities int    `json:"activities,omitempty"`
	Days      int     `json:"days,omitempty"`
	LastReset string  `json:"last_reset,omitempty"`
	PctUsed   float64 `json:"percent_used,omitempty"`
}

type GearStats struct {
	Distance   float64 `json:"distance,omitempty"`
	ElapsedTime float64 `json:"elapsed_time,omitempty"`
	MovingTime float64 `json:"moving_time,omitempty"`
	Activities int     `json:"activities,omitempty"`
}

type Chat struct {
	ID             int    `json:"id,omitempty"`
	Type           string `json:"type,omitempty"`
	Name           string `json:"name,omitempty"`
	Description    string `json:"description,omitempty"`
	ActivityID     string `json:"activity_id,omitempty"`
	AthleteID      string `json:"athlete_id,omitempty"`
	OtherAthleteID string `json:"other_athlete_id,omitempty"`
	NewMsgCount    int    `json:"new_message_count,omitempty"`
}

type Message struct {
	ID         int    `json:"id,omitempty"`
	AthleteID  string `json:"athlete_id,omitempty"`
	Name       string `json:"name,omitempty"`
	Content    string `json:"content,omitempty"`
	Created    string `json:"created,omitempty"`
	Type       string `json:"type,omitempty"`
}

type NewMessage struct {
	ChatID       int    `json:"chat_id,omitempty"`
	ToAthleteID  string `json:"to_athlete_id,omitempty"`
	Content      string `json:"content"`
}

type SendResponse struct {
	ID      int     `json:"id,omitempty"`
	Message Message `json:"message,omitempty"`
}

type CustomItem struct {
	ID          int    `json:"id,omitempty"`
	AthleteID   string `json:"athlete_id,omitempty"`
	Type        string `json:"type,omitempty"`
	Visibility  string `json:"visibility,omitempty"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Content     any    `json:"content,omitempty"`
	Index       int    `json:"index,omitempty"`
}

type WeatherConfig struct {
	Forecasts []Forecast `json:"forecasts,omitempty"`
}

type Forecast struct {
	ID       int    `json:"id,omitempty"`
	Provider string `json:"provider,omitempty"`
	Label    string `json:"label,omitempty"`
	Lat      float64 `json:"lat,omitempty"`
	Lon      float64 `json:"lon,omitempty"`
	Enabled  bool   `json:"enabled,omitempty"`
}

type WeatherDTO struct {
	Forecasts []Forecast `json:"forecasts,omitempty"`
}

type DoomedEvent struct {
	ID         int    `json:"id,omitempty"`
	ExternalID string `json:"external_id,omitempty"`
}

type DeleteEventsResponse struct {
	EventsDeleted int `json:"eventsDeleted,omitempty"`
}

type DeleteResponse struct {
	ID        string `json:"id,omitempty"`
	AthleteID string `json:"icu_athlete_id,omitempty"`
}

type SummaryWithCats struct {
	Date        string  `json:"date,omitempty"`
	AthleteID   string  `json:"athlete_id,omitempty"`
	AthleteName string  `json:"athlete_name,omitempty"`
	Fitness     float64 `json:"fitness,omitempty"`
	Fatigue     float64 `json:"fatigue,omitempty"`
	Form        float64 `json:"form,omitempty"`
	TrainingLoad int    `json:"training_load,omitempty"`
	Weight      float64 `json:"weight,omitempty"`
	Distance    float64 `json:"distance,omitempty"`
}
