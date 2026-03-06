# NovelForge 后端

基于 Hertz + Eino 的 NovelForge 后端服务初始化骨架。

## 当前骨架包含的能力

- Hertz HTTP 服务启动与生命周期入口
- 运行时 YAML 配置加载与快速失败校验（`AppConfig`、`ServerConfig`、`LLMConfig`）
- Request ID 中间件与 panic 恢复中间件
- 健康检查接口：
  - `GET /healthz`
  - `GET /readyz`
- API 分组占位接口：
  - `GET /api/v1/placeholder`
- LLM 抽象与占位工厂装配（尚未接入业务链路）

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

- `internal/domain` 和 `internal/service` 下尚未实现业务逻辑
- 尚未提供具体的 LLM Provider 客户端实现；工厂目前仅返回占位客户端元数据
- 尚未实现存储仓储层（见 `internal/infra/storage/doc.go`）
