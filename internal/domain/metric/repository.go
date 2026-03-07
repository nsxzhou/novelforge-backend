package metric

import "context"

// ListByProjectParams 定义项目下的指标(metric)过滤器。
type ListByProjectParams struct {
	ProjectID string
	EventName string
	Limit     int
	Offset    int
}

// MetricEventRepository 定义指标(metric)事件持久化行为。
type MetricEventRepository interface {
	Append(ctx context.Context, event *MetricEvent) error
	ListByProject(ctx context.Context, params ListByProjectParams) ([]*MetricEvent, error)
}
