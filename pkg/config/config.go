package config

import (
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

// PromptConfig maps generation kinds to prompt template file names.
type PromptConfig map[string]string

// LLMConfig holds LLM provider wiring options.
type LLMConfig struct {
	Provider       string       `yaml:"provider"`
	Model          string       `yaml:"model"`
	BaseURL        string       `yaml:"base_url"`
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
	if err := yaml.Unmarshal(content, cfg); err != nil {
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
	requiredKinds := []string{
		"asset_generation",
		"chapter_generation",
	}
	allowedKinds := map[string]struct{}{
		"asset_generation":     {},
		"chapter_generation":   {},
		"chapter_continuation": {},
		"chapter_rewrite":      {},
	}

	for _, kind := range requiredKinds {
		filename, ok := c[kind]
		if !ok || strings.TrimSpace(filename) == "" {
			return fmt.Errorf("%s must not be empty", kind)
		}
	}

	for kind, filename := range c {
		if strings.TrimSpace(kind) == "" {
			return fmt.Errorf("prompt kind must not be empty")
		}
		if _, ok := allowedKinds[kind]; !ok {
			return fmt.Errorf("%q is not a supported prompt kind", kind)
		}
		if strings.TrimSpace(filename) == "" {
			return fmt.Errorf("%s must not be empty", kind)
		}
	}

	return nil
}

// Validate validates LLM configuration.
func (c LLMConfig) Validate() error {
	if c.Provider == "" {
		return fmt.Errorf("provider must not be empty")
	}

	switch c.Provider {
	case LLMProviderOpenAICompatible:
		if strings.TrimSpace(c.Model) == "" {
			return fmt.Errorf("model must not be empty")
		}
		if strings.TrimSpace(c.BaseURL) == "" {
			return fmt.Errorf("base_url must not be empty")
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
	default:
		return fmt.Errorf("provider must be %q", LLMProviderOpenAICompatible)
	}
}
