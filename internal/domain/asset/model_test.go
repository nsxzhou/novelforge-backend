package asset

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func validAsset() Asset {
	now := time.Date(2026, 3, 6, 12, 0, 0, 0, time.UTC)
	return Asset{
		ID:        uuid.NewString(),
		ProjectID: uuid.NewString(),
		Type:      TypeOutline,
		Title:     "Outline",
		Content:   "Chapter outline",
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func TestAssetValidate(t *testing.T) {
	asset := validAsset()
	if err := asset.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestAssetValidateRejectsInvalidType(t *testing.T) {
	asset := validAsset()
	asset.Type = "invalid"

	if err := asset.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want error")
	}
}

func TestAssetValidateRejectsEmptyContent(t *testing.T) {
	asset := validAsset()
	asset.Content = ""

	if err := asset.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want error")
	}
}
