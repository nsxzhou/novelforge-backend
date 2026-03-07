package chapter

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func validChapter() Chapter {
	now := time.Date(2026, 3, 6, 12, 0, 0, 0, time.UTC)
	return Chapter{
		ID:        uuid.NewString(),
		ProjectID: uuid.NewString(),
		Title:     "Chapter 1",
		Ordinal:   1,
		Status:    StatusDraft,
		Content:   "Opening scene",
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func TestChapterValidate(t *testing.T) {
	chapter := validChapter()
	if err := chapter.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestChapterValidateRejectsNonPositiveOrdinal(t *testing.T) {
	chapter := validChapter()
	chapter.Ordinal = 0

	if err := chapter.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want error")
	}
}

func TestChapterValidateRequiresConfirmedMetadata(t *testing.T) {
	chapter := validChapter()
	chapter.Status = StatusConfirmed
	chapter.CurrentDraftID = uuid.NewString()

	if err := chapter.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want error")
	}
}
