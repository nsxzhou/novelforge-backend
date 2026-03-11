package config

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	StorageProviderMemory   = "memory"
	StorageProviderPostgres = "postgres"

	LLMProviderOpenAICompatible = "openai_compatible"
)

// AppConfig holds all runtime configuration for the service.
type AppConfig struct {
	Server  ServerConfig  `yaml:"server"`
	Storage StorageConfig `yaml:"storage"`
	LLM     LLMConfig     `yaml:"llm"`
}

// ServerConfig holds HTTP server related runtime options.
type ServerConfig struct {
	Host                string `yaml:"host"`
	Port                int    `yaml:"port"`
	ReadTimeoutSeconds  int    `yaml:"read_timeout_seconds"`
	WriteTimeoutSeconds int    `yaml:"write_timeout_seconds"`
}

// StorageConfig holds repository provider wiring options.
type StorageConfig struct {
	Provider string         `yaml:"provider"`
	Postgres PostgresConfig `yaml:"postgres"`
}

// PostgresConfig holds PostgreSQL runtime options.
type PostgresConfig struct {
	URLEnv                 string `yaml:"url_env"`
	MaxOpenConns           int    `yaml:"max_open_conns"`
	MaxIdleConns           int    `yaml:"max_idle_conns"`
	ConnMaxLifetimeSeconds int    `yaml:"conn_max_lifetime_seconds"`
}

// PromptCapability 标识一类受支持的 Prompt 能力。
type PromptCapability string

const (
	PromptCapabilityAssetGeneration     PromptCapability = "asset_generation"
	PromptCapabilityChapterGeneration   PromptCapability = "chapter_generation"
	PromptCapabilityChapterContinuation PromptCapability = "chapter_continuation"
	PromptCapabilityChapterRewrite      PromptCapability = "chapter_rewrite"
	PromptCapabilityProjectRefinement   PromptCapability = "project_refinement"
	PromptCapabilityAssetRefinement     PromptCapability = "asset_refinement"
)

// AllPromptCapabilities 返回当前版本支持的全部 Prompt 能力。
func AllPromptCapabilities() []PromptCapability {
	return []PromptCapability{
		PromptCapabilityAssetGeneration,
		PromptCapabilityChapterGeneration,
		PromptCapabilityChapterContinuation,
		PromptCapabilityChapterRewrite,
		PromptCapabilityProjectRefinement,
		PromptCapabilityAssetRefinement,
	}
}

// PromptConfig holds prompt template file names for every supported capability.
type PromptConfig struct {
	AssetGeneration     string `yaml:"asset_generation"`
	ChapterGeneration   string `yaml:"chapter_generation"`
	ChapterContinuation string `yaml:"chapter_continuation"`
	ChapterRewrite      string `yaml:"chapter_rewrite"`
	ProjectRefinement   string `yaml:"project_refinement"`
	AssetRefinement     string `yaml:"asset_refinement"`
}

// LLMConfig holds LLM provider wiring options.
type LLMConfig struct {
	// Env 字段保存的是“环境变量名”，而不是最终 provider/model/base_url 值。
	ProviderEnv    string       `yaml:"provider_env"`
	ModelEnv       string       `yaml:"model_env"`
	BaseURLEnv     string       `yaml:"base_url_env"`
	APIKeyEnv      string       `yaml:"api_key_env"`
	TimeoutSeconds int          `yaml:"timeout_seconds"`
	Prompts        PromptConfig `yaml:"prompts"`
}

// Load loads configuration from YAML file and validates it.
func Load(path string) (*AppConfig, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file %q: %w", path, err)
	}

	cfg := &AppConfig{}
	decoder := yaml.NewDecoder(bytes.NewReader(content))
	decoder.KnownFields(true)
	if err := decoder.Decode(cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config yaml: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Validate fail-fast validates all nested config blocks.
func (c AppConfig) Validate() error {
	if err := c.Server.Validate(); err != nil {
		return fmt.Errorf("invalid server config: %w", err)
	}
	if err := c.Storage.Validate(); err != nil {
		return fmt.Errorf("invalid storage config: %w", err)
	}
	if err := c.LLM.Validate(); err != nil {
		return fmt.Errorf("invalid llm config: %w", err)
	}
	return nil
}

// Validate validates server configuration.
func (c ServerConfig) Validate() error {
	if c.Host == "" {
		return fmt.Errorf("host must not be empty")
	}
	if c.Port <= 0 || c.Port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}
	if c.ReadTimeoutSeconds <= 0 {
		return fmt.Errorf("read_timeout_seconds must be greater than 0")
	}
	if c.WriteTimeoutSeconds <= 0 {
		return fmt.Errorf("write_timeout_seconds must be greater than 0")
	}
	return nil
}

// Address returns server listen address.
func (c ServerConfig) Address() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

// Validate validates storage configuration.
func (c StorageConfig) Validate() error {
	if c.Provider == "" {
		return fmt.Errorf("provider must not be empty")
	}

	switch c.Provider {
	case StorageProviderMemory:
		return nil
	case StorageProviderPostgres:
		if err := c.Postgres.Validate(); err != nil {
			return fmt.Errorf("invalid postgres config: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("provider must be %q or %q", StorageProviderMemory, StorageProviderPostgres)
	}
}

// Validate validates PostgreSQL configuration.
func (c PostgresConfig) Validate() error {
	if c.URLEnv == "" {
		return fmt.Errorf("url_env must not be empty")
	}
	if c.MaxOpenConns < 0 {
		return fmt.Errorf("max_open_conns must be greater than or equal to 0")
	}
	if c.MaxIdleConns < 0 {
		return fmt.Errorf("max_idle_conns must be greater than or equal to 0")
	}
	if c.ConnMaxLifetimeSeconds < 0 {
		return fmt.Errorf("conn_max_lifetime_seconds must be greater than or equal to 0")
	}
	if c.MaxOpenConns > 0 && c.MaxIdleConns > c.MaxOpenConns {
		return fmt.Errorf("max_idle_conns must be less than or equal to max_open_conns")
	}
	return nil
}

// Validate validates prompt configuration.
func (c PromptConfig) Validate() error {
	for _, capability := range AllPromptCapabilities() {
		if strings.TrimSpace(c.FilenameFor(capability)) == "" {
			return fmt.Errorf("%s must not be empty", capability)
		}
	}
	return nil
}

// FilenameFor 返回指定能力对应的模板文件名。
func (c PromptConfig) FilenameFor(capability PromptCapability) string {
	switch capability {
	case PromptCapabilityAssetGeneration:
		return c.AssetGeneration
	case PromptCapabilityChapterGeneration:
		return c.ChapterGeneration
	case PromptCapabilityChapterContinuation:
		return c.ChapterContinuation
	case PromptCapabilityChapterRewrite:
		return c.ChapterRewrite
	case PromptCapabilityProjectRefinement:
		return c.ProjectRefinement
	case PromptCapabilityAssetRefinement:
		return c.AssetRefinement
	default:
		return ""
	}
}

// Validate validates LLM configuration.
func (c LLMConfig) Validate() error {
	if strings.TrimSpace(c.ProviderEnv) == "" {
		return fmt.Errorf("provider_env must not be empty")
	}
	if strings.TrimSpace(c.ModelEnv) == "" {
		return fmt.Errorf("model_env must not be empty")
	}
	if strings.TrimSpace(c.BaseURLEnv) == "" {
		return fmt.Errorf("base_url_env must not be empty")
	}
	if strings.TrimSpace(c.APIKeyEnv) == "" {
		return fmt.Errorf("api_key_env must not be empty")
	}
	if c.TimeoutSeconds <= 0 {
		return fmt.Errorf("timeout_seconds must be greater than 0")
	}
	if err := c.Prompts.Validate(); err != nil {
		return fmt.Errorf("invalid prompts config: %w", err)
	}
	return nil
}
