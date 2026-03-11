package service

import (
	conversationdomain "novelforge/backend/internal/domain/conversation"
	generationdomain "novelforge/backend/internal/domain/generation"
	"novelforge/backend/pkg/config"
)

// PromptCapabilityForGenerationKind 将生成任务种类映射为 Prompt 能力。
func PromptCapabilityForGenerationKind(kind string) (config.PromptCapability, bool) {
	switch kind {
	case generationdomain.KindAssetGeneration:
		return config.PromptCapabilityAssetGeneration, true
	case generationdomain.KindChapterGeneration:
		return config.PromptCapabilityChapterGeneration, true
	case generationdomain.KindChapterContinuation:
		return config.PromptCapabilityChapterContinuation, true
	case generationdomain.KindChapterRewrite:
		return config.PromptCapabilityChapterRewrite, true
	default:
		return "", false
	}
}

// PromptCapabilityForConversationTarget 将对话目标类型映射为 Prompt 能力。
func PromptCapabilityForConversationTarget(targetType string) (config.PromptCapability, bool) {
	switch targetType {
	case conversationdomain.TargetTypeProject:
		return config.PromptCapabilityProjectRefinement, true
	case conversationdomain.TargetTypeAsset:
		return config.PromptCapabilityAssetRefinement, true
	default:
		return "", false
	}
}
