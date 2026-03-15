package memory

import (
	"context"
	"fmt"
	"sync"

	"inkmuse/backend/internal/domain/metric"
)

// MetricEventRepository 在内存中存储仅追加(append-only)的指标(metric)事件。
type MetricEventRepository struct {
	mu    sync.RWMutex
	items map[string]*metric.MetricEvent
	order []string
}

// NewMetricEventRepository 创建内存指标(metric)事件存储库。
func NewMetricEventRepository() *MetricEventRepository {
	return &MetricEventRepository{
		items: make(map[string]*metric.MetricEvent),
	}
}

func (r *MetricEventRepository) Append(_ context.Context, entity *metric.MetricEvent) error {
	if entity == nil {
		return fmt.Errorf("metric event must not be nil")
	}
	if err := entity.Validate(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.items[entity.ID]; exists {
		return ErrAlreadyExists
	}

	r.items[entity.ID] = cloneMetricEvent(entity)
	r.order = append(r.order, entity.ID)
	return nil
}

func (r *MetricEventRepository) ListByProject(_ context.Context, params metric.ListByProjectParams) ([]*metric.MetricEvent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*metric.MetricEvent, 0, len(r.order))
	for _, id := range r.order {
		entity := r.items[id]
		if entity.ProjectID != params.ProjectID {
			continue
		}
		if params.EventName != "" && entity.EventName != params.EventName {
			continue
		}
		result = append(result, cloneMetricEvent(entity))
	}

	start, end := sliceBounds(params.Limit, params.Offset, len(result))
	return result[start:end], nil
}
