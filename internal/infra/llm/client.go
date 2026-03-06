package llm

import "github.com/cloudwego/eino/components/model"

// Client abstracts provider-specific LLM client wiring.
type Client interface {
	Provider() string
	Model() string
	ChatModel() model.ToolCallingChatModel
}

type placeholderClient struct {
	provider string
	model    string
	chat     model.ToolCallingChatModel
}

func (c *placeholderClient) Provider() string {
	return c.provider
}

func (c *placeholderClient) Model() string {
	return c.model
}

func (c *placeholderClient) ChatModel() model.ToolCallingChatModel {
	return c.chat
}
