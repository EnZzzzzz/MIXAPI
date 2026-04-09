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
1. **记录完整请求**：记录用户发送的完整请求体（JSON 格式）
2. **记录完整响应**：记录模型返回的完整响应内容
3. **支持导出**：可按时间范围导出原始数据（JSON/CSV）
4. **开关控制**：支持开启/关闭详细记录功能
5. **数据清理**：支持自动/手动清理过期数据

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

### 5.1 导出接口

```
GET /api/log/export?start_timestamp={start}&end_timestamp={end}&format={format}

参数：
- start_timestamp: 开始时间戳（秒）
- end_timestamp: 结束时间戳（秒）
- format: 导出格式 json|csv（默认 json）

响应：
Content-Type: application/octet-stream
Content-Disposition: attachment; filename="logs_20240409_20240410.json"
```

### 5.2 清理接口（已有增强）

```
DELETE /api/log/?target_timestamp={timestamp}&clean_body_only=true

参数：
- target_timestamp: 清理此时间之前的数据
- clean_body_only: 仅清理详细内容，保留元数据
```

## 六、实现计划

### Phase 1: 核心功能 (本周)
- [ ] 修改 Log 模型，增加 response_body 字段
- [ ] 修改 relay-text.go，捕获完整请求/响应
- [ ] 实现 RecordConsumeLog 参数传递
- [ ] 数据库 migration 脚本

### Phase 2: 导出功能 (下周)
- [ ] 实现 ExportLogs 接口
- [ ] 前端导出按钮
- [ ] 支持 CSV/JSON 格式

### Phase 3: 清理优化 (后续)
- [ ] 自动清理定时任务
- [ ] 仅清理大字段保留元数据
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

- `model/log.go` - 日志模型
- `relay/relay-text.go` - 文本请求处理
- `controller/log.go` - 日志接口
- `web/src/pages/Setting/Operation/SettingsLog.js` - 日志设置页面

---

**创建日期**: 2024-04-09
**版本**: V1.0
**状态**: 待实现
