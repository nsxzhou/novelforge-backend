package metric

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func validMetricEvent() MetricEvent {
	return MetricEvent{
		ID:         uuid.NewString(),
		EventName:  EventProjectCreated,
		ProjectID:  uuid.NewString(),
		ChapterID:  uuid.NewString(),
		Labels:     map[string]string{"source": "test"},
		Stats:      map[string]float64{"token_usage": 128},
		OccurredAt: time.Date(2026, 3, 6, 12, 0, 0, 0, time.UTC),
	}
}

func TestMetricEventValidate(t *testing.T) {
	event := validMetricEvent()
	if err := event.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestMetricEventValidateRejectsEmptyEventName(t *testing.T) {
	event := validMetricEvent()
	event.EventName = ""

	if err := event.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want error")
	}
}

func TestMetricEventValidateRejectsInvalidChapterID(t *testing.T) {
	event := validMetricEvent()
	event.ChapterID = "invalid"

	if err := event.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want error")
	}
}
