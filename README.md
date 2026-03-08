# NovelForge 后端

基于 Hertz + Eino 的 NovelForge 后端服务。

## 当前已具备的能力

### 基础设施层

- Hertz HTTP 服务启动与生命周期入口
- 运行时 YAML 配置加载与快速失败校验（`AppConfig`、`ServerConfig`、`LLMConfig`、`StorageConfig`）
- Request ID 中间件与 panic 恢复中间件
- 健康检查接口：
  - `GET /healthz`
  - `GET /readyz`
- 已接入项目与资产基础 API：
  - `POST /api/v1/projects`
  - `GET /api/v1/projects`
  - `GET /api/v1/projects/:projectID`
  - `PUT /api/v1/projects/:projectID`
  - `POST /api/v1/projects/:projectID/assets`
  - `GET /api/v1/projects/:projectID/assets`
  - `GET /api/v1/assets/:assetID`
  - `PUT /api/v1/assets/:assetID`
  - `DELETE /api/v1/assets/:assetID`
- LLM 抽象与占位工厂装配（尚未接入业务链路）

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

### 服务用例层 (`internal/service`)

当前已提供 Project / Asset 的应用用例实现，其余领域仍以接口与依赖声明为主：

- `service/project`：项目创建 / 列表 / 查询 / 更新
- `service/asset`：资产创建 / 列表 / 按类型过滤 / 查询 / 更新 / 删除
- `service/chapter`、`service/conversation`、`service/generation`、`service/metric`：当前仍以接口与依赖声明为主

## 本地开发（默认 PostgreSQL）

1. 准备配置文件：

   ```bash
   cp configs/config.yaml.example configs/config.yaml
   ```

2. 启动本地 PostgreSQL：

   ```bash
   docker compose up -d postgres
   ```

3. 设置环境变量：

   ```bash
   export NOVELFORGE_DATABASE_URL="postgres://novelforge:novelforge@127.0.0.1:5432/novelforge?sslmode=disable"
   export NOVELFORGE_LLM_API_KEY="your-key"
   ```

4. 执行数据库迁移：

   ```bash
   go run ./cmd/migrate -config configs/config.yaml
   ```

5. 启动服务：

   ```bash
   go run ./cmd/server -config configs/config.yaml
   ```

也可以使用辅助脚本，它会在 `provider=postgres` 时自动执行迁移：

```bash
./scripts/run-local.sh
```

## 配置说明

默认示例配置已指向 PostgreSQL：

```yaml
storage:
  provider: "postgres"
  postgres:
    url_env: "NOVELFORGE_DATABASE_URL"
    max_open_conns: 10
    max_idle_conns: 5
    conn_max_lifetime_seconds: 300
```

数据库 schema 迁移文件位于：

- `migrations/000001_init.up.sql`
- `migrations/000001_init.down.sql`

## 健康检查

- `GET /healthz`：仅检查进程是否存活
- `GET /readyz`：在 `postgres` 模式下会额外检查数据库连通性

## 测试

```bash
go test ./...
```

当前重点覆盖：

- 项目 / 资产 HTTP handler 集成测试（基于内存仓储）
- PostgreSQL repository SQL 路径测试（基于 sqlmock）
- 本地 PostgreSQL 运行态验证流程：先执行 `go run ./cmd/migrate -config configs/config.yaml`，再启动 `go run ./cmd/server -config configs/config.yaml`

## 当前刻意保留的边界

- priority #2 的项目 / 资产链路已完成，但对话、章节生成、当前稿确认、指标采集等后续业务接口尚未实现
- 尚未提供具体的 LLM Provider 客户端实现；工厂目前仅返回占位客户端元数据
- `postgres` 模式下服务启动不会自动执行 migration；当前仍要求先显式运行 `cmd/migrate`
- `memory` provider 仍然保留，但目标是用于测试而不是默认运行态持久化
