# 模型输入输出记录功能 (V1.0)

## 一、需求背景

MixAPI 作为 AI 大模型网关，需要记录每个请求的完整输入和输出内容，用于：
- 数据审计与合规
- 问题排查与调试
- 模型效果分析
- 用户行为分析

## 二、现状分析

### 2.1 现有日志系统
- 已有 `logs` 表记录基本调用信息（用户、模型、token 数、耗时等）
- 有 `UserInput` 字段但只记录简单的用户消息摘要
- 支持独立日志数据库配置（`LOG_SQL_DSN`）
- 支持手动清理历史日志

### 2.2 数据规模预估
| 指标 | 数值 |
|------|------|
| 日均请求量 | 2-3 万次 |
| 单次请求最大内容 | 200KB |
| 日数据量 | 4-6 GB |
| 月数据量 | 120-180 GB |
| 年数据量 | ~1.5 TB |

## 三、需求目标

### 3.1 功能需求
| 序号 | 功能 | 状态 | 说明 |
|------|------|------|------|
| 1 | 记录完整请求 | ✅ 已完成 | `user_input` 字段已存储完整请求 JSON |
| 2 | 记录完整响应 | ✅ 已完成 | `response_body` 字段已存储完整响应 JSON |
| 3 | 支持导出 | 🔄 部分完成 | 后端接口已实现，前端页面待添加 |
| 4 | 开关控制 | ✅ 已完成 | `LogConsumeEnabled` 配置可控制 |
| 5 | 数据清理 | 🔄 部分完成 | 基础删除已实现，待增强"仅清理内容"模式 |

### 3.2 非功能需求
1. **性能**：不影响主流程响应时间（异步记录）
2. **存储**：V1 版本使用数据库存储（MEDIUMTEXT 类型）
3. **查询**：支持按用户、时间、模型查询
4. **保留**：默认保留 30 天，超期自动清理

## 四、技术方案

### 4.1 存储方案选型

| 方案 | 优点 | 缺点 | 适用性 |
|------|------|------|--------|
| **数据库 (V1)** | 实现简单，查询方便 | 存储成本高，扩展性差 | ✅ V1 选用 |
| 对象存储 | 成本低，可存海量数据 | 需要额外查询 | V2 迁移目标 |
| 混合存储 | 兼顾性能和成本 | 架构复杂 | 未来方案 |

### 4.2 数据库表结构

```sql
-- logs 表修改
ALTER TABLE logs
MODIFY COLUMN user_input MEDIUMTEXT COMMENT '用户输入内容(完整请求JSON)',
ADD COLUMN response_body MEDIUMTEXT COMMENT '模型响应内容(完整响应JSON)';
```

**字段说明：**
- `user_input`: 完整的请求体 JSON，最大 16MB
- `response_body`: 完整的响应体 JSON，最大 16MB
- 使用 `MEDIUMTEXT` 类型，单条最大支持 16MB

### 4.3 代码架构

```
┌─────────────────────────────────────────────────────────┐
│  relay-text.go (流量入口)                                │
│  ├── 获取完整请求体                                      │
│  ├── 获取完整响应体                                      │
│  └── 调用 RecordConsumeLog()                            │
└─────────────────────────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────┐
│  model/log.go                                            │
│  ├── Log 结构体（新增 response_body 字段）               │
│  ├── RecordConsumeLogParams（新增 ResponseBody 字段）    │
│  └── RecordConsumeLog() 保存到数据库                     │
└─────────────────────────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────┐
│  controller/log.go                                       │
│  └── ExportLogs() 导出接口                               │
└─────────────────────────────────────────────────────────┘
```

### 4.4 配置项

| 配置项 | 说明 | 默认值 |
|--------|------|--------|
| `LogDetailEnabled` | 是否开启详细记录 | `true` |
| `LogDetailMaxSize` | 单条记录最大大小（KB） | `10240` (10MB) |
| `LogDetailRetentionDays` | 详细内容保留天数 | `30` |

## 五、接口设计

### 5.1 导出接口 ✅ 已实现

```
GET /api/log/export?start_timestamp={start}&end_timestamp={end}&format={format}&username={username}&model_name={model_name}

参数：
- start_timestamp: 开始时间戳（秒）
- end_timestamp: 结束时间戳（秒）
- format: 导出格式 json|csv（默认 json）
- username: 按用户名筛选（可选）
- model_name: 按模型名称筛选（可选）

响应：
Content-Type: application/octet-stream
Content-Disposition: attachment; filename="logs_20240409_20240410.json"

状态：✅ 后端已实现（controller/log.go:190-244）
       ⏳ 前端待添加导出按钮
```

### 5.2 清理接口 🔄 待增强

**当前实现（基础功能）✅**
```
DELETE /api/log/?start_timestamp={start}&end_timestamp={end}

参数：
- start_timestamp: 开始时间戳（秒）
- end_timestamp: 结束时间戳（秒）
- target_timestamp: 兼容旧参数，相当于 end_timestamp

状态：✅ 已实现（controller/log.go:153-188）
       ⚠️ 前端仅支持单时间点选择，待改为时间范围
```

**待增强功能 ⏳**
```
DELETE /api/log/?start_timestamp={start}&end_timestamp={end}&clean_mode={mode}

新增参数：
- clean_mode: 清理模式
  - `all` - 删除整行（默认，兼容当前）
  - `body_only` - 仅清理 user_input 和 response_body，保留元数据

状态：⏳ 后端待实现 CleanLogBodiesOnly 函数
       ⏳ 前端待添加清理模式选择
```

## 六、实现进度

### Phase 1: 核心功能 ✅ 已完成
- [x] 修改 Log 模型，增加 response_body 字段
- [x] 修改 relay-text.go，捕获完整请求/响应
- [x] 实现 RecordConsumeLog 参数传递
- [x] 数据库 migration 脚本

### Phase 2: 导出功能 🔄 进行中
- [x] 实现 ExportLogs 接口（controller/log.go）
- [x] 支持 CSV/JSON 格式（convertLogsToCSV）
- [ ] 前端导出按钮和参数选择界面
- [ ] 大文件流式导出优化（当前一次性加载到内存）

### Phase 3: 清理优化 ⏳ 待开始
- [ ] 删除接口增强：支持 `clean_mode` 参数
- [ ] 实现 `CleanLogBodiesOnly` 函数（仅清理大字段）
- [ ] 前端：时间范围选择器（开始-结束）
- [ ] 前端：清理模式选择（全部删除/仅清理内容）
- [ ] 危险操作二次确认弹窗
- [ ] 自动清理定时任务
- [ ] 存储占用监控

## 七、风险评估

| 风险 | 影响 | 缓解措施 |
|------|------|----------|
| 数据库膨胀 | 高 | 设置 10MB 单条上限，30 天自动清理 |
| 性能影响 | 中 | 异步记录，不阻塞主流程 |
| 敏感数据泄露 | 高 | 保留现有权限控制，仅管理员可导出 |

## 八、后续规划 (V2)

### 8.1 对象存储迁移
- 数据量 >100GB 后迁移到对象存储
- 小数据 (<10KB) 仍存数据库
- 大数据 (>=10KB) 存对象存储

### 8.2 压缩优化
- 对重复内容使用引用存储
- 启用 gzip 压缩存储

## 九、附录

### 9.1 数据表示例

```json
{
  "id": 12345,
  "user_id": 100,
  "created_at": 1712630400,
  "model_name": "gpt-4",
  "prompt_tokens": 1500,
  "completion_tokens": 500,
  "user_input": "{\"messages\":[{\"role\":\"user\",\"content\":\"Hello\"}],\"model\":\"gpt-4\"}",
  "response_body": "{\"id\":\"chatcmpl-xxx\",\"choices\":[{\"message\":{\"content\":\"Hi!\"}}]}"
}
```

### 9.2 相关文件

| 文件 | 说明 | 状态 |
|------|------|------|
| `model/log.go` | 日志模型，含 Log 结构体和数据库操作 | ✅ 已更新 response_body |
| `model/log.go:474-493` | ExportLogs 函数 | ✅ 已实现 |
| `model/log.go:440-472` | DeleteLogsByTimeRange 函数 | ✅ 已实现基础版 |
| `controller/log.go:190-275` | 导出接口控制器 | ✅ 已实现 |
| `controller/log.go:153-188` | 删除接口控制器 | ✅ 已实现基础版 |
| `router/api-router.go:157` | 导出路由 /api/log/export | ✅ 已注册 |
| `router/api-router.go:151` | 删除路由 /api/log/ | ✅ 已注册 |
| `relay/relay-text.go` | 文本请求处理，捕获请求/响应 | ✅ 已实现 |
| `web/src/pages/Setting/Operation/SettingsLog.js` | 日志设置页面 | 🔄 待增强 |

### 9.3 后续优化建议

1. **导出优化**：当前一次性查询所有数据到内存，大数据量时可能 OOM，建议：
   - 分批查询（每批 1000 条）
   - 使用流式响应
   - 添加导出进度提示

2. **定时清理**：添加后台定时任务自动清理超期数据

3. **存储优化**：数据量 >100GB 后考虑迁移到对象存储

## 七、前端页面改造计划

### 当前页面 `SettingsLog.js`
- 仅支持单时间点选择清理
- 无导出功能入口

### 改造后页面结构
```
日志设置
├── [开关] 启用额度消费日志记录
├── [开关] 启用详细内容记录（控制 user_input/response_body）
│
├── 【历史日志导出】
│   ├── 开始时间 [DatePicker]
│   ├── 结束时间 [DatePicker]
│   ├── 格式选择 [JSON/CSV 单选]
│   ├── 用户名筛选 [输入框，可选]
│   └── [导出日志] 按钮
│
└── 【历史日志清理】
    ├── 开始时间 [DatePicker]
    ├── 结束时间 [DatePicker]
    ├── 清理模式 [全部删除/仅清理内容 单选]
    └── [清理日志] 按钮（带二次确认）
```

### 交互细节
- **导出**：点击后显示 loading，使用 `axios` 下载文件流
- **清理二次确认**：
  - 全部删除：警告弹窗提示"此操作将永久删除 N 条日志记录，不可恢复"
  - 仅清理内容：提示"将保留日志元数据，仅删除用户输入和模型响应内容"

---

**创建日期**: 2024-04-09
**最后更新**: 2026-04-09
**版本**: V1.0
**状态**: 🔄 部分完成（Phase 1 已完成，Phase 2/3 进行中）
