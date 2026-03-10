package postgres

import (
	"context"
	"database/sql"
	"regexp"
	"testing"
	"time"

	assetdomain "novelforge/backend/internal/domain/asset"
	chapterdomain "novelforge/backend/internal/domain/chapter"
	conversationdomain "novelforge/backend/internal/domain/conversation"
	generationdomain "novelforge/backend/internal/domain/generation"
	metricdomain "novelforge/backend/internal/domain/metric"
	projectdomain "novelforge/backend/internal/domain/project"
	"novelforge/backend/internal/infra/storage/shared"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
)

func postgresTestTime() time.Time {
	return time.Date(2026, 3, 7, 12, 0, 0, 0, time.UTC)
}

func newTestProjectEntity(now time.Time) *projectdomain.Project {
	return &projectdomain.Project{
		ID:        uuid.NewString(),
		Title:     "Novel",
		Summary:   "Summary",
		Status:    projectdomain.StatusDraft,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func newTestAssetEntity(now time.Time) *assetdomain.Asset {
	return &assetdomain.Asset{
		ID:        uuid.NewString(),
		ProjectID: uuid.NewString(),
		Type:      assetdomain.TypeOutline,
		Title:     "Outline",
		Content:   "Body",
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func TestProjectRepositoryCreate(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	repo := NewProjectRepository(db)
	entity := newTestProjectEntity(postgresTestTime())
	mock.ExpectExec(`INSERT INTO projects \(id, title, summary, status, created_at, updated_at\)\s+VALUES \(\$1, \$2, \$3, \$4, \$5, \$6\)`).
		WithArgs(entity.ID, entity.Title, entity.Summary, entity.Status, entity.CreatedAt, entity.UpdatedAt).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := repo.Create(context.Background(), entity); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations not met: %v", err)
	}
}

func TestProjectRepositoryGetByID(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	repo := NewProjectRepository(db)
	entity := newTestProjectEntity(postgresTestTime())
	rows := sqlmock.NewRows([]string{"id", "title", "summary", "status", "created_at", "updated_at"}).
		AddRow(entity.ID, entity.Title, entity.Summary, entity.Status, entity.CreatedAt, entity.UpdatedAt)
	mock.ExpectQuery(`SELECT id, title, summary, status, created_at, updated_at\s+FROM projects\s+WHERE id = \$1`).
		WithArgs(entity.ID).
		WillReturnRows(rows)

	got, err := repo.GetByID(context.Background(), entity.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if got.ID != entity.ID || got.Title != entity.Title || got.Status != entity.Status {
		t.Fatalf("GetByID() = %#v, want %#v", got, entity)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations not met: %v", err)
	}
}

func TestProjectRepositoryGetByIDMapsNotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	repo := NewProjectRepository(db)
	mock.ExpectQuery(regexp.QuoteMeta(`
		SELECT id, title, summary, status, created_at, updated_at
		FROM projects
		WHERE id = $1
	`)).WithArgs("missing").WillReturnError(sql.ErrNoRows)

	_, err = repo.GetByID(context.Background(), "missing")
	if err != shared.ErrNotFound {
		t.Fatalf("GetByID() error = %v, want %v", err, shared.ErrNotFound)
	}
}

func TestProjectRepositoryListByStatus(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	repo := NewProjectRepository(db)
	first := newTestProjectEntity(postgresTestTime())
	second := newTestProjectEntity(postgresTestTime().Add(time.Minute))
	second.Title = "Novel 2"
	second.Summary = "Summary 2"
	rows := sqlmock.NewRows([]string{"id", "title", "summary", "status", "created_at", "updated_at"}).
		AddRow(first.ID, first.Title, first.Summary, first.Status, first.CreatedAt, first.UpdatedAt).
		AddRow(second.ID, second.Title, second.Summary, second.Status, second.CreatedAt, second.UpdatedAt)
	mock.ExpectQuery(`SELECT id, title, summary, status, created_at, updated_at\s+FROM projects\s+WHERE status = \$1 ORDER BY created_at ASC, id ASC LIMIT \$2 OFFSET \$3`).
		WithArgs(projectdomain.StatusDraft, 10, 0).
		WillReturnRows(rows)

	items, err := repo.List(context.Background(), projectdomain.ListParams{Status: projectdomain.StatusDraft, Limit: 10, Offset: 0})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(items) != 2 || items[0].ID != first.ID || items[1].ID != second.ID {
		t.Fatalf("List() = %#v, want ordered draft projects", items)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations not met: %v", err)
	}
}

func TestProjectRepositoryUpdate(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	repo := NewProjectRepository(db)
	entity := newTestProjectEntity(postgresTestTime())
	entity.Title = "Novel Revised"
	entity.Summary = "Updated summary"
	entity.Status = projectdomain.StatusActive
	entity.UpdatedAt = entity.CreatedAt.Add(time.Minute)
	mock.ExpectExec(`UPDATE projects\s+SET title = \$2, summary = \$3, status = \$4, updated_at = \$5\s+WHERE id = \$1`).
		WithArgs(entity.ID, entity.Title, entity.Summary, entity.Status, entity.UpdatedAt).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := repo.Update(context.Background(), entity); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations not met: %v", err)
	}
}

func TestProjectRepositoryUpdateMapsNotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	repo := NewProjectRepository(db)
	entity := newTestProjectEntity(postgresTestTime())
	entity.UpdatedAt = entity.CreatedAt.Add(time.Minute)
	mock.ExpectExec(`UPDATE projects\s+SET title = \$2, summary = \$3, status = \$4, updated_at = \$5\s+WHERE id = \$1`).
		WithArgs(entity.ID, entity.Title, entity.Summary, entity.Status, entity.UpdatedAt).
		WillReturnResult(sqlmock.NewResult(0, 0))

	err = repo.Update(context.Background(), entity)
	if err != shared.ErrNotFound {
		t.Fatalf("Update() error = %v, want %v", err, shared.ErrNotFound)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations not met: %v", err)
	}
}

func TestProjectRepositoryUpdateIfUnchanged(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	repo := NewProjectRepository(db)
	now := postgresTestTime()
	entity := newTestProjectEntity(now)
	entity.Title = "Novel Revised"
	entity.Summary = "Updated summary"
	entity.Status = projectdomain.StatusActive
	entity.UpdatedAt = now.Add(time.Minute)

	mock.ExpectExec(regexp.QuoteMeta(`
		UPDATE projects
		SET title = $2, summary = $3, status = $4, updated_at = $5
		WHERE id = $1 AND updated_at = $6
	`)).WithArgs(entity.ID, entity.Title, entity.Summary, entity.Status, entity.UpdatedAt, now).WillReturnResult(sqlmock.NewResult(0, 1))

	updated, err := repo.UpdateIfUnchanged(context.Background(), entity, now)
	if err != nil {
		t.Fatalf("UpdateIfUnchanged() error = %v", err)
	}
	if !updated {
		t.Fatal("UpdateIfUnchanged() updated = false, want true")
	}

	mock.ExpectExec(regexp.QuoteMeta(`
		UPDATE projects
		SET title = $2, summary = $3, status = $4, updated_at = $5
		WHERE id = $1 AND updated_at = $6
	`)).WithArgs(entity.ID, entity.Title, entity.Summary, entity.Status, entity.UpdatedAt, now.Add(2*time.Minute)).WillReturnResult(sqlmock.NewResult(0, 0))

	updated, err = repo.UpdateIfUnchanged(context.Background(), entity, now.Add(2*time.Minute))
	if err != nil {
		t.Fatalf("UpdateIfUnchanged() stale error = %v", err)
	}
	if updated {
		t.Fatal("UpdateIfUnchanged() updated = true, want false")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations not met: %v", err)
	}
}

func TestProjectRepositoryCreateMapsAlreadyExists(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	repo := NewProjectRepository(db)
	now := postgresTestTime()
	entity := &projectdomain.Project{ID: uuid.NewString(), Title: "Novel", Summary: "Summary", Status: projectdomain.StatusDraft, CreatedAt: now, UpdatedAt: now}
	mock.ExpectExec(regexp.QuoteMeta(`
		INSERT INTO projects (id, title, summary, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`)).WithArgs(entity.ID, entity.Title, entity.Summary, entity.Status, entity.CreatedAt, entity.UpdatedAt).WillReturnError(testSQLStateError{state: "23505"})

	err = repo.Create(context.Background(), entity)
	if err != shared.ErrAlreadyExists {
		t.Fatalf("Create() error = %v, want %v", err, shared.ErrAlreadyExists)
	}
}

func TestAssetRepositoryCreate(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	repo := NewAssetRepository(db)
	entity := newTestAssetEntity(postgresTestTime())
	mock.ExpectExec(`INSERT INTO assets \(id, project_id, type, title, content, created_at, updated_at\)\s+VALUES \(\$1, \$2, \$3, \$4, \$5, \$6, \$7\)`).
		WithArgs(entity.ID, entity.ProjectID, entity.Type, entity.Title, entity.Content, entity.CreatedAt, entity.UpdatedAt).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := repo.Create(context.Background(), entity); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations not met: %v", err)
	}
}

func TestAssetRepositoryGetByID(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	repo := NewAssetRepository(db)
	entity := newTestAssetEntity(postgresTestTime())
	rows := sqlmock.NewRows([]string{"id", "project_id", "type", "title", "content", "created_at", "updated_at"}).
		AddRow(entity.ID, entity.ProjectID, entity.Type, entity.Title, entity.Content, entity.CreatedAt, entity.UpdatedAt)
	mock.ExpectQuery(`SELECT id, project_id, type, title, content, created_at, updated_at\s+FROM assets\s+WHERE id = \$1`).
		WithArgs(entity.ID).
		WillReturnRows(rows)

	got, err := repo.GetByID(context.Background(), entity.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if got.ID != entity.ID || got.ProjectID != entity.ProjectID || got.Type != entity.Type {
		t.Fatalf("GetByID() = %#v, want %#v", got, entity)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations not met: %v", err)
	}
}

func TestAssetRepositoryGetByIDMapsNotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	repo := NewAssetRepository(db)
	mock.ExpectQuery(`SELECT id, project_id, type, title, content, created_at, updated_at\s+FROM assets\s+WHERE id = \$1`).
		WithArgs("missing").
		WillReturnError(sql.ErrNoRows)

	_, err = repo.GetByID(context.Background(), "missing")
	if err != shared.ErrNotFound {
		t.Fatalf("GetByID() error = %v, want %v", err, shared.ErrNotFound)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations not met: %v", err)
	}
}

func TestAssetRepositoryListByProject(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	repo := NewAssetRepository(db)
	projectID := uuid.NewString()
	first := newTestAssetEntity(postgresTestTime())
	first.ProjectID = projectID
	second := newTestAssetEntity(postgresTestTime().Add(time.Minute))
	second.ProjectID = projectID
	second.Type = assetdomain.TypeCharacter
	second.Title = "Hero"
	second.Content = "Character sheet"
	rows := sqlmock.NewRows([]string{"id", "project_id", "type", "title", "content", "created_at", "updated_at"}).
		AddRow(first.ID, first.ProjectID, first.Type, first.Title, first.Content, first.CreatedAt, first.UpdatedAt).
		AddRow(second.ID, second.ProjectID, second.Type, second.Title, second.Content, second.CreatedAt, second.UpdatedAt)
	mock.ExpectQuery(`SELECT id, project_id, type, title, content, created_at, updated_at\s+FROM assets\s+WHERE project_id = \$1\s+ORDER BY created_at ASC, id ASC LIMIT \$2 OFFSET \$3`).
		WithArgs(projectID, 10, 0).
		WillReturnRows(rows)

	items, err := repo.ListByProject(context.Background(), assetdomain.ListByProjectParams{ProjectID: projectID, Limit: 10, Offset: 0})
	if err != nil {
		t.Fatalf("ListByProject() error = %v", err)
	}
	if len(items) != 2 || items[0].ID != first.ID || items[1].ID != second.ID {
		t.Fatalf("ListByProject() = %#v, want ordered assets", items)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations not met: %v", err)
	}
}

func TestAssetRepositoryListByProjectAndType(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	repo := NewAssetRepository(db)
	projectID := uuid.NewString()
	createdAt := postgresTestTime()
	updatedAt := createdAt.Add(time.Minute)
	rows := sqlmock.NewRows([]string{"id", "project_id", "type", "title", "content", "created_at", "updated_at"}).
		AddRow(uuid.NewString(), projectID, assetdomain.TypeOutline, "Outline", "Body", createdAt, updatedAt)
	mock.ExpectQuery(regexp.QuoteMeta(`
		SELECT id, project_id, type, title, content, created_at, updated_at
		FROM assets
		WHERE project_id = $1 AND type = $2
		ORDER BY created_at ASC, id ASC
	 LIMIT $3 OFFSET $4`)).WithArgs(projectID, assetdomain.TypeOutline, 10, 0).WillReturnRows(rows)

	items, err := repo.ListByProjectAndType(context.Background(), assetdomain.ListByProjectAndTypeParams{ProjectID: projectID, Type: assetdomain.TypeOutline, Limit: 10, Offset: 0})
	if err != nil {
		t.Fatalf("ListByProjectAndType() error = %v", err)
	}
	if len(items) != 1 || items[0].Type != assetdomain.TypeOutline {
		t.Fatalf("ListByProjectAndType() = %#v, want one outline asset", items)
	}
}

func TestAssetRepositoryUpdate(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	repo := NewAssetRepository(db)
	entity := newTestAssetEntity(postgresTestTime())
	entity.Type = assetdomain.TypeCharacter
	entity.Title = "Hero"
	entity.Content = "Character sheet"
	entity.UpdatedAt = entity.CreatedAt.Add(time.Minute)
	mock.ExpectExec(`UPDATE assets\s+SET type = \$2, title = \$3, content = \$4, updated_at = \$5\s+WHERE id = \$1`).
		WithArgs(entity.ID, entity.Type, entity.Title, entity.Content, entity.UpdatedAt).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := repo.Update(context.Background(), entity); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations not met: %v", err)
	}
}

func TestAssetRepositoryUpdateMapsNotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	repo := NewAssetRepository(db)
	entity := newTestAssetEntity(postgresTestTime())
	entity.UpdatedAt = entity.CreatedAt.Add(time.Minute)
	mock.ExpectExec(`UPDATE assets\s+SET type = \$2, title = \$3, content = \$4, updated_at = \$5\s+WHERE id = \$1`).
		WithArgs(entity.ID, entity.Type, entity.Title, entity.Content, entity.UpdatedAt).
		WillReturnResult(sqlmock.NewResult(0, 0))

	err = repo.Update(context.Background(), entity)
	if err != shared.ErrNotFound {
		t.Fatalf("Update() error = %v, want %v", err, shared.ErrNotFound)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations not met: %v", err)
	}
}

func TestAssetRepositoryUpdateIfUnchanged(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	repo := NewAssetRepository(db)
	now := postgresTestTime()
	entity := newTestAssetEntity(now)
	entity.Type = assetdomain.TypeCharacter
	entity.Title = "Hero"
	entity.Content = "Character sheet"
	entity.UpdatedAt = now.Add(time.Minute)

	mock.ExpectExec(regexp.QuoteMeta(`
		UPDATE assets
		SET type = $2, title = $3, content = $4, updated_at = $5
		WHERE id = $1 AND updated_at = $6
	`)).WithArgs(entity.ID, entity.Type, entity.Title, entity.Content, entity.UpdatedAt, now).WillReturnResult(sqlmock.NewResult(0, 1))

	updated, err := repo.UpdateIfUnchanged(context.Background(), entity, now)
	if err != nil {
		t.Fatalf("UpdateIfUnchanged() error = %v", err)
	}
	if !updated {
		t.Fatal("UpdateIfUnchanged() updated = false, want true")
	}

	mock.ExpectExec(regexp.QuoteMeta(`
		UPDATE assets
		SET type = $2, title = $3, content = $4, updated_at = $5
		WHERE id = $1 AND updated_at = $6
	`)).WithArgs(entity.ID, entity.Type, entity.Title, entity.Content, entity.UpdatedAt, now.Add(2*time.Minute)).WillReturnResult(sqlmock.NewResult(0, 0))

	updated, err = repo.UpdateIfUnchanged(context.Background(), entity, now.Add(2*time.Minute))
	if err != nil {
		t.Fatalf("UpdateIfUnchanged() stale error = %v", err)
	}
	if updated {
		t.Fatal("UpdateIfUnchanged() updated = true, want false")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations not met: %v", err)
	}
}

func TestAssetRepositoryDelete(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	repo := NewAssetRepository(db)
	assetID := uuid.NewString()
	mock.ExpectExec(`DELETE FROM assets WHERE id = \$1`).
		WithArgs(assetID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := repo.Delete(context.Background(), assetID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations not met: %v", err)
	}
}

func TestAssetRepositoryDeleteMapsNotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	repo := NewAssetRepository(db)
	assetID := uuid.NewString()
	mock.ExpectExec(`DELETE FROM assets WHERE id = \$1`).
		WithArgs(assetID).
		WillReturnResult(sqlmock.NewResult(0, 0))

	err = repo.Delete(context.Background(), assetID)
	if err != shared.ErrNotFound {
		t.Fatalf("Delete() error = %v, want %v", err, shared.ErrNotFound)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations not met: %v", err)
	}
}

func TestChapterRepositoryRoundTripOptionalFields(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	repo := NewChapterRepository(db)
	now := postgresTestTime()
	confirmedAt := now.Add(time.Minute)
	confirmedBy := uuid.NewString()
	entity := &chapterdomain.Chapter{
		ID:                      uuid.NewString(),
		ProjectID:               uuid.NewString(),
		Title:                   "Chapter 1",
		Ordinal:                 1,
		Status:                  chapterdomain.StatusDraft,
		Content:                 "Body",
		CurrentDraftID:          uuid.NewString(),
		CurrentDraftConfirmedAt: &confirmedAt,
		CurrentDraftConfirmedBy: confirmedBy,
		CreatedAt:               now,
		UpdatedAt:               now,
	}
	mock.ExpectExec(regexp.QuoteMeta(`
		INSERT INTO chapters (
			id, project_id, title, ordinal, status, content,
			current_draft_id, current_draft_confirmed_at, current_draft_confirmed_by,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`)).WithArgs(entity.ID, entity.ProjectID, entity.Title, entity.Ordinal, entity.Status, entity.Content, sql.NullString{String: entity.CurrentDraftID, Valid: true}, sql.NullTime{Time: confirmedAt, Valid: true}, sql.NullString{String: confirmedBy, Valid: true}, entity.CreatedAt, entity.UpdatedAt).WillReturnResult(sqlmock.NewResult(0, 1))
	if err := repo.Create(context.Background(), entity); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	rows := sqlmock.NewRows([]string{"id", "project_id", "title", "ordinal", "status", "content", "current_draft_id", "current_draft_confirmed_at", "current_draft_confirmed_by", "created_at", "updated_at"}).
		AddRow(entity.ID, entity.ProjectID, entity.Title, entity.Ordinal, entity.Status, entity.Content, entity.CurrentDraftID, confirmedAt, confirmedBy, entity.CreatedAt, entity.UpdatedAt)
	mock.ExpectQuery(regexp.QuoteMeta(`
		SELECT id, project_id, title, ordinal, status, content,
			current_draft_id, current_draft_confirmed_at, current_draft_confirmed_by,
			created_at, updated_at
		FROM chapters
		WHERE id = $1
	`)).WithArgs(entity.ID).WillReturnRows(rows)

	got, err := repo.GetByID(context.Background(), entity.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if got.CurrentDraftConfirmedBy != confirmedBy || got.CurrentDraftConfirmedAt == nil || !got.CurrentDraftConfirmedAt.Equal(confirmedAt) {
		t.Fatalf("GetByID() = %#v, want confirmed fields populated", got)
	}
}

func TestChapterRepositoryUpdateIfUnchanged(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	repo := NewChapterRepository(db)
	now := postgresTestTime()
	entity := &chapterdomain.Chapter{
		ID:             uuid.NewString(),
		ProjectID:      uuid.NewString(),
		Title:          "Chapter 1",
		Ordinal:        1,
		Status:         chapterdomain.StatusDraft,
		Content:        "Body",
		CurrentDraftID: uuid.NewString(),
		CreatedAt:      now,
		UpdatedAt:      now.Add(time.Minute),
	}

	mock.ExpectExec(regexp.QuoteMeta(`
		UPDATE chapters
		SET project_id = $2, title = $3, ordinal = $4, status = $5, content = $6,
			current_draft_id = $7, current_draft_confirmed_at = $8, current_draft_confirmed_by = $9,
			updated_at = $10
		WHERE id = $1 AND updated_at = $11
	`)).WithArgs(
		entity.ID,
		entity.ProjectID,
		entity.Title,
		entity.Ordinal,
		entity.Status,
		entity.Content,
		sql.NullString{String: entity.CurrentDraftID, Valid: true},
		sql.NullTime{},
		sql.NullString{},
		entity.UpdatedAt,
		now,
	).WillReturnResult(sqlmock.NewResult(0, 1))

	updated, err := repo.UpdateIfUnchanged(context.Background(), entity, now)
	if err != nil {
		t.Fatalf("UpdateIfUnchanged() error = %v", err)
	}
	if !updated {
		t.Fatal("UpdateIfUnchanged() updated = false, want true")
	}

	mock.ExpectExec(regexp.QuoteMeta(`
		UPDATE chapters
		SET project_id = $2, title = $3, ordinal = $4, status = $5, content = $6,
			current_draft_id = $7, current_draft_confirmed_at = $8, current_draft_confirmed_by = $9,
			updated_at = $10
		WHERE id = $1 AND updated_at = $11
	`)).WithArgs(
		entity.ID,
		entity.ProjectID,
		entity.Title,
		entity.Ordinal,
		entity.Status,
		entity.Content,
		sql.NullString{String: entity.CurrentDraftID, Valid: true},
		sql.NullTime{},
		sql.NullString{},
		entity.UpdatedAt,
		now.Add(2*time.Minute),
	).WillReturnResult(sqlmock.NewResult(0, 0))

	updated, err = repo.UpdateIfUnchanged(context.Background(), entity, now.Add(2*time.Minute))
	if err != nil {
		t.Fatalf("UpdateIfUnchanged() stale error = %v", err)
	}
	if updated {
		t.Fatal("UpdateIfUnchanged() updated = true, want false")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations not met: %v", err)
	}
}

func TestConversationRepositoryAppendMessage(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	repo := NewConversationRepository(db)
	now := postgresTestTime()
	conversationID := uuid.NewString()
	projectID := uuid.NewString()
	targetID := uuid.NewString()
	message1 := conversationdomain.Message{ID: uuid.NewString(), Role: conversationdomain.MessageRoleUser, Content: "Hello", CreatedAt: now}
	message2 := conversationdomain.Message{ID: uuid.NewString(), Role: conversationdomain.MessageRoleAssistant, Content: "Draft ready", CreatedAt: now.Add(time.Minute)}
	rows := sqlmock.NewRows([]string{"id", "project_id", "target_type", "target_id", "messages", "pending_suggestion", "created_at", "updated_at"}).
		AddRow(conversationID, projectID, conversationdomain.TargetTypeProject, targetID, `[{"id":"`+message1.ID+`","role":"user","content":"Hello","created_at":"2026-03-07T12:00:00Z"}]`, `null`, now, now)
	mock.ExpectQuery(regexp.QuoteMeta(`
		SELECT id, project_id, target_type, target_id, messages, pending_suggestion, created_at, updated_at
		FROM conversations
		WHERE id = $1
	`)).WithArgs(conversationID).WillReturnRows(rows)
	mock.ExpectExec(regexp.QuoteMeta(`
		UPDATE conversations
		SET messages = $2::jsonb, updated_at = $3
		WHERE id = $1
	`)).WithArgs(conversationID, sqlmock.AnyArg(), message2.CreatedAt).WillReturnResult(sqlmock.NewResult(0, 1))

	if err := repo.AppendMessage(context.Background(), conversationdomain.AppendMessageParams{ConversationID: conversationID, Message: message2}); err != nil {
		t.Fatalf("AppendMessage() error = %v", err)
	}
}

func TestConversationRepositoryUpdateIfUnchanged(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	repo := NewConversationRepository(db)
	now := postgresTestTime()
	entity := &conversationdomain.Conversation{
		ID:         uuid.NewString(),
		ProjectID:  uuid.NewString(),
		TargetType: conversationdomain.TargetTypeProject,
		TargetID:   uuid.NewString(),
		Messages: []conversationdomain.Message{
			{
				ID:        uuid.NewString(),
				Role:      conversationdomain.MessageRoleUser,
				Content:   "Hello",
				CreatedAt: now,
			},
		},
		CreatedAt: now,
		UpdatedAt: now.Add(time.Minute),
	}

	mock.ExpectExec(regexp.QuoteMeta(`
		UPDATE conversations
		SET messages = $2::jsonb, pending_suggestion = $3::jsonb, updated_at = $4
		WHERE id = $1 AND updated_at = $5
	`)).WithArgs(entity.ID, sqlmock.AnyArg(), sqlmock.AnyArg(), entity.UpdatedAt, now).WillReturnResult(sqlmock.NewResult(0, 1))

	updated, err := repo.UpdateIfUnchanged(context.Background(), entity, now)
	if err != nil {
		t.Fatalf("UpdateIfUnchanged() error = %v", err)
	}
	if !updated {
		t.Fatal("UpdateIfUnchanged() updated = false, want true")
	}

	mock.ExpectExec(regexp.QuoteMeta(`
		UPDATE conversations
		SET messages = $2::jsonb, pending_suggestion = $3::jsonb, updated_at = $4
		WHERE id = $1 AND updated_at = $5
	`)).WithArgs(entity.ID, sqlmock.AnyArg(), sqlmock.AnyArg(), entity.UpdatedAt, now.Add(2*time.Minute)).WillReturnResult(sqlmock.NewResult(0, 0))

	updated, err = repo.UpdateIfUnchanged(context.Background(), entity, now.Add(2*time.Minute))
	if err != nil {
		t.Fatalf("UpdateIfUnchanged() stale error = %v", err)
	}
	if updated {
		t.Fatal("UpdateIfUnchanged() updated = true, want false")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations not met: %v", err)
	}
}

func TestGenerationRecordRepositoryUpdateStatus(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	repo := NewGenerationRecordRepository(db)
	updatedAt := postgresTestTime().Add(time.Minute)
	mock.ExpectExec(regexp.QuoteMeta(`
		UPDATE generation_records
		SET status = $2, output_ref = $3, token_usage = $4, duration_millis = $5,
			error_message = $6, updated_at = $7
		WHERE id = $1
	`)).WithArgs("record-1", generationdomain.StatusSucceeded, "draft-1", 128, int64(900), "", updatedAt).WillReturnResult(sqlmock.NewResult(0, 1))

	err = repo.UpdateStatus(context.Background(), generationdomain.UpdateStatusParams{
		ID:             "record-1",
		Status:         generationdomain.StatusSucceeded,
		OutputRef:      "draft-1",
		TokenUsage:     128,
		DurationMillis: 900,
		UpdatedAt:      updatedAt,
	})
	if err != nil {
		t.Fatalf("UpdateStatus() error = %v", err)
	}
}

func TestMetricEventRepositoryListByProject(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	repo := NewMetricEventRepository(db)
	projectID := uuid.NewString()
	chapterID := uuid.NewString()
	occurredAt := postgresTestTime()
	rows := sqlmock.NewRows([]string{"id", "event_name", "project_id", "chapter_id", "labels", "stats", "occurred_at"}).
		AddRow(uuid.NewString(), metricdomain.EventChapterGenerated, projectID, chapterID, `{"source":"test"}`, `{"token_usage":128}`, occurredAt)
	mock.ExpectQuery(regexp.QuoteMeta(`
		SELECT id, event_name, project_id, chapter_id, labels, stats, occurred_at
		FROM metric_events
		WHERE project_id = $1 AND event_name = $2 ORDER BY occurred_at ASC, id ASC LIMIT $3 OFFSET $4`)).WithArgs(projectID, metricdomain.EventChapterGenerated, 10, 0).WillReturnRows(rows)

	items, err := repo.ListByProject(context.Background(), metricdomain.ListByProjectParams{ProjectID: projectID, EventName: metricdomain.EventChapterGenerated, Limit: 10, Offset: 0})
	if err != nil {
		t.Fatalf("ListByProject() error = %v", err)
	}
	if len(items) != 1 || items[0].ChapterID != chapterID {
		t.Fatalf("ListByProject() = %#v, want one metric event", items)
	}
}
