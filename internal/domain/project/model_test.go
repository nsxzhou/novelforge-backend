package project

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func validProject() Project {
	now := time.Date(2026, 3, 6, 12, 0, 0, 0, time.UTC)
	return Project{
		ID:        uuid.NewString(),
		Title:     "Project Title",
		Summary:   "Project summary",
		Status:    StatusDraft,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func TestProjectValidate(t *testing.T) {
	project := validProject()
	if err := project.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestProjectValidateRejectsEmptySummary(t *testing.T) {
	project := validProject()
	project.Summary = "   "

	if err := project.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want error")
	}
}

func TestProjectValidateRejectsUpdatedAtBeforeCreatedAt(t *testing.T) {
	project := validProject()
	project.UpdatedAt = project.CreatedAt.Add(-time.Minute)

	if err := project.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want error")
	}
}
