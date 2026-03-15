package metric

import (
	"context"

	metricdomain "inkmuse/backend/internal/domain/metric"
)

// Dependencies 声明指标(metric)用例所需的领域依赖项。
type Dependencies struct {
	MetricEvents metricdomain.MetricEventRepository
}

// UseCase 定义指标(metric)的应用边界。
type UseCase interface {
	Append(ctx context.Context, event *metricdomain.MetricEvent) error
	ListByProject(ctx context.Context, params metricdomain.ListByProjectParams) ([]*metricdomain.MetricEvent, error)
}
