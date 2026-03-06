package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// AppConfig holds all runtime configuration for the service.
type AppConfig struct {
	Server ServerConfig `yaml:"server"`
	LLM    LLMConfig    `yaml:"llm"`
}

// ServerConfig holds HTTP server related runtime options.
type ServerConfig struct {
	Host                string `yaml:"host"`
	Port                int    `yaml:"port"`
	ReadTimeoutSeconds  int    `yaml:"read_timeout_seconds"`
	WriteTimeoutSeconds int    `yaml:"write_timeout_seconds"`
}

// LLMConfig holds LLM provider wiring options.
type LLMConfig struct {
	Provider  string `yaml:"provider"`
	Model     string `yaml:"model"`
	BaseURL   string `yaml:"base_url"`
	APIKeyEnv string `yaml:"api_key_env"`
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

// Validate validates LLM configuration.
func (c LLMConfig) Validate() error {
	if c.Provider == "" {
		return fmt.Errorf("provider must not be empty")
	}
	if c.Model == "" {
		return fmt.Errorf("model must not be empty")
	}
	if c.APIKeyEnv == "" {
		return fmt.Errorf("api_key_env must not be empty")
	}
	return nil
}
