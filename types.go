package icu

type Athlete struct {
	ID            string          `json:"id"`
	Name          string          `json:"name"`
	FirstName     string          `json:"firstname"`
	LastName      string          `json:"lastname"`
	Weight        float64         `json:"weight,omitempty"`
	Height        float64         `json:"height,omitempty"`
	HeightUnits   string          `json:"heightUnits,omitempty"`
	Email         string          `json:"email,omitempty"`
	Sex           string          `json:"sex,omitempty"`
	City          string          `json:"city,omitempty"`
	State         string          `json:"state,omitempty"`
	Country       string          `json:"country,omitempty"`
	Timezone      string          `json:"timezone,omitempty"`
	Locale        string          `json:"locale,omitempty"`
	Bio           string          `json:"bio,omitempty"`
	RestingHR     int             `json:"icuRestingHr,omitempty"`
	ICUWeight     float64         `json:"icuWeight,omitempty"`
	FTP           int             `json:"icuFtp,omitempty"`
	FormAsPercent bool            `json:"icuFormAsPercent,omitempty"`
	Plan          string          `json:"plan,omitempty"`
	StravaID      int64           `json:"stravaId,omitempty"`
	StravaAuthd   bool            `json:"stravaAuthorized,omitempty"`
	DateOfBirth   string          `json:"icuDateOfBirth,omitempty"`
	APIKey        string          `json:"icuApiKey,omitempty"`
	SportSettings []SportSettings `json:"sportSettings,omitempty"`
	CustomItems   []CustomItem    `json:"customItems,omitempty"`
}

type AthleteProfile struct {
	Athlete      AthleteForProfile `json:"athlete"`
	CustomItems  []CustomItem      `json:"customItems,omitempty"`
	SharedFolder []Folder          `json:"sharedFolders,omitempty"`
}

type AthleteForProfile struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	City     string `json:"city,omitempty"`
	State    string `json:"state,omitempty"`
	Country  string `json:"country,omitempty"`
	Timezone string `json:"timezone,omitempty"`
	Sex      string `json:"sex,omitempty"`
	Bio      string `json:"bio,omitempty"`
	IsCoach  bool   `json:"icuCoach,omitempty"`
}

type AthleteUpdate struct {
	Weight      *float64 `json:"weight,omitempty"`
	Height      *float64 `json:"height,omitempty"`
	HeightUnits *string  `json:"heightUnits,omitempty"`
	ICUWeight   *float64 `json:"icuWeight,omitempty"`
	RestingHR   *int     `json:"icuRestingHr,omitempty"`
	Name        *string  `json:"name,omitempty"`
	Sex         *string  `json:"sex,omitempty"`
	Timezone    *string  `json:"timezone,omitempty"`
	Bio         *string  `json:"bio,omitempty"`
}

type AthleteTrainingPlan struct {
	AthleteID       string `json:"athleteId,omitempty"`
	PlanID          int    `json:"trainingPlanId,omitempty"`
	PlanStartDate   string `json:"trainingPlanStartDate,omitempty"`
	PlanLastApplied string `json:"trainingPlanLastApplied,omitempty"`
}

type AthleteTrainingPlanUpdate struct {
	ID        string `json:"id"`
	PlanID    int    `json:"trainingPlanId,omitempty"`
	StartDate string `json:"trainingPlanStartDate,omitempty"`
	PlanAlias string `json:"trainingPlanAlias,omitempty"`
}

type Activity struct {
	ID                 string     `json:"id,omitempty"`
	Name               string     `json:"name,omitempty"`
	Description        string     `json:"description,omitempty"`
	Type               string     `json:"type,omitempty"`
	StartDateLocal     string     `json:"startDateLocal,omitempty"`
	StartDate          string     `json:"startDate,omitempty"`
	MovingTime         int        `json:"movingTime,omitempty"`
	ElapsedTime        int        `json:"elapsedTime,omitempty"`
	Distance           float64    `json:"distance,omitempty"`
	TotalElevationGain float64    `json:"totalElevationGain,omitempty"`
	AverageSpeed       float64    `json:"averageSpeed,omitempty"`
	MaxSpeed           float64    `json:"maxSpeed,omitempty"`
	HasHeartRate       bool       `json:"hasHeartrate,omitempty"`
	AverageHeartRate   int        `json:"averageHeartrate,omitempty"`
	MaxHeartRate       int        `json:"maxHeartrate,omitempty"`
	AveragePower       int        `json:"icuAverageWatts,omitempty"`
	WeightedAvgPower   int        `json:"icuWeightedAvgWatts,omitempty"`
	AverageCadence     float64    `json:"averageCadence,omitempty"`
	Calories           int        `json:"calories,omitempty"`
	TrainingLoad       int        `json:"icuTrainingLoad,omitempty"`
	Intensity          float64    `json:"icuIntensity,omitempty"`
	FTP                int        `json:"icuFtp,omitempty"`
	CriticalPower      int        `json:"icuPmCp,omitempty"`
	WPrime             int        `json:"icuPmWPrime,omitempty"`
	PMax               int        `json:"icuPmPMax,omitempty"`
	FTPWatts           int        `json:"icuPmFtp,omitempty"`
	RollingFTP         int        `json:"icuRollingFtp,omitempty"`
	JoulesAboveFTP     int        `json:"icuJoulesAboveFtp,omitempty"`
	MaxWbalDepletion   int        `json:"icuMaxWbalDepletion,omitempty"`
	Decoupling         float64    `json:"decoupling,omitempty"`
	EfficiencyFactor   float64    `json:"icuEfficiencyFactor,omitempty"`
	VariabilityIndex   float64    `json:"icuVariabilityIndex,omitempty"`
	PowerHR            float64    `json:"icuPowerHr,omitempty"`
	PowerHRZ2          float64    `json:"icuPowerHrZ2,omitempty"`
	PowerHRZ2Mins      int        `json:"icuPowerHrZ2Mins,omitempty"`
	CadenceZ2          int        `json:"icuCadenceZ2,omitempty"`
	RPE                int        `json:"icuRpe,omitempty"`
	Feel               int        `json:"feel,omitempty"`
	PerceivedExertion  float64    `json:"perceivedExertion,omitempty"`
	SessionRPE         int        `json:"sessionRpe,omitempty"`
	Compliance         float64    `json:"compliance,omitempty"`
	AverageTemp        float64    `json:"averageTemp,omitempty"`
	AverageWeatherTemp float64    `json:"averageWeatherTemp,omitempty"`
	AverageFeelsLike   float64    `json:"averageFeelsLike,omitempty"`
	StrainScore        float64    `json:"strainScore,omitempty"`
	Source             string     `json:"source,omitempty"`
	StravaID           string     `json:"stravaId,omitempty"`
	ExternalID         string     `json:"externalId,omitempty"`
	DeviceName         string     `json:"deviceName,omitempty"`
	Tags               []string   `json:"tags,omitempty"`
	ZoneTimes          []ZoneTime `json:"icuZoneTimes,omitempty"`
	HRZoneTimes        []int      `json:"icuHrZoneTimes,omitempty"`
	Pace               float64    `json:"pace,omitempty"`
	ThresholdPace      float64    `json:"thresholdPace,omitempty"`
	LTHR               int        `json:"lthr,omitempty"`
	AthleteMaxHR       int        `json:"athleteMaxHr,omitempty"`
	ATL                float64    `json:"icuAtl,omitempty"`
	CTL                float64    `json:"icuCtl,omitempty"`
}

type ZoneTime struct {
	ID   string `json:"id"`
	Secs int    `json:"secs"`
}

type UploadResponse struct {
	ID         string       `json:"id,omitempty"`
	AthleteID  string       `json:"icuAthleteId,omitempty"`
	Activities []ActivityID `json:"activities,omitempty"`
}

type ActivityID struct {
	ID        string `json:"id"`
	AthleteID string `json:"icuAthleteId,omitempty"`
}

type ActivityMini struct {
	ID             string `json:"id"`
	StartDateLocal string `json:"startDateLocal,omitempty"`
	Type           string `json:"type,omitempty"`
	Name           string `json:"name,omitempty"`
}

type ActivitySearchResult struct {
	ID             string   `json:"id"`
	Name           string   `json:"name,omitempty"`
	StartDateLocal string   `json:"startDateLocal,omitempty"`
	Type           string   `json:"type,omitempty"`
	Distance       float64  `json:"distance,omitempty"`
	MovingTime     int      `json:"movingTime,omitempty"`
	Tags           []string `json:"tags,omitempty"`
	Description    string   `json:"description,omitempty"`
}

type Wellness struct {
	ID           string      `json:"id"`
	Weight       float64     `json:"weight,omitempty"`
	RestingHR    int         `json:"restingHr,omitempty"`
	HRV          float64     `json:"hrv,omitempty"`
	HRVSDNN      float64     `json:"hrvSdnn,omitempty"`
	SleepSecs    int         `json:"sleepSecs,omitempty"`
	SleepScore   float64     `json:"sleepScore,omitempty"`
	SleepQuality int         `json:"sleepQuality,omitempty"`
	AvgSleepHR   float64     `json:"avgSleepingHr,omitempty"`
	Readiness    float64     `json:"readiness,omitempty"`
	BaevskySI    float64     `json:"baevskySi,omitempty"`
	SpO2         float64     `json:"spO2,omitempty"`
	Systolic     int         `json:"systolic,omitempty"`
	Diastolic    int         `json:"diastolic,omitempty"`
	Kcal         int         `json:"kcalConsumed,omitempty"`
	Soreness     int         `json:"soreness,omitempty"`
	Fatigue      int         `json:"fatigue,omitempty"`
	Stress       int         `json:"stress,omitempty"`
	Mood         int         `json:"mood,omitempty"`
	Motivation   int         `json:"motivation,omitempty"`
	Injury       int         `json:"injury,omitempty"`
	Hydration    int         `json:"hydration,omitempty"`
	HydrationVol float64     `json:"hydrationVolume,omitempty"`
	BloodGlucose float64     `json:"bloodGlucose,omitempty"`
	Lactate      float64     `json:"lactate,omitempty"`
	BodyFat      float64     `json:"bodyFat,omitempty"`
	Abdomen      float64     `json:"abdomen,omitempty"`
	VO2Max       float64     `json:"vo2max,omitempty"`
	Steps        int         `json:"steps,omitempty"`
	Respiration  float64     `json:"respiration,omitempty"`
	Comments     string      `json:"comments,omitempty"`
	CTL          float64     `json:"ctl,omitempty"`
	ATL          float64     `json:"atl,omitempty"`
	RampRate     float64     `json:"rampRate,omitempty"`
	SportInfo    []SportInfo `json:"sportInfo,omitempty"`
	Locked       bool        `json:"locked,omitempty"`
}

type SportInfo struct {
	Type   string  `json:"type"`
	EFTP   float64 `json:"eftp,omitempty"`
	WPrime float64 `json:"wPrime,omitempty"`
	PMax   float64 `json:"pMax,omitempty"`
}

type Event struct {
	ID                int      `json:"id,omitempty"`
	StartDateLocal    string   `json:"startDateLocal,omitempty"`
	EndDateLocal      string   `json:"endDateLocal,omitempty"`
	Category          string   `json:"category,omitempty"`
	Type              string   `json:"type,omitempty"`
	Name              string   `json:"name,omitempty"`
	Description       string   `json:"description,omitempty"`
	TrainingLoad      int      `json:"icuTrainingLoad,omitempty"`
	MovingTime        int      `json:"movingTime,omitempty"`
	Distance          float64  `json:"distance,omitempty"`
	Color             string   `json:"color,omitempty"`
	Indoor            bool     `json:"indoor,omitempty"`
	FTP               int      `json:"icuFtp,omitempty"`
	ATL               float64  `json:"icuAtl,omitempty"`
	CTL               float64  `json:"icuCtl,omitempty"`
	Target            string   `json:"target,omitempty"`
	UID               string   `json:"uid,omitempty"`
	CalendarID        int      `json:"calendarId,omitempty"`
	Tags              []string `json:"tags,omitempty"`
	ExternalID        string   `json:"externalId,omitempty"`
	HideFromAthlete   bool     `json:"hideFromAthlete,omitempty"`
	AthleteCannotEdit bool     `json:"athleteCannotEdit,omitempty"`
	Intensity         float64  `json:"icuIntensity,omitempty"`
	StrainScore       float64  `json:"strainScore,omitempty"`
	WorkoutDoc        any      `json:"workoutDoc,omitempty"`
	WorkoutFileBase64 string   `json:"workoutFileBase64,omitempty"`
	WorkoutFilename   string   `json:"workoutFilename,omitempty"`
}

// EventEx represents the request body for creating or updating calendar events.
// Field names use snake_case to match the Intervals.icu API contract.
//
//nolint:tagliatelle // API requires snake_case
type EventEx struct {
	StartDateLocal  string   `json:"start_date_local,omitempty"`
	EndDateLocal    string   `json:"end_date_local,omitempty"`
	Category        string   `json:"category,omitempty"`
	Type            string   `json:"type,omitempty"`
	Name            string   `json:"name,omitempty"`
	Description     string   `json:"description,omitempty"`
	TrainingLoad    int      `json:"icu_training_load,omitempty"`
	MovingTime      int      `json:"moving_time,omitempty"`
	Distance        float64  `json:"distance,omitempty"`
	Color           string   `json:"color,omitempty"`
	Indoor          bool     `json:"indoor,omitempty"`
	Target          string   `json:"target,omitempty"`
	Tags            []string `json:"tags,omitempty"`
	ExternalID      string   `json:"external_id,omitempty"`
	FileContents    string   `json:"file_contents,omitempty"`
	FileContentsB64 string   `json:"file_contents_base64,omitempty"`
	Filename        string   `json:"filename,omitempty"`
}

type Workout struct {
	ID           int      `json:"id,omitempty"`
	Name         string   `json:"name,omitempty"`
	Description  string   `json:"description,omitempty"`
	Type         string   `json:"type,omitempty"`
	TrainingLoad int      `json:"icuTrainingLoad,omitempty"`
	MovingTime   int      `json:"movingTime,omitempty"`
	Distance     float64  `json:"distance,omitempty"`
	Indoor       bool     `json:"indoor,omitempty"`
	Target       string   `json:"target,omitempty"`
	FolderID     int      `json:"folderId,omitempty"`
	Day          int      `json:"day,omitempty"`
	Tags         []string `json:"tags,omitempty"`
	Intensity    float64  `json:"icuIntensity,omitempty"`
	WorkoutDoc   any      `json:"workoutDoc,omitempty"`
}

type WorkoutEx struct {
	Name            string   `json:"name,omitempty"`
	Description     string   `json:"description,omitempty"`
	Type            string   `json:"type,omitempty"`
	FolderID        int      `json:"folderId,omitempty"`
	TrainingLoad    int      `json:"icuTrainingLoad,omitempty"`
	MovingTime      int      `json:"movingTime,omitempty"`
	Indoor          bool     `json:"indoor,omitempty"`
	Target          string   `json:"target,omitempty"`
	Tags            []string `json:"tags,omitempty"`
	FileContents    string   `json:"fileContents,omitempty"`
	FileContentsB64 string   `json:"fileContentsBase64,omitempty"`
	Filename        string   `json:"filename,omitempty"`
}

type Folder struct {
	ID            int       `json:"id,omitempty"`
	Name          string    `json:"name,omitempty"`
	Description   string    `json:"description,omitempty"`
	Type          string    `json:"type,omitempty"`
	Visibility    string    `json:"visibility,omitempty"`
	StartDate     string    `json:"startDateLocal,omitempty"`
	Children      []Workout `json:"children,omitempty"`
	ActivityTypes []string  `json:"activityTypes,omitempty"`
}

type FolderCreate struct {
	Name          string   `json:"name"`
	Description   string   `json:"description,omitempty"`
	Type          string   `json:"type,omitempty"`
	Visibility    string   `json:"visibility,omitempty"`
	ActivityTypes []string `json:"activityTypes,omitempty"`
}

type SportSettings struct {
	ID            int       `json:"id,omitempty"`
	AthleteID     string    `json:"athleteId,omitempty"`
	Types         []string  `json:"types,omitempty"`
	FTP           int       `json:"ftp,omitempty"`
	IndoorFTP     int       `json:"indoorFtp,omitempty"`
	WPrime        int       `json:"wPrime,omitempty"`
	PMax          int       `json:"pMax,omitempty"`
	LTHR          int       `json:"lthr,omitempty"`
	MaxHR         int       `json:"maxHr,omitempty"`
	PowerZones    []int     `json:"powerZones,omitempty"`
	HRZones       []int     `json:"hrZones,omitempty"`
	PaceZones     []float64 `json:"paceZones,omitempty"`
	ThresholdPace float64   `json:"thresholdPace,omitempty"`
	PaceUnits     string    `json:"paceUnits,omitempty"`
	HRLoadType    string    `json:"hrLoadType,omitempty"`
	PaceLoadType  string    `json:"paceLoadType,omitempty"`
	GapModel      string    `json:"gapModel,omitempty"`
}

type Interval struct {
	StartIndex    int     `json:"startIndex,omitempty"`
	EndIndex      int     `json:"endIndex,omitempty"`
	Type          string  `json:"type,omitempty"`
	Distance      float64 `json:"distance,omitempty"`
	MovingTime    int     `json:"movingTime,omitempty"`
	AvgPower      int     `json:"averageWatts,omitempty"`
	MaxPower      int     `json:"maxWatts,omitempty"`
	AvgHR         int     `json:"averageHeartrate,omitempty"`
	MaxHR         int     `json:"maxHeartrate,omitempty"`
	Intensity     int     `json:"intensity,omitempty"`
	TrainingLoad  float64 `json:"trainingLoad,omitempty"`
	Joules        int     `json:"joules,omitempty"`
	Zone          int     `json:"zone,omitempty"`
	AvgCadence    float64 `json:"averageCadence,omitempty"`
	ElevationGain float64 `json:"totalElevationGain,omitempty"`
	GroupID       string  `json:"groupId,omitempty"`
}

type IntervalsDTO struct {
	ID        string          `json:"id,omitempty"`
	Analyzed  string          `json:"analyzed,omitempty"`
	Intervals []Interval      `json:"icuIntervals,omitempty"`
	Groups    []IntervalGroup `json:"icuGroups,omitempty"`
}

type IntervalGroup struct {
	StartIndex   int     `json:"startIndex,omitempty"`
	EndIndex     int     `json:"endIndex,omitempty"`
	Distance     float64 `json:"distance,omitempty"`
	MovingTime   int     `json:"movingTime,omitempty"`
	AvgPower     int     `json:"averageWatts,omitempty"`
	MaxPower     int     `json:"maxWatts,omitempty"`
	AvgHR        int     `json:"averageHeartrate,omitempty"`
	MaxHR        int     `json:"maxHeartrate,omitempty"`
	Intensity    int     `json:"intensity,omitempty"`
	TrainingLoad float64 `json:"trainingLoad,omitempty"`
	Joules       int     `json:"joules,omitempty"`
	Zone         int     `json:"zone,omitempty"`
	Count        int     `json:"count,omitempty"`
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
	AvgTemp      float64 `json:"averageTemp,omitempty"`
	MinTemp      float64 `json:"minWeatherTemp,omitempty"`
	MaxTemp      float64 `json:"maxWeatherTemp,omitempty"`
	AvgFeelsLike float64 `json:"averageFeelsLike,omitempty"`
	AvgWindSpeed float64 `json:"averageWindSpeed,omitempty"`
	HeadwindPct  float64 `json:"headwindPercent,omitempty"`
	TailwindPct  float64 `json:"tailwindPercent,omitempty"`
	Description  string  `json:"description,omitempty"`
}

type BestEfforts struct {
	Efforts []Effort `json:"efforts,omitempty"`
}

type Effort struct {
	StartIndex int     `json:"startIndex,omitempty"`
	EndIndex   int     `json:"endIndex,omitempty"`
	Average    float64 `json:"average,omitempty"`
	Duration   int     `json:"duration,omitempty"`
	Distance   float64 `json:"distance,omitempty"`
}

type DataCurve struct {
	ID           string    `json:"id,omitempty"`
	Label        string    `json:"label,omitempty"`
	StartDate    string    `json:"startDateLocal,omitempty"`
	EndDate      string    `json:"endDateLocal,omitempty"`
	Days         int       `json:"days,omitempty"`
	MovingTime   int       `json:"movingTime,omitempty"`
	TrainingLoad int       `json:"trainingLoad,omitempty"`
	Weight       float64   `json:"weight,omitempty"`
	Secs         []int     `json:"secs,omitempty"`
	Values       []int     `json:"values,omitempty"`
	Distance     []float64 `json:"distance,omitempty"`
}

type (
	PowerCurve struct{ DataCurve }
	HRCurve    struct{ DataCurve }
	PaceCurve  struct{ DataCurve }
)

type PowerHRCurve struct {
	AthleteID  string `json:"athleteId,omitempty"`
	MinWatts   int    `json:"minWatts,omitempty"`
	MaxWatts   int    `json:"maxWatts,omitempty"`
	BucketSize int    `json:"bucketSize,omitempty"`
	FTP        int    `json:"ftp,omitempty"`
	LTHR       int    `json:"lthr,omitempty"`
	MaxHR      int    `json:"maxHr,omitempty"`
}

type PowerModel struct {
	Type          string `json:"type,omitempty"`
	CriticalPower int    `json:"criticalPower,omitempty"`
	WPrime        int    `json:"wPrime,omitempty"`
	PMax          int    `json:"pMax,omitempty"`
	FTP           int    `json:"ftp,omitempty"`
}

type Route struct {
	AthleteID     string   `json:"athleteId,omitempty"`
	RouteID       int64    `json:"routeId,omitempty"`
	Name          string   `json:"name,omitempty"`
	Description   string   `json:"description,omitempty"`
	RenameActs    bool     `json:"renameActivities,omitempty"`
	Commute       bool     `json:"commute,omitempty"`
	Tags          []string `json:"tags,omitempty"`
	Distance      float64  `json:"distance,omitempty"`
	ActivityCount int      `json:"activityCount,omitempty"`
}

type RouteSimilarity struct {
	Similarity float64 `json:"similarity,omitempty"`
}

type Gear struct {
	ID         string         `json:"id,omitempty"`
	AthleteID  string         `json:"athleteId,omitempty"`
	Type       string         `json:"type,omitempty"`
	Name       string         `json:"name,omitempty"`
	Distance   float64        `json:"distance,omitempty"`
	Time       float64        `json:"time,omitempty"`
	Activities int            `json:"activities,omitempty"`
	Retired    string         `json:"retired,omitempty"`
	Reminders  []GearReminder `json:"reminders,omitempty"`
}

type GearReminder struct {
	ID         int     `json:"id,omitempty"`
	Name       string  `json:"name,omitempty"`
	Distance   float64 `json:"distance,omitempty"`
	Time       float64 `json:"time,omitempty"`
	Activities int     `json:"activities,omitempty"`
	Days       int     `json:"days,omitempty"`
	LastReset  string  `json:"lastReset,omitempty"`
	PctUsed    float64 `json:"percentUsed,omitempty"`
}

type GearStats struct {
	Distance    float64 `json:"distance,omitempty"`
	ElapsedTime float64 `json:"elapsedTime,omitempty"`
	MovingTime  float64 `json:"movingTime,omitempty"`
	Activities  int     `json:"activities,omitempty"`
}

type Chat struct {
	ID             int    `json:"id,omitempty"`
	Type           string `json:"type,omitempty"`
	Name           string `json:"name,omitempty"`
	Description    string `json:"description,omitempty"`
	ActivityID     string `json:"activityId,omitempty"`
	AthleteID      string `json:"athleteId,omitempty"`
	OtherAthleteID string `json:"otherAthleteId,omitempty"`
	NewMsgCount    int    `json:"newMessageCount,omitempty"`
}

type Message struct {
	ID        int    `json:"id,omitempty"`
	AthleteID string `json:"athleteId,omitempty"`
	Name      string `json:"name,omitempty"`
	Content   string `json:"content,omitempty"`
	Created   string `json:"created,omitempty"`
	Type      string `json:"type,omitempty"`
}

type NewMessage struct {
	ChatID      int    `json:"chatId,omitempty"`
	ToAthleteID string `json:"toAthleteId,omitempty"`
	Content     string `json:"content"`
}

type SendResponse struct {
	ID      int     `json:"id,omitempty"`
	Message Message `json:"message,omitempty"`
}

type CustomItem struct {
	ID          int    `json:"id,omitempty"`
	AthleteID   string `json:"athleteId,omitempty"`
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
	ID       int     `json:"id,omitempty"`
	Provider string  `json:"provider,omitempty"`
	Label    string  `json:"label,omitempty"`
	Lat      float64 `json:"lat,omitempty"`
	Lon      float64 `json:"lon,omitempty"`
	Enabled  bool    `json:"enabled,omitempty"`
}

type WeatherDTO struct {
	Forecasts []Forecast `json:"forecasts,omitempty"`
}

type DoomedEvent struct {
	ID         int    `json:"id,omitempty"`
	ExternalID string `json:"externalId,omitempty"`
}

type DeleteEventsResponse struct {
	EventsDeleted int `json:"eventsDeleted,omitempty"`
}

type DeleteResponse struct {
	ID        string `json:"id,omitempty"`
	AthleteID string `json:"icuAthleteId,omitempty"`
}

type SummaryWithCats struct {
	Date         string  `json:"date,omitempty"`
	AthleteID    string  `json:"athleteId,omitempty"`
	AthleteName  string  `json:"athleteName,omitempty"`
	Fitness      float64 `json:"fitness,omitempty"`
	Fatigue      float64 `json:"fatigue,omitempty"`
	Form         float64 `json:"form,omitempty"`
	TrainingLoad int     `json:"trainingLoad,omitempty"`
	Weight       float64 `json:"weight,omitempty"`
	Distance     float64 `json:"distance,omitempty"`
}
