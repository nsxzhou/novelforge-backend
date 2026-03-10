package metric

import (
	"context"
	"errors"
	"testing"
	"time"

	metricdomain "novelforge/backend/internal/domain/metric"
	"novelforge/backend/internal/infra/storage/memory"
	appservice "novelforge/backend/internal/service"

	"github.com/google/uuid"
)

func TestAppendMetricEventDefaultsAndTrims(t *testing.T) {
	repo := memory.NewMetricEventRepository()
	uc := NewUseCase(Dependencies{MetricEvents: repo})
	projectID := uuid.NewString()
	chapterID := uuid.NewString()
	event := &metricdomain.MetricEvent{
		EventName: "  operation_completed  ",
		ProjectID: "  " + projectID + "  ",
		ChapterID: "  " + chapterID + "  ",
	}

	if err := uc.Append(context.Background(), event); err != nil {
		t.Fatalf("Append() error = %v", err)
	}
	if event.ID == "" {
		t.Fatal("Append() ID = empty, want generated UUID")
	}
	if _, err := uuid.Parse(event.ID); err != nil {
		t.Fatalf("Append() ID = %q, want valid UUID", event.ID)
	}
	if event.EventName != metricdomain.EventOperationCompleted {
		t.Fatalf("Append() EventName = %q, want %q", event.EventName, metricdomain.EventOperationCompleted)
	}
	if event.ProjectID != projectID {
		t.Fatalf("Append() ProjectID = %q, want %q", event.ProjectID, projectID)
	}
	if event.ChapterID != chapterID {
		t.Fatalf("Append() ChapterID = %q, want %q", event.ChapterID, chapterID)
	}
	if event.OccurredAt.IsZero() {
		t.Fatal("Append() OccurredAt = zero, want non-zero")
	}
	if event.Labels == nil {
		t.Fatal("Append() Labels = nil, want initialized map")
	}
	if event.Stats == nil {
		t.Fatal("Append() Stats = nil, want initialized map")
	}

	items, err := uc.ListByProject(context.Background(), metricdomain.ListByProjectParams{ProjectID: projectID})
	if err != nil {
		t.Fatalf("ListByProject() error = %v", err)
	}
	if len(items) != 1 || items[0].ID != event.ID {
		t.Fatalf("ListByProject() = %#v, want one appended event", items)
	}
}

func TestAppendMetricEventConvertsConflict(t *testing.T) {
	repo := memory.NewMetricEventRepository()
	uc := NewUseCase(Dependencies{MetricEvents: repo})
	projectID := uuid.NewString()
	eventID := uuid.NewString()
	timestamp := time.Date(2026, 3, 10, 10, 0, 0, 0, time.UTC)
	first := &metricdomain.MetricEvent{
		ID:         eventID,
		EventName:  metricdomain.EventOperationCompleted,
		ProjectID:  projectID,
		OccurredAt: timestamp,
		Labels:     map[string]string{},
		Stats:      map[string]float64{},
	}
	second := &metricdomain.MetricEvent{
		ID:         eventID,
		EventName:  metricdomain.EventOperationFailed,
		ProjectID:  projectID,
		OccurredAt: timestamp.Add(time.Second),
		Labels:     map[string]string{},
		Stats:      map[string]float64{},
	}

	if err := uc.Append(context.Background(), first); err != nil {
		t.Fatalf("Append() first error = %v", err)
	}
	err := uc.Append(context.Background(), second)
	if !errors.Is(err, appservice.ErrConflict) {
		t.Fatalf("Append() second error = %v, want ErrConflict", err)
	}
}

func TestAppendMetricEventRejectsNil(t *testing.T) {
	uc := NewUseCase(Dependencies{MetricEvents: memory.NewMetricEventRepository()})

	err := uc.Append(context.Background(), nil)
	if !errors.Is(err, appservice.ErrInvalidInput) {
		t.Fatalf("Append() error = %v, want ErrInvalidInput", err)
	}
}

func TestListByProjectValidatesInput(t *testing.T) {
	uc := NewUseCase(Dependencies{MetricEvents: memory.NewMetricEventRepository()})
	projectID := uuid.NewString()

	_, err := uc.ListByProject(context.Background(), metricdomain.ListByProjectParams{ProjectID: "invalid"})
	if !errors.Is(err, appservice.ErrInvalidInput) {
		t.Fatalf("ListByProject() invalid project error = %v, want ErrInvalidInput", err)
	}

	_, err = uc.ListByProject(context.Background(), metricdomain.ListByProjectParams{ProjectID: projectID, Limit: -1})
	if !errors.Is(err, appservice.ErrInvalidInput) {
		t.Fatalf("ListByProject() invalid limit error = %v, want ErrInvalidInput", err)
	}

	_, err = uc.ListByProject(context.Background(), metricdomain.ListByProjectParams{ProjectID: projectID, Offset: -1})
	if !errors.Is(err, appservice.ErrInvalidInput) {
		t.Fatalf("ListByProject() invalid offset error = %v, want ErrInvalidInput", err)
	}
}

func TestListByProjectTrimsEventName(t *testing.T) {
	repo := memory.NewMetricEventRepository()
	uc := NewUseCase(Dependencies{MetricEvents: repo})
	projectID := uuid.NewString()
	timestamp := time.Date(2026, 3, 10, 10, 0, 0, 0, time.UTC)
	event := &metricdomain.MetricEvent{
		ID:         uuid.NewString(),
		EventName:  metricdomain.EventOperationCompleted,
		ProjectID:  projectID,
		OccurredAt: timestamp,
		Labels:     map[string]string{},
		Stats:      map[string]float64{},
	}
	if err := repo.Append(context.Background(), event); err != nil {
		t.Fatalf("repo.Append() error = %v", err)
	}

	items, err := uc.ListByProject(context.Background(), metricdomain.ListByProjectParams{
		ProjectID: "  " + projectID + "  ",
		EventName: "  " + metricdomain.EventOperationCompleted + "  ",
	})
	if err != nil {
		t.Fatalf("ListByProject() error = %v", err)
	}
	if len(items) != 1 || items[0].ID != event.ID {
		t.Fatalf("ListByProject() = %#v, want one filtered event", items)
	}
}
