package memory

import (
	"novelforge/backend/internal/domain/asset"
	"novelforge/backend/internal/domain/chapter"
	"novelforge/backend/internal/domain/conversation"
	"novelforge/backend/internal/domain/generation"
	"novelforge/backend/internal/domain/metric"
	"novelforge/backend/internal/domain/project"
)

func sliceBounds(limit, offset, length int) (int, int) {
	if offset < 0 {
		offset = 0
	}
	if offset >= length {
		return length, length
	}
	end := length
	if limit > 0 && offset+limit < end {
		end = offset + limit
	}
	return offset, end
}

func cloneProject(src *project.Project) *project.Project {
	if src == nil {
		return nil
	}
	dst := *src
	return &dst
}

func cloneAsset(src *asset.Asset) *asset.Asset {
	if src == nil {
		return nil
	}
	dst := *src
	return &dst
}

func cloneChapter(src *chapter.Chapter) *chapter.Chapter {
	if src == nil {
		return nil
	}
	dst := *src
	if src.CurrentDraftConfirmedAt != nil {
		confirmedAt := *src.CurrentDraftConfirmedAt
		dst.CurrentDraftConfirmedAt = &confirmedAt
	}
	return &dst
}

func cloneConversation(src *conversation.Conversation) *conversation.Conversation {
	if src == nil {
		return nil
	}
	dst := *src
	if len(src.Messages) > 0 {
		dst.Messages = make([]conversation.Message, len(src.Messages))
		copy(dst.Messages, src.Messages)
	}
	return &dst
}

func cloneGenerationRecord(src *generation.GenerationRecord) *generation.GenerationRecord {
	if src == nil {
		return nil
	}
	dst := *src
	return &dst
}

func cloneMetricEvent(src *metric.MetricEvent) *metric.MetricEvent {
	if src == nil {
		return nil
	}
	dst := *src
	if src.Labels != nil {
		dst.Labels = make(map[string]string, len(src.Labels))
		for key, value := range src.Labels {
			dst.Labels[key] = value
		}
	}
	if src.Stats != nil {
		dst.Stats = make(map[string]float64, len(src.Stats))
		for key, value := range src.Stats {
			dst.Stats[key] = value
		}
	}
	return &dst
}
