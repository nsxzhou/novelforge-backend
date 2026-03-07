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
- API 分组占位接口：
  - `GET /api/v1/placeholder`
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
- 内存存储实现（`storage/memory`）：已为上述 6 个聚合各提供完整的 CRUD 内存仓储及测试
- 存储配置支持（`StorageConfig`）：通过 `provider` 字段切换后端，当前支持 `memory`

### 服务用例层 (`internal/service`)

6 个领域的应用用例接口（`UseCase`）及依赖声明（`Dependencies`）已定义，尚未提供具体实现：

- `service/project`、`service/asset`、`service/chapter`
- `service/conversation`、`service/generation`、`service/metric`

## 快速开始

1. 准备配置文件：

   ```bash
   cp configs/config.yaml.example configs/config.yaml
   ```

2. 设置必需的环境变量：

   ```bash
   export NOVELFORGE_LLM_API_KEY="your-key"
   ```

3. 启动服务：

   ```bash
   go run ./cmd/server -config configs/config.yaml
   ```

也可以使用辅助脚本：

```bash
./scripts/run-local.sh
```

## 当前刻意保留的边界

- 服务用例层（`internal/service`）仅定义接口，尚未实现业务逻辑
- 尚未提供具体的 LLM Provider 客户端实现；工厂目前仅返回占位客户端元数据
- HTTP Handler 层尚未实现业务接口（仅健康检查与占位路由）
- 存储层当前仅有内存实现，持久化（如 SQLite / PostgreSQL）待后续扩展
