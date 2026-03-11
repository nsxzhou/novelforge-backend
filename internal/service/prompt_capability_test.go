package service

import (
	"testing"

	conversationdomain "novelforge/backend/internal/domain/conversation"
	generationdomain "novelforge/backend/internal/domain/generation"
	"novelforge/backend/pkg/config"
)

func TestPromptCapabilityForGenerationKind(t *testing.T) {
	tests := []struct {
		name string
		kind string
		want config.PromptCapability
		ok   bool
	}{
		{name: "asset generation", kind: generationdomain.KindAssetGeneration, want: config.PromptCapabilityAssetGeneration, ok: true},
		{name: "chapter generation", kind: generationdomain.KindChapterGeneration, want: config.PromptCapabilityChapterGeneration, ok: true},
		{name: "chapter continuation", kind: generationdomain.KindChapterContinuation, want: config.PromptCapabilityChapterContinuation, ok: true},
		{name: "chapter rewrite", kind: generationdomain.KindChapterRewrite, want: config.PromptCapabilityChapterRewrite, ok: true},
		{name: "unsupported", kind: "unknown_kind", want: "", ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := PromptCapabilityForGenerationKind(tt.kind)
			if ok != tt.ok || got != tt.want {
				t.Fatalf("PromptCapabilityForGenerationKind(%q) = (%q, %v), want (%q, %v)", tt.kind, got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestPromptCapabilityForConversationTarget(t *testing.T) {
	tests := []struct {
		name   string
		target string
		want   config.PromptCapability
		ok     bool
	}{
		{name: "project", target: conversationdomain.TargetTypeProject, want: config.PromptCapabilityProjectRefinement, ok: true},
		{name: "asset", target: conversationdomain.TargetTypeAsset, want: config.PromptCapabilityAssetRefinement, ok: true},
		{name: "unsupported", target: "chapter", want: "", ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := PromptCapabilityForConversationTarget(tt.target)
			if ok != tt.ok || got != tt.want {
				t.Fatalf("PromptCapabilityForConversationTarget(%q) = (%q, %v), want (%q, %v)", tt.target, got, ok, tt.want, tt.ok)
			}
		})
	}
}
