package icu

import "fmt"

// NullableSeries is a stream series that preserves missing samples.
// When Present[i] is false, Values[i] is undefined and must not be treated as zero.
type NullableSeries struct {
	Values  []float64
	Present []bool
}

// NullableStreamData maps stream type to a nullable series of equal length.
type NullableStreamData map[string]NullableSeries

// Len returns the common series length, or 0 when empty.
func (series NullableSeries) Len() int {
	return len(series.Values)
}

// At returns the value and whether it is present.
func (series NullableSeries) At(index int) (float64, bool) {
	if index < 0 || index >= len(series.Values) || index >= len(series.Present) {
		return 0, false
	}
	if !series.Present[index] {
		return 0, false
	}

	return series.Values[index], true
}

// DenseOrZero returns a dense float64 slice with missing samples as 0.
// Useful for callers that intentionally want zero-filled series.
func (series NullableSeries) DenseOrZero() []float64 {
	if len(series.Values) == 0 {
		return nil
	}
	out := make([]float64, len(series.Values))
	copy(out, series.Values)
	for i := range series.Present {
		if !series.Present[i] {
			out[i] = 0
		}
	}

	return out
}

// PreserveNullableStreams converts raw activity streams while keeping JSON null
// samples as absent. Existing NormalizeStreams continues to coerce null→0.
func PreserveNullableStreams(raw []ActivityStream) (NullableStreamData, error) {
	result := make(NullableStreamData)
	minLen := -1

	for i := range raw {
		stream := &raw[i]
		if stream.Type == "" {
			continue
		}

		series, err := coerceToNullableSeries(stream.Data)
		if err != nil {
			return nil, fmt.Errorf("stream %q: %w", stream.Type, err)
		}
		if series.Len() == 0 {
			continue
		}
		if minLen < 0 || series.Len() < minLen {
			minLen = series.Len()
		}
		if _, exists := result[stream.Type]; exists {
			return nil, fmt.Errorf("duplicate stream type %q", stream.Type)
		}
		result[stream.Type] = series
	}

	for key, series := range result {
		series.Values = series.Values[:minLen]
		series.Present = series.Present[:minLen]
		result[key] = series
	}

	return result, nil
}

func coerceToNullableSeries(data any) (NullableSeries, error) {
	if data == nil {
		return NullableSeries{}, nil
	}

	switch values := data.(type) {
	case []float64:
		return denseNullable(values), nil
	case []int:
		converted := convertIntSlice(values)
		return denseNullable(converted), nil
	case []int64:
		converted := make([]float64, len(values))
		for i, v := range values {
			converted[i] = float64(v)
		}
		return denseNullable(converted), nil
	case []int32:
		converted := make([]float64, len(values))
		for i, v := range values {
			converted[i] = float64(v)
		}
		return denseNullable(converted), nil
	case []float32:
		converted := make([]float64, len(values))
		for i, v := range values {
			converted[i] = float64(v)
		}
		return denseNullable(converted), nil
	case []any:
		return nullableFromAny(values)
	default:
		return NullableSeries{}, fmt.Errorf("unsupported data type: %T", data)
	}
}

func denseNullable(values []float64) NullableSeries {
	if len(values) == 0 {
		return NullableSeries{}
	}
	present := make([]bool, len(values))
	out := make([]float64, len(values))
	copy(out, values)
	for i := range present {
		present[i] = true
	}

	return NullableSeries{Values: out, Present: present}
}

func nullableFromAny(values []any) (NullableSeries, error) {
	out := make([]float64, len(values))
	present := make([]bool, len(values))
	for i, val := range values {
		if val == nil {
			continue
		}
		f, ok := toFloat64(val)
		if !ok {
			return NullableSeries{}, fmt.Errorf("element %d is not numeric: %T", i, val)
		}
		out[i] = f
		present[i] = true
	}

	return NullableSeries{Values: out, Present: present}, nil
}

// NullableStream returns a series by type, trying common aliases.
func NullableStream(streams NullableStreamData, key string) NullableSeries {
	if streams == nil {
		return NullableSeries{}
	}
	if series, ok := streams[key]; ok {
		return series
	}
	switch key {
	case "watts":
		if series, ok := streams["power"]; ok {
			return series
		}
	case "heartrate":
		if series, ok := streams["heart_rate"]; ok {
			return series
		}
	case "velocity_smooth":
		if series, ok := streams["velocity"]; ok {
			return series
		}
	}

	return NullableSeries{}
}
