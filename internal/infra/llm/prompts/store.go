package prompts

import (
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

// Store keeps prompt templates keyed by generation kind.
type Store struct {
	templates map[string]*Template
}

// LoadStore loads and validates configured prompt templates.
func LoadStore(cfg config.PromptConfig) (*Store, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate prompts config: %w", err)
	}

	templates := make(map[string]*Template, len(cfg))
	for kind, filename := range cfg {
		tmpl, err := loadTemplate(kind, filename)
		if err != nil {
			return nil, err
		}
		templates[kind] = tmpl
	}

	return &Store{templates: templates}, nil
}

// Get returns the template configured for a generation kind.
func (s *Store) Get(kind string) (*Template, bool) {
	if s == nil {
		return nil, false
	}
	value, ok := s.templates[kind]
	return value, ok
}

func loadTemplate(kind, filename string) (*Template, error) {
	content, err := fs.ReadFile(templateFS, filename)
	if err != nil {
		return nil, fmt.Errorf("read prompt template %q for %q: %w", filename, kind, err)
	}

	var raw fileTemplate
	if err := yaml.Unmarshal(content, &raw); err != nil {
		return nil, fmt.Errorf("unmarshal prompt template %q for %q: %w", filename, kind, err)
	}

	if strings.TrimSpace(raw.System) == "" {
		return nil, fmt.Errorf("prompt template %q for %q field %q must not be empty", filename, kind, "system")
	}
	if strings.TrimSpace(raw.User) == "" {
		return nil, fmt.Errorf("prompt template %q for %q field %q must not be empty", filename, kind, "user")
	}

	systemTemplate, err := texttemplate.New(kind + ":system").Parse(raw.System)
	if err != nil {
		return nil, fmt.Errorf("parse prompt template %q for %q system: %w", filename, kind, err)
	}
	userTemplate, err := texttemplate.New(kind + ":user").Parse(raw.User)
	if err != nil {
		return nil, fmt.Errorf("parse prompt template %q for %q user: %w", filename, kind, err)
	}

	return &Template{
		System:         raw.System,
		User:           raw.User,
		systemTemplate: systemTemplate,
		userTemplate:   userTemplate,
	}, nil
}
