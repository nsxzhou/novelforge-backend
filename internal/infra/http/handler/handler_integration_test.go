package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	apiroutes "novelforge/backend/api/http"
	assetdomain "novelforge/backend/internal/domain/asset"
	chapterdomain "novelforge/backend/internal/domain/chapter"
	conversationdomain "novelforge/backend/internal/domain/conversation"
	generationdomain "novelforge/backend/internal/domain/generation"
	projectdomain "novelforge/backend/internal/domain/project"
	"novelforge/backend/internal/infra/http/middleware"
	"novelforge/backend/internal/infra/llm/prompts"
	"novelforge/backend/internal/infra/storage/memory"
	appservice "novelforge/backend/internal/service"
	assetservice "novelforge/backend/internal/service/asset"
	chapterservice "novelforge/backend/internal/service/chapter"
	conversationservice "novelforge/backend/internal/service/conversation"
	metricservice "novelforge/backend/internal/service/metric"
	projectservice "novelforge/backend/internal/service/project"
	"novelforge/backend/pkg/config"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

type projectResponse struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Summary   string `json:"summary"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type projectListResponse struct {
	Projects []projectResponse `json:"projects"`
}

type assetResponse struct {
	ID        string `json:"id"`
	ProjectID string `json:"project_id"`
	Type      string `json:"type"`
	Title     string `json:"title"`
	Content   string `json:"content"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type assetListResponse struct {
	Assets []assetResponse `json:"assets"`
}

type assetGenerationResponse struct {
	Asset            assetResponse            `json:"asset"`
	GenerationRecord generationRecordResponse `json:"generation_record"`
}

type chapterResponse struct {
	ID                      string  `json:"id"`
	ProjectID               string  `json:"project_id"`
	Title                   string  `json:"title"`
	Ordinal                 int     `json:"ordinal"`
	Status                  string  `json:"status"`
	Content                 string  `json:"content"`
	CurrentDraftID          string  `json:"current_draft_id,omitempty"`
	CurrentDraftConfirmedAt *string `json:"current_draft_confirmed_at,omitempty"`
	CurrentDraftConfirmedBy string  `json:"current_draft_confirmed_by,omitempty"`
	CreatedAt               string  `json:"created_at"`
	UpdatedAt               string  `json:"updated_at"`
}

type chapterListResponse struct {
	Chapters []chapterResponse `json:"chapters"`
}

type generationRecordResponse struct {
	ID               string `json:"id"`
	ProjectID        string `json:"project_id"`
	ChapterID        string `json:"chapter_id,omitempty"`
	ConversationID   string `json:"conversation_id,omitempty"`
	Kind             string `json:"kind"`
	Status           string `json:"status"`
	InputSnapshotRef string `json:"input_snapshot_ref"`
	OutputRef        string `json:"output_ref"`
	TokenUsage       int    `json:"token_usage"`
	DurationMillis   int64  `json:"duration_millis"`
	ErrorMessage     string `json:"error_message,omitempty"`
	CreatedAt        string `json:"created_at"`
	UpdatedAt        string `json:"updated_at"`
}

type chapterGenerationResponse struct {
	Chapter          chapterResponse          `json:"chapter"`
	GenerationRecord generationRecordResponse `json:"generation_record"`
}

type conversationMessageResponse struct {
	ID        string `json:"id"`
	Role      string `json:"role"`
	Content   string `json:"content"`
	CreatedAt string `json:"created_at"`
}

type pendingSuggestionResponse struct {
	Title   string `json:"title,omitempty"`
	Summary string `json:"summary,omitempty"`
	Content string `json:"content,omitempty"`
}

type conversationResponse struct {
	ID                string                        `json:"id"`
	ProjectID         string                        `json:"project_id"`
	TargetType        string                        `json:"target_type"`
	TargetID          string                        `json:"target_id"`
	Messages          []conversationMessageResponse `json:"messages"`
	PendingSuggestion *pendingSuggestionResponse    `json:"pending_suggestion"`
	CreatedAt         string                        `json:"created_at"`
	UpdatedAt         string                        `json:"updated_at"`
}

type conversationListResponse struct {
	Conversations []conversationResponse `json:"conversations"`
}

type confirmConversationResponse struct {
	Conversation conversationResponse `json:"conversation"`
	Project      *projectResponse     `json:"project,omitempty"`
	Asset        *assetResponse       `json:"asset,omitempty"`
}

const testUserIDHeader = middleware.UserIDHeader

func TestProjectAndAssetRoutesIntegration(t *testing.T) {
	h := newTestServer()

	createProject := performJSONRequest(h, consts.MethodPost, "/api/v1/projects", `{"title":"Novel","summary":"Story summary","status":"draft"}`)
	assertStatusCode(t, createProject.Code, consts.StatusCreated)

	var createdProject projectResponse
	decodeResponseBody(t, createProject, &createdProject)
	if createdProject.ID == "" {
		t.Fatal("expected created project id")
	}
	if createdProject.Status != "draft" {
		t.Fatalf("created project status = %q, want draft", createdProject.Status)
	}

	listProjects := performRequest(h, consts.MethodGet, "/api/v1/projects?status=draft&limit=10&offset=0", "")
	assertStatusCode(t, listProjects.Code, consts.StatusOK)

	var projectList projectListResponse
	decodeResponseBody(t, listProjects, &projectList)
	if len(projectList.Projects) != 1 {
		t.Fatalf("len(projects) = %d, want 1", len(projectList.Projects))
	}

	getProject := performRequest(h, consts.MethodGet, "/api/v1/projects/"+createdProject.ID, "")
	assertStatusCode(t, getProject.Code, consts.StatusOK)

	updateProject := performJSONRequest(h, consts.MethodPut, "/api/v1/projects/"+createdProject.ID, `{"title":"Novel Revised","summary":"Updated summary","status":"active"}`)
	assertStatusCode(t, updateProject.Code, consts.StatusOK)

	var updatedProject projectResponse
	decodeResponseBody(t, updateProject, &updatedProject)
	if updatedProject.Title != "Novel Revised" || updatedProject.Status != "active" {
		t.Fatalf("updated project = %#v, want revised active project", updatedProject)
	}

	createAsset := performJSONRequest(h, consts.MethodPost, "/api/v1/projects/"+createdProject.ID+"/assets", `{"type":"outline","title":"Outline","content":"Outline body"}`)
	assertStatusCode(t, createAsset.Code, consts.StatusCreated)

	var createdAsset assetResponse
	decodeResponseBody(t, createAsset, &createdAsset)
	if createdAsset.ID == "" {
		t.Fatal("expected created asset id")
	}
	if createdAsset.ProjectID != createdProject.ID {
		t.Fatalf("created asset project_id = %q, want %q", createdAsset.ProjectID, createdProject.ID)
	}

	generateAsset := performJSONRequest(h, consts.MethodPost, "/api/v1/projects/"+createdProject.ID+"/assets/generate", `{"type":"character","instruction":"生成一个主角人设，冷静且有野心。"}`)
	assertStatusCode(t, generateAsset.Code, consts.StatusCreated)
	var generated assetGenerationResponse
	decodeResponseBody(t, generateAsset, &generated)
	if generated.Asset.ID == "" || generated.Asset.ProjectID != createdProject.ID || generated.Asset.Type != "character" {
		t.Fatalf("generated asset = %#v, want generated character asset", generated.Asset)
	}
	if generated.GenerationRecord.ID == "" || generated.GenerationRecord.Kind != generationdomain.KindAssetGeneration || generated.GenerationRecord.Status != generationdomain.StatusSucceeded {
		t.Fatalf("generated record = %#v, want succeeded asset_generation record", generated.GenerationRecord)
	}
	if generated.GenerationRecord.OutputRef != generated.Asset.ID {
		t.Fatalf("generated output_ref = %q, want asset id %q", generated.GenerationRecord.OutputRef, generated.Asset.ID)
	}

	listAssets := performRequest(h, consts.MethodGet, "/api/v1/projects/"+createdProject.ID+"/assets?limit=10&offset=0", "")
	assertStatusCode(t, listAssets.Code, consts.StatusOK)

	var assetList assetListResponse
	decodeResponseBody(t, listAssets, &assetList)
	if len(assetList.Assets) != 2 {
		t.Fatalf("len(assets) = %d, want 2", len(assetList.Assets))
	}

	listAssetsByType := performRequest(h, consts.MethodGet, "/api/v1/projects/"+createdProject.ID+"/assets?type=outline", "")
	assertStatusCode(t, listAssetsByType.Code, consts.StatusOK)
	decodeResponseBody(t, listAssetsByType, &assetList)
	if len(assetList.Assets) != 1 || assetList.Assets[0].ID != createdAsset.ID {
		t.Fatalf("typed asset list = %#v, want created asset", assetList.Assets)
	}

	getAsset := performRequest(h, consts.MethodGet, "/api/v1/assets/"+createdAsset.ID, "")
	assertStatusCode(t, getAsset.Code, consts.StatusOK)

	updateAsset := performJSONRequest(h, consts.MethodPut, "/api/v1/assets/"+createdAsset.ID, `{"type":"character","title":"Hero","content":"Character sheet"}`)
	assertStatusCode(t, updateAsset.Code, consts.StatusOK)

	var updatedAsset assetResponse
	decodeResponseBody(t, updateAsset, &updatedAsset)
	if updatedAsset.Type != "character" || updatedAsset.Title != "Hero" {
		t.Fatalf("updated asset = %#v, want updated character asset", updatedAsset)
	}

	deleteAsset := performRequest(h, consts.MethodDelete, "/api/v1/assets/"+createdAsset.ID, "")
	assertStatusCode(t, deleteAsset.Code, consts.StatusNoContent)
	if deleteAsset.Body.String() != "" {
		t.Fatalf("delete response body = %q, want empty", deleteAsset.Body.String())
	}

	missingAsset := performRequest(h, consts.MethodGet, "/api/v1/assets/"+createdAsset.ID, "")
	assertStatusCode(t, missingAsset.Code, consts.StatusNotFound)
	assertErrorResponse(t, missingAsset)
}

func TestRouteValidationAndHealthIntegration(t *testing.T) {
	h := newTestServer()

	healthz := performRequest(h, consts.MethodGet, "/healthz", "")
	assertStatusCode(t, healthz.Code, consts.StatusOK)
	if string(healthz.Header().Peek("X-Request-ID")) == "" {
		t.Fatal("expected X-Request-ID header on healthz response")
	}

	readyz := performRequest(h, consts.MethodGet, "/readyz", "")
	assertStatusCode(t, readyz.Code, consts.StatusOK)

	invalidProjectBody := performJSONRequest(h, consts.MethodPost, "/api/v1/projects", `{"title":"","summary":"","status":""}`)
	assertStatusCode(t, invalidProjectBody.Code, consts.StatusBadRequest)
	assertErrorResponse(t, invalidProjectBody)

	invalidProjectID := performRequest(h, consts.MethodGet, "/api/v1/projects/not-a-uuid", "")
	assertStatusCode(t, invalidProjectID.Code, consts.StatusBadRequest)
	assertErrorResponse(t, invalidProjectID)

	invalidProjectQuery := performRequest(h, consts.MethodGet, "/api/v1/projects?limit=-1", "")
	assertStatusCode(t, invalidProjectQuery.Code, consts.StatusBadRequest)
	assertErrorResponse(t, invalidProjectQuery)

	missingProject := performRequest(h, consts.MethodGet, "/api/v1/projects/11111111-1111-1111-1111-111111111111", "")
	assertStatusCode(t, missingProject.Code, consts.StatusNotFound)
	assertErrorResponse(t, missingProject)

	invalidAssetProjectID := performJSONRequest(h, consts.MethodPost, "/api/v1/projects/not-a-uuid/assets", `{"type":"outline","title":"Outline","content":"Body"}`)
	assertStatusCode(t, invalidAssetProjectID.Code, consts.StatusBadRequest)
	assertErrorResponse(t, invalidAssetProjectID)
}

func TestAssetIDValidationIntegration(t *testing.T) {
	h := newTestServer()

	testCases := []struct {
		name   string
		method string
		url    string
		body   string
	}{
		{name: "get asset", method: consts.MethodGet, url: "/api/v1/assets/not-a-uuid"},
		{name: "update asset", method: consts.MethodPut, url: "/api/v1/assets/not-a-uuid", body: `{"type":"outline","title":"Outline","content":"Body"}`},
		{name: "delete asset", method: consts.MethodDelete, url: "/api/v1/assets/not-a-uuid"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			recorder := performRequest(h, tc.method, tc.url, tc.body)
			assertStatusCode(t, recorder.Code, consts.StatusBadRequest)
			assertErrorResponse(t, recorder)
		})
	}
}

func TestRouteQueryAndJSONValidationIntegration(t *testing.T) {
	h := newTestServer()
	createdProject := createTestProject(t, h)
	createdAsset := createTestAsset(t, h, createdProject.ID)

	queryTests := []struct {
		name string
		url  string
	}{
		{name: "project limit must be numeric", url: "/api/v1/projects?limit=abc"},
		{name: "project offset must be non-negative", url: "/api/v1/projects?offset=-1"},
		{name: "asset type must not be empty", url: "/api/v1/projects/" + createdProject.ID + "/assets?type="},
		{name: "asset type must be valid", url: "/api/v1/projects/" + createdProject.ID + "/assets?type=invalid"},
		{name: "asset limit must be numeric", url: "/api/v1/projects/" + createdProject.ID + "/assets?limit=abc"},
		{name: "asset offset must be non-negative", url: "/api/v1/projects/" + createdProject.ID + "/assets?offset=-1"},
	}

	for _, tc := range queryTests {
		t.Run(tc.name, func(t *testing.T) {
			recorder := performRequest(h, consts.MethodGet, tc.url, "")
			assertStatusCode(t, recorder.Code, consts.StatusBadRequest)
			assertErrorResponse(t, recorder)
		})
	}

	projectMalformedJSON := performJSONRequest(h, consts.MethodPut, "/api/v1/projects/"+createdProject.ID, `{"title":`)
	assertStatusCode(t, projectMalformedJSON.Code, consts.StatusBadRequest)
	assertErrorResponse(t, projectMalformedJSON)

	assetMalformedJSON := performJSONRequest(h, consts.MethodPut, "/api/v1/assets/"+createdAsset.ID, `{"type":`)
	assertStatusCode(t, assetMalformedJSON.Code, consts.StatusBadRequest)
	assertErrorResponse(t, assetMalformedJSON)

	assetGenerateMalformedJSON := performJSONRequest(h, consts.MethodPost, "/api/v1/projects/"+createdProject.ID+"/assets/generate", `{"type":`)
	assertStatusCode(t, assetGenerateMalformedJSON.Code, consts.StatusBadRequest)
	assertErrorResponse(t, assetGenerateMalformedJSON)
}

func TestServiceErrorMappingIntegration(t *testing.T) {
	t.Run("conflict maps to 409", func(t *testing.T) {
		h := newTestServerWithUseCases(
			stubProjectUseCase{
				create: func(context.Context, *projectdomain.Project) error {
					return appservice.WrapConflict(errors.New("duplicate project"))
				},
			},
			stubAssetUseCase{},
			stubConversationUseCase{},
		)

		recorder := performJSONRequest(h, consts.MethodPost, "/api/v1/projects", `{"title":"Novel","summary":"Story summary","status":"draft"}`)
		assertStatusCode(t, recorder.Code, consts.StatusConflict)
		assertErrorContains(t, recorder, "duplicate project")
	})

	t.Run("unexpected error maps to 500", func(t *testing.T) {
		h := newTestServerWithUseCases(
			stubProjectUseCase{},
			stubAssetUseCase{
				getByID: func(context.Context, string) (*assetdomain.Asset, error) {
					return nil, errors.New("database offline")
				},
			},
			stubConversationUseCase{},
		)

		recorder := performRequest(h, consts.MethodGet, "/api/v1/assets/11111111-1111-1111-1111-111111111111", "")
		assertStatusCode(t, recorder.Code, consts.StatusInternalServerError)
		assertErrorMessage(t, recorder, "internal server error")
	})
}

func TestConversationRoutesIntegration(t *testing.T) {
	const (
		projectID      = "11111111-1111-1111-1111-111111111111"
		conversationID = "33333333-3333-3333-3333-333333333333"
	)

	baseTime := time.Date(2026, 3, 9, 12, 0, 0, 0, time.UTC)
	startedConversation := &conversationdomain.Conversation{
		ID:         conversationID,
		ProjectID:  projectID,
		TargetType: conversationdomain.TargetTypeProject,
		TargetID:   projectID,
		Messages: []conversationdomain.Message{
			{ID: "44444444-4444-4444-4444-444444444444", Role: conversationdomain.MessageRoleUser, Content: "Polish the project title.", CreatedAt: baseTime},
			{ID: "55555555-5555-5555-5555-555555555555", Role: conversationdomain.MessageRoleAssistant, Content: `{"title":"Refined title","summary":"Refined summary"}`, CreatedAt: baseTime.Add(time.Minute)},
		},
		PendingSuggestion: &conversationdomain.PendingSuggestion{Title: "Refined title", Summary: "Refined summary"},
		CreatedAt:         baseTime,
		UpdatedAt:         baseTime.Add(time.Minute),
	}
	updatedConversation := &conversationdomain.Conversation{
		ID:         conversationID,
		ProjectID:  projectID,
		TargetType: conversationdomain.TargetTypeProject,
		TargetID:   projectID,
		Messages: []conversationdomain.Message{
			{ID: "44444444-4444-4444-4444-444444444444", Role: conversationdomain.MessageRoleUser, Content: "Polish the project title.", CreatedAt: baseTime},
			{ID: "55555555-5555-5555-5555-555555555555", Role: conversationdomain.MessageRoleAssistant, Content: `{"title":"Refined title","summary":"Refined summary"}`, CreatedAt: baseTime.Add(time.Minute)},
			{ID: "66666666-6666-6666-6666-666666666666", Role: conversationdomain.MessageRoleUser, Content: "Make it darker.", CreatedAt: baseTime.Add(2 * time.Minute)},
			{ID: "77777777-7777-7777-7777-777777777777", Role: conversationdomain.MessageRoleAssistant, Content: `{"title":"Updated title","summary":"Updated summary"}`, CreatedAt: baseTime.Add(3 * time.Minute)},
		},
		PendingSuggestion: &conversationdomain.PendingSuggestion{Title: "Updated title", Summary: "Updated summary"},
		CreatedAt:         baseTime,
		UpdatedAt:         baseTime.Add(3 * time.Minute),
	}
	confirmedConversation := &conversationdomain.Conversation{
		ID:         conversationID,
		ProjectID:  projectID,
		TargetType: conversationdomain.TargetTypeProject,
		TargetID:   projectID,
		Messages: []conversationdomain.Message{
			{ID: "44444444-4444-4444-4444-444444444444", Role: conversationdomain.MessageRoleUser, Content: "Polish the project title.", CreatedAt: baseTime},
			{ID: "55555555-5555-5555-5555-555555555555", Role: conversationdomain.MessageRoleAssistant, Content: `{"title":"Refined title","summary":"Refined summary"}`, CreatedAt: baseTime.Add(time.Minute)},
			{ID: "66666666-6666-6666-6666-666666666666", Role: conversationdomain.MessageRoleUser, Content: "Make it darker.", CreatedAt: baseTime.Add(2 * time.Minute)},
			{ID: "77777777-7777-7777-7777-777777777777", Role: conversationdomain.MessageRoleAssistant, Content: `{"title":"Updated title","summary":"Updated summary"}`, CreatedAt: baseTime.Add(3 * time.Minute)},
			{ID: "88888888-8888-8888-8888-888888888888", Role: conversationdomain.MessageRoleSystem, Content: "已确认最新项目建议并写回项目。", CreatedAt: baseTime.Add(4 * time.Minute)},
		},
		CreatedAt: baseTime,
		UpdatedAt: baseTime.Add(4 * time.Minute),
	}
	confirmedProject := &projectdomain.Project{
		ID:        projectID,
		Title:     "Final title",
		Summary:   "Final summary",
		Status:    projectdomain.StatusDraft,
		CreatedAt: baseTime.Add(-time.Hour),
		UpdatedAt: baseTime.Add(4 * time.Minute),
	}

	h := newTestServerWithUseCases(
		stubProjectUseCase{},
		stubAssetUseCase{},
		stubConversationUseCase{
			start: func(_ context.Context, params conversationservice.StartParams) (*conversationdomain.Conversation, error) {
				if params.ProjectID != projectID || params.TargetType != conversationdomain.TargetTypeProject || params.TargetID != projectID || params.Message != "Polish the project title." {
					t.Fatalf("Start params = %#v, want expected project conversation payload", params)
				}
				return startedConversation, nil
			},
			reply: func(_ context.Context, params conversationservice.ReplyParams) (*conversationdomain.Conversation, error) {
				if params.ConversationID != conversationID || params.Message != "Make it darker." {
					t.Fatalf("Reply params = %#v, want expected reply payload", params)
				}
				return updatedConversation, nil
			},
			getByID: func(_ context.Context, id string) (*conversationdomain.Conversation, error) {
				if id != conversationID {
					t.Fatalf("GetByID id = %q, want %q", id, conversationID)
				}
				return updatedConversation, nil
			},
			list: func(_ context.Context, params conversationservice.ListParams) ([]*conversationdomain.Conversation, error) {
				if params.ProjectID != projectID || params.TargetType != conversationdomain.TargetTypeProject || params.TargetID != projectID || params.Limit != 10 || params.Offset != 0 {
					t.Fatalf("List params = %#v, want expected list payload", params)
				}
				return []*conversationdomain.Conversation{updatedConversation}, nil
			},
			confirm: func(_ context.Context, id string) (*conversationservice.ConfirmResult, error) {
				if id != conversationID {
					t.Fatalf("Confirm id = %q, want %q", id, conversationID)
				}
				return &conversationservice.ConfirmResult{Conversation: confirmedConversation, Project: confirmedProject}, nil
			},
		},
	)

	startRecorder := performJSONRequest(h, consts.MethodPost, "/api/v1/projects/"+projectID+"/conversations", `{"target_type":"project","target_id":"11111111-1111-1111-1111-111111111111","message":"Polish the project title."}`)
	assertStatusCode(t, startRecorder.Code, consts.StatusCreated)
	var started conversationResponse
	decodeResponseBody(t, startRecorder, &started)
	if started.ID != conversationID || started.PendingSuggestion == nil || started.PendingSuggestion.Title != "Refined title" {
		t.Fatalf("started conversation = %#v, want pending project suggestion", started)
	}
	if len(started.Messages) != 2 {
		t.Fatalf("len(started messages) = %d, want 2", len(started.Messages))
	}

	replyRecorder := performJSONRequest(h, consts.MethodPost, "/api/v1/conversations/"+conversationID+"/messages", `{"message":"Make it darker."}`)
	assertStatusCode(t, replyRecorder.Code, consts.StatusOK)
	var replied conversationResponse
	decodeResponseBody(t, replyRecorder, &replied)
	if replied.PendingSuggestion == nil || replied.PendingSuggestion.Title != "Updated title" || replied.PendingSuggestion.Summary != "Updated summary" {
		t.Fatalf("replied conversation = %#v, want updated pending suggestion", replied)
	}
	if len(replied.Messages) != 4 {
		t.Fatalf("len(replied messages) = %d, want 4", len(replied.Messages))
	}

	getRecorder := performRequest(h, consts.MethodGet, "/api/v1/conversations/"+conversationID, "")
	assertStatusCode(t, getRecorder.Code, consts.StatusOK)
	var got conversationResponse
	decodeResponseBody(t, getRecorder, &got)
	if got.ID != conversationID || len(got.Messages) != 4 {
		t.Fatalf("get conversation = %#v, want updated conversation", got)
	}

	listRecorder := performRequest(h, consts.MethodGet, "/api/v1/projects/"+projectID+"/conversations?target_type=project&target_id="+projectID+"&limit=10&offset=0", "")
	assertStatusCode(t, listRecorder.Code, consts.StatusOK)
	var listed conversationListResponse
	decodeResponseBody(t, listRecorder, &listed)
	if len(listed.Conversations) != 1 || listed.Conversations[0].ID != conversationID {
		t.Fatalf("listed conversations = %#v, want single updated conversation", listed.Conversations)
	}

	confirmRecorder := performRequest(h, consts.MethodPost, "/api/v1/conversations/"+conversationID+"/confirm", "")
	assertStatusCode(t, confirmRecorder.Code, consts.StatusOK)
	var confirmed confirmConversationResponse
	decodeResponseBody(t, confirmRecorder, &confirmed)
	if confirmed.Project == nil || confirmed.Project.Title != "Final title" || confirmed.Project.Summary != "Final summary" {
		t.Fatalf("confirmed project = %#v, want final project payload", confirmed.Project)
	}
	if confirmed.Conversation.PendingSuggestion != nil {
		t.Fatalf("confirmed pending suggestion = %#v, want nil", confirmed.Conversation.PendingSuggestion)
	}
	if got := confirmed.Conversation.Messages[len(confirmed.Conversation.Messages)-1].Role; got != conversationdomain.MessageRoleSystem {
		t.Fatalf("confirmed last role = %q, want system", got)
	}
}

func TestConversationConfirmAssetResponseIntegration(t *testing.T) {
	const (
		projectID      = "11111111-1111-1111-1111-111111111111"
		assetID        = "22222222-2222-2222-2222-222222222222"
		conversationID = "33333333-3333-3333-3333-333333333333"
	)

	baseTime := time.Date(2026, 3, 9, 12, 0, 0, 0, time.UTC)
	confirmedConversation := &conversationdomain.Conversation{
		ID:         conversationID,
		ProjectID:  projectID,
		TargetType: conversationdomain.TargetTypeAsset,
		TargetID:   assetID,
		Messages: []conversationdomain.Message{
			{ID: "44444444-4444-4444-4444-444444444444", Role: conversationdomain.MessageRoleUser, Content: "优化资产。", CreatedAt: baseTime},
			{ID: "55555555-5555-5555-5555-555555555555", Role: conversationdomain.MessageRoleAssistant, Content: `{"title":"更新标题","content":"更新内容"}`, CreatedAt: baseTime.Add(time.Minute)},
			{ID: "66666666-6666-6666-6666-666666666666", Role: conversationdomain.MessageRoleSystem, Content: "已确认最新资产建议并写回资产。", CreatedAt: baseTime.Add(2 * time.Minute)},
		},
		CreatedAt: baseTime,
		UpdatedAt: baseTime.Add(2 * time.Minute),
	}
	confirmedAsset := &assetdomain.Asset{
		ID:        assetID,
		ProjectID: projectID,
		Type:      assetdomain.TypeOutline,
		Title:     "更新标题",
		Content:   "更新内容",
		CreatedAt: baseTime.Add(-time.Hour),
		UpdatedAt: baseTime.Add(2 * time.Minute),
	}

	h := newTestServerWithUseCases(
		stubProjectUseCase{},
		stubAssetUseCase{},
		stubConversationUseCase{
			confirm: func(_ context.Context, id string) (*conversationservice.ConfirmResult, error) {
				if id != conversationID {
					t.Fatalf("Confirm id = %q, want %q", id, conversationID)
				}
				return &conversationservice.ConfirmResult{
					Conversation: confirmedConversation,
					Asset:        confirmedAsset,
				}, nil
			},
		},
	)

	recorder := performRequest(h, consts.MethodPost, "/api/v1/conversations/"+conversationID+"/confirm", "")
	assertStatusCode(t, recorder.Code, consts.StatusOK)

	var confirmed confirmConversationResponse
	decodeResponseBody(t, recorder, &confirmed)
	if confirmed.Project != nil {
		t.Fatalf("confirmed project = %#v, want nil", confirmed.Project)
	}
	if confirmed.Asset == nil {
		t.Fatal("confirmed asset = nil, want non-nil")
	}
	if confirmed.Asset.ID != assetID || confirmed.Asset.Title != "更新标题" || confirmed.Asset.Content != "更新内容" {
		t.Fatalf("confirmed asset = %#v, want confirmed asset payload", confirmed.Asset)
	}
}

func TestConversationRouteValidationAndErrorMappingIntegration(t *testing.T) {
	const (
		projectID      = "11111111-1111-1111-1111-111111111111"
		conversationID = "33333333-3333-3333-3333-333333333333"
	)

	t.Run("start malformed json returns 400", func(t *testing.T) {
		h := newTestServerWithUseCases(stubProjectUseCase{}, stubAssetUseCase{}, stubConversationUseCase{})
		recorder := performJSONRequest(h, consts.MethodPost, "/api/v1/projects/"+projectID+"/conversations", `{"target_type":`)
		assertStatusCode(t, recorder.Code, consts.StatusBadRequest)
		assertErrorResponse(t, recorder)
	})

	t.Run("reply malformed json returns 400", func(t *testing.T) {
		h := newTestServerWithUseCases(stubProjectUseCase{}, stubAssetUseCase{}, stubConversationUseCase{})
		recorder := performJSONRequest(h, consts.MethodPost, "/api/v1/conversations/"+conversationID+"/messages", `{"message":`)
		assertStatusCode(t, recorder.Code, consts.StatusBadRequest)
		assertErrorResponse(t, recorder)
	})

	t.Run("list invalid query returns 400", func(t *testing.T) {
		h := newTestServerWithUseCases(stubProjectUseCase{}, stubAssetUseCase{}, stubConversationUseCase{})
		recorder := performRequest(h, consts.MethodGet, "/api/v1/projects/"+projectID+"/conversations?target_type=", "")
		assertStatusCode(t, recorder.Code, consts.StatusBadRequest)
		assertErrorContains(t, recorder, "target_type must not be empty")
	})

	t.Run("start invalid input maps to 400", func(t *testing.T) {
		h := newTestServerWithUseCases(
			stubProjectUseCase{},
			stubAssetUseCase{},
			stubConversationUseCase{
				start: func(context.Context, conversationservice.StartParams) (*conversationdomain.Conversation, error) {
					return nil, appservice.WrapInvalidInput(errors.New("target_type must be one of project, asset"))
				},
			},
		)
		recorder := performJSONRequest(h, consts.MethodPost, "/api/v1/projects/"+projectID+"/conversations", `{"target_type":"project","target_id":"11111111-1111-1111-1111-111111111111","message":"Polish the project title."}`)
		assertStatusCode(t, recorder.Code, consts.StatusBadRequest)
		assertErrorContains(t, recorder, "target_type must be one of project, asset")
	})

	t.Run("get not found maps to 404", func(t *testing.T) {
		h := newTestServerWithUseCases(
			stubProjectUseCase{},
			stubAssetUseCase{},
			stubConversationUseCase{
				getByID: func(context.Context, string) (*conversationdomain.Conversation, error) {
					return nil, appservice.WrapNotFound(errors.New("missing conversation"))
				},
			},
		)
		recorder := performRequest(h, consts.MethodGet, "/api/v1/conversations/"+conversationID, "")
		assertStatusCode(t, recorder.Code, consts.StatusNotFound)
		assertErrorContains(t, recorder, "missing conversation")
	})

	t.Run("confirm unexpected error maps to 500", func(t *testing.T) {
		h := newTestServerWithUseCases(
			stubProjectUseCase{},
			stubAssetUseCase{},
			stubConversationUseCase{
				confirm: func(context.Context, string) (*conversationservice.ConfirmResult, error) {
					return nil, errors.New("conversation repository offline")
				},
			},
		)
		recorder := performRequest(h, consts.MethodPost, "/api/v1/conversations/"+conversationID+"/confirm", "")
		assertStatusCode(t, recorder.Code, consts.StatusInternalServerError)
		assertErrorMessage(t, recorder, "internal server error")
	})
}

func TestChapterRoutesIntegration(t *testing.T) {
	const (
		projectID           = "11111111-1111-1111-1111-111111111111"
		chapterID           = "22222222-2222-2222-2222-222222222222"
		generationID        = "33333333-3333-3333-3333-333333333333"
		continuedGeneration = "44444444-4444-4444-4444-444444444444"
		rewrittenGeneration = "55555555-5555-5555-5555-555555555555"
		confirmedBy         = "66666666-6666-6666-6666-666666666666"
	)

	baseTime := time.Date(2026, 3, 9, 14, 0, 0, 0, time.UTC)
	generatedChapter := &chapterdomain.Chapter{
		ID:             chapterID,
		ProjectID:      projectID,
		Title:          "第一章 王城初见",
		Ordinal:        1,
		Status:         chapterdomain.StatusDraft,
		Content:        "首章完整正文。",
		CurrentDraftID: generationID,
		CreatedAt:      baseTime,
		UpdatedAt:      baseTime,
	}
	continuedChapter := &chapterdomain.Chapter{
		ID:             chapterID,
		ProjectID:      projectID,
		Title:          "第一章 王城初见",
		Ordinal:        1,
		Status:         chapterdomain.StatusDraft,
		Content:        "续写后的完整正文。",
		CurrentDraftID: continuedGeneration,
		CreatedAt:      baseTime,
		UpdatedAt:      baseTime.Add(time.Minute),
	}
	rewrittenChapter := &chapterdomain.Chapter{
		ID:             chapterID,
		ProjectID:      projectID,
		Title:          "第一章 王城初见",
		Ordinal:        1,
		Status:         chapterdomain.StatusDraft,
		Content:        "局部改写后的完整正文。",
		CurrentDraftID: rewrittenGeneration,
		CreatedAt:      baseTime,
		UpdatedAt:      baseTime.Add(2 * time.Minute),
	}
	generatedRecord := &generationdomain.GenerationRecord{
		ID:               generationID,
		ProjectID:        projectID,
		ChapterID:        chapterID,
		Kind:             generationdomain.KindChapterGeneration,
		Status:           generationdomain.StatusSucceeded,
		InputSnapshotRef: "generate input",
		OutputRef:        generatedChapter.Content,
		TokenUsage:       0,
		DurationMillis:   12,
		CreatedAt:        baseTime,
		UpdatedAt:        baseTime,
	}
	continuedRecord := &generationdomain.GenerationRecord{
		ID:               continuedGeneration,
		ProjectID:        projectID,
		ChapterID:        chapterID,
		Kind:             generationdomain.KindChapterContinuation,
		Status:           generationdomain.StatusSucceeded,
		InputSnapshotRef: "continue input",
		OutputRef:        continuedChapter.Content,
		TokenUsage:       0,
		DurationMillis:   18,
		CreatedAt:        baseTime.Add(time.Minute),
		UpdatedAt:        baseTime.Add(time.Minute),
	}
	rewrittenRecord := &generationdomain.GenerationRecord{
		ID:               rewrittenGeneration,
		ProjectID:        projectID,
		ChapterID:        chapterID,
		Kind:             generationdomain.KindChapterRewrite,
		Status:           generationdomain.StatusSucceeded,
		InputSnapshotRef: "rewrite input",
		OutputRef:        rewrittenChapter.Content,
		TokenUsage:       0,
		DurationMillis:   21,
		CreatedAt:        baseTime.Add(2 * time.Minute),
		UpdatedAt:        baseTime.Add(2 * time.Minute),
	}
	confirmedAt := baseTime.Add(3 * time.Minute)
	confirmedChapter := &chapterdomain.Chapter{
		ID:                      chapterID,
		ProjectID:               projectID,
		Title:                   "第一章 王城初见",
		Ordinal:                 1,
		Status:                  chapterdomain.StatusConfirmed,
		Content:                 rewrittenChapter.Content,
		CurrentDraftID:          rewrittenGeneration,
		CurrentDraftConfirmedAt: &confirmedAt,
		CurrentDraftConfirmedBy: confirmedBy,
		CreatedAt:               baseTime,
		UpdatedAt:               confirmedAt,
	}

	h := newTestServerWithAllUseCases(
		stubProjectUseCase{},
		stubAssetUseCase{},
		stubChapterUseCase{
			generate: func(_ context.Context, params chapterservice.GenerateParams) (*chapterservice.GenerateResult, error) {
				if params.ProjectID != projectID || params.Title != "第一章 王城初见" || params.Ordinal != 1 || params.Instruction != "写出主角第一次进入王城时的压迫感。" {
					t.Fatalf("Generate params = %#v, want expected payload", params)
				}
				return &chapterservice.GenerateResult{Chapter: generatedChapter, GenerationRecord: generatedRecord}, nil
			},
			listByProject: func(_ context.Context, params chapterdomain.ListByProjectParams) ([]*chapterdomain.Chapter, error) {
				if params.ProjectID != projectID || params.Limit != 10 || params.Offset != 0 {
					t.Fatalf("ListByProject params = %#v, want expected payload", params)
				}
				return []*chapterdomain.Chapter{generatedChapter}, nil
			},
			getByID: func(_ context.Context, id string) (*chapterdomain.Chapter, error) {
				if id != chapterID {
					t.Fatalf("GetByID id = %q, want %q", id, chapterID)
				}
				return rewrittenChapter, nil
			},
			continueFn: func(_ context.Context, params chapterservice.ContinueParams) (*chapterservice.ContinueResult, error) {
				if params.ChapterID != chapterID || params.Instruction != "继续写主角离开王城前的冲突。" {
					t.Fatalf("Continue params = %#v, want expected payload", params)
				}
				return &chapterservice.ContinueResult{Chapter: continuedChapter, GenerationRecord: continuedRecord}, nil
			},
			rewrite: func(_ context.Context, params chapterservice.RewriteParams) (*chapterservice.RewriteResult, error) {
				if params.ChapterID != chapterID || params.TargetText != "旧片段" || params.Instruction != "把这一段改得更紧张。" {
					t.Fatalf("Rewrite params = %#v, want expected payload", params)
				}
				return &chapterservice.RewriteResult{Chapter: rewrittenChapter, GenerationRecord: rewrittenRecord}, nil
			},
			confirm: func(_ context.Context, params chapterservice.ConfirmParams) (*chapterdomain.Chapter, error) {
				if params.ChapterID != chapterID || params.ConfirmedBy != confirmedBy {
					t.Fatalf("Confirm params = %#v, want expected payload", params)
				}
				return confirmedChapter, nil
			},
		},
		stubConversationUseCase{},
	)

	createRecorder := performJSONRequest(h, consts.MethodPost, "/api/v1/projects/"+projectID+"/chapters", `{"title":"第一章 王城初见","ordinal":1,"instruction":"写出主角第一次进入王城时的压迫感。"}`)
	assertStatusCode(t, createRecorder.Code, consts.StatusCreated)
	var created chapterGenerationResponse
	decodeResponseBody(t, createRecorder, &created)
	if created.Chapter.ID != chapterID || created.GenerationRecord.ID != generationID {
		t.Fatalf("create response = %#v, want generated chapter and record", created)
	}

	listRecorder := performRequest(h, consts.MethodGet, "/api/v1/projects/"+projectID+"/chapters?limit=10&offset=0", "")
	assertStatusCode(t, listRecorder.Code, consts.StatusOK)
	var listed chapterListResponse
	decodeResponseBody(t, listRecorder, &listed)
	if len(listed.Chapters) != 1 || listed.Chapters[0].ID != chapterID {
		t.Fatalf("listed chapters = %#v, want single chapter", listed.Chapters)
	}

	getRecorder := performRequest(h, consts.MethodGet, "/api/v1/chapters/"+chapterID, "")
	assertStatusCode(t, getRecorder.Code, consts.StatusOK)
	var got chapterResponse
	decodeResponseBody(t, getRecorder, &got)
	if got.ID != chapterID || got.Content != rewrittenChapter.Content {
		t.Fatalf("get chapter = %#v, want rewritten chapter payload", got)
	}

	continueRecorder := performJSONRequest(h, consts.MethodPost, "/api/v1/chapters/"+chapterID+"/continue", `{"instruction":"继续写主角离开王城前的冲突。"}`)
	assertStatusCode(t, continueRecorder.Code, consts.StatusOK)
	var continued chapterGenerationResponse
	decodeResponseBody(t, continueRecorder, &continued)
	if continued.Chapter.Content != continuedChapter.Content || continued.GenerationRecord.Kind != generationdomain.KindChapterContinuation {
		t.Fatalf("continue response = %#v, want continuation payload", continued)
	}

	rewriteRecorder := performJSONRequest(h, consts.MethodPost, "/api/v1/chapters/"+chapterID+"/rewrite", `{"target_text":"旧片段","instruction":"把这一段改得更紧张。"}`)
	assertStatusCode(t, rewriteRecorder.Code, consts.StatusOK)
	var rewritten chapterGenerationResponse
	decodeResponseBody(t, rewriteRecorder, &rewritten)
	if rewritten.Chapter.Content != rewrittenChapter.Content || rewritten.GenerationRecord.Kind != generationdomain.KindChapterRewrite {
		t.Fatalf("rewrite response = %#v, want rewrite payload", rewritten)
	}

	confirmRecorder := performJSONRequestWithHeaders(
		h,
		consts.MethodPost,
		"/api/v1/chapters/"+chapterID+"/confirm",
		"",
		ut.Header{Key: testUserIDHeader, Value: confirmedBy},
	)
	assertStatusCode(t, confirmRecorder.Code, consts.StatusOK)
	var confirmed chapterResponse
	decodeResponseBody(t, confirmRecorder, &confirmed)
	if confirmed.Status != chapterdomain.StatusConfirmed {
		t.Fatalf("confirm status = %q, want %q", confirmed.Status, chapterdomain.StatusConfirmed)
	}
	if confirmed.CurrentDraftID != confirmedChapter.CurrentDraftID {
		t.Fatalf("confirm current_draft_id = %q, want %q", confirmed.CurrentDraftID, confirmedChapter.CurrentDraftID)
	}
	if confirmed.CurrentDraftConfirmedAt == nil || *confirmed.CurrentDraftConfirmedAt != confirmedAt.Format("2006-01-02T15:04:05Z07:00") {
		t.Fatalf("confirm current_draft_confirmed_at = %#v, want %q", confirmed.CurrentDraftConfirmedAt, confirmedAt.Format("2006-01-02T15:04:05Z07:00"))
	}
	if confirmed.CurrentDraftConfirmedBy != confirmedBy {
		t.Fatalf("confirm current_draft_confirmed_by = %q, want %q", confirmed.CurrentDraftConfirmedBy, confirmedBy)
	}
}

func TestChapterRouteValidationAndErrorMappingIntegration(t *testing.T) {
	const (
		projectID = "11111111-1111-1111-1111-111111111111"
		chapterID = "22222222-2222-2222-2222-222222222222"
	)

	t.Run("create malformed json returns 400", func(t *testing.T) {
		h := newTestServerWithAllUseCases(stubProjectUseCase{}, stubAssetUseCase{}, stubChapterUseCase{}, stubConversationUseCase{})
		recorder := performJSONRequest(h, consts.MethodPost, "/api/v1/projects/"+projectID+"/chapters", `{"title":`)
		assertStatusCode(t, recorder.Code, consts.StatusBadRequest)
		assertErrorResponse(t, recorder)
	})

	t.Run("continue malformed json returns 400", func(t *testing.T) {
		h := newTestServerWithAllUseCases(stubProjectUseCase{}, stubAssetUseCase{}, stubChapterUseCase{}, stubConversationUseCase{})
		recorder := performJSONRequest(h, consts.MethodPost, "/api/v1/chapters/"+chapterID+"/continue", `{"instruction":`)
		assertStatusCode(t, recorder.Code, consts.StatusBadRequest)
		assertErrorResponse(t, recorder)
	})

	t.Run("rewrite malformed json returns 400", func(t *testing.T) {
		h := newTestServerWithAllUseCases(stubProjectUseCase{}, stubAssetUseCase{}, stubChapterUseCase{}, stubConversationUseCase{})
		recorder := performJSONRequest(h, consts.MethodPost, "/api/v1/chapters/"+chapterID+"/rewrite", `{"target_text":`)
		assertStatusCode(t, recorder.Code, consts.StatusBadRequest)
		assertErrorResponse(t, recorder)
	})

	t.Run("list invalid query returns 400", func(t *testing.T) {
		h := newTestServerWithAllUseCases(stubProjectUseCase{}, stubAssetUseCase{}, stubChapterUseCase{}, stubConversationUseCase{})
		recorder := performRequest(h, consts.MethodGet, "/api/v1/projects/"+projectID+"/chapters?limit=abc", "")
		assertStatusCode(t, recorder.Code, consts.StatusBadRequest)
		assertErrorResponse(t, recorder)
	})

	t.Run("create invalid input maps to 400", func(t *testing.T) {
		h := newTestServerWithAllUseCases(
			stubProjectUseCase{},
			stubAssetUseCase{},
			stubChapterUseCase{
				generate: func(context.Context, chapterservice.GenerateParams) (*chapterservice.GenerateResult, error) {
					return nil, appservice.WrapInvalidInput(errors.New("ordinal must be greater than 0"))
				},
			},
			stubConversationUseCase{},
		)
		recorder := performJSONRequest(h, consts.MethodPost, "/api/v1/projects/"+projectID+"/chapters", `{"title":"第一章","ordinal":0,"instruction":"开始写。"}`)
		assertStatusCode(t, recorder.Code, consts.StatusBadRequest)
		assertErrorContains(t, recorder, "ordinal must be greater than 0")
	})

	t.Run("get not found maps to 404", func(t *testing.T) {
		h := newTestServerWithAllUseCases(
			stubProjectUseCase{},
			stubAssetUseCase{},
			stubChapterUseCase{
				getByID: func(context.Context, string) (*chapterdomain.Chapter, error) {
					return nil, appservice.WrapNotFound(errors.New("missing chapter"))
				},
			},
			stubConversationUseCase{},
		)
		recorder := performRequest(h, consts.MethodGet, "/api/v1/chapters/"+chapterID, "")
		assertStatusCode(t, recorder.Code, consts.StatusNotFound)
		assertErrorContains(t, recorder, "missing chapter")
	})

	t.Run("confirm missing user id returns 401", func(t *testing.T) {
		h := newTestServerWithAllUseCases(stubProjectUseCase{}, stubAssetUseCase{}, stubChapterUseCase{}, stubConversationUseCase{})
		recorder := performRequest(h, consts.MethodPost, "/api/v1/chapters/"+chapterID+"/confirm", "")
		assertStatusCode(t, recorder.Code, consts.StatusUnauthorized)
		assertErrorMessage(t, recorder, "user_id must be a valid UUID")
	})

	t.Run("confirm invalid user id returns 401", func(t *testing.T) {
		h := newTestServerWithAllUseCases(stubProjectUseCase{}, stubAssetUseCase{}, stubChapterUseCase{}, stubConversationUseCase{})
		recorder := performJSONRequestWithHeaders(
			h,
			consts.MethodPost,
			"/api/v1/chapters/"+chapterID+"/confirm",
			"",
			ut.Header{Key: testUserIDHeader, Value: "not-a-uuid"},
		)
		assertStatusCode(t, recorder.Code, consts.StatusUnauthorized)
		assertErrorMessage(t, recorder, "user_id must be a valid UUID")
	})

	t.Run("confirm conflict maps to 409", func(t *testing.T) {
		h := newTestServerWithAllUseCases(
			stubProjectUseCase{},
			stubAssetUseCase{},
			stubChapterUseCase{
				confirm: func(context.Context, chapterservice.ConfirmParams) (*chapterdomain.Chapter, error) {
					return nil, appservice.WrapConflict(errors.New("draft already confirmed"))
				},
			},
			stubConversationUseCase{},
		)
		recorder := performJSONRequestWithHeaders(
			h,
			consts.MethodPost,
			"/api/v1/chapters/"+chapterID+"/confirm",
			"",
			ut.Header{Key: testUserIDHeader, Value: "33333333-3333-3333-3333-333333333333"},
		)
		assertStatusCode(t, recorder.Code, consts.StatusConflict)
		assertErrorContains(t, recorder, "draft already confirmed")
	})

	t.Run("confirm stale update maps to 409", func(t *testing.T) {
		h := newTestServerWithAllUseCases(
			stubProjectUseCase{},
			stubAssetUseCase{},
			stubChapterUseCase{
				confirm: func(context.Context, chapterservice.ConfirmParams) (*chapterdomain.Chapter, error) {
					return nil, appservice.WrapConflict(errors.New("chapter was modified during confirmation; please retry"))
				},
			},
			stubConversationUseCase{},
		)
		recorder := performJSONRequestWithHeaders(
			h,
			consts.MethodPost,
			"/api/v1/chapters/"+chapterID+"/confirm",
			"",
			ut.Header{Key: testUserIDHeader, Value: "33333333-3333-3333-3333-333333333333"},
		)
		assertStatusCode(t, recorder.Code, consts.StatusConflict)
		assertErrorContains(t, recorder, "please retry")
	})

	t.Run("confirm not found maps to 404", func(t *testing.T) {
		h := newTestServerWithAllUseCases(
			stubProjectUseCase{},
			stubAssetUseCase{},
			stubChapterUseCase{
				confirm: func(context.Context, chapterservice.ConfirmParams) (*chapterdomain.Chapter, error) {
					return nil, appservice.WrapNotFound(errors.New("missing generation record"))
				},
			},
			stubConversationUseCase{},
		)
		recorder := performJSONRequestWithHeaders(
			h,
			consts.MethodPost,
			"/api/v1/chapters/"+chapterID+"/confirm",
			"",
			ut.Header{Key: testUserIDHeader, Value: "33333333-3333-3333-3333-333333333333"},
		)
		assertStatusCode(t, recorder.Code, consts.StatusNotFound)
		assertErrorContains(t, recorder, "missing generation record")
	})

	t.Run("continue unexpected error maps to 500", func(t *testing.T) {
		h := newTestServerWithAllUseCases(
			stubProjectUseCase{},
			stubAssetUseCase{},
			stubChapterUseCase{
				continueFn: func(context.Context, chapterservice.ContinueParams) (*chapterservice.ContinueResult, error) {
					return nil, errors.New("llm backend offline")
				},
			},
			stubConversationUseCase{},
		)
		recorder := performRequest(h, consts.MethodPost, "/api/v1/chapters/"+chapterID+"/continue", `{"instruction":"继续写。"}`)
		assertStatusCode(t, recorder.Code, consts.StatusInternalServerError)
		assertErrorMessage(t, recorder, "internal server error")
	})
}

func newTestServer() *server.Hertz {
	projectRepo := memory.NewProjectRepository()
	assetRepo := memory.NewAssetRepository()
	generationRepo := memory.NewGenerationRecordRepository()
	metricRepo := memory.NewMetricEventRepository()
	metricUseCase := metricservice.NewUseCase(metricservice.Dependencies{MetricEvents: metricRepo})
	promptStore, err := prompts.LoadStore(config.PromptConfig{
		AssetGeneration:     "asset_generation.yaml",
		ChapterGeneration:   "chapter_generation.yaml",
		ChapterContinuation: "chapter_continuation.yaml",
		ChapterRewrite:      "chapter_rewrite.yaml",
		ProjectRefinement:   "project_refinement.yaml",
		AssetRefinement:     "asset_refinement.yaml",
	})
	if err != nil {
		panic(err)
	}
	conversationRepo := memory.NewConversationRepository()
	projectUseCase := projectservice.NewUseCase(projectservice.Dependencies{Projects: projectRepo})
	assetUseCase := assetservice.NewUseCase(assetservice.Dependencies{
		Assets:            assetRepo,
		Projects:          projectRepo,
		GenerationRecords: generationRepo,
		LLMClient: &stubLLMClient{
			chatModel: &stubChatModel{
				generate: func(context.Context, []*schema.Message, ...model.Option) (*schema.Message, error) {
					return &schema.Message{Content: `{"title":"自动生成资产","content":"自动生成内容"}`}, nil
				},
			},
		},
		PromptStore: promptStore,
		Metrics:     metricUseCase,
	})
	conversationUseCase := stubConversationUseCase{}
	_ = conversationRepo

	return newTestServerWithUseCases(projectUseCase, assetUseCase, conversationUseCase)
}

func newTestServerWithUseCases(projectUseCase projectservice.UseCase, assetUseCase assetservice.UseCase, conversationUseCase conversationservice.UseCase) *server.Hertz {
	return newTestServerWithAllUseCases(projectUseCase, assetUseCase, stubChapterUseCase{}, conversationUseCase)
}

func newTestServerWithAllUseCases(projectUseCase projectservice.UseCase, assetUseCase assetservice.UseCase, chapterUseCase chapterservice.UseCase, conversationUseCase conversationservice.UseCase) *server.Hertz {
	testConfig := config.ServerConfig{
		Host:                "127.0.0.1",
		Port:                18080,
		ReadTimeoutSeconds:  1,
		WriteTimeoutSeconds: 1,
	}

	h := server.Default(
		server.WithHostPorts(testConfig.Address()),
		server.WithReadTimeout(time.Duration(testConfig.ReadTimeoutSeconds)*time.Second),
		server.WithWriteTimeout(time.Duration(testConfig.WriteTimeoutSeconds)*time.Second),
	)
	h.Use(middleware.RequestID(), middleware.Recovery(), middleware.UserContext())
	apiroutes.RegisterRoutes(h, apiroutes.Dependencies{
		Projects:      projectUseCase,
		Assets:        assetUseCase,
		Chapters:      chapterUseCase,
		Conversations: conversationUseCase,
	})
	return h
}

func createTestProject(t *testing.T, h *server.Hertz) projectResponse {
	t.Helper()

	recorder := performJSONRequest(h, consts.MethodPost, "/api/v1/projects", `{"title":"Novel","summary":"Story summary","status":"draft"}`)
	assertStatusCode(t, recorder.Code, consts.StatusCreated)

	var project projectResponse
	decodeResponseBody(t, recorder, &project)
	return project
}

func createTestAsset(t *testing.T, h *server.Hertz, projectID string) assetResponse {
	t.Helper()

	recorder := performJSONRequest(h, consts.MethodPost, "/api/v1/projects/"+projectID+"/assets", `{"type":"outline","title":"Outline","content":"Outline body"}`)
	assertStatusCode(t, recorder.Code, consts.StatusCreated)

	var asset assetResponse
	decodeResponseBody(t, recorder, &asset)
	return asset
}

func performRequest(h *server.Hertz, method, url, body string) *ut.ResponseRecorder {
	return performRequestWithHeaders(h, method, url, body)
}

func performRequestWithHeaders(h *server.Hertz, method, url, body string, extraHeaders ...ut.Header) *ut.ResponseRecorder {
	var requestBody *ut.Body
	if body != "" {
		requestBody = &ut.Body{Body: bytes.NewBufferString(body), Len: len(body)}
	}

	headers := append([]ut.Header{}, extraHeaders...)
	if body != "" {
		headers = append(headers, ut.Header{Key: "Content-Type", Value: consts.MIMEApplicationJSON})
	}
	return ut.PerformRequest(h.Engine, method, url, requestBody, headers...)
}

func performJSONRequest(h *server.Hertz, method, url, body string) *ut.ResponseRecorder {
	return performRequest(h, method, url, body)
}

func performJSONRequestWithHeaders(h *server.Hertz, method, url, body string, extraHeaders ...ut.Header) *ut.ResponseRecorder {
	return performRequestWithHeaders(h, method, url, body, extraHeaders...)
}

func assertStatusCode(t *testing.T, got, want int) {
	t.Helper()
	if got != want {
		t.Fatalf("status code = %d, want %d", got, want)
	}
}

func decodeResponseBody(t *testing.T, recorder *ut.ResponseRecorder, target any) {
	t.Helper()
	if err := json.Unmarshal(recorder.Body.Bytes(), target); err != nil {
		t.Fatalf("unmarshal response body %q: %v", recorder.Body.String(), err)
	}
}

func assertErrorResponse(t *testing.T, recorder *ut.ResponseRecorder) {
	t.Helper()
	var response map[string]string
	decodeResponseBody(t, recorder, &response)
	if strings.TrimSpace(response["error"]) == "" {
		t.Fatalf("error response = %#v, want non-empty error message", response)
	}
}

func assertErrorMessage(t *testing.T, recorder *ut.ResponseRecorder, want string) {
	t.Helper()
	var response map[string]string
	decodeResponseBody(t, recorder, &response)
	if response["error"] != want {
		t.Fatalf("error message = %q, want %q", response["error"], want)
	}
}

func assertErrorContains(t *testing.T, recorder *ut.ResponseRecorder, wantSubstring string) {
	t.Helper()
	var response map[string]string
	decodeResponseBody(t, recorder, &response)
	if !strings.Contains(response["error"], wantSubstring) {
		t.Fatalf("error message = %q, want substring %q", response["error"], wantSubstring)
	}
}

type stubProjectUseCase struct {
	create  func(context.Context, *projectdomain.Project) error
	getByID func(context.Context, string) (*projectdomain.Project, error)
	list    func(context.Context, projectdomain.ListParams) ([]*projectdomain.Project, error)
	update  func(context.Context, *projectdomain.Project) error
}

func (s stubProjectUseCase) Create(ctx context.Context, project *projectdomain.Project) error {
	if s.create != nil {
		return s.create(ctx, project)
	}
	return errors.New("unexpected Create call")
}

func (s stubProjectUseCase) GetByID(ctx context.Context, id string) (*projectdomain.Project, error) {
	if s.getByID != nil {
		return s.getByID(ctx, id)
	}
	return nil, errors.New("unexpected GetByID call")
}

func (s stubProjectUseCase) List(ctx context.Context, params projectdomain.ListParams) ([]*projectdomain.Project, error) {
	if s.list != nil {
		return s.list(ctx, params)
	}
	return nil, errors.New("unexpected List call")
}

func (s stubProjectUseCase) Update(ctx context.Context, project *projectdomain.Project) error {
	if s.update != nil {
		return s.update(ctx, project)
	}
	return errors.New("unexpected Update call")
}

type stubAssetUseCase struct {
	create            func(context.Context, *assetdomain.Asset) error
	generate          func(context.Context, assetservice.GenerateParams) (*assetservice.GenerateResult, error)
	getByID           func(context.Context, string) (*assetdomain.Asset, error)
	listByProject     func(context.Context, assetdomain.ListByProjectParams) ([]*assetdomain.Asset, error)
	listByProjectType func(context.Context, assetdomain.ListByProjectAndTypeParams) ([]*assetdomain.Asset, error)
	update            func(context.Context, *assetdomain.Asset) error
	delete            func(context.Context, string) error
}

func (s stubAssetUseCase) Create(ctx context.Context, asset *assetdomain.Asset) error {
	if s.create != nil {
		return s.create(ctx, asset)
	}
	return errors.New("unexpected Create call")
}

func (s stubAssetUseCase) Generate(ctx context.Context, params assetservice.GenerateParams) (*assetservice.GenerateResult, error) {
	if s.generate != nil {
		return s.generate(ctx, params)
	}
	return nil, errors.New("unexpected Generate call")
}

func (s stubAssetUseCase) GetByID(ctx context.Context, id string) (*assetdomain.Asset, error) {
	if s.getByID != nil {
		return s.getByID(ctx, id)
	}
	return nil, errors.New("unexpected GetByID call")
}

func (s stubAssetUseCase) ListByProject(ctx context.Context, params assetdomain.ListByProjectParams) ([]*assetdomain.Asset, error) {
	if s.listByProject != nil {
		return s.listByProject(ctx, params)
	}
	return nil, errors.New("unexpected ListByProject call")
}

func (s stubAssetUseCase) ListByProjectAndType(ctx context.Context, params assetdomain.ListByProjectAndTypeParams) ([]*assetdomain.Asset, error) {
	if s.listByProjectType != nil {
		return s.listByProjectType(ctx, params)
	}
	return nil, errors.New("unexpected ListByProjectAndType call")
}

func (s stubAssetUseCase) Update(ctx context.Context, asset *assetdomain.Asset) error {
	if s.update != nil {
		return s.update(ctx, asset)
	}
	return errors.New("unexpected Update call")
}

func (s stubAssetUseCase) Delete(ctx context.Context, id string) error {
	if s.delete != nil {
		return s.delete(ctx, id)
	}
	return errors.New("unexpected Delete call")
}

type stubChapterUseCase struct {
	create        func(context.Context, *chapterdomain.Chapter) error
	getByID       func(context.Context, string) (*chapterdomain.Chapter, error)
	listByProject func(context.Context, chapterdomain.ListByProjectParams) ([]*chapterdomain.Chapter, error)
	update        func(context.Context, *chapterdomain.Chapter) error
	generate      func(context.Context, chapterservice.GenerateParams) (*chapterservice.GenerateResult, error)
	continueFn    func(context.Context, chapterservice.ContinueParams) (*chapterservice.ContinueResult, error)
	rewrite       func(context.Context, chapterservice.RewriteParams) (*chapterservice.RewriteResult, error)
	confirm       func(context.Context, chapterservice.ConfirmParams) (*chapterdomain.Chapter, error)
}

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

func (s stubChapterUseCase) Create(ctx context.Context, chapter *chapterdomain.Chapter) error {
	if s.create != nil {
		return s.create(ctx, chapter)
	}
	return errors.New("unexpected Create call")
}

func (s stubChapterUseCase) GetByID(ctx context.Context, id string) (*chapterdomain.Chapter, error) {
	if s.getByID != nil {
		return s.getByID(ctx, id)
	}
	return nil, errors.New("unexpected GetByID call")
}

func (s stubChapterUseCase) ListByProject(ctx context.Context, params chapterdomain.ListByProjectParams) ([]*chapterdomain.Chapter, error) {
	if s.listByProject != nil {
		return s.listByProject(ctx, params)
	}
	return nil, errors.New("unexpected ListByProject call")
}

func (s stubChapterUseCase) Update(ctx context.Context, chapter *chapterdomain.Chapter) error {
	if s.update != nil {
		return s.update(ctx, chapter)
	}
	return errors.New("unexpected Update call")
}

func (s stubChapterUseCase) Generate(ctx context.Context, params chapterservice.GenerateParams) (*chapterservice.GenerateResult, error) {
	if s.generate != nil {
		return s.generate(ctx, params)
	}
	return nil, errors.New("unexpected Generate call")
}

func (s stubChapterUseCase) Continue(ctx context.Context, params chapterservice.ContinueParams) (*chapterservice.ContinueResult, error) {
	if s.continueFn != nil {
		return s.continueFn(ctx, params)
	}
	return nil, errors.New("unexpected Continue call")
}

func (s stubChapterUseCase) Rewrite(ctx context.Context, params chapterservice.RewriteParams) (*chapterservice.RewriteResult, error) {
	if s.rewrite != nil {
		return s.rewrite(ctx, params)
	}
	return nil, errors.New("unexpected Rewrite call")
}

func (s stubChapterUseCase) Confirm(ctx context.Context, params chapterservice.ConfirmParams) (*chapterdomain.Chapter, error) {
	if s.confirm != nil {
		return s.confirm(ctx, params)
	}
	return nil, errors.New("unexpected Confirm call")
}

type stubConversationUseCase struct {
	start   func(context.Context, conversationservice.StartParams) (*conversationdomain.Conversation, error)
	reply   func(context.Context, conversationservice.ReplyParams) (*conversationdomain.Conversation, error)
	confirm func(context.Context, string) (*conversationservice.ConfirmResult, error)
	getByID func(context.Context, string) (*conversationdomain.Conversation, error)
	list    func(context.Context, conversationservice.ListParams) ([]*conversationdomain.Conversation, error)
}

func (s stubConversationUseCase) Start(ctx context.Context, params conversationservice.StartParams) (*conversationdomain.Conversation, error) {
	if s.start != nil {
		return s.start(ctx, params)
	}
	return nil, errors.New("unexpected Start call")
}

func (s stubConversationUseCase) Reply(ctx context.Context, params conversationservice.ReplyParams) (*conversationdomain.Conversation, error) {
	if s.reply != nil {
		return s.reply(ctx, params)
	}
	return nil, errors.New("unexpected Reply call")
}

func (s stubConversationUseCase) Confirm(ctx context.Context, id string) (*conversationservice.ConfirmResult, error) {
	if s.confirm != nil {
		return s.confirm(ctx, id)
	}
	return nil, errors.New("unexpected Confirm call")
}

func (s stubConversationUseCase) GetByID(ctx context.Context, id string) (*conversationdomain.Conversation, error) {
	if s.getByID != nil {
		return s.getByID(ctx, id)
	}
	return nil, errors.New("unexpected GetByID call")
}

func (s stubConversationUseCase) List(ctx context.Context, params conversationservice.ListParams) ([]*conversationdomain.Conversation, error) {
	if s.list != nil {
		return s.list(ctx, params)
	}
	return nil, errors.New("unexpected List call")
}
