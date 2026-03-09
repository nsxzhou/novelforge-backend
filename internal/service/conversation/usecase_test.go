package conversation

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	assetdomain "novelforge/backend/internal/domain/asset"
	conversationdomain "novelforge/backend/internal/domain/conversation"
	projectdomain "novelforge/backend/internal/domain/project"
	"novelforge/backend/internal/infra/llm"
	"novelforge/backend/internal/infra/llm/prompts"
	"novelforge/backend/internal/infra/storage/memory"
	"novelforge/backend/pkg/config"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

type stubChatModel struct {
	generate func(context.Context, []*schema.Message, ...model.Option) (*schema.Message, error)
}

func (s *stubChatModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	if s.generate != nil {
		return s.generate(ctx, input, opts...)
	}
	return nil, errors.New("unexpected Generate call")
}

func (s *stubChatModel) Stream(context.Context, []*schema.Message, ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, errors.New("unexpected Stream call")
}

func (s *stubChatModel) WithTools(_ []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return s, nil
}

type stubLLMClient struct {
	chatModel model.ToolCallingChatModel
}

func (s *stubLLMClient) Provider() string { return "stub" }
func (s *stubLLMClient) Model() string    { return "stub-model" }
func (s *stubLLMClient) ChatModel() model.ToolCallingChatModel {
	return s.chatModel
}

func loadTestPromptStore(t *testing.T) *prompts.Store {
	t.Helper()
	store, err := prompts.LoadStore(config.PromptConfig{
		"asset_generation":     "asset_generation.yaml",
		"chapter_generation":   "chapter_generation.yaml",
		"chapter_continuation": "chapter_continuation.yaml",
		"chapter_rewrite":      "chapter_rewrite.yaml",
		"project_refinement":   "project_refinement.yaml",
		"asset_refinement":     "asset_refinement.yaml",
	})
	if err != nil {
		t.Fatalf("LoadStore() error = %v", err)
	}
	return store
}

func createProjectEntity(t *testing.T, repo projectdomain.ProjectRepository) *projectdomain.Project {
	t.Helper()
	now := time.Date(2026, 3, 9, 10, 0, 0, 0, time.UTC)
	entity := &projectdomain.Project{
		ID:        "11111111-1111-1111-1111-111111111111",
		Title:     "Old title",
		Summary:   "Old summary",
		Status:    projectdomain.StatusDraft,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := repo.Create(context.Background(), entity); err != nil {
		t.Fatalf("Create(project) error = %v", err)
	}
	return entity
}

func createAssetEntity(t *testing.T, repo assetdomain.AssetRepository, projectID string) *assetdomain.Asset {
	t.Helper()
	now := time.Date(2026, 3, 9, 11, 0, 0, 0, time.UTC)
	entity := &assetdomain.Asset{
		ID:        "22222222-2222-2222-2222-222222222222",
		ProjectID: projectID,
		Type:      assetdomain.TypeOutline,
		Title:     "Old outline",
		Content:   "Old content",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := repo.Create(context.Background(), entity); err != nil {
		t.Fatalf("Create(asset) error = %v", err)
	}
	return entity
}

func TestUseCaseStartProjectConversationStoresPendingSuggestion(t *testing.T) {
	projectRepo := memory.NewProjectRepository()
	assetRepo := memory.NewAssetRepository()
	conversationRepo := memory.NewConversationRepository()
	project := createProjectEntity(t, projectRepo)
	promptStore := loadTestPromptStore(t)

	var gotMessages []*schema.Message
	useCase := NewUseCase(Dependencies{
		Conversations: conversationRepo,
		Projects:      projectRepo,
		Assets:        assetRepo,
		PromptStore:   promptStore,
		LLMClient: &stubLLMClient{chatModel: &stubChatModel{generate: func(_ context.Context, input []*schema.Message, _ ...model.Option) (*schema.Message, error) {
			gotMessages = input
			return &schema.Message{Content: `{"title":"Refined title","summary":"Refined summary"}`}, nil
		}}},
	})

	conversation, err := useCase.Start(context.Background(), StartParams{
		ProjectID:  project.ID,
		TargetType: conversationdomain.TargetTypeProject,
		TargetID:   project.ID,
		Message:    "Please refine the title and summary.",
	})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if conversation.PendingSuggestion == nil {
		t.Fatal("PendingSuggestion = nil, want non-nil")
	}
	if conversation.PendingSuggestion.Title != "Refined title" || conversation.PendingSuggestion.Summary != "Refined summary" {
		t.Fatalf("PendingSuggestion = %#v, want refined project suggestion", conversation.PendingSuggestion)
	}
	if len(conversation.Messages) != 2 {
		t.Fatalf("len(Messages) = %d, want 2", len(conversation.Messages))
	}
	if conversation.Messages[0].Role != conversationdomain.MessageRoleUser || conversation.Messages[1].Role != conversationdomain.MessageRoleAssistant {
		t.Fatalf("message roles = %#v, want user then assistant", []string{conversation.Messages[0].Role, conversation.Messages[1].Role})
	}
	stored, err := conversationRepo.GetByID(context.Background(), conversation.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if stored.PendingSuggestion == nil || stored.PendingSuggestion.Title != "Refined title" {
		t.Fatalf("stored PendingSuggestion = %#v, want persisted suggestion", stored.PendingSuggestion)
	}
	if len(gotMessages) != 2 || gotMessages[0].Role != schema.System || gotMessages[1].Role != schema.User {
		t.Fatalf("Generate input = %#v, want system and user prompts", gotMessages)
	}
	if !strings.Contains(gotMessages[1].Content, "Please refine the title and summary.") {
		t.Fatalf("user prompt = %q, want latest user message", gotMessages[1].Content)
	}
}

func TestUseCaseReplyAssetConversationReplacesPendingSuggestion(t *testing.T) {
	projectRepo := memory.NewProjectRepository()
	assetRepo := memory.NewAssetRepository()
	conversationRepo := memory.NewConversationRepository()
	project := createProjectEntity(t, projectRepo)
	asset := createAssetEntity(t, assetRepo, project.ID)
	promptStore := loadTestPromptStore(t)

	startUseCase := NewUseCase(Dependencies{
		Conversations: conversationRepo,
		Projects:      projectRepo,
		Assets:        assetRepo,
		PromptStore:   promptStore,
		LLMClient: &stubLLMClient{chatModel: &stubChatModel{generate: func(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.Message, error) {
			return &schema.Message{Content: `{"title":"Initial outline","content":"Initial outline content"}`}, nil
		}}},
	})
	conversation, err := startUseCase.Start(context.Background(), StartParams{
		ProjectID:  project.ID,
		TargetType: conversationdomain.TargetTypeAsset,
		TargetID:   asset.ID,
		Message:    "Draft the outline.",
	})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	replyUseCase := NewUseCase(Dependencies{
		Conversations: conversationRepo,
		Projects:      projectRepo,
		Assets:        assetRepo,
		PromptStore:   promptStore,
		LLMClient: &stubLLMClient{chatModel: &stubChatModel{generate: func(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.Message, error) {
			return &schema.Message{Content: `{"title":"Updated outline","content":"Updated outline content"}`}, nil
		}}},
	})
	updated, err := replyUseCase.Reply(context.Background(), ReplyParams{
		ConversationID: conversation.ID,
		Message:        "Make it darker.",
	})
	if err != nil {
		t.Fatalf("Reply() error = %v", err)
	}
	if updated.PendingSuggestion == nil {
		t.Fatal("PendingSuggestion = nil, want non-nil")
	}
	if updated.PendingSuggestion.Title != "Updated outline" || updated.PendingSuggestion.Content != "Updated outline content" {
		t.Fatalf("PendingSuggestion = %#v, want replaced asset suggestion", updated.PendingSuggestion)
	}
	if len(updated.Messages) != 4 {
		t.Fatalf("len(Messages) = %d, want 4", len(updated.Messages))
	}
	if updated.Messages[2].Role != conversationdomain.MessageRoleUser || updated.Messages[3].Role != conversationdomain.MessageRoleAssistant {
		t.Fatalf("reply message roles = %#v, want user then assistant", []string{updated.Messages[2].Role, updated.Messages[3].Role})
	}
}

func TestUseCaseConfirmProjectAppliesSuggestionAndClearsPending(t *testing.T) {
	projectRepo := memory.NewProjectRepository()
	assetRepo := memory.NewAssetRepository()
	conversationRepo := memory.NewConversationRepository()
	project := createProjectEntity(t, projectRepo)
	promptStore := loadTestPromptStore(t)

	startUseCase := NewUseCase(Dependencies{
		Conversations: conversationRepo,
		Projects:      projectRepo,
		Assets:        assetRepo,
		PromptStore:   promptStore,
		LLMClient: &stubLLMClient{chatModel: &stubChatModel{generate: func(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.Message, error) {
			return &schema.Message{Content: `{"title":"Confirmed title","summary":"Confirmed summary"}`}, nil
		}}},
	})
	conversation, err := startUseCase.Start(context.Background(), StartParams{
		ProjectID:  project.ID,
		TargetType: conversationdomain.TargetTypeProject,
		TargetID:   project.ID,
		Message:    "Polish the project.",
	})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	result, err := startUseCase.Confirm(context.Background(), conversation.ID)
	if err != nil {
		t.Fatalf("Confirm() error = %v", err)
	}
	if result.Project == nil {
		t.Fatal("Project = nil, want updated project")
	}
	if result.Project.Title != "Confirmed title" || result.Project.Summary != "Confirmed summary" {
		t.Fatalf("Project = %#v, want confirmed values", result.Project)
	}
	if result.Conversation.PendingSuggestion != nil {
		t.Fatalf("PendingSuggestion = %#v, want nil", result.Conversation.PendingSuggestion)
	}
	if got := result.Conversation.Messages[len(result.Conversation.Messages)-1]; got.Role != conversationdomain.MessageRoleSystem {
		t.Fatalf("last message role = %q, want system", got.Role)
	}
	storedProject, err := projectRepo.GetByID(context.Background(), project.ID)
	if err != nil {
		t.Fatalf("GetByID(project) error = %v", err)
	}
	if storedProject.Title != "Confirmed title" || storedProject.Summary != "Confirmed summary" {
		t.Fatalf("stored project = %#v, want updated values", storedProject)
	}
}

func TestUseCaseStartRejectsInvalidLLMJSON(t *testing.T) {
	projectRepo := memory.NewProjectRepository()
	assetRepo := memory.NewAssetRepository()
	conversationRepo := memory.NewConversationRepository()
	project := createProjectEntity(t, projectRepo)
	promptStore := loadTestPromptStore(t)

	useCase := NewUseCase(Dependencies{
		Conversations: conversationRepo,
		Projects:      projectRepo,
		Assets:        assetRepo,
		PromptStore:   promptStore,
		LLMClient: &stubLLMClient{chatModel: &stubChatModel{generate: func(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.Message, error) {
			return &schema.Message{Content: `{"title":"Broken","summary":"Still broken"} trailing`}, nil
		}}},
	})

	_, err := useCase.Start(context.Background(), StartParams{
		ProjectID:  project.ID,
		TargetType: conversationdomain.TargetTypeProject,
		TargetID:   project.ID,
		Message:    "Try it.",
	})
	if err == nil {
		t.Fatal("Start() error = nil, want invalid JSON error")
	}
	if !strings.Contains(err.Error(), "invalid llm json") {
		t.Fatalf("Start() error = %v, want invalid llm json", err)
	}
}

var _ llm.Client = (*stubLLMClient)(nil)
