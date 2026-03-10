package metric

import (
	"context"
	"fmt"
	"strings"
	"time"

	metricdomain "novelforge/backend/internal/domain/metric"
	appservice "novelforge/backend/internal/service"

	"github.com/google/uuid"
)

type useCase struct {
	metricEvents metricdomain.MetricEventRepository
}

// NewUseCase 创建指标(metric)用例实现。
func NewUseCase(deps Dependencies) UseCase {
	return &useCase{metricEvents: deps.MetricEvents}
}

func (u *useCase) Append(ctx context.Context, event *metricdomain.MetricEvent) error {
	if event == nil {
		return appservice.WrapInvalidInput(fmt.Errorf("metric event must not be nil"))
	}

	event.ID = strings.TrimSpace(event.ID)
	event.EventName = strings.TrimSpace(event.EventName)
	event.ProjectID = strings.TrimSpace(event.ProjectID)
	event.ChapterID = strings.TrimSpace(event.ChapterID)
	if event.ID == "" {
		event.ID = uuid.NewString()
	}
	if event.OccurredAt.IsZero() {
		event.OccurredAt = time.Now().UTC()
	} else {
		event.OccurredAt = event.OccurredAt.UTC()
	}
	if event.Labels == nil {
		event.Labels = map[string]string{}
	}
	if event.Stats == nil {
		event.Stats = map[string]float64{}
	}

	if err := event.Validate(); err != nil {
		return appservice.WrapInvalidInput(err)
	}
	if err := u.metricEvents.Append(ctx, event); err != nil {
		return appservice.TranslateStorageError(err)
	}
	return nil
}

func (u *useCase) ListByProject(ctx context.Context, params metricdomain.ListByProjectParams) ([]*metricdomain.MetricEvent, error) {
	params.ProjectID = strings.TrimSpace(params.ProjectID)
	params.EventName = strings.TrimSpace(params.EventName)
	if _, err := uuid.Parse(params.ProjectID); err != nil {
		return nil, appservice.WrapInvalidInput(fmt.Errorf("project_id must be a valid UUID"))
	}
	if params.Limit < 0 {
		return nil, appservice.WrapInvalidInput(fmt.Errorf("limit must be greater than or equal to 0"))
	}
	if params.Offset < 0 {
		return nil, appservice.WrapInvalidInput(fmt.Errorf("offset must be greater than or equal to 0"))
	}

	items, err := u.metricEvents.ListByProject(ctx, params)
	if err != nil {
		return nil, appservice.TranslateStorageError(err)
	}
	return items, nil
}
