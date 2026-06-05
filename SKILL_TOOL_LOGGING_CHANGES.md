# Skill / Tool 调用日志拦截功能 — 改动总结

> 生成日期: 2026-06-05  
> 功能: 记录 AI 请求中的 tools/skills/functions/MCP servers 到 `logs.other` JSON 字段

---

## 1. 功能概述

在不修改任何 handler 文件的前提下（AOP 风格），通过中间件 + handler 双路径提取请求中的工具/技能名称，写入消费日志的 `other` JSON 字段，用于后续统计分析哪些技能被频繁调用。

### 核心设计原则

| 原则 | 说明 |
|------|------|
| **AOP 风格** | 不改 handler 文件，中间件切入（非 Claude 格式走中间件路径） |
| **双源读取** | 优先读 `relayInfo.OriginalTools`（Claude handler 原生填充），回退读 Gin Context（中间件提供） |
| **不影响官方升级** | 升级时只需确认 `router/relay-router.go` 注册行还在 |
| **环境变量控制** | `LOG_REQUEST_TOOLS=true` 必须显式开启，零性能开销关闭 |

---

## 2. 改动文件清单 (5 个文件)

| 文件 | 类型 | 改动量 | 说明 |
|------|------|--------|------|
| `middleware/tool_extractor.go` | **新增** | ~92 行 | 中间件：`gjson` 解析原始 body 提取 tools，存 Context |
| `constant/context_key.go` | **修改** | +3 行 | 新增 `ContextKeyOriginalTools` 常量 |
| `router/relay-router.go` | **修改** | +1 行 | 注册 `ToolExtractorMiddleware()` 到 `/v1/*` 路由 |
| `relay/common/relay_info.go` | **修改** | +8 行 | 新增 `OriginalTools` 和 `OriginalMcpServers` 字段 |
| `service/log_info_generate.go` | **修改** | ~+110 行 | 新增 `appendToolInfo`、`extractToolNamesFromSources`、`extractSingleToolName` |
| `relay/claude_handler.go` | **修改** | +9 行 | Claude handler 中填充 `OriginalTools` 和 `OriginalMcpServers` |

---

## 3. 各文件详细改动

### 3.1 `middleware/tool_extractor.go` (新增)

完整的 AOP 中间件，在请求到达 handler 之前从原始 body 中提取工具名称。

```go
// 入口函数 — 环境变量控制开关
func ToolExtractorMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        if os.Getenv("LOG_REQUEST_TOOLS") != "true" {
            c.Next()  // 未启用时直接跳过，零开销
            return
        }
        // 从 BodyStorage 读取原始请求体
        storage, err := common.GetBodyStorage(c)
        // ...
        requestBody, err := storage.Bytes()
        // 使用 gjson 解析 tools
        names := extractToolNames(requestBody)
        if len(names) > 0 {
            common.SetContextKey(c, constant.ContextKeyOriginalTools, names)
        }
        c.Next()
    }
}
```

**`extractToolNames` 支持的请求格式:**

| 格式 | JSON 路径 | 示例 |
|------|-----------|------|
| Claude Messages | `tools[].name` | `[{"name": "skill_code_review", ...}]` |
| OpenAI Chat | `tools[].function.name` | `[{"function": {"name": "web_search"}, ...}]` |
| OpenAI (旧) | `functions[].name` | `[{"name": "get_weather", ...}]` |
| 通用回退 | `tools[].type` | `[{"type": "skill_name", ...}]` |

**关键依赖:** 使用 `github.com/tidwall/gjson` 做零反序列化 JSON 解析，避免完整 body 反序列化的性能开销。

---

### 3.2 `constant/context_key.go` (修改)

新增 Context Key 常量，用于中间件和日志服务之间的数据传递：

```go
// ContextKeyOriginalTools stores pre-extracted tool/skill names from the raw
// request body, populated by the ToolExtractor middleware for consumption log.
ContextKeyOriginalTools ContextKey = "original_tools"
```

---

### 3.3 `router/relay-router.go` (修改)

在 `/v1` 路由组中注册中间件，位于 `TokenAuth()` 和 `ModelRequestRateLimit()` 之后、`Distribute()` 之前：

```go
relayV1Router := router.Group("/v1")
relayV1Router.Use(middleware.RouteTag("relay"))
relayV1Router.Use(middleware.SystemPerformanceCheck())
relayV1Router.Use(middleware.TokenAuth())
relayV1Router.Use(middleware.ModelRequestRateLimit())
relayV1Router.Use(middleware.ToolExtractorMiddleware())  // ← 新增行
{
    // WebSocket / HTTP routes...
}
```

**位置选择原因:** 在 `Distribute()` 之前执行，此时请求 body 尚未被消费/修改，`BodyStorage` 中仍可读取原始 JSON。

---

### 3.4 `relay/common/relay_info.go` (修改)

在 `RelayInfo` 结构体中新增两个字段，供 Claude handler 原生填充：

```go
// OriginalTools records the "tools" field from the parsed request body
// for logging purposes (Claude skills, function calls, MCP servers, etc.).
// Only populated when LOG_REQUEST_TOOLS=true.
OriginalTools json.RawMessage `json:"original_tools,omitempty"`

// OriginalMcpServers records the "mcp_servers" field from the parsed Claude
// request body for logging purposes.
// Only populated when LOG_REQUEST_TOOLS=true.
OriginalMcpServers json.RawMessage `json:"original_mcp_servers,omitempty"`
```

---

### 3.5 `relay/claude_handler.go` (修改)

在 Claude handler 的请求处理入口处，提取 tools 和 mcp_servers 存入 relayInfo：

```go
// Extract tools/mcp_servers from original request for consumption log
if claudeReq.Tools != nil {
    if toolsJson, err := common.Marshal(claudeReq.Tools); err == nil {
        info.OriginalTools = toolsJson
    }
}
if len(claudeReq.McpServers) > 0 {
    info.OriginalMcpServers = claudeReq.McpServers
}
```

**为什么 Claude 需要单独处理:** Claude handler 在解析请求时已经完整反序列化了 `ClaudeRequest`，可以直接从结构体中取出 tools/mcp_servers，无需再走中间件的 gjson 解析路径。

---

### 3.6 `service/log_info_generate.go` (修改)

#### 3.6.1 调用入口

在 `GenerateTextOtherInfo` 函数末尾添加调用：

```go
func GenerateTextOtherInfo(...) map[string]interface{} {
    // ... 原有逻辑 ...
    appendToolInfo(ctx, relayInfo, other)  // ← 新增调用
    return other
}
```

#### 3.6.2 `appendToolInfo` — 核心日志写入函数

```go
func appendToolInfo(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, other map[string]interface{}) {
    // 1. 环境变量检查
    if os.Getenv("LOG_REQUEST_TOOLS") != "true" { return }

    // 2. 可选的正则过滤
    var nameFilter *regexp.Regexp
    if pattern := os.Getenv("LOG_TOOL_FILTER_REGEX"); pattern != "" {
        nameFilter, _ = regexp.Compile(pattern)
    }

    // 3. 从双源提取工具名
    names := extractToolNamesFromSources(ctx, relayInfo, nameFilter)
    if len(names) > 0 {
        other["tools"] = names
        other["tool_count"] = len(names)
    }

    // 4. MCP servers (仅 Claude handler 路径)
    if len(relayInfo.OriginalMcpServers) > 0 {
        // 解析 → 提取名称 → 正则过滤 → 写入 other
        other["mcp_servers"] = mcpNames
        other["mcp_server_count"] = len(mcpNames)
    }
}
```

#### 3.6.3 `extractToolNamesFromSources` — 双源读取策略

```
优先级:
  源1: relayInfo.OriginalTools (Claude handler 原生填充，json.RawMessage)
       ↓ 如果为空
  源2: ctx[ContextKeyOriginalTools] (中间件 AOP 路径，[]string)
```

**源1 处理流程:**
- `common.Unmarshal(relayInfo.OriginalTools, &toolsList)` 反序列化为 `[]interface{}`
- 遍历每个 tool 对象，调用 `extractSingleToolName` 提取名称
- 应用正则过滤

**源2 处理流程:**
- 从 Gin Context 读取 `[]string` 类型（中间件已预提取）
- 应用正则过滤

#### 3.6.4 `extractSingleToolName` — 单工具名提取

```go
func extractSingleToolName(tool map[string]interface{}) string {
    // Claude 格式: tool.name
    if name, ok := tool["name"].(string); ok && name != "" {
        return name
    }
    // OpenAI 格式: tool.function.name
    if fn, ok := tool["function"].(map[string]interface{}); ok {
        if name, ok := fn["name"].(string); ok && name != "" {
            return name
        }
    }
    return ""
}
```

---

## 4. 数据流全链路

```
客户端请求
  │
  ▼
Gin Engine
  │
  ├── BodyStorageCleanup (全局中间件, 请求结束后清理)
  │
  ├── /v1 路由组
  │     ├── SystemPerformanceCheck
  │     ├── TokenAuth
  │     ├── ModelRequestRateLimit
  │     ├── ToolExtractorMiddleware  ◄── [AOP 路径] gjson 解析 body
  │     │     └── extractToolNames() → ctx[ContextKeyOriginalTools]
  │     │
  │     └── Distribute
  │           │
  │           ▼
  │     Handler (Claude / OpenAI / ...)
  │           │
  │           ├── [Claude handler] ── relayInfo.OriginalTools = tools JSON
  │           │                    ── relayInfo.OriginalMcpServers = mcp JSON
  │           │
  │           ▼
  │     GenerateTextOtherInfo(ctx, relayInfo, ...)
  │           │
  │           └── appendToolInfo(ctx, relayInfo, other)
  │                 │
  │                 ├── 源1: relayInfo.OriginalTools (有值则用)
  │                 ├── 源2: ctx[ContextKeyOriginalTools] (回退)
  │                 │
  │                 ├── LOG_TOOL_FILTER_REGEX 过滤
  │                 │
  │                 ├── other["tools"] = ["skill_xxx", "web_search"]
  │                 ├── other["tool_count"] = 2
  │                 ├── other["mcp_servers"] = ["filesystem"]
  │                 └── other["mcp_server_count"] = 1
  │
  ▼
model.RecordConsumeLog → logs.other (JSON 字段入库)
```

---

## 5. 环境变量配置

| 变量 | 必须 | 默认值 | 说明 |
|------|------|--------|------|
| `LOG_REQUEST_TOOLS` | ✅ | 不设/空=关闭 | 必须设为 `true` 才启用工具记录 |
| `LOG_TOOL_FILTER_REGEX` | ❌ | 空=记录全部 | Go regexp 正则，只记录匹配的工具名 |

### 正则过滤示例

| 正则 | 效果 |
|------|------|
| `^skill_` | 只记录以 `skill_` 开头的工具 |
| `code_review\|web_search` | 只记录这两个工具 |
| `.*` | 记录所有工具（等同于不设） |
| 空 / 不设 | 记录所有工具 |
| 无效正则（编译失败） | 静默回退为记录所有工具 |

---

## 6. 日志输出格式

`logs.other` JSON 字段中新增的内容：

```json
{
  "model_ratio": 1.0,
  "group_ratio": 1.0,
  "tools": ["skill_code_review", "web_search", "filesystem_read"],
  "tool_count": 3,
  "mcp_servers": ["filesystem", "github"],
  "mcp_server_count": 2,
  "... 其他原有字段 ..."
}
```

---

## 7. 数据库查询示例

### MySQL (>= 5.7.8)

```sql
-- 查询包含工具调用的日志
SELECT * FROM logs WHERE other->'$.tools' IS NOT NULL;

-- 统计各工具调用次数
SELECT jt.tool_name, COUNT(*) AS call_count
FROM logs,
     JSON_TABLE(other->'$.tools', '$[*]' COLUMNS(tool_name VARCHAR(255) PATH '$')) AS jt
GROUP BY jt.tool_name
ORDER BY call_count DESC;

-- 正则过滤查特定技能
SELECT * FROM logs WHERE JSON_CONTAINS(other->'$.tools', '"skill_code_review"');
```

### PostgreSQL (>= 9.6)

```sql
-- 查询包含工具调用的日志
SELECT * FROM logs WHERE other->>'tools' IS NOT NULL;

-- 统计各工具调用次数
SELECT jsonb_array_elements_text(other->'tools') AS tool_name, COUNT(*) AS call_count
FROM logs
WHERE other->'tools' IS NOT NULL
GROUP BY tool_name
ORDER BY call_count DESC;
```

### SQLite

```sql
-- 查询包含工具调用的日志
SELECT * FROM logs WHERE json_extract(other, '$.tools') IS NOT NULL;

-- 统计各工具调用次数
SELECT json_each.value AS tool_name, COUNT(*) AS call_count
FROM logs, json_each(json_extract(logs.other, '$.tools'))
GROUP BY tool_name
ORDER BY call_count DESC;
```

---

## 8. 部署指南

### 8.1 编译

```bash
cd new-api-main
GOPROXY=https://goproxy.cn,direct CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -ldflags "-s -w" -o new-api
```

### 8.2 Docker 打包

```dockerfile
# Dockerfile.local
FROM calciumion/new-api:latest
COPY new-api /new-api
```

```bash
docker pull calciumion/new-api:latest
docker build -f Dockerfile.local -t new-api:patched .
```

### 8.3 docker-compose.yml 配置

```yaml
services:
  new-api:
    image: new-api:patched  # 替换原镜像
    environment:
      - LOG_REQUEST_TOOLS=true
      - LOG_TOOL_FILTER_REGEX=^skill_   # 可选: 只记录 skill_ 开头的工具
```

### 8.4 版本升级

```bash
docker pull calciumion/new-api:latest          # 拉新版基础镜像
docker build -f Dockerfile.local -t new-api:patched .  # 重新打包（约 5 秒）
docker compose up -d                            # 重启
```

> ⚠️ **升级注意:** 确认 `router/relay-router.go` 中的 `middleware.ToolExtractorMiddleware()` 注册行仍然存在，如被上游覆盖只需补回一行。

---

## 9. 性能影响评估

| 场景 | 影响 |
|------|------|
| `LOG_REQUEST_TOOLS` 未设/非 true | **零开销** — 中间件和 `appendToolInfo` 均在第一行 return |
| `LOG_REQUEST_TOOLS=true`，小 body | 极低 — `gjson.GetBytes` 是零拷贝解析 |
| `LOG_REQUEST_TOOLS=true`，大 body (100+ tools) | 低 — 仅遍历 name 字段，不做完整反序列化 |
| 正则编译 | 每次请求编译一次（可优化为缓存，当前开销可忽略） |

---

## 10. 架构关系图

```
                    ┌─────────────────────────┐
                    │     ToolExtractor       │
                    │     Middleware          │
                    │  (gjson → Context)      │
                    └───────────┬─────────────┘
                                │ ctx[ContextKeyOriginalTools]
                                ▼
┌──────────────┐    ┌─────────────────────────┐    ┌──────────────────┐
│ Claude       │    │  GenerateTextOtherInfo  │    │  logs.other      │
│ Handler      │───▶│  → appendToolInfo()     │───▶│  (JSON in DB)    │
│ (OriginalTools│    │  → 双源读取 + 正则过滤  │    │                  │
│  + McpServers)│    └─────────────────────────┘    └──────────────────┘
└──────────────┘
  源1 (优先)                    汇聚点                     持久化
```

---

## 11. 新增 import 汇总

| 文件 | 新增 import |
|------|------------|
| `middleware/tool_extractor.go` | `os`, `gjson`, `common`, `constant` |
| `service/log_info_generate.go` | `os`, `regexp` |
| `relay/claude_handler.go` | 无新增（使用已有的 `common.Marshal`） |
| `relay/common/relay_info.go` | `encoding/json`（已有） |