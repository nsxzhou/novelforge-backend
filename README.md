# NovelForge 后端

基于 Hertz + Eino 的 NovelForge 后端服务。

## 当前已具备的能力

### 基础设施层

- Hertz HTTP 服务启动与生命周期入口
- 运行时 YAML 配置加载与快速失败校验（`AppConfig`、`ServerConfig`、`LLMConfig`、`StorageConfig`）
- Request ID、panic 恢复、CORS 白名单中间件
- 健康检查接口：
  - `GET /healthz`
  - `GET /readyz`
- 已接入项目、资产、章节与对话微调 API：
  - `POST /api/v1/projects`
  - `GET /api/v1/projects`
  - `GET /api/v1/projects/:projectID`
  - `PUT /api/v1/projects/:projectID`
  - `POST /api/v1/projects/:projectID/assets`
  - `POST /api/v1/projects/:projectID/assets/generate`
  - `GET /api/v1/projects/:projectID/assets`
  - `POST /api/v1/projects/:projectID/chapters`
  - `GET /api/v1/projects/:projectID/chapters`
  - `POST /api/v1/projects/:projectID/conversations`
  - `GET /api/v1/projects/:projectID/conversations`
  - `GET /api/v1/assets/:assetID`
  - `PUT /api/v1/assets/:assetID`
  - `DELETE /api/v1/assets/:assetID`
  - `GET /api/v1/chapters/:chapterID`
  - `POST /api/v1/chapters/:chapterID/confirm`
  - `POST /api/v1/chapters/:chapterID/continue`
  - `POST /api/v1/chapters/:chapterID/rewrite`
  - `GET /api/v1/conversations/:conversationID`
  - `POST /api/v1/conversations/:conversationID/messages`
  - `POST /api/v1/conversations/:conversationID/confirm`
- 已接入 OpenAI 兼容 LLM 客户端装配、Prompt 模板注册表，以及 Project / Asset 对话驱动微调链路（建议生成 -> 显式确认 -> 写回）和章节生成主链路（生成 / 续写 / 局部重写）
- Prompt 能力映射已类型化（`PromptCapability`），业务侧不再依赖散落的字符串键
- 前端子仓库已完成 V1 对接页面，覆盖项目入口、设定工坊、对话微调、章节生成与当前稿确认流程；联调验收条目维护在 `../frontend/docs/前端开发优先级-V1.md`

### 领域模型层 (`internal/domain`)

V1 全部 6 个领域聚合已落地，每个聚合包含实体定义（`model.go`）、仓储接口（`repository.go`）及模型单元测试（`model_test.go`）：

| 聚合     | 包路径                | 核心实体                      |
| -------- | --------------------- | ----------------------------- |
| 项目     | `domain/project`      | `Project`                     |
| 资产     | `domain/asset`        | `Asset`（世界观/角色/大纲等） |
| 章节     | `domain/chapter`      | `Chapter`                     |
| 对话     | `domain/conversation` | `Conversation`                |
| 生成记录 | `domain/generation`   | `GenerationRecord`            |
| 指标事件 | `domain/metric`       | `MetricEvent`                 |

### 存储层 (`internal/infra/storage`)

- 统一仓储组合结构 `Repositories` 及工厂函数 `NewRepositories`
- 内存存储实现（`storage/memory`）：保留用于轻量测试与回归
- PostgreSQL 存储实现（`storage/postgres`）：覆盖 6 个聚合、迁移入口和数据库就绪检查
- 存储配置支持（`StorageConfig`）：通过 `provider` 字段切换 `memory` 与 `postgres`
- PostgreSQL provider 已支持事务执行器（`TxRunner`），并通过 `context` 透传到各 repository 执行 SQL

### 服务用例层 (`internal/service`)

当前已提供 Project / Asset / Conversation / Chapter / Metric 的应用用例实现：

- `service/project`：项目创建 / 列表 / 查询 / 更新
- `service/asset`：资产创建 / AI 生成 / 列表 / 按类型过滤 / 查询 / 更新 / 删除
- `service/conversation`：对话发起 / 继续 / 查询 / 按 project/target 列表 / 显式确认写回 Project / Asset（confirm 路径在事务内执行，保证目标写回与会话清理原子提交）
- `service/chapter`：章节生成 / 列表 / 查询 / 当前稿确认 / 续写 / 局部重写，并为章节生成流创建和更新 `GenerationRecord`
- `service/metric`：指标事件 append/list 用例实现，已接入章节与对话微调业务采集流程

### 埋点与可观测性（V1 采集）

- 当前已采集动作：
  - asset：`generate`
  - chapter：`generate` / `continue` / `rewrite` / `confirm`
  - conversation：`start` / `reply` / `confirm`
- 统一事件名：
  - `operation_completed`
  - `operation_failed`
- 通过 `labels` 细分维度：
  - 通用：`domain`、`action`
  - chapter：`generation_kind`（生成类动作）、`error_kind`（失败时）
  - conversation：`target_type`、`error_kind`（失败时）
- `stats` 当前包含：
  - `duration_ms`：动作端到端耗时
  - `token_usage`：V1 当前口径为 `0`（后续接入模型真实 usage）
- 降级策略：
  - 埋点写入失败不影响主业务流程，仅记录 warning 日志
  - 无法确定合法 `project_id` 的失败场景会跳过落库

## 本地开发（默认 PostgreSQL）

1. 准备配置文件：

   ```bash
   cp configs/config.yaml.example configs/config.yaml
   ```

2. 启动本地 PostgreSQL：

   ```bash
   docker compose up -d postgres
   ```

3. 准备环境变量文件：

   ```bash
   cp .env.example .env
   ```

   然后按需修改 `.env` 中的 `NOVELFORGE_*` 配置（尤其是 `NOVELFORGE_LLM_API_KEY`）。

4. 启动服务（`provider=postgres` 时会自动执行 migration）：

   ```bash
   ./scripts/run-local.sh
   ```

   `./scripts/run-local.sh` 会在检测到 `backend/.env` 时自动加载该文件。

## 配置说明

默认示例配置已指向 PostgreSQL，并启用 OpenAI 兼容 LLM provider：

```yaml
server:
  host: "0.0.0.0"
  port: 8080
  read_timeout_seconds: 15
  write_timeout_seconds: 15
  cors:
    allowed_origins:
      - "http://localhost:5173"
      - "http://127.0.0.1:5173"

storage:
  provider: "postgres"
  postgres:
    url_env: "NOVELFORGE_DATABASE_URL"
    max_open_conns: 10
    max_idle_conns: 5
    conn_max_lifetime_seconds: 300

llm:
  provider_env: "NOVELFORGE_LLM_PROVIDER"
  model_env: "NOVELFORGE_LLM_MODEL"
  base_url_env: "NOVELFORGE_LLM_BASE_URL"
  api_key_env: "NOVELFORGE_LLM_API_KEY"
  timeout_seconds: 60
  prompts:
    asset_generation: "asset_generation.yaml"
    chapter_generation: "chapter_generation.yaml"
    chapter_continuation: "chapter_continuation.yaml"
    chapter_rewrite: "chapter_rewrite.yaml"
    project_refinement: "project_refinement.yaml"
    asset_refinement: "asset_refinement.yaml"
```

说明：`provider_env/model_env/base_url_env/api_key_env` 存的是“环境变量名”，运行时会从这些变量读取真实值。

`server.cors.allowed_origins` 用于配置允许的跨域来源列表。预检请求（`OPTIONS`）仅对白名单来源放行；如果配置为空，默认放行本地开发来源 `http://localhost:5173` 和 `http://127.0.0.1:5173`。

Prompt 模板文件位于：

- `internal/infra/llm/prompts/asset_generation.yaml`
- `internal/infra/llm/prompts/chapter_generation.yaml`
- `internal/infra/llm/prompts/chapter_continuation.yaml`
- `internal/infra/llm/prompts/chapter_rewrite.yaml`
- `internal/infra/llm/prompts/project_refinement.yaml`
- `internal/infra/llm/prompts/asset_refinement.yaml`

当前模板文件固定包含两个顶层字段：

- `system`
- `user`

Prompt 模板内容通过 `go:embed` 编译进二进制。服务启动时会按配置预加载并校验模板文件语法；修改模板文件后需要重新构建并重新部署服务。`llm.prompts` 配置采用显式字段并启用 YAML unknown field 校验，避免配置键与能力映射漂移。当前 `asset_generation` / `project_refinement` / `asset_refinement` 已接入资产生成与 Project / Asset 对话微调链路；`chapter_generation` / `chapter_continuation` / `chapter_rewrite` 已接入章节生成、续写与局部重写业务。

数据库 schema 迁移文件位于：

- `migrations/000001_init.up.sql`
- `migrations/000001_init.down.sql`
- `migrations/000002_conversation_pending_suggestion.up.sql`
- `migrations/000002_conversation_pending_suggestion.down.sql`

## 健康检查

- `GET /healthz`：仅检查进程是否存活
- `GET /readyz`：在 `postgres` 模式下会额外检查数据库连通性

## 测试

```bash
go test ./...
```

当前重点覆盖：

- 项目 / 资产 / 对话微调 HTTP handler 集成测试（基于内存仓储）
- Conversation service 单元测试覆盖 `start / reply / get_by_id / list / confirm(project|asset)` 主路径与关键失败分支（无 pending suggestion、target 归属校验等）
- Conversation confirm 已覆盖 optimistic locking 冲突路径（project/asset/conversation 并发更新返回冲突）与事务回滚路径（目标写回后会话更新失败时整体回滚）
- Chapter service 与 handler 测试覆盖公共用例 `create / get_by_id / list_by_project / update`，以及章节生成链路中的当前稿确认、重复确认幂等、未完成草稿拒绝与冲突映射
- Chapter rewrite 成功链路测试覆盖 `GenerationRecord` 成功落库与章节确认状态重置（`current_draft_confirmed_at/by` 清空）
- Conversation confirm handler 集成测试覆盖 asset 响应分支（`project` 为空、`asset` 返回已确认内容）
- Metric service 与 chapter/conversation 埋点集成单元测试（成功/失败事件、降级策略）
- PostgreSQL repository SQL 路径测试（含 `pending_suggestion` 持久化，基于 sqlmock）
- LLM 配置校验、OpenAI 兼容客户端工厂、Prompt Store 加载与渲染、bootstrap 装配测试
- 配置层补充 `config.Load()` 成功/失败分支与 `ServerConfig.Address()` 单元测试
- 本地 PostgreSQL 运行态验证流程：直接执行 `go run ./cmd/server -config configs/config.yaml`（启动阶段自动执行 migration）

## 当前刻意保留的边界

- 项目 / 资产 CRUD、Project / Asset 对话微调，以及章节生成 / 当前稿确认 / 续写 / 局部重写链路已完成；章节生成流会创建并持久化 `GenerationRecord`
- `POST /api/v1/chapters/:chapterID/confirm` 已实现显式的“确认当前稿”业务流；请求需通过 `X-User-ID` 请求头传入合法 UUID，系统会把该值写入 `current_draft_confirmed_by`
- 当前稿确认仅允许作用于 `current_draft_id` 指向且状态为 `succeeded` 的生成记录；同一草稿重复确认保持幂等；若章节在确认期间被续写/改写并更新，则返回冲突错误提示重试
- `POST /api/v1/conversations/:conversationID/confirm` 在 `postgres` 运行态下以单事务执行目标写回与会话状态更新；若会话更新失败会回滚目标写入
- V1 仅完成埋点采集，不包含可视化看板；`token_usage` 口径当前沿用业务记录字段（默认 0）
- 直接通过 `cmd/server` 启动 `postgres` 模式服务时会在 bootstrap 阶段自动执行 migration；若迁移失败则启动直接失败
- `memory` provider 仍然保留，但目标是用于测试而不是默认运行态持久化
