package conversation

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	assetdomain "novelforge/backend/internal/domain/asset"
	conversationdomain "novelforge/backend/internal/domain/conversation"
	metricdomain "novelforge/backend/internal/domain/metric"
	projectdomain "novelforge/backend/internal/domain/project"
	"novelforge/backend/internal/infra/llm"
	"novelforge/backend/internal/infra/llm/prompts"
	"novelforge/backend/internal/infra/storage/memory"
	appservice "novelforge/backend/internal/service"
	metricservice "novelforge/backend/internal/service/metric"
	"novelforge/backend/pkg/config"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
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

func newMetricUseCase(metricRepo metricdomain.MetricEventRepository) metricservice.UseCase {
	return metricservice.NewUseCase(metricservice.Dependencies{MetricEvents: metricRepo})
}

func TestUseCaseStartProjectConversationStoresPendingSuggestion(t *testing.T) {
	projectRepo := memory.NewProjectRepository()
	assetRepo := memory.NewAssetRepository()
	conversationRepo := memory.NewConversationRepository()
	metricRepo := memory.NewMetricEventRepository()
	project := createProjectEntity(t, projectRepo)
	promptStore := loadTestPromptStore(t)

	var gotMessages []*schema.Message
	useCase := NewUseCase(Dependencies{
		Conversations: conversationRepo,
		Projects:      projectRepo,
		Assets:        assetRepo,
		Metrics:       newMetricUseCase(metricRepo),
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
	events, err := metricRepo.ListByProject(context.Background(), metricdomain.ListByProjectParams{
		ProjectID: project.ID,
		EventName: metricdomain.EventOperationCompleted,
	})
	if err != nil {
		t.Fatalf("ListByProject(metric) error = %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("len(events) = %d, want 1", len(events))
	}
	event := events[0]
	if event.Labels["domain"] != "conversation" || event.Labels["action"] != "start" {
		t.Fatalf("event labels = %#v, want conversation/start", event.Labels)
	}
	if event.Labels["target_type"] != conversationdomain.TargetTypeProject {
		t.Fatalf("event labels[target_type] = %q, want %q", event.Labels["target_type"], conversationdomain.TargetTypeProject)
	}
}

func TestUseCaseReplyAssetConversationReplacesPendingSuggestion(t *testing.T) {
	projectRepo := memory.NewProjectRepository()
	assetRepo := memory.NewAssetRepository()
	conversationRepo := memory.NewConversationRepository()
	metricRepo := memory.NewMetricEventRepository()
	project := createProjectEntity(t, projectRepo)
	asset := createAssetEntity(t, assetRepo, project.ID)
	promptStore := loadTestPromptStore(t)

	startUseCase := NewUseCase(Dependencies{
		Conversations: conversationRepo,
		Projects:      projectRepo,
		Assets:        assetRepo,
		Metrics:       newMetricUseCase(metricRepo),
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
		Metrics:       newMetricUseCase(metricRepo),
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

	completedEvents, err := metricRepo.ListByProject(context.Background(), metricdomain.ListByProjectParams{
		ProjectID: project.ID,
		EventName: metricdomain.EventOperationCompleted,
	})
	if err != nil {
		t.Fatalf("ListByProject(metric) error = %v", err)
	}
	if len(completedEvents) != 2 {
		t.Fatalf("len(completedEvents) = %d, want 2", len(completedEvents))
	}
	replyEvent := completedEvents[1]
	if replyEvent.Labels["domain"] != "conversation" || replyEvent.Labels["action"] != "reply" {
		t.Fatalf("reply event labels = %#v, want conversation/reply", replyEvent.Labels)
	}
	if replyEvent.Labels["target_type"] != conversationdomain.TargetTypeAsset {
		t.Fatalf("reply event labels[target_type] = %q, want %q", replyEvent.Labels["target_type"], conversationdomain.TargetTypeAsset)
	}
}

func TestUseCaseConfirmProjectAppliesSuggestionAndClearsPending(t *testing.T) {
	projectRepo := memory.NewProjectRepository()
	assetRepo := memory.NewAssetRepository()
	conversationRepo := memory.NewConversationRepository()
	metricRepo := memory.NewMetricEventRepository()
	project := createProjectEntity(t, projectRepo)
	promptStore := loadTestPromptStore(t)

	startUseCase := NewUseCase(Dependencies{
		Conversations: conversationRepo,
		Projects:      projectRepo,
		Assets:        assetRepo,
		Metrics:       newMetricUseCase(metricRepo),
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

	completedEvents, err := metricRepo.ListByProject(context.Background(), metricdomain.ListByProjectParams{
		ProjectID: project.ID,
		EventName: metricdomain.EventOperationCompleted,
	})
	if err != nil {
		t.Fatalf("ListByProject(metric) error = %v", err)
	}
	if len(completedEvents) != 2 {
		t.Fatalf("len(completedEvents) = %d, want 2", len(completedEvents))
	}
	confirmEvent := completedEvents[1]
	if confirmEvent.Labels["domain"] != "conversation" || confirmEvent.Labels["action"] != "confirm" {
		t.Fatalf("confirm event labels = %#v, want conversation/confirm", confirmEvent.Labels)
	}
}

func TestUseCaseStartRejectsInvalidLLMJSON(t *testing.T) {
	projectRepo := memory.NewProjectRepository()
	assetRepo := memory.NewAssetRepository()
	conversationRepo := memory.NewConversationRepository()
	metricRepo := memory.NewMetricEventRepository()
	project := createProjectEntity(t, projectRepo)
	promptStore := loadTestPromptStore(t)

	useCase := NewUseCase(Dependencies{
		Conversations: conversationRepo,
		Projects:      projectRepo,
		Assets:        assetRepo,
		Metrics:       newMetricUseCase(metricRepo),
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

	failedEvents, listErr := metricRepo.ListByProject(context.Background(), metricdomain.ListByProjectParams{
		ProjectID: project.ID,
		EventName: metricdomain.EventOperationFailed,
	})
	if listErr != nil {
		t.Fatalf("ListByProject(metric) error = %v", listErr)
	}
	if len(failedEvents) != 1 {
		t.Fatalf("len(failedEvents) = %d, want 1", len(failedEvents))
	}
	if failedEvents[0].Labels["error_kind"] != "invalid_input" {
		t.Fatalf("failed event labels[error_kind] = %q, want %q", failedEvents[0].Labels["error_kind"], "invalid_input")
	}
}

func TestUseCaseReplySkipsMetricWhenProjectIDUnavailable(t *testing.T) {
	projectRepo := memory.NewProjectRepository()
	assetRepo := memory.NewAssetRepository()
	conversationRepo := memory.NewConversationRepository()
	metricRepo := memory.NewMetricEventRepository()
	useCase := NewUseCase(Dependencies{
		Conversations: conversationRepo,
		Projects:      projectRepo,
		Assets:        assetRepo,
		Metrics:       newMetricUseCase(metricRepo),
		PromptStore:   loadTestPromptStore(t),
		LLMClient:     &stubLLMClient{chatModel: &stubChatModel{}},
	})

	_, err := useCase.Reply(context.Background(), ReplyParams{
		ConversationID: "11111111-1111-1111-1111-111111111111",
		Message:        "继续优化。",
	})
	if err == nil {
		t.Fatal("Reply() error = nil, want not found")
	}
	if !errors.Is(err, appservice.ErrNotFound) {
		t.Fatalf("Reply() error = %v, want ErrNotFound", err)
	}
	failedEvents, listErr := metricRepo.ListByProject(context.Background(), metricdomain.ListByProjectParams{
		ProjectID: uuid.NewString(),
		EventName: metricdomain.EventOperationFailed,
	})
	if listErr != nil {
		t.Fatalf("ListByProject(metric) error = %v", listErr)
	}
	if len(failedEvents) != 0 {
		t.Fatalf("len(failedEvents) = %d, want 0 when project_id is unavailable", len(failedEvents))
	}
}

func TestUseCaseGetByIDAndListFlow(t *testing.T) {
	projectRepo := memory.NewProjectRepository()
	assetRepo := memory.NewAssetRepository()
	conversationRepo := memory.NewConversationRepository()
	project := createProjectEntity(t, projectRepo)
	asset := createAssetEntity(t, assetRepo, project.ID)
	baseTime := time.Date(2026, 3, 9, 12, 0, 0, 0, time.UTC)

	projectConversation := &conversationdomain.Conversation{
		ID:         "33333333-3333-3333-3333-333333333333",
		ProjectID:  project.ID,
		TargetType: conversationdomain.TargetTypeProject,
		TargetID:   project.ID,
		Messages: []conversationdomain.Message{
			{ID: "44444444-4444-4444-4444-444444444444", Role: conversationdomain.MessageRoleUser, Content: "优化项目标题。", CreatedAt: baseTime},
		},
		CreatedAt: baseTime,
		UpdatedAt: baseTime,
	}
	assetConversation := &conversationdomain.Conversation{
		ID:         "55555555-5555-5555-5555-555555555555",
		ProjectID:  project.ID,
		TargetType: conversationdomain.TargetTypeAsset,
		TargetID:   asset.ID,
		Messages: []conversationdomain.Message{
			{ID: "66666666-6666-6666-6666-666666666666", Role: conversationdomain.MessageRoleUser, Content: "优化资产。", CreatedAt: baseTime.Add(time.Minute)},
		},
		CreatedAt: baseTime.Add(time.Minute),
		UpdatedAt: baseTime.Add(time.Minute),
	}
	if err := conversationRepo.Create(context.Background(), projectConversation); err != nil {
		t.Fatalf("Create(project conversation) error = %v", err)
	}
	if err := conversationRepo.Create(context.Background(), assetConversation); err != nil {
		t.Fatalf("Create(asset conversation) error = %v", err)
	}

	useCase := NewUseCase(Dependencies{
		Conversations: conversationRepo,
		Projects:      projectRepo,
		Assets:        assetRepo,
	})

	got, err := useCase.GetByID(context.Background(), projectConversation.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if got.ID != projectConversation.ID || got.TargetType != conversationdomain.TargetTypeProject {
		t.Fatalf("GetByID() = %#v, want project conversation", got)
	}

	allItems, err := useCase.List(context.Background(), ListParams{
		ProjectID: project.ID,
		Limit:     10,
		Offset:    0,
	})
	if err != nil {
		t.Fatalf("List(project) error = %v", err)
	}
	if len(allItems) != 2 {
		t.Fatalf("len(List(project)) = %d, want 2", len(allItems))
	}

	assetItems, err := useCase.List(context.Background(), ListParams{
		ProjectID:  project.ID,
		TargetType: conversationdomain.TargetTypeAsset,
		TargetID:   asset.ID,
		Limit:      10,
		Offset:     0,
	})
	if err != nil {
		t.Fatalf("List(target) error = %v", err)
	}
	if len(assetItems) != 1 || assetItems[0].ID != assetConversation.ID {
		t.Fatalf("List(target) = %#v, want one asset conversation", assetItems)
	}
}

func TestUseCaseGetByIDAndListValidation(t *testing.T) {
	projectRepo := memory.NewProjectRepository()
	assetRepo := memory.NewAssetRepository()
	conversationRepo := memory.NewConversationRepository()
	useCase := NewUseCase(Dependencies{
		Conversations: conversationRepo,
		Projects:      projectRepo,
		Assets:        assetRepo,
	})

	if _, err := useCase.GetByID(context.Background(), "not-a-uuid"); !errors.Is(err, appservice.ErrInvalidInput) {
		t.Fatalf("GetByID(invalid id) error = %v, want invalid input", err)
	}
	if _, err := useCase.GetByID(context.Background(), "11111111-1111-1111-1111-111111111111"); !errors.Is(err, appservice.ErrNotFound) {
		t.Fatalf("GetByID(not found) error = %v, want not found", err)
	}

	if _, err := useCase.List(context.Background(), ListParams{ProjectID: "not-a-uuid"}); !errors.Is(err, appservice.ErrInvalidInput) {
		t.Fatalf("List(invalid project_id) error = %v, want invalid input", err)
	}
	if _, err := useCase.List(context.Background(), ListParams{ProjectID: "11111111-1111-1111-1111-111111111111"}); !errors.Is(err, appservice.ErrNotFound) {
		t.Fatalf("List(project not found) error = %v, want not found", err)
	}

	project := createProjectEntity(t, projectRepo)
	if _, err := useCase.List(context.Background(), ListParams{ProjectID: project.ID, TargetType: conversationdomain.TargetTypeProject}); !errors.Is(err, appservice.ErrInvalidInput) {
		t.Fatalf("List(target_type only) error = %v, want invalid input", err)
	}
}

func TestUseCaseConfirmAssetAppliesSuggestionAndClearsPending(t *testing.T) {
	projectRepo := memory.NewProjectRepository()
	assetRepo := memory.NewAssetRepository()
	conversationRepo := memory.NewConversationRepository()
	metricRepo := memory.NewMetricEventRepository()
	project := createProjectEntity(t, projectRepo)
	asset := createAssetEntity(t, assetRepo, project.ID)
	baseTime := time.Date(2026, 3, 9, 12, 0, 0, 0, time.UTC)

	conversationEntity := &conversationdomain.Conversation{
		ID:         "33333333-3333-3333-3333-333333333333",
		ProjectID:  project.ID,
		TargetType: conversationdomain.TargetTypeAsset,
		TargetID:   asset.ID,
		Messages: []conversationdomain.Message{
			{ID: "44444444-4444-4444-4444-444444444444", Role: conversationdomain.MessageRoleUser, Content: "优化资产。", CreatedAt: baseTime},
			{ID: "55555555-5555-5555-5555-555555555555", Role: conversationdomain.MessageRoleAssistant, Content: `{"title":"更新标题","content":"更新内容"}`, CreatedAt: baseTime.Add(time.Minute)},
		},
		PendingSuggestion: &conversationdomain.PendingSuggestion{Title: "更新标题", Content: "更新内容"},
		CreatedAt:         baseTime,
		UpdatedAt:         baseTime.Add(time.Minute),
	}
	if err := conversationRepo.Create(context.Background(), conversationEntity); err != nil {
		t.Fatalf("Create(conversation) error = %v", err)
	}

	useCase := NewUseCase(Dependencies{
		Conversations: conversationRepo,
		Projects:      projectRepo,
		Assets:        assetRepo,
		Metrics:       newMetricUseCase(metricRepo),
	})

	result, err := useCase.Confirm(context.Background(), conversationEntity.ID)
	if err != nil {
		t.Fatalf("Confirm() error = %v", err)
	}
	if result.Asset == nil {
		t.Fatal("Asset = nil, want updated asset")
	}
	if result.Asset.Title != "更新标题" || result.Asset.Content != "更新内容" {
		t.Fatalf("Asset = %#v, want confirmed values", result.Asset)
	}
	if result.Conversation.PendingSuggestion != nil {
		t.Fatalf("PendingSuggestion = %#v, want nil", result.Conversation.PendingSuggestion)
	}
	lastMessage := result.Conversation.Messages[len(result.Conversation.Messages)-1]
	if lastMessage.Role != conversationdomain.MessageRoleSystem || !strings.Contains(lastMessage.Content, "asset") {
		t.Fatalf("last message = %#v, want system confirm message for asset", lastMessage)
	}

	storedAsset, err := assetRepo.GetByID(context.Background(), asset.ID)
	if err != nil {
		t.Fatalf("GetByID(asset) error = %v", err)
	}
	if storedAsset.Title != "更新标题" || storedAsset.Content != "更新内容" {
		t.Fatalf("stored asset = %#v, want updated values", storedAsset)
	}

	completedEvents, err := metricRepo.ListByProject(context.Background(), metricdomain.ListByProjectParams{
		ProjectID: project.ID,
		EventName: metricdomain.EventOperationCompleted,
	})
	if err != nil {
		t.Fatalf("ListByProject(metric) error = %v", err)
	}
	if len(completedEvents) != 1 {
		t.Fatalf("len(completedEvents) = %d, want 1", len(completedEvents))
	}
	if completedEvents[0].Labels["action"] != "confirm" || completedEvents[0].Labels["target_type"] != conversationdomain.TargetTypeAsset {
		t.Fatalf("confirm event labels = %#v, want confirm/asset", completedEvents[0].Labels)
	}
}

func TestUseCaseConfirmRejectsInvalidAssetConversation(t *testing.T) {
	projectRepo := memory.NewProjectRepository()
	assetRepo := memory.NewAssetRepository()
	conversationRepo := memory.NewConversationRepository()
	project := createProjectEntity(t, projectRepo)
	asset := createAssetEntity(t, assetRepo, project.ID)
	baseTime := time.Date(2026, 3, 9, 12, 0, 0, 0, time.UTC)

	withoutPending := &conversationdomain.Conversation{
		ID:         "33333333-3333-3333-3333-333333333333",
		ProjectID:  project.ID,
		TargetType: conversationdomain.TargetTypeAsset,
		TargetID:   asset.ID,
		Messages: []conversationdomain.Message{
			{ID: "44444444-4444-4444-4444-444444444444", Role: conversationdomain.MessageRoleUser, Content: "无 pending。", CreatedAt: baseTime},
		},
		CreatedAt: baseTime,
		UpdatedAt: baseTime,
	}
	if err := conversationRepo.Create(context.Background(), withoutPending); err != nil {
		t.Fatalf("Create(without pending) error = %v", err)
	}

	useCase := NewUseCase(Dependencies{
		Conversations: conversationRepo,
		Projects:      projectRepo,
		Assets:        assetRepo,
	})

	if _, err := useCase.Confirm(context.Background(), "not-a-uuid"); !errors.Is(err, appservice.ErrInvalidInput) {
		t.Fatalf("Confirm(invalid id) error = %v, want invalid input", err)
	}
	if _, err := useCase.Confirm(context.Background(), withoutPending.ID); !errors.Is(err, appservice.ErrInvalidInput) {
		t.Fatalf("Confirm(without pending) error = %v, want invalid input", err)
	}

	otherProjectRepo := memory.NewProjectRepository()
	otherAssetRepo := memory.NewAssetRepository()
	otherConversationRepo := memory.NewConversationRepository()
	otherProject := createProjectEntity(t, otherProjectRepo)
	otherAsset := createAssetEntity(t, otherAssetRepo, otherProject.ID)
	otherConversation := &conversationdomain.Conversation{
		ID:                "55555555-5555-5555-5555-555555555555",
		ProjectID:         otherProject.ID,
		TargetType:        conversationdomain.TargetTypeAsset,
		TargetID:          otherAsset.ID,
		PendingSuggestion: &conversationdomain.PendingSuggestion{Title: "新标题", Content: "新内容"},
		CreatedAt:         baseTime,
		UpdatedAt:         baseTime,
	}
	otherAsset.ProjectID = "66666666-6666-6666-6666-666666666666"
	if err := otherAssetRepo.Update(context.Background(), otherAsset); err != nil {
		t.Fatalf("Update(other asset project_id) error = %v", err)
	}
	if err := otherConversationRepo.Create(context.Background(), otherConversation); err != nil {
		t.Fatalf("Create(other conversation) error = %v", err)
	}

	otherUseCase := NewUseCase(Dependencies{
		Conversations: otherConversationRepo,
		Projects:      otherProjectRepo,
		Assets:        otherAssetRepo,
	})
	if _, err := otherUseCase.Confirm(context.Background(), otherConversation.ID); !errors.Is(err, appservice.ErrInvalidInput) {
		t.Fatalf("Confirm(asset mismatch) error = %v, want invalid input", err)
	}
}

var _ llm.Client = (*stubLLMClient)(nil)
