package prompts

import (
	"bytes"
	"embed"
	"fmt"
	"io/fs"
	"strings"
	texttemplate "text/template"

	"novelforge/backend/pkg/config"

	"gopkg.in/yaml.v3"
)

//go:embed *.yaml
var embeddedTemplates embed.FS

var templateFS fs.FS = embeddedTemplates

type fileTemplate struct {
	System string `yaml:"system"`
	User   string `yaml:"user"`
}

// Template holds a validated prompt template pair for one generation kind.
type Template struct {
	System string
	User   string

	systemTemplate *texttemplate.Template
	userTemplate   *texttemplate.Template
}

// Render executes both system and user templates with the provided data.
func (t *Template) Render(data any) (string, string, error) {
	if t == nil {
		return "", "", fmt.Errorf("template must not be nil")
	}

	system, err := renderTextTemplate(t.systemTemplate, data)
	if err != nil {
		return "", "", fmt.Errorf("render system template: %w", err)
	}
	user, err := renderTextTemplate(t.userTemplate, data)
	if err != nil {
		return "", "", fmt.Errorf("render user template: %w", err)
	}
	return system, user, nil
}

// Store keeps prompt templates keyed by generation kind.
type Store struct {
	templates map[config.PromptCapability]*Template
}

// LoadStore loads and validates configured prompt templates.
func LoadStore(cfg config.PromptConfig) (*Store, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate prompts config: %w", err)
	}

	capabilities := config.AllPromptCapabilities()
	templates := make(map[config.PromptCapability]*Template, len(capabilities))
	for _, capability := range capabilities {
		tmpl, err := loadTemplate(capability, cfg.FilenameFor(capability))
		if err != nil {
			return nil, err
		}
		templates[capability] = tmpl
	}

	return &Store{templates: templates}, nil
}

// TemplateSnapshot 包含一个 capability 的原始模板文本。
type TemplateSnapshot struct {
	System string
	User   string
}

// Get returns the template configured for a generation kind.
func (s *Store) Get(capability config.PromptCapability) (*Template, bool) {
	if s == nil {
		return nil, false
	}
	value, ok := s.templates[capability]
	return value, ok
}

// List 返回所有默认模板的原始文本快照。
func (s *Store) List() map[config.PromptCapability]TemplateSnapshot {
	if s == nil {
		return nil
	}
	result := make(map[config.PromptCapability]TemplateSnapshot, len(s.templates))
	for capability, tmpl := range s.templates {
		result[capability] = TemplateSnapshot{
			System: tmpl.System,
			User:   tmpl.User,
		}
	}
	return result
}

// ParseTemplate 验证并解析模板文本，返回可用于渲染的 Template。
func ParseTemplate(capability, system, user string) (*Template, error) {
	if strings.TrimSpace(system) == "" {
		return nil, fmt.Errorf("system template must not be empty")
	}
	if strings.TrimSpace(user) == "" {
		return nil, fmt.Errorf("user template must not be empty")
	}

	systemTemplate, err := texttemplate.New(capability + ":system").Parse(system)
	if err != nil {
		return nil, fmt.Errorf("parse system template: %w", err)
	}
	userTemplate, err := texttemplate.New(capability + ":user").Parse(user)
	if err != nil {
		return nil, fmt.Errorf("parse user template: %w", err)
	}

	return &Template{
		System:         system,
		User:           user,
		systemTemplate: systemTemplate,
		userTemplate:   userTemplate,
	}, nil
}

func loadTemplate(capability config.PromptCapability, filename string) (*Template, error) {
	content, err := fs.ReadFile(templateFS, filename)
	if err != nil {
		return nil, fmt.Errorf("read prompt template %q for %q: %w", filename, capability, err)
	}

	var raw fileTemplate
	if err := yaml.Unmarshal(content, &raw); err != nil {
		return nil, fmt.Errorf("unmarshal prompt template %q for %q: %w", filename, capability, err)
	}

	if strings.TrimSpace(raw.System) == "" {
		return nil, fmt.Errorf("prompt template %q for %q field %q must not be empty", filename, capability, "system")
	}
	if strings.TrimSpace(raw.User) == "" {
		return nil, fmt.Errorf("prompt template %q for %q field %q must not be empty", filename, capability, "user")
	}

	systemTemplate, err := texttemplate.New(string(capability) + ":system").Parse(raw.System)
	if err != nil {
		return nil, fmt.Errorf("parse prompt template %q for %q system: %w", filename, capability, err)
	}
	userTemplate, err := texttemplate.New(string(capability) + ":user").Parse(raw.User)
	if err != nil {
		return nil, fmt.Errorf("parse prompt template %q for %q user: %w", filename, capability, err)
	}

	return &Template{
		System:         raw.System,
		User:           raw.User,
		systemTemplate: systemTemplate,
		userTemplate:   userTemplate,
	}, nil
}

func renderTextTemplate(tmpl *texttemplate.Template, data any) (string, error) {
	var buffer bytes.Buffer
	if err := tmpl.Execute(&buffer, data); err != nil {
		return "", err
	}
	return buffer.String(), nil
}
