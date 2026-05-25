package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientGetAthlete(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}

		if r.URL.Path != "/api/v1/athlete/0" {
			t.Errorf("expected /api/v1/athlete/0, got %s", r.URL.Path)
		}

		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Basic ") {
			t.Errorf("missing Basic auth header: %s", auth)
		}

		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"id":"i123","name":"Juan","weight":81}`)
	}))

	defer srv.Close()

	client := &Client{httpClient: srv.Client(), apiKey: "test-key", athleteID: "0", baseURL: srv.URL}

	var a Athlete

	err := client.Get("athlete", nil, nil, &a)
	if err != nil {
		t.Fatal(err)
	}

	if a.ID != "i123" || a.Name != "Juan" {
		t.Errorf("unexpected athlete: %+v", a)
	}
}

func TestClientPostEvent(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}

		if r.URL.Path != "/api/v1/athlete/0/events" {
			t.Errorf("expected /api/v1/athlete/0/events, got %s", r.URL.Path)
		}

		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("expected application/json, got %s", ct)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, `{"id":42,"name":"Test Workout","category":"WORKOUT"}`)
	}))

	defer srv.Close()

	client := &Client{httpClient: srv.Client(), apiKey: "test-key", athleteID: "0", baseURL: srv.URL}

	var ev EventEx
	ev.StartDateLocal = "2026-05-25T07:00:00"
	ev.Category = "WORKOUT"
	ev.Name = "Test Workout"
	ev.Type = "Ride"
	ev.MovingTime = 3600

	var result Event

	err := client.Post("events", nil, nil, ev, &result)
	if err != nil {
		t.Fatal(err)
	}

	if result.ID != 42 {
		t.Errorf("expected id 42, got %d", result.ID)
	}
}

func TestClientPutWellnessBulk(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("expected PUT, got %s", r.Method)
		}

		if r.URL.Path != "/api/v1/athlete/0/wellness-bulk" {
			t.Errorf("expected /wellness-bulk, got %s", r.URL.Path)
		}

		w.WriteHeader(http.StatusOK)
	}))

	defer srv.Close()

	client := &Client{httpClient: srv.Client(), apiKey: "test-key", athleteID: "0", baseURL: srv.URL}

	var w1 Wellness
	w1.ID = "2026-05-24"
	w1.Weight = 81

	var w2 Wellness
	w2.ID = "2026-05-23"
	w2.Weight = 81.2

	body := []Wellness{w1, w2}

	err := client.Put("wellness-bulk", nil, nil, body, nil)
	if err != nil {
		t.Fatal(err)
	}
}

func TestClientDeleteActivity(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}

		w.WriteHeader(http.StatusOK)
		io.WriteString(w, `{"id":"i123","icuAthleteId":"i445643"}`)
	}))

	defer srv.Close()

	client := &Client{httpClient: srv.Client(), apiKey: "test-key", athleteID: "0", baseURL: srv.URL}

	var resp DeleteResponse

	err := client.Delete("activity", []string{"i123"}, nil, &resp)
	if err != nil {
		t.Fatal(err)
	}

	if resp.ID != "i123" {
		t.Errorf("expected id i123, got %s", resp.ID)
	}
}

func TestClientErrorHandling(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		io.WriteString(w, `{"error":"invalid api key"}`)
	}))

	defer srv.Close()

	client := &Client{httpClient: srv.Client(), apiKey: "bad-key", athleteID: "0", baseURL: srv.URL}

	err := client.Get("athlete", nil, nil, nil)
	if err == nil {
		t.Fatal("expected error for 401")
	}

	if !strings.Contains(err.Error(), "401") {
		t.Errorf("expected 401 in error, got: %v", err)
	}
}

func TestClientQueryParams(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		oldest := r.URL.Query().Get("oldest")

		newest := r.URL.Query().Get("newest")
		if oldest != "2026-05-20" || newest != "2026-05-24" {
			t.Errorf("unexpected query: oldest=%s newest=%s", oldest, newest)
		}

		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `[]`)
	}))

	defer srv.Close()

	client := &Client{httpClient: srv.Client(), apiKey: "test-key", athleteID: "0", baseURL: srv.URL}

	var activities []Activity

	err := client.Get("activities", nil, map[string]string{"oldest": "2026-05-20", "newest": "2026-05-24"}, &activities)
	if err != nil {
		t.Fatal(err)
	}
}

func TestClientUploadFile(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}

		ct := r.Header.Get("Content-Type")
		if !strings.Contains(ct, "multipart/form-data") {
			t.Errorf("expected multipart, got %s", ct)
		}

		w.WriteHeader(http.StatusOK)
		io.WriteString(w, `{"activities":[]}`)
	}))

	defer srv.Close()

	client := &Client{httpClient: srv.Client(), apiKey: "test-key", athleteID: "0", baseURL: srv.URL}

	var resp UploadResponse

	err := client.UploadFile("activities", "", "testdata/activity.fit", map[string]string{"name": "Test"}, &resp)
	if err == nil {
		t.Fatal("expected file not found error")
	}
}

func TestClientDownload(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Write([]byte("fake fit data"))
	}))

	defer srv.Close()

	client := &Client{httpClient: srv.Client(), apiKey: "test-key", athleteID: "0", baseURL: srv.URL}

	data, err := client.Download("activity", []string{"i123", "file"}, nil)
	if err != nil {
		t.Fatal(err)
	}

	if string(data) != "fake fit data" {
		t.Errorf("unexpected download: %s", string(data))
	}
}

func TestClientAthleteIDInPath(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/athlete/i445643/wellness/2026-05-24" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"id":"2026-05-24","weight":81}`)
	}))

	defer srv.Close()

	client := &Client{httpClient: srv.Client(), apiKey: "test-key", athleteID: "i445643", baseURL: srv.URL}

	var w Wellness

	err := client.Get("wellness", []string{"2026-05-24"}, nil, &w)
	if err != nil {
		t.Fatal(err)
	}

	if w.ID != "2026-05-24" {
		t.Errorf("unexpected wellness id: %s", w.ID)
	}
}

func TestClientJSONError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `not json`)
	}))

	defer srv.Close()

	client := &Client{httpClient: srv.Client(), apiKey: "test-key", athleteID: "0", baseURL: srv.URL}

	var a Athlete

	err := client.Get("athlete", nil, nil, &a)
	if err == nil {
		t.Fatal("expected JSON parse error")
	}
}
