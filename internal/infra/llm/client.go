package llm

import "github.com/cloudwego/eino/components/model"

// Client abstracts provider-specific LLM client wiring.
type Client interface {
	Provider() string
	Model() string
	ChatModel() model.ToolCallingChatModel
}
