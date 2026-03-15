package llm

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	openaimodel "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// Registry manages multiple LLM providers with failover and implements the Client interface.
type Registry struct {
	mu      sync.RWMutex
	clients []*registeredClient // sorted by priority
}

type registeredClient struct {
	config ProviderConfig
	client Client // the actual llm.Client
}

// NewRegistry creates a Registry from a list of provider configurations.
func NewRegistry(configs []ProviderConfig) (*Registry, error) {
	r := &Registry{}
	for _, cfg := range configs {
		if err := r.AddProvider(cfg); err != nil {
			return nil, fmt.Errorf("add provider %q: %w", cfg.ID, err)
		}
	}
	return r, nil
}

// Provider returns the provider name of the first enabled client.
func (r *Registry) Provider() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, rc := range r.clients {
		if rc.config.Enabled {
			return rc.client.Provider()
		}
	}
	return ""
}

// Model returns the model name of the first enabled client.
func (r *Registry) Model() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, rc := range r.clients {
		if rc.config.Enabled {
			return rc.client.Model()
		}
	}
	return ""
}

// ChatModel returns a failoverChatModel that tries providers in priority order.
func (r *Registry) ChatModel() model.ToolCallingChatModel {
	return &failoverChatModel{registry: r}
}

// AddProvider adds a new provider to the registry.
func (r *Registry) AddProvider(cfg ProviderConfig) error {
	if cfg.ID == "" {
		return errors.New("provider id must not be empty")
	}
	if cfg.Provider == "" {
		return errors.New("provider type must not be empty")
	}
	if cfg.Model == "" {
		return errors.New("model must not be empty")
	}
	if cfg.BaseURL == "" {
		return errors.New("base_url must not be empty")
	}
	if cfg.APIKey == "" {
		return errors.New("api_key must not be empty")
	}
	if cfg.TimeoutSec <= 0 {
		cfg.TimeoutSec = 60
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Check for duplicate ID.
	for _, rc := range r.clients {
		if rc.config.ID == cfg.ID {
			return fmt.Errorf("provider %q already exists", cfg.ID)
		}
	}

	client, err := buildClient(cfg)
	if err != nil {
		return fmt.Errorf("build client for provider %q: %w", cfg.ID, err)
	}

	r.clients = append(r.clients, &registeredClient{config: cfg, client: client})
	r.sortClientsLocked()
	return nil
}

// RemoveProvider removes a provider from the registry by ID.
func (r *Registry) RemoveProvider(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	idx := -1
	for i, rc := range r.clients {
		if rc.config.ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		return fmt.Errorf("provider %q not found", id)
	}

	// Don't allow removing the last enabled provider.
	if r.clients[idx].config.Enabled {
		enabledCount := 0
		for _, rc := range r.clients {
			if rc.config.Enabled {
				enabledCount++
			}
		}
		if enabledCount <= 1 {
			return errors.New("cannot remove the last enabled provider")
		}
	}

	r.clients = append(r.clients[:idx], r.clients[idx+1:]...)
	return nil
}

// UpdateProvider updates an existing provider's configuration.
func (r *Registry) UpdateProvider(cfg ProviderConfig) error {
	if cfg.ID == "" {
		return errors.New("provider id must not be empty")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	idx := -1
	for i, rc := range r.clients {
		if rc.config.ID == cfg.ID {
			idx = i
			break
		}
	}
	if idx < 0 {
		return fmt.Errorf("provider %q not found", cfg.ID)
	}

	// Merge: only update fields that are set in the incoming config.
	existing := r.clients[idx].config
	if cfg.Provider != "" {
		existing.Provider = cfg.Provider
	}
	if cfg.Model != "" {
		existing.Model = cfg.Model
	}
	if cfg.BaseURL != "" {
		existing.BaseURL = cfg.BaseURL
	}
	if cfg.APIKey != "" {
		existing.APIKey = cfg.APIKey
	}
	if cfg.TimeoutSec > 0 {
		existing.TimeoutSec = cfg.TimeoutSec
	}
	existing.Priority = cfg.Priority
	existing.Enabled = cfg.Enabled

	client, err := buildClient(existing)
	if err != nil {
		return fmt.Errorf("rebuild client for provider %q: %w", cfg.ID, err)
	}

	r.clients[idx] = &registeredClient{config: existing, client: client}
	r.sortClientsLocked()
	return nil
}

// ListProviders returns the configuration of all registered providers.
func (r *Registry) ListProviders() []ProviderConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()

	configs := make([]ProviderConfig, len(r.clients))
	for i, rc := range r.clients {
		configs[i] = rc.config
	}
	return configs
}

// SetEnabled enables or disables a provider by ID.
func (r *Registry) SetEnabled(id string, enabled bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	idx := -1
	for i, rc := range r.clients {
		if rc.config.ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		return fmt.Errorf("provider %q not found", id)
	}

	// Don't allow disabling the last enabled provider.
	if !enabled && r.clients[idx].config.Enabled {
		enabledCount := 0
		for _, rc := range r.clients {
			if rc.config.Enabled {
				enabledCount++
			}
		}
		if enabledCount <= 1 {
			return errors.New("cannot disable the last enabled provider")
		}
	}

	r.clients[idx].config.Enabled = enabled
	return nil
}

// enabledClients returns a snapshot of currently enabled clients sorted by priority.
func (r *Registry) enabledClients() []*registeredClient {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*registeredClient, 0, len(r.clients))
	for _, rc := range r.clients {
		if rc.config.Enabled {
			result = append(result, rc)
		}
	}
	return result
}

func (r *Registry) sortClientsLocked() {
	sort.SliceStable(r.clients, func(i, j int) bool {
		return r.clients[i].config.Priority < r.clients[j].config.Priority
	})
}

// buildClient creates an llm.Client from a ProviderConfig.
func buildClient(cfg ProviderConfig) (Client, error) {
	chatModel, err := newOpenAIChatModel(context.Background(), &openaimodel.ChatModelConfig{
		APIKey:  cfg.APIKey,
		Model:   cfg.Model,
		BaseURL: cfg.BaseURL,
		Timeout: time.Duration(cfg.TimeoutSec) * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("init openai compatible chat model: %w", err)
	}

	return &openAICompatibleClient{
		provider: cfg.Provider,
		model:    cfg.Model,
		chat:     chatModel,
	}, nil
}

// failoverChatModel implements model.ToolCallingChatModel with failover across providers.
type failoverChatModel struct {
	registry *Registry
	tools    []*schema.ToolInfo // tools bound via WithTools
}

// Generate tries providers in priority order, failing over on retriable errors.
func (f *failoverChatModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	clients := f.registry.enabledClients()
	if len(clients) == 0 {
		return nil, errors.New("no enabled llm providers available")
	}

	var lastErr error
	for _, rc := range clients {
		chatModel := rc.client.ChatModel()
		if f.tools != nil {
			var err error
			chatModel, err = chatModel.WithTools(f.tools)
			if err != nil {
				lastErr = fmt.Errorf("provider %q: bind tools: %w", rc.config.ID, err)
				continue
			}
		}

		msg, err := chatModel.Generate(ctx, input, opts...)
		if err == nil {
			return msg, nil
		}

		lastErr = fmt.Errorf("provider %q: %w", rc.config.ID, err)

		if !isRetriableError(err) {
			return nil, lastErr
		}
		// Retriable error: try next provider.
	}
	return nil, fmt.Errorf("all providers failed: %w", lastErr)
}

// Stream tries providers in priority order for starting the stream.
// Once a stream starts successfully, it is returned without further failover.
func (f *failoverChatModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	clients := f.registry.enabledClients()
	if len(clients) == 0 {
		return nil, errors.New("no enabled llm providers available")
	}

	var lastErr error
	for _, rc := range clients {
		chatModel := rc.client.ChatModel()
		if f.tools != nil {
			var err error
			chatModel, err = chatModel.WithTools(f.tools)
			if err != nil {
				lastErr = fmt.Errorf("provider %q: bind tools: %w", rc.config.ID, err)
				continue
			}
		}

		reader, err := chatModel.Stream(ctx, input, opts...)
		if err == nil {
			return reader, nil
		}

		lastErr = fmt.Errorf("provider %q: %w", rc.config.ID, err)

		if !isRetriableError(err) {
			return nil, lastErr
		}
		// Retriable error: try next provider.
	}
	return nil, fmt.Errorf("all providers failed: %w", lastErr)
}

// WithTools returns a new failoverChatModel with the given tools bound to all underlying models.
func (f *failoverChatModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return &failoverChatModel{
		registry: f.registry,
		tools:    tools,
	}, nil
}

// isRetriableError determines if an error is worth retrying with the next provider.
// It does NOT retry on HTTP 4xx client errors (bad request, auth, etc.).
func isRetriableError(err error) bool {
	if err == nil {
		return false
	}

	// Context cancellation is not retriable -- the caller cancelled.
	if errors.Is(err, context.Canceled) {
		return false
	}

	// Deadline exceeded could be a per-provider timeout, so we could retry
	// with the next provider, but if it's the caller's context deadline,
	// we shouldn't. We check both cases: if the error IS context.DeadlineExceeded
	// from the original context, don't retry.
	if errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	// Check for HTTP 4xx errors in the error message (heuristic).
	errMsg := strings.ToLower(err.Error())
	if strings.Contains(errMsg, "status code 4") ||
		strings.Contains(errMsg, "status code: 4") ||
		strings.Contains(errMsg, "400 bad request") ||
		strings.Contains(errMsg, "401 unauthorized") ||
		strings.Contains(errMsg, "403 forbidden") ||
		strings.Contains(errMsg, "404 not found") ||
		strings.Contains(errMsg, "422 unprocessable") ||
		strings.Contains(errMsg, "429 too many requests") {
		return false
	}

	// All other errors (network, 5xx, EOF, etc.) are retriable.
	return true
}
