package memory

import (
	"context"
	"testing"
	"time"

	assetdomain "novelforge/backend/internal/domain/asset"
	chapterdomain "novelforge/backend/internal/domain/chapter"
	conversationdomain "novelforge/backend/internal/domain/conversation"
	generationdomain "novelforge/backend/internal/domain/generation"
	metricdomain "novelforge/backend/internal/domain/metric"
	projectdomain "novelforge/backend/internal/domain/project"

	"github.com/google/uuid"
)

func testTime() time.Time {
	return time.Date(2026, 3, 6, 12, 0, 0, 0, time.UTC)
}

func testProject() *projectdomain.Project {
	now := testTime()
	return &projectdomain.Project{
		ID:        uuid.NewString(),
		Title:     "Test Project",
		Summary:   "Summary",
		Status:    projectdomain.StatusDraft,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func testAsset(projectID string) *assetdomain.Asset {
	now := testTime()
	return &assetdomain.Asset{
		ID:        uuid.NewString(),
		ProjectID: projectID,
		Type:      assetdomain.TypeOutline,
		Title:     "Outline",
		Content:   "Outline content",
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func testChapter(projectID string) *chapterdomain.Chapter {
	now := testTime()
	return &chapterdomain.Chapter{
		ID:        uuid.NewString(),
		ProjectID: projectID,
		Title:     "Chapter 1",
		Ordinal:   1,
		Status:    chapterdomain.StatusDraft,
		Content:   "Body",
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func testConversation(projectID, targetID string) *conversationdomain.Conversation {
	now := testTime()
	return &conversationdomain.Conversation{
		ID:         uuid.NewString(),
		ProjectID:  projectID,
		TargetType: conversationdomain.TargetTypeProject,
		TargetID:   targetID,
		Messages: []conversationdomain.Message{{
			ID:        uuid.NewString(),
			Role:      conversationdomain.MessageRoleUser,
			Content:   "Hello",
			CreatedAt: now,
		}},
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func testGeneration(projectID, chapterID, conversationID string) *generationdomain.GenerationRecord {
	now := testTime()
	return &generationdomain.GenerationRecord{
		ID:               uuid.NewString(),
		ProjectID:        projectID,
		ChapterID:        chapterID,
		ConversationID:   conversationID,
		Kind:             generationdomain.KindChapterGeneration,
		Status:           generationdomain.StatusPending,
		InputSnapshotRef: "snapshot-1",
		OutputRef:        "draft-1",
		TokenUsage:       100,
		DurationMillis:   500,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
}

func testMetric(projectID, chapterID string) *metricdomain.MetricEvent {
	return &metricdomain.MetricEvent{
		ID:         uuid.NewString(),
		EventName:  metricdomain.EventChapterGenerated,
		ProjectID:  projectID,
		ChapterID:  chapterID,
		Labels:     map[string]string{"source": "test"},
		Stats:      map[string]float64{"token_usage": 100},
		OccurredAt: testTime(),
	}
}

func TestProjectRepositoryCRUD(t *testing.T) {
	repo := NewProjectRepository()
	ctx := context.Background()
	project := testProject()

	if err := repo.Create(ctx, project); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	got, err := repo.GetByID(ctx, project.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if got.Title != project.Title {
		t.Fatalf("GetByID().Title = %q, want %q", got.Title, project.Title)
	}

	project.Title = "Updated Title"
	project.UpdatedAt = project.UpdatedAt.Add(time.Minute)
	if err := repo.Update(ctx, project); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	list, err := repo.List(ctx, projectdomain.ListParams{Status: projectdomain.StatusDraft})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(list) != 1 || list[0].Title != "Updated Title" {
		t.Fatalf("List() = %#v, want updated project", list)
	}
}

func TestAssetRepositoryFiltersAndDelete(t *testing.T) {
	repo := NewAssetRepository()
	ctx := context.Background()
	projectID := uuid.NewString()
	outline := testAsset(projectID)
	character := testAsset(projectID)
	character.ID = uuid.NewString()
	character.Type = assetdomain.TypeCharacter
	character.Title = "Hero"

	if err := repo.Create(ctx, outline); err != nil {
		t.Fatalf("Create() outline error = %v", err)
	}
	if err := repo.Create(ctx, character); err != nil {
		t.Fatalf("Create() character error = %v", err)
	}

	filtered, err := repo.ListByProjectAndType(ctx, assetdomain.ListByProjectAndTypeParams{ProjectID: projectID, Type: assetdomain.TypeCharacter})
	if err != nil {
		t.Fatalf("ListByProjectAndType() error = %v", err)
	}
	if len(filtered) != 1 || filtered[0].ID != character.ID {
		t.Fatalf("ListByProjectAndType() = %#v, want character asset", filtered)
	}

	if err := repo.Delete(ctx, outline.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	remaining, err := repo.ListByProject(ctx, assetdomain.ListByProjectParams{ProjectID: projectID})
	if err != nil {
		t.Fatalf("ListByProject() error = %v", err)
	}
	if len(remaining) != 1 || remaining[0].ID != character.ID {
		t.Fatalf("ListByProject() = %#v, want only remaining asset", remaining)
	}
}

func TestChapterRepositoryUpdate(t *testing.T) {
	repo := NewChapterRepository()
	ctx := context.Background()
	chapter := testChapter(uuid.NewString())

	if err := repo.Create(ctx, chapter); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	chapter.Title = "Chapter 1 Revised"
	chapter.UpdatedAt = chapter.UpdatedAt.Add(time.Minute)
	if err := repo.Update(ctx, chapter); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	got, err := repo.GetByID(ctx, chapter.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if got.Title != chapter.Title {
		t.Fatalf("GetByID().Title = %q, want %q", got.Title, chapter.Title)
	}
}

func TestConversationRepositoryAppendMessage(t *testing.T) {
	repo := NewConversationRepository()
	ctx := context.Background()
	projectID := uuid.NewString()
	conversation := testConversation(projectID, uuid.NewString())

	if err := repo.Create(ctx, conversation); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	message := conversationdomain.Message{
		ID:        uuid.NewString(),
		Role:      conversationdomain.MessageRoleAssistant,
		Content:   "Draft ready",
		CreatedAt: conversation.UpdatedAt.Add(time.Minute),
	}
	if err := repo.AppendMessage(ctx, conversationdomain.AppendMessageParams{ConversationID: conversation.ID, Message: message}); err != nil {
		t.Fatalf("AppendMessage() error = %v", err)
	}

	got, err := repo.GetByID(ctx, conversation.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if len(got.Messages) != 2 {
		t.Fatalf("len(Messages) = %d, want 2", len(got.Messages))
	}
	if got.Messages[1].Content != message.Content {
		t.Fatalf("Messages[1].Content = %q, want %q", got.Messages[1].Content, message.Content)
	}
}

func TestGenerationRecordRepositoryUpdateStatus(t *testing.T) {
	repo := NewGenerationRecordRepository()
	ctx := context.Background()
	record := testGeneration(uuid.NewString(), uuid.NewString(), uuid.NewString())

	if err := repo.Create(ctx, record); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	params := generationdomain.UpdateStatusParams{
		ID:             record.ID,
		Status:         generationdomain.StatusSucceeded,
		OutputRef:      "draft-2",
		TokenUsage:     180,
		DurationMillis: 900,
		UpdatedAt:      record.UpdatedAt.Add(time.Minute),
	}
	if err := repo.UpdateStatus(ctx, params); err != nil {
		t.Fatalf("UpdateStatus() error = %v", err)
	}

	got, err := repo.GetByID(ctx, record.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if got.Status != generationdomain.StatusSucceeded || got.OutputRef != "draft-2" {
		t.Fatalf("GetByID() = %#v, want updated status/output", got)
	}
}

func TestMetricEventRepositoryAppendOrder(t *testing.T) {
	repo := NewMetricEventRepository()
	ctx := context.Background()
	projectID := uuid.NewString()
	chapterID := uuid.NewString()
	first := testMetric(projectID, chapterID)
	second := testMetric(projectID, chapterID)
	second.ID = uuid.NewString()
	second.EventName = metricdomain.EventChapterConfirmed
	second.OccurredAt = second.OccurredAt.Add(time.Minute)

	if err := repo.Append(ctx, first); err != nil {
		t.Fatalf("Append() first error = %v", err)
	}
	if err := repo.Append(ctx, second); err != nil {
		t.Fatalf("Append() second error = %v", err)
	}

	list, err := repo.ListByProject(ctx, metricdomain.ListByProjectParams{ProjectID: projectID})
	if err != nil {
		t.Fatalf("ListByProject() error = %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("len(ListByProject()) = %d, want 2", len(list))
	}
	if list[0].ID != first.ID || list[1].ID != second.ID {
		t.Fatalf("ListByProject() order = %#v, want append order", list)
	}
}
