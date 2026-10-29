package icu

import "testing"

// Streams-mode activities: the mixed consumed average is 2.67, the API one 4.
func streamsModeActivities() []Activity {
	return []Activity{
		{
			ID: "a", Type: "Ride", StartDateLocal: "2026-10-20T08:00:00", Decoupling: 4, DecouplingSource: DecouplingSourceAPI,
			DecouplingAPI: new(4.0), DecouplingStreamsStatus: DecouplingStatusNotMeasurable,
		},
		{
			ID: "b", Type: "Ride", StartDateLocal: "2026-10-21T08:00:00", Decoupling: 1, DecouplingSource: DecouplingSourceStreams,
			DecouplingAPI: new(5.0), DecouplingStreams: new(1.0), DecouplingStreamsStatus: DecouplingStatusMeasured,
		},
		{ID: "c", Type: "Ride", StartDateLocal: "2026-10-22T08:00:00", Decoupling: 3},
	}
}

func TestDownstreamDecouplingUsesAPIAverage(t *testing.T) {
	t.Parallel()
	cycling := AnalyzeCyclingActivities(streamsModeActivities(), AnalysisOptions{})
	if got := microcycleFatigueDurability(&cycling, nil).AverageDecoupling; got != 4 {
		t.Fatalf("microcycle averageDecoupling = %v, want API average 4", got)
	}
	var history TrainingPlanHistory
	addCurrentTrainingStateToHistory(&history, streamsModeActivities(), TrainingPlanOptions{}, nil)
	if history.CurrentDecoupling != 4 {
		t.Fatalf("plan currentDecoupling = %v, want API average 4", history.CurrentDecoupling)
	}
}
