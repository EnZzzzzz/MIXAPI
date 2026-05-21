# CLAUDE.md

本文档为 Claude Code (claude.ai/code) 提供在本仓库中处理代码的指南。

## 项目概述

MIXAPI 是一个 AI 模型 API 网关，聚合多个 LLM API 并将其转换为统一的 OpenAI 兼容、Claude 兼容和 Gemini 兼容的端点。它使用 Go 编写，带有 React 前端。

- **Go 模块**: `one-api`
- **版本**: `v1.2`（在 `VERSION` 中追踪）
- **Go 版本**: `1.23.0`
- **默认端口**: `3000`

## 常用命令

### 后端

```bash
# 在本地运行后端（需要 .env 文件或使用默认值）
go run main.go

# 构建生产二进制文件
make build-go

# 构建 Docker 镜像（Linux，静态，无 CGO）
make build-docker

# 完整构建：前端 + Go 二进制文件 + Docker 镜像
make all
```

### 前端

前端位于 `web/` 目录中。它是一个使用 Semi UI 和 TailwindCSS 的 React 18 + Vite 应用。

```bash
cd web

# 开发服务器
npm run dev

# 构建（生产环境）
npm run build

# 使用 Prettier 进行代码检查/格式化
npm run lint
npm run lint:fix
```

### 完整开发构建

```bash
make build-and-start   # 构建前端，然后启动后端
make stop              # 停止端口 3000 上的后端服务
```

### 数据库

- 默认（无 `SQL_DSN`）：SQLite（`one-api.db`）
- 生产环境：MySQL（`SQL_DSN=root:pass@tcp(localhost:3306)/mixapi?parseTime=true`）
- 也支持 PostgreSQL
- 可选的独立日志数据库：`LOG_SQL_DSN`

## 架构

### 请求流程

1. **路由器** (`router/`) 注册四个路由组：
   - `SetApiRouter` — 公共/内部 API（`/api/*`）
   - `SetDashboardRouter` — 管理后台 API（`/api/*`）
   - `SetRelayRouter` — 模型推理端点（`/v1/*`、`/v1beta`、`/pg/*`）
   - `SetWebRouter` — 提供嵌入式 React 前端（`web/dist`）或代理到 `FRONTEND_BASE_URL`

2. **中间件** (`middleware/`)
   - `TokenAuth()` 验证中继端点的 Bearer 令牌。
   - `Distribute()` 根据令牌的分组、模型映射和配置的权重选择模型通道。
   - `ModelRequestRateLimit()` 强制执行每个模型的速率限制。
   - `UserAuth()` / `AdminAuth()` 保护后台路由。

3. **控制器** (`controller/`)
   - `controller/relay.go` 中的 `Relay()` 是推理请求的主要入口。它从路径确定 `relayMode`，调用 `relay/` 中的适当助手函数，并在失败时最多重试 `common.RetryTimes` 次。

4. **中继** (`relay/`)
   - `relay-text.go`、`relay-adaptor.go` 等包含核心转发逻辑。
   - `GetAdaptor(apiType)` 返回目标提供商（OpenAI、Claude、Gemini、Azure、AWS Bedrock 等）的 `channel.Adaptor`。
   - 每个提供商在 `relay/channel/<provider>/` 中实现 `channel.Adaptor`。
   - 助手函数（`TextHelper`、`AudioHelper`、`ImageHelper` 等）序列化请求、发送 HTTP 调用、流式返回响应，并计算令牌配额。

5. **服务** (`service/`)
   - 不属于控制器或模型的业务逻辑：配额计算、令牌计数、HTTP 客户端设置、Webhook 通知、日志导出等。

6. **模型** (`model/`)
   - GORM 模型和数据库操作。
   - `DB` 是主数据库；`LOG_DB` 是可选的日志数据库。
   - `InitDB()` 从 DSN 选择驱动程序（MySQL/SQLite/PostgreSQL）。
   - 通道缓存和选项映射在后台 goroutine 中同步（`SyncChannelCache`、`SyncOptions`）。

### 关键包

- `common/` — 共享工具、日志记录器、上下文助手、环境解析、数据库类型常量。
- `constant/` — 枚举：API 类型、中继模式、上下文键、通道状态。
- `dto/` — 与 OpenAI/Claude/Gemini API 对齐的请求/响应结构体。
- `setting/` — 运行时配置子包：操作设置、模型设置、比率设置、速率限制等。
- `types/` — 错误类型（例如 `NewAPIError`）。

### 后台 Goroutine

`main.go` 在启动时启动多个后台任务：
- `SyncChannelCache` — 刷新通道可用性。
- `SyncOptions` — 热重载系统选项。
- `UpdateQuotaData` — 仪表盘统计数据。
- `AutomaticallyUpdateChannels` / `AutomaticallyTestChannels` — 可选的通道健康检查。
- `UpdateTaskBulk` / `UpdateMidjourneyTaskBulk` — 任务轮询（当 `NODE_TYPE=master` 时）。
- `StartLogExportCleaner` — 清理旧的异步日志导出。
- `InitBatchUpdater` — 当 `BATCH_UPDATE_ENABLED=true` 时批量写入数据库。

## 环境变量

来自 `.env.example` 的重要变量：

- `SQL_DSN` / `LOG_SQL_DSN` — 数据库连接。
- `REDIS_CONN_STRING` — Redis 缓存。
- `MEMORY_CACHE_ENABLED` — 内存缓存回退。
- `SYNC_FREQUENCY` — 缓存同步间隔（秒）。
- `SESSION_SECRET` / `CRYPTO_SECRET` — 会话和加密密钥。
- `PORT` — 服务器端口（默认 3000）。
- `DEBUG=true` — 启用调试日志。
- `ENABLE_PPROF=true` — 在 `:8005` 上暴露 pprof。
- `RELAY_TIMEOUT` / `STREAMING_TIMEOUT` — 上游超时时间。
- `CHANNEL_UPDATE_FREQUENCY` / `CHANNEL_TEST_FREQUENCY` — 自动通道刷新/测试频率。
- `BATCH_UPDATE_ENABLED` / `BATCH_UPDATE_INTERVAL` — 批量写入优化。
- `NODE_TYPE=master` 或 `slave` — 集群节点类型。

## 注意事项

- 前端构建产物（`web/dist`）通过 `//go:embed` 嵌入到 Go 二进制文件中。
- Docker 构建需要预构建的 Linux 二进制文件（`mixapi`），因为 `Dockerfile` 只复制它；`make build-docker` 处理交叉编译步骤。
- 在某些后台任务中使用 `bytedance/gopkg` 的 `gopool` 进行 goroutine 池化。
- 项目中没有 Go 单元测试；如果添加测试，请使用 `go test ./...`。
