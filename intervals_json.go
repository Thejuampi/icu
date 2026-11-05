package icu

import "encoding/json"

// The intervals.icu API returns intervals in snake_case (icu_intervals,
// average_watts, ...); these decoders also accept the camelCase keys this
// package writes, like the other snake-aware types.

type intervalAlias Interval

func (interval *Interval) UnmarshalJSON(data []byte) error {
	var alias intervalAlias

	return unmarshalWithSnakeFields(data, interval, &alias,
		func(a intervalAlias) Interval { return Interval(a) }, (*Interval).applySnakeFields, nil)
}

func (interval *Interval) applySnakeFields(raw map[string]json.RawMessage) {
	for key, target := range map[string]*int{
		"start_time": &interval.StartTime, "end_time": &interval.EndTime, "elapsed_time": &interval.ElapsedTime,
		"weighted_average_watts": &interval.WeightedPower, "start_index": &interval.StartIndex,
		"end_index": &interval.EndIndex, "moving_time": &interval.MovingTime, "average_watts": &interval.AvgPower,
		"max_watts": &interval.MaxPower, "average_heartrate": &interval.AvgHR, "max_heartrate": &interval.MaxHR,
	} {
		copyRawIfSet(target, raw, key)
	}
	copyRawIfSet(&interval.TrainingLoad, raw, "training_load")
	copyRawIfSet(&interval.AvgCadence, raw, "average_cadence")
	copyRawIfSet(&interval.ElevationGain, raw, "total_elevation_gain")
	copyRawIfSet(&interval.GroupID, raw, "group_id")
}

type intervalGroupAlias IntervalGroup

func (group *IntervalGroup) UnmarshalJSON(data []byte) error {
	var alias intervalGroupAlias

	return unmarshalWithSnakeFields(data, group, &alias,
		func(a intervalGroupAlias) IntervalGroup { return IntervalGroup(a) }, (*IntervalGroup).applySnakeFields, nil)
}

func (group *IntervalGroup) applySnakeFields(raw map[string]json.RawMessage) {
	copyRawIfSet(&group.ElapsedTime, raw, "elapsed_time")
	copyRawIfSet(&group.WeightedPower, raw, "weighted_average_watts")
	copyRawIfSet(&group.StartIndex, raw, "start_index")
	copyRawIfSet(&group.EndIndex, raw, "end_index")
	copyRawIfSet(&group.MovingTime, raw, "moving_time")
	copyRawIfSet(&group.AvgPower, raw, "average_watts")
	copyRawIfSet(&group.MaxPower, raw, "max_watts")
	copyRawIfSet(&group.AvgHR, raw, "average_heartrate")
	copyRawIfSet(&group.MaxHR, raw, "max_heartrate")
	copyRawIfSet(&group.TrainingLoad, raw, "training_load")
}

type intervalsDTOAlias IntervalsDTO

func (dto *IntervalsDTO) UnmarshalJSON(data []byte) error {
	var alias intervalsDTOAlias

	return unmarshalWithSnakeFields(data, dto, &alias,
		func(a intervalsDTOAlias) IntervalsDTO { return IntervalsDTO(a) }, (*IntervalsDTO).applySnakeFields, nil)
}

func (dto *IntervalsDTO) applySnakeFields(raw map[string]json.RawMessage) {
	copyRawValueIfSet(&dto.Intervals, raw, "icu_intervals")
	copyRawValueIfSet(&dto.Groups, raw, "icu_groups")
}
