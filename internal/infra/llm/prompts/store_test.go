package prompts

import (
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	"novelforge/backend/internal/domain/generation"
	"novelforge/backend/pkg/config"
)

func validPromptConfig() config.PromptConfig {
	return config.PromptConfig{
		generation.KindAssetGeneration:      "asset_generation.yaml",
		generation.KindChapterGeneration:    "chapter_generation.yaml",
		generation.KindChapterContinuation:  "chapter_continuation.yaml",
		generation.KindChapterRewrite:       "chapter_rewrite.yaml",
		"project_refinement":              "project_refinement.yaml",
		"asset_refinement":                "asset_refinement.yaml",
	}
}

func withTemplateFS(t *testing.T, filesystem fs.FS) {
	t.Helper()
	previousFS := templateFS
	templateFS = filesystem
	t.Cleanup(func() {
		templateFS = previousFS
	})
}

func TestLoadStoreSuccess(t *testing.T) {
	withTemplateFS(t, fstest.MapFS{
		"asset_generation.yaml":      {Data: []byte("system: |\n  asset system\nuser: |\n  asset user {{ .Instruction }}\n")},
		"chapter_generation.yaml":    {Data: []byte("system: |\n  chapter system\nuser: |\n  chapter user {{ .OutlineContext }}\n")},
		"chapter_continuation.yaml":  {Data: []byte("system: |\n  continuation system\nuser: |\n  continuation user {{ .CurrentChapterContent }}\n")},
		"chapter_rewrite.yaml":       {Data: []byte("system: |\n  rewrite system\nuser: |\n  rewrite user {{ .TargetText }}\n")},
		"project_refinement.yaml":    {Data: []byte("system: |\n  refinement system\nuser: |\n  refine project {{ .LatestUserMessage }}\n")},
		"asset_refinement.yaml":      {Data: []byte("system: |\n  asset refinement system\nuser: |\n  refine asset {{ .LatestUserMessage }}\n")},
	})

	store, err := LoadStore(validPromptConfig())
	if err != nil {
		t.Fatalf("LoadStore() error = %v", err)
	}

	assetTemplate, ok := store.Get(generation.KindAssetGeneration)
	if !ok {
		t.Fatal("Get(asset_generation) = false, want true")
	}
	if !strings.Contains(assetTemplate.System, "asset system") {
		t.Fatalf("System = %q, want asset template content", assetTemplate.System)
	}
	if assetTemplate.systemTemplate == nil || assetTemplate.userTemplate == nil {
		t.Fatal("LoadStore() did not parse templates")
	}

	if _, ok := store.Get(generation.KindChapterRewrite); !ok {
		t.Fatal("Get(chapter_rewrite) = false, want true")
	}
	projectTemplate, ok := store.Get("project_refinement")
	if !ok {
		t.Fatal("Get(project_refinement) = false, want true")
	}
	renderedSystem, renderedUser, err := projectTemplate.Render(map[string]string{"LatestUserMessage": "polish title"})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if !strings.Contains(renderedSystem, "refinement system") || !strings.Contains(renderedUser, "polish title") {
		t.Fatalf("Render() = (%q, %q), want rendered refinement template", renderedSystem, renderedUser)
	}
}

func TestLoadStoreMissingFile(t *testing.T) {
	withTemplateFS(t, fstest.MapFS{
		"asset_generation.yaml":     {Data: []byte("system: asset system\nuser: asset user\n")},
		"chapter_generation.yaml":   {Data: []byte("system: chapter system\nuser: chapter user\n")},
		"chapter_continuation.yaml": {Data: []byte("system: continuation system\nuser: continuation user\n")},
		"project_refinement.yaml":   {Data: []byte("system: refinement system\nuser: refinement user\n")},
		"asset_refinement.yaml":     {Data: []byte("system: asset refinement system\nuser: asset refinement user\n")},
	})

	_, err := LoadStore(validPromptConfig())
	if err == nil {
		t.Fatal("LoadStore() error = nil, want missing file error")
	}
	if !strings.Contains(err.Error(), "chapter_rewrite.yaml") {
		t.Fatalf("LoadStore() error = %v, want missing file name", err)
	}
}

func TestLoadStoreMissingField(t *testing.T) {
	withTemplateFS(t, fstest.MapFS{
		"asset_generation.yaml":     {Data: []byte("system: asset system\nuser: asset user\n")},
		"chapter_generation.yaml":   {Data: []byte("system: chapter system\nuser: chapter user\n")},
		"chapter_continuation.yaml": {Data: []byte("system: continuation system\nuser: continuation user\n")},
		"chapter_rewrite.yaml":      {Data: []byte("system: rewrite system\n")},
		"project_refinement.yaml":   {Data: []byte("system: refinement system\nuser: refinement user\n")},
		"asset_refinement.yaml":     {Data: []byte("system: asset refinement system\nuser: asset refinement user\n")},
	})

	_, err := LoadStore(validPromptConfig())
	if err == nil {
		t.Fatal("LoadStore() error = nil, want missing field error")
	}
	if !strings.Contains(err.Error(), "field \"user\" must not be empty") {
		t.Fatalf("LoadStore() error = %v, want missing user field error", err)
	}
}

func TestLoadStoreInvalidTemplateSyntax(t *testing.T) {
	withTemplateFS(t, fstest.MapFS{
		"asset_generation.yaml":     {Data: []byte("system: asset system\nuser: asset user\n")},
		"chapter_generation.yaml":   {Data: []byte("system: chapter system\nuser: chapter user\n")},
		"chapter_continuation.yaml": {Data: []byte("system: continuation system\nuser: continuation user\n")},
		"chapter_rewrite.yaml":      {Data: []byte("system: rewrite system\nuser: \"{{ .TargetText\"\n")},
		"project_refinement.yaml":   {Data: []byte("system: refinement system\nuser: refinement user\n")},
		"asset_refinement.yaml":     {Data: []byte("system: asset refinement system\nuser: asset refinement user\n")},
	})

	_, err := LoadStore(validPromptConfig())
	if err == nil {
		t.Fatal("LoadStore() error = nil, want parse error")
	}
	if !strings.Contains(err.Error(), "parse prompt template") {
		t.Fatalf("LoadStore() error = %v, want parse error", err)
	}
}
