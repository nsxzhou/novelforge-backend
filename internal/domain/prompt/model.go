package prompt

import "time"

// ProjectPromptOverride 存储项目级 prompt 模板覆盖。
type ProjectPromptOverride struct {
	ProjectID  string
	Capability string // config.PromptCapability 的字符串值
	System     string // 完整 system prompt 模板文本
	User       string // 完整 user prompt 模板文本
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
