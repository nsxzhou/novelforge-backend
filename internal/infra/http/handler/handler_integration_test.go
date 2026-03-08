package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	assetdomain "novelforge/backend/internal/domain/asset"
	projectdomain "novelforge/backend/internal/domain/project"
	httpinfra "novelforge/backend/internal/infra/http"
	"novelforge/backend/internal/infra/storage/memory"
	appservice "novelforge/backend/internal/service"
	assetservice "novelforge/backend/internal/service/asset"
	projectservice "novelforge/backend/internal/service/project"
	"novelforge/backend/pkg/config"

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

	listAssets := performRequest(h, consts.MethodGet, "/api/v1/projects/"+createdProject.ID+"/assets?limit=10&offset=0", "")
	assertStatusCode(t, listAssets.Code, consts.StatusOK)

	var assetList assetListResponse
	decodeResponseBody(t, listAssets, &assetList)
	if len(assetList.Assets) != 1 {
		t.Fatalf("len(assets) = %d, want 1", len(assetList.Assets))
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
		)

		recorder := performRequest(h, consts.MethodGet, "/api/v1/assets/11111111-1111-1111-1111-111111111111", "")
		assertStatusCode(t, recorder.Code, consts.StatusInternalServerError)
		assertErrorMessage(t, recorder, "internal server error")
	})
}

func newTestServer() *server.Hertz {
	projectRepo := memory.NewProjectRepository()
	assetRepo := memory.NewAssetRepository()
	projectUseCase := projectservice.NewUseCase(projectservice.Dependencies{Projects: projectRepo})
	assetUseCase := assetservice.NewUseCase(assetservice.Dependencies{
		Assets:   assetRepo,
		Projects: projectRepo,
	})

	return newTestServerWithUseCases(projectUseCase, assetUseCase)
}

func newTestServerWithUseCases(projectUseCase projectservice.UseCase, assetUseCase assetservice.UseCase) *server.Hertz {
	testConfig := config.ServerConfig{
		Host:                "127.0.0.1",
		Port:                18080,
		ReadTimeoutSeconds:  1,
		WriteTimeoutSeconds: 1,
	}

	return httpinfra.NewServer(testConfig, httpinfra.Dependencies{
		Projects: projectUseCase,
		Assets:   assetUseCase,
	})
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
	var requestBody *ut.Body
	if body != "" {
		requestBody = &ut.Body{Body: bytes.NewBufferString(body), Len: len(body)}
	}

	headers := []ut.Header{}
	if body != "" {
		headers = append(headers, ut.Header{Key: "Content-Type", Value: consts.MIMEApplicationJSON})
	}
	return ut.PerformRequest(h.Engine, method, url, requestBody, headers...)
}

func performJSONRequest(h *server.Hertz, method, url, body string) *ut.ResponseRecorder {
	return performRequest(h, method, url, body)
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
