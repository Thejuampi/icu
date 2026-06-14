package icu

import "fmt"

type StreamData map[string][]float64

func NormalizeStreams(raw []ActivityStream) (StreamData, error) {
	result := make(StreamData)

	minLen := -1

	for i := range raw {
		stream := &raw[i]
		if stream.Type == "" {
			continue
		}

		data, err := coerceToFloat64Slice(stream.Data)
		if err != nil {
			return nil, fmt.Errorf("stream %q: %w", stream.Type, err)
		}

		if len(data) == 0 {
			continue
		}

		if minLen < 0 || len(data) < minLen {
			minLen = len(data)
		}

		if _, exists := result[stream.Type]; exists {
			return nil, fmt.Errorf("duplicate stream type %q", stream.Type)
		}
		result[stream.Type] = data
	}

	for k := range result {
		result[k] = result[k][:minLen]
	}

	return result, nil
}

func coerceToFloat64Slice(data any) ([]float64, error) {
	if data == nil {
		return nil, nil
	}

	switch values := data.(type) {
	case []float64:
		return values, nil
	case []int:
		return convertIntSlice(values), nil
	case []int64:
		result := make([]float64, len(values))
		for i, v := range values {
			result[i] = float64(v)
		}
		return result, nil
	case []int32:
		result := make([]float64, len(values))
		for i, v := range values {
			result[i] = float64(v)
		}
		return result, nil
	case []float32:
		result := make([]float64, len(values))
		for i, v := range values {
			result[i] = float64(v)
		}
		return result, nil
	case []any:
		result := make([]float64, len(values))
		for i, val := range values {
			f, ok := toFloat64(val)
			if !ok {
				return nil, fmt.Errorf("element %d is not numeric: %T", i, val)
			}
			result[i] = f
		}
		return result, nil
	default:
		return nil, fmt.Errorf("unsupported data type: %T", data)
	}
}

func convertIntSlice(values []int) []float64 {
	result := make([]float64, len(values))
	for i, v := range values {
		result[i] = float64(v)
	}
	return result
}

func toFloat64(v any) (float64, bool) {
	switch num := v.(type) {
	case float64:
		return num, true
	case float32:
		return float64(num), true
	case int:
		return float64(num), true
	case int64:
		return float64(num), true
	case int32:
		return float64(num), true
	default:
		return 0, false
	}
}
