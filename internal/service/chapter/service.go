package chapter

import (
	"context"

	assetdomain "inkmuse/backend/internal/domain/asset"
	chapterdomain "inkmuse/backend/internal/domain/chapter"
	generationdomain "inkmuse/backend/internal/domain/generation"
	projectdomain "inkmuse/backend/internal/domain/project"
	promptdomain "inkmuse/backend/internal/domain/prompt"
	"inkmuse/backend/internal/infra/llm"
	"inkmuse/backend/internal/infra/llm/prompts"
	metricservice "inkmuse/backend/internal/service/metric"

	"github.com/cloudwego/eino/schema"
)

// Dependencies 声明章节(chapter)用例所需的领域依赖项。
type Dependencies struct {
	Chapters          chapterdomain.ChapterRepository
	Projects          projectdomain.ProjectRepository
	Assets            assetdomain.AssetRepository
	GenerationRecords generationdomain.GenerationRecordRepository
	LLMClient         llm.Client
	PromptStore       *prompts.Store
	PromptOverrides   promptdomain.OverrideRepository
	Metrics           metricservice.UseCase
}

// GenerateParams 定义生成章节所需参数。
type GenerateParams struct {
	ProjectID   string
	Title       string
	Ordinal     int
	Instruction string
}

// ContinueParams 定义续写章节所需参数。
type ContinueParams struct {
	ChapterID   string
	Instruction string
}

// RewriteParams 定义改写章节所需参数。
type RewriteParams struct {
	ChapterID   string
	Instruction string
	TargetText  string
}

// ConfirmParams 定义确认当前章节草稿所需参数。
type ConfirmParams struct {
	ChapterID   string
	ConfirmedBy string
}

// GenerateResult 定义章节生成结果。
type GenerateResult struct {
	Chapter          *chapterdomain.Chapter
	GenerationRecord *generationdomain.GenerationRecord
}

// ContinueResult 定义章节续写结果。
type ContinueResult struct {
	Chapter          *chapterdomain.Chapter
	GenerationRecord *generationdomain.GenerationRecord
}

// RewriteResult 定义章节改写结果。
type RewriteResult struct {
	Chapter          *chapterdomain.Chapter
	GenerationRecord *generationdomain.GenerationRecord
}

// GenerateStreamResult 定义章节流式生成结果。
type GenerateStreamResult struct {
	ChapterID  string
	RecordID   string
	Record     *generationdomain.GenerationRecord
	Stream     *schema.StreamReader[*schema.Message]
	OnComplete func(content string) (*GenerateResult, error)
	OnError    func(err error)
}

// ContinueStreamResult 定义章节流式续写结果。
type ContinueStreamResult struct {
	Record     *generationdomain.GenerationRecord
	Stream     *schema.StreamReader[*schema.Message]
	OnComplete func(content string) (*ContinueResult, error)
	OnError    func(err error)
}

// RewriteStreamResult 定义章节流式改写结果。
type RewriteStreamResult struct {
	Record     *generationdomain.GenerationRecord
	Stream     *schema.StreamReader[*schema.Message]
	OnComplete func(content string) (*RewriteResult, error)
	OnError    func(err error)
}

// UseCase 定义章节(chapter)的应用边界。
type UseCase interface {
	Create(ctx context.Context, chapter *chapterdomain.Chapter) error
	GetByID(ctx context.Context, id string) (*chapterdomain.Chapter, error)
	ListByProject(ctx context.Context, params chapterdomain.ListByProjectParams) ([]*chapterdomain.Chapter, error)
	Update(ctx context.Context, chapter *chapterdomain.Chapter) error
	Generate(ctx context.Context, params GenerateParams) (*GenerateResult, error)
	Continue(ctx context.Context, params ContinueParams) (*ContinueResult, error)
	Rewrite(ctx context.Context, params RewriteParams) (*RewriteResult, error)
	Confirm(ctx context.Context, params ConfirmParams) (*chapterdomain.Chapter, error)
	GenerateStream(ctx context.Context, params GenerateParams) (*GenerateStreamResult, error)
	ContinueStream(ctx context.Context, params ContinueParams) (*ContinueStreamResult, error)
	RewriteStream(ctx context.Context, params RewriteParams) (*RewriteStreamResult, error)
}
