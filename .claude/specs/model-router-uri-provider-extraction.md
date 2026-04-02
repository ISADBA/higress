# Model Router URI Provider Extraction

## 需求概述

支持从 URI 路径中提取 provider 信息，实现零配置的多渠道路由。

## 功能需求

### 1. 配置项

新增配置项 `providers`（字符串列表）：

```yaml
providers:
  - official
  - backup
  - test
```

### 2. URI 提取规则

**匹配模式：** `/{provider}/{remaining_path}`

- 提取路径第一段作为 provider 候选
- 验证是否在 `providers` 列表中
- 不在列表中则跳过处理（放行）

**示例：**
- `/official/v1/messages` → provider: `official`, path: `/v1/messages`
- `/backup/v1beta/chat` → provider: `backup`, path: `/v1beta/chat`
- `/unknown/v1/test` → 不处理（unknown 不在列表）
- `/v1/messages` → 不处理（无 provider 段）

### 3. URI 重写

**时机：** `onHttpRequestHeaders` 阶段

**操作：**
1. 提取 provider 后立即重写 `:path` header
2. 保留 query 参数
3. 存储 provider 到 context

**示例：**
- `/official/v1/messages?key=xxx` → `/v1/messages?key=xxx`

### 4. Model 修改

**时机：** `onHttpRequestBody` 阶段

**操作：**
1. 从 context 获取 provider
2. 提取 body 中的 model 值
3. 检查 model 是否已包含该 provider 前缀
4. 如果没有，则添加 `{provider}/` 前缀
5. 修改 body 中的 model 字段
6. 设置 `x-higress-llm-model` header 触发 reroute

**去重逻辑：**
```
if strings.HasPrefix(model, provider+"/") {
    // 已包含前缀，不处理
} else {
    // 添加前缀
    model = provider + "/" + model
}
```

**示例：**
- provider: `official`, model: `[REDACTED].5` → `official/[REDACTED].5`
- provider: `official`, model: `official/[REDACTED].5` → 保持不变

## 实现要点

### onHttpRequestHeaders 阶段

```go
1. 获取 :path header
2. 解析路径：strings.SplitN(path, "/", 3)
3. 提取第一段作为 provider 候选
4. 验证是否在 config.providers 列表中
5. 如果匹配：
   - 重写 :path（去掉 provider 段，保留 query）
   - 存储 provider 到 ctx.SetContext("uriProvider", provider)
6. 继续后续处理
```

### onHttpRequestBody 阶段

```go
1. 检查 context 中是否有 uriProvider
2. 如果有：
   - 提取 model 值
   - 如果 model 为空（Gemini 原生协议等特殊场景）：
     * 只处理 URI，不修改 body
     * 留 TODO 标记，后续完善
   - 如果 model 存在：
     * 检查是否已有前缀
     * 添加前缀（如需要）
     * 修改 body
     * 设置 x-higress-llm-model
3. 继续现有逻辑（addProviderHeader 等）
```

## 配置示例

```yaml
modelKey: model
modelToHeader: x-higress-llm-model
addProviderHeader: x-higress-llm-provider
providers:
  - official
  - backup
  - test
enableOnPathSuffix:
  - /completions
  - /messages
```

## 测试场景

### 场景 1：正常提取
- 请求：`POST /official/v1/messages`
- Body: `{"model": "[REDACTED].5"}`
- 结果：
  - Path: `/v1/messages`
  - Body: `{"model": "official/[REDACTED].5"}`
  - Header: `x-higress-llm-model: official/[REDACTED].5`

### 场景 2：已有前缀
- 请求：`POST /official/v1/messages`
- Body: `{"model": "official/[REDACTED].5"}`
- 结果：
  - Path: `/v1/messages`
  - Body: `{"model": "official/[REDACTED].5"}` (不变)
  - Header: `x-higress-llm-model: official/[REDACTED].5`

### 场景 3：不在列表
- 请求：`POST /unknown/v1/messages`
- 结果：不处理，原样转发

### 场景 4：保留 query
- 请求：`POST /official/v1/messages?key=xxx&foo=bar`
- 结果：Path: `/v1/messages?key=xxx&foo=bar`

### 场景 5：无 provider 段
- 请求：`POST /v1/messages`
- 结果：不处理，走现有逻辑

### 场景 6：无 model 参数（Gemini 等特殊协议）
- 请求：`POST /official/v1beta/generateContent`
- Body: `{"contents": [...]}`（无 model 字段）
- 结果：
  - Path: `/v1beta/generateContent`
  - Body: 不修改
  - Header: 不设置 x-higress-llm-model
  - 代码留 TODO 标记

## 兼容性

- ✅ 不影响现有功能（autoRouting、addProviderHeader）
- ✅ 可与现有 model 前缀方案共存
- ✅ 配置可选（不配置 providers 则不启用）
