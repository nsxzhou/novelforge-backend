package llm

import "github.com/cloudwego/eino/components/model"

type openAICompatibleClient struct {
	provider string
	model    string
	chat     model.ToolCallingChatModel
}

func (c *openAICompatibleClient) Provider() string {
	return c.provider
}

func (c *openAICompatibleClient) Model() string {
	return c.model
}

func (c *openAICompatibleClient) ChatModel() model.ToolCallingChatModel {
	return c.chat
}
