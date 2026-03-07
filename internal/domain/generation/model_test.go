package generation

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func validGenerationRecord() GenerationRecord {
	now := time.Date(2026, 3, 6, 12, 0, 0, 0, time.UTC)
	return GenerationRecord{
		ID:               uuid.NewString(),
		ProjectID:        uuid.NewString(),
		ChapterID:        uuid.NewString(),
		ConversationID:   uuid.NewString(),
		Kind:             KindChapterGeneration,
		Status:           StatusPending,
		InputSnapshotRef: "snapshot-1",
		OutputRef:        "draft-1",
		TokenUsage:       128,
		DurationMillis:   1500,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
}

func TestGenerationRecordValidate(t *testing.T) {
	record := validGenerationRecord()
	if err := record.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestGenerationRecordValidateRejectsInvalidKind(t *testing.T) {
	record := validGenerationRecord()
	record.Kind = "invalid"

	if err := record.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want error")
	}
}

func TestGenerationRecordValidateRejectsNegativeTokenUsage(t *testing.T) {
	record := validGenerationRecord()
	record.TokenUsage = -1

	if err := record.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want error")
	}
}
