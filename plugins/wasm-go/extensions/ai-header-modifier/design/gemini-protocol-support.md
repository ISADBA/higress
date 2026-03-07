# Gemini 协议支持 - 设计文档

## 概述

本设计文档描述了如何增强 ai-header-modifier 插件以支持 Google Gemini 原生 API 协议。Gemini API 使用独特的请求格式，其中模型名称嵌入在 URL 路径中，API 密钥通过查询参数传递。此增强功能将使插件能够检测这种协议格式，提取必要信息，并将其转换为 Higress AI 网关使用的标准请求头格式。

### 设计目标

- 自动检测 Gemini 原生协议请求
- 从 URL 路径中提取模型名称
- 从查询参数中提取 API 密钥和提供商信息
- 向请求体添加模型属性以确保下游兼容性
- 保持与现有插件功能的完全向后兼容性

### 非目标

- 不修改 Gemini API 的响应格式
- 不实现 Gemini 特定的请求验证逻辑
- 不处理 Gemini 协议的流式响应

## 架构

### 高层架构

插件将在现有的请求处理流程中添加 Gemini 协议检测和处理逻辑：

```
请求到达
    ↓
处理自定义请求头（静态、固定源、优先级源）
    ↓
检查是否有请求体
    ↓
检查路径是否匹配启用后缀
    ↓
[新增] 检测 Gemini 协议（路径以 /v1/models/ 开头）
    ↓
    ├─→ 是 Gemini 协议
    │       ↓
    │   从路径提取模型名称
    │       ↓
    │   从查询参数提取 API 密钥和提供商
    │       ↓
    │   设置请求头
    │       ↓
    │   修改请求体（添加 model 属性）
    │
    └─→ 非 Gemini 协议
            ↓
        现有处理逻辑（JSON/Multipart）
```

### 处理流程

1. **请求头处理阶段** (`onHttpRequestHeaders`)
   - 执行现有的自定义请求头处理
   - 检测 Gemini 协议（路径以 `/v1/models/` 开头）
   - 如果是 Gemini 协议：
     - 解析 URL 路径和查询参数
     - 从路径提取模型名称
     - 从查询参数提取 API 密钥和提供商
     - 设置相应的请求头
   - 如果不是 Gemini 协议：
     - 继续现有的路径后缀检查逻辑

2. **请求体处理阶段** (`onHttpRequestBody`)
   - 如果是 Gemini 协议：
     - 解析 JSON 请求体
     - 如果请求体不包含 `model` 属性，则添加
     - 根据提供商值格式化 model 属性
   - 如果不是 Gemini 协议：
     - 使用现有的 JSON/Multipart 处理逻辑

## 组件和接口

### 新增函数

#### 1. `isGeminiProtocol(path string) bool`

检测请求是否使用 Gemini 原生协议。

**输入：**
- `path`: 完整的请求路径（包含查询参数）

**输出：**
- `bool`: 如果路径以 `/v1/models/` 开头则返回 true

**实现逻辑：**
```go
func isGeminiProtocol(path string) bool {
    // 提取 URI 部分（去除查询参数）
    uri := path
    if idx := strings.Index(path, "?"); idx != -1 {
        uri = path[:idx]
    }
    return strings.HasPrefix(uri, "/v1/models/")
}
```

#### 2. `extractModelFromPath(path string) string`

从 Gemini 协议路径中提取模型名称。

**输入：**
- `path`: 请求路径（例如 `/v1/models/gemini-3.1-pro-preview:generateContent`）

**输出：**
- `string`: 提取的模型名称（例如 `gemini-3.1-pro-preview`）

**实现逻辑：**
```go
func extractModelFromPath(path string) string {
    // 去除查询参数
    uri := path
    if idx := strings.Index(path, "?"); idx != -1 {
        uri = path[:idx]
    }
    
    // 路径格式: /v1/models/{model}:operation 或 /v1/models/{model}/operation
    prefix := "/v1/models/"
    if !strings.HasPrefix(uri, prefix) {
        return ""
    }
    
    // 提取模型名称部分
    modelPart := uri[len(prefix):]
    
    // 查找 : 或 / 作为模型名称的结束标记
    endIdx := len(modelPart)
    if idx := strings.IndexAny(modelPart, ":/"); idx != -1 {
        endIdx = idx
    }
    
    return modelPart[:endIdx]
}
```

#### 3. `parseQueryParams(queryString string) map[string]string`

解析 URL 查询字符串为键值对映射。

**输入：**
- `queryString`: 查询字符串（例如 `key=abc123&provider=gemini`）

**输出：**
- `map[string]string`: 查询参数的键值对映射

**实现逻辑：**
```go
func parseQueryParams(queryString string) map[string]string {
    params := make(map[string]string)
    if queryString == "" {
        return params
    }
    
    // 分割参数对
    pairs := strings.Split(queryString, "&")
    for _, pair := range pairs {
        // 分割键值
        kv := strings.SplitN(pair, "=", 2)
        if len(kv) == 2 {
            // URL 解码值
            key := kv[0]
            value := kv[1]
            // 简单的 URL 解码（处理 %XX 格式）
            value = strings.ReplaceAll(value, "+", " ")
            params[key] = value
        }
    }
    
    return params
}
```

#### 4. `processGeminiProtocol(config AiHeaderModifierConfig, path string, log log.Log)`

处理 Gemini 协议请求，提取信息并设置请求头。

**输入：**
- `config`: 插件配置
- `path`: 完整的请求路径
- `log`: 日志记录器

**输出：**
- 无（通过副作用设置请求头）

**实现逻辑：**
```go
func processGeminiProtocol(config AiHeaderModifierConfig, path string, log log.Log) {
    log.Debug("Detected Gemini native protocol")
    
    // 提取模型名称
    modelName := extractModelFromPath(path)
    if modelName == "" {
        log.Warn("Failed to extract model name from Gemini protocol path")
    } else {
        log.Debugf("Extracted model name: %s", modelName)
        // 设置模型请求头
        if config.ModelToHeader != "" {
            err := proxywasm.ReplaceHttpRequestHeader(config.ModelToHeader, modelName)
            if err != nil {
                log.Warnf("Failed to set model header: %v", err)
            } else {
                log.Debugf("Set header %s=%s", config.ModelToHeader, modelName)
            }
        }
    }
    
    // 解析查询参数
    queryString := ""
    if idx := strings.Index(path, "?"); idx != -1 {
        queryString = path[idx+1:]
    }
    
    params := parseQueryParams(queryString)
    
    // 提取 API 密钥
    if apiKey, ok := params["key"]; ok && apiKey != "" {
        log.Debug("Extracted API key from query parameter")
        
        // 设置 x-mse-consumer-apikey
        err := proxywasm.ReplaceHttpRequestHeader("x-mse-consumer-apikey", apiKey)
        if err != nil {
            log.Warnf("Failed to set x-mse-consumer-apikey header: %v", err)
        } else {
            log.Debug("Set header x-mse-consumer-apikey")
        }
        
        // 检查 x-api-key 是否存在或为空
        existingApiKey, err := proxywasm.GetHttpRequestHeader("x-api-key")
        if err != nil || existingApiKey == "" {
            // x-api-key 不存在或为空，设置它
            err := proxywasm.ReplaceHttpRequestHeader("x-api-key", apiKey)
            if err != nil {
                log.Warnf("Failed to set x-api-key header: %v", err)
            } else {
                log.Debug("Set header x-api-key")
            }
        } else {
            log.Debug("x-api-key header already exists, skipping")
        }
    } else {
        log.Warn("API key not found in query parameters")
    }
    
    // 提取提供商
    provider := config.DefaultProvider
    if p, ok := params["provider"]; ok && p != "" {
        provider = p
        log.Debugf("Using provider from query parameter: %s", provider)
    } else {
        log.Debugf("Using default provider: %s", provider)
    }
    
    // 设置提供商请求头
    if config.AddProviderHeader != "" {
        err := proxywasm.ReplaceHttpRequestHeader(config.AddProviderHeader, provider)
        if err != nil {
            log.Warnf("Failed to set provider header: %v", err)
        } else {
            log.Debugf("Set header %s=%s", config.AddProviderHeader, provider)
        }
    }
}
```

#### 5. `addModelToBody(body []byte, modelName string, provider string, log log.Log) []byte`

向 Gemini 请求体添加 model 属性。

**输入：**
- `body`: 原始请求体
- `modelName`: 模型名称
- `provider`: 提供商名称
- `log`: 日志记录器

**输出：**
- `[]byte`: 修改后的请求体

**实现逻辑：**
```go
func addModelToBody(body []byte, modelName string, provider string, log log.Log) []byte {
    // 验证 JSON
    if !gjson.ValidBytes(body) {
        log.Warn("Invalid JSON body, skipping model addition")
        return body
    }
    
    // 检查是否已存在 model 属性
    if gjson.GetBytes(body, "model").Exists() {
        log.Debug("Model property already exists in body, skipping")
        return body
    }
    
    // 构造 model 值
    modelValue := modelName
    if provider != "default" {
        modelValue = provider + "/" + modelName
    }
    
    // 添加 model 属性
    newBody, err := sjson.SetBytes(body, "model", modelValue)
    if err != nil {
        log.Warnf("Failed to add model property to body: %v", err)
        return body
    }
    
    log.Debugf("Added model property to body: %s", modelValue)
    return newBody
}
```

### 修改现有函数

#### `onHttpRequestHeaders`

需要在现有逻辑中添加 Gemini 协议检测：

```go
func onHttpRequestHeaders(ctx wrapper.HttpContext, config AiHeaderModifierConfig, log log.Log) types.Action {
    // 处理自定义请求头（保持不变）
    processStaticHeaders(config, log)
    processFixedSourceHeaders(config, log)
    processPrioritySourceHeaders(config, log)
    
    // 获取请求路径
    path, err := proxywasm.GetHttpRequestHeader(":path")
    if err != nil {
        log.Warnf("Failed to get request path: %v", err)
        return types.ActionContinue
    }
    
    // [新增] 检测 Gemini 协议
    if isGeminiProtocol(path) {
        processGeminiProtocol(config, path, log)
        
        // 检查是否有请求体
        if !ctx.HasRequestBody() {
            log.Debug("No request body, skipping body processing")
            return types.ActionContinue
        }
        
        // 标记为 Gemini 模式
        config.mode = ModeJSON // Gemini 使用 JSON 格式
        ctx.SetContext("config", config)
        ctx.SetContext("isGemini", true)
        
        // 移除 content-length 以允许请求体缓冲
        proxywasm.RemoveHttpRequestHeader("content-length")
        return types.HeaderStopIteration
    }
    
    // 现有的路径后缀检查逻辑（保持不变）
    // ...
}
```

#### `onHttpRequestBody`

需要添加 Gemini 协议的请求体处理：

```go
func onHttpRequestBody(ctx wrapper.HttpContext, config AiHeaderModifierConfig, body []byte, log log.Log) types.Action {
    // 从上下文获取配置
    storedConfig, ok := ctx.GetContext("config").(AiHeaderModifierConfig)
    if !ok {
        log.Warn("Failed to retrieve config from context")
        return types.ActionContinue
    }
    
    // [新增] 检查是否为 Gemini 协议
    if isGemini, ok := ctx.GetContext("isGemini").(bool); ok && isGemini {
        // 获取模型名称和提供商
        modelName, _ := proxywasm.GetHttpRequestHeader(storedConfig.ModelToHeader)
        provider, _ := proxywasm.GetHttpRequestHeader(storedConfig.AddProviderHeader)
        
        if modelName != "" {
            newBody := addModelToBody(body, modelName, provider, log)
            if len(newBody) > 0 && len(newBody) != len(body) {
                err := proxywasm.ReplaceHttpRequestBody(newBody)
                if err != nil {
                    log.Warnf("Failed to replace request body: %v", err)
                }
            }
        }
        return types.ActionContinue
    }
    
    // 现有的处理逻辑（保持不变）
    switch storedConfig.mode {
    case ModeJSON:
        processJSONBody(storedConfig, body, log)
    case ModeMultipart:
        processMultipartBody(storedConfig, body, log)
    default:
        log.Debug("Bypass mode, no processing needed")
    }
    
    return types.ActionContinue
}
```

## 数据模型

### URL 路径格式

Gemini 协议使用以下 URL 格式：

```
/v1/models/{model-name}:{operation}?key={api-key}&provider={provider}
```

**组成部分：**
- **路径前缀**: `/v1/models/` - 固定前缀，用于识别 Gemini 协议
- **模型名称**: `{model-name}` - 模型标识符（例如 `gemini-3.1-pro-preview`）
- **操作**: `:{operation}` - API 操作（例如 `:generateContent`）
- **查询参数**:
  - `key`: API 密钥（必需）
  - `provider`: 提供商标识符（可选）

**示例：**
```
/v1/models/gemini-3.1-pro-preview:generateContent?key=ak_ezE3snddlIygrxxxxBuaNEtfsd2Ys&provider=gemini
```

### 请求头映射

| 源 | 目标请求头 | 说明 |
|---|---|---|
| URL 路径中的模型名称 | `x-higress-llm-model` | 从路径提取的模型名称 |
| 查询参数 `key` | `x-mse-consumer-apikey` | API 密钥（用于消费者识别） |
| 查询参数 `key` | `x-api-key` | API 密钥（仅当 x-api-key 不存在或为空时设置） |
| 查询参数 `provider` 或配置的默认值 | `x-request-llm-provider` | 提供商标识符 |

### 请求体修改

对于 Gemini 协议请求，如果请求体不包含 `model` 属性，插件将添加该属性：

**添加规则：**
- 如果 `provider == "default"`: `model = "{model-name}"`
- 如果 `provider != "default"`: `model = "{provider}/{model-name}"`

**示例：**

原始请求体：
```json
{
  "contents": [
    {
      "parts": [
        {"text": "Hello"}
      ]
    }
  ]
}
```

修改后（provider = "gemini"）：
```json
{
  "contents": [
    {
      "parts": [
        {"text": "Hello"}
      ]
    }
  ],
  "model": "gemini/gemini-3.1-pro-preview"
}
```

修改后（provider = "default"）：
```json
{
  "contents": [
    {
      "parts": [
        {"text": "Hello"}
      ]
    }
  ],
  "model": "gemini-3.1-pro-preview"
}
```

### 配置结构

现有的 `AiHeaderModifierConfig` 结构无需修改，所有必要的配置字段已存在：

```go
type AiHeaderModifierConfig struct {
    ModelKey           string   // 用于从请求体提取模型的键（现有功能）
    AddProviderHeader  string   // 提供商请求头名称
    DefaultProvider    string   // 默认提供商值
    ModelToHeader      string   // 模型请求头名称
    EnableOnPathSuffix []string // 启用路径后缀（现有功能）
    
    // ... 其他字段保持不变
}
```

### 上下文数据

在请求处理过程中，以下数据将存储在上下文中：

```go
ctx.SetContext("config", config)      // AiHeaderModifierConfig
ctx.SetContext("isGemini", true)      // bool - 标记是否为 Gemini 协议
```


## 正确性属性

*属性是一个特征或行为，应该在系统的所有有效执行中保持为真——本质上是关于系统应该做什么的形式化陈述。属性作为人类可读规范和机器可验证正确性保证之间的桥梁。*

### 属性 1：Gemini 协议检测

*对于任何*请求路径，如果路径的 URI 部分（去除查询参数后）以 "/v1/models/" 开头，则应该被识别为 Gemini 协议；否则不应该被识别为 Gemini 协议。

**验证需求：1.1**

### 属性 2：模型名称提取

*对于任何*符合 Gemini 协议格式的路径（`/v1/models/{model}:{operation}` 或 `/v1/models/{model}/{operation}`），提取的模型名称应该等于 "/v1/models/" 之后、第一个 ":" 或 "/" 字符之前的字符串。

**验证需求：2.1**

### 属性 3：提取信息到请求头的映射

*对于任何*有效的 Gemini 协议请求，如果成功从路径提取模型名称，则 "x-higress-llm-model" 请求头应该被设置为该模型名称；如果成功从查询参数提取 API 密钥，则 "x-mse-consumer-apikey" 请求头应该被设置为该密钥值；如果存在 provider 查询参数或配置了默认提供商，则 "x-request-llm-provider" 请求头应该被设置为相应的提供商值。

**验证需求：2.2, 3.2, 4.1, 4.2**

### 属性 4：提取失败的容错性

*对于任何*无效格式的 Gemini 协议路径（无法提取模型名称），系统应该继续处理而不崩溃，且不应该设置模型相关的请求头。

**验证需求：2.3**

### 属性 5：API 密钥缺失的容错性

*对于任何*不包含 "key" 查询参数的 Gemini 协议请求，系统应该继续处理而不崩溃，且不应该设置 "x-mse-consumer-apikey" 请求头。

**验证需求：3.3**

### 属性 6：提供商参数处理

*对于任何*Gemini 协议请求，如果查询参数包含 "provider"，则应该使用该值；如果不包含 "provider" 但配置了 defaultProvider，则应该使用配置值；如果两者都不存在，则应该使用 "default"。

**验证需求：4.1, 4.2, 4.3**

### 属性 7：请求体 model 属性添加

*对于任何*有效的 JSON 格式的 Gemini 协议请求体，如果不包含 "model" 属性，则处理后应该包含该属性；如果已包含 "model" 属性，则该属性值应该保持不变。

**验证需求：5.1, 5.4**

### 属性 8：model 属性格式化

*对于任何*需要添加 model 属性的 Gemini 请求体，如果提供商为 "default"，则 model 值应该等于模型名称；如果提供商不为 "default"，则 model 值应该等于 "{提供商}/{模型名称}" 格式。

**验证需求：5.2, 5.3**

### 属性 9：无效 JSON 的容错性

*对于任何*非有效 JSON 格式的请求体，系统应该继续处理而不崩溃，且不应该修改请求体。

**验证需求：5.5**

### 属性 10：URL 解析正确性

*对于任何*包含查询参数的 URL 路径，应该在 "?" 字符处正确分离为 URI 部分和查询字符串部分，并且查询字符串应该被正确解析为由 "&" 分隔的键值对映射。

**验证需求：6.1, 6.2**

### 属性 11：自定义请求头处理的独立性

*对于任何*Gemini 协议请求，如果配置了静态请求头、固定源请求头或优先级源请求头，这些请求头应该被正确处理和设置，不受 Gemini 协议处理的影响。

**验证需求：7.2**

## 错误处理

### 错误场景和处理策略

1. **无效的路径格式**
   - 场景：路径以 `/v1/models/` 开头但无法提取模型名称
   - 处理：记录警告日志，继续处理，不设置模型请求头
   - 影响：请求继续转发，但可能缺少路由所需的请求头

2. **缺失 API 密钥**
   - 场景：查询参数中不包含 `key` 参数
   - 处理：记录警告日志，继续处理，不设置 API 密钥请求头
   - 影响：下游服务可能因缺少认证信息而拒绝请求

3. **无效的 JSON 请求体**
   - 场景：请求体不是有效的 JSON 格式
   - 处理：记录警告日志，跳过请求体修改，保持原始请求体
   - 影响：请求体不被修改，可能缺少 model 属性

4. **请求头设置失败**
   - 场景：调用 `proxywasm.ReplaceHttpRequestHeader` 失败
   - 处理：记录警告日志，继续处理其他请求头
   - 影响：部分请求头可能未设置，但不影响其他处理

5. **请求体修改失败**
   - 场景：调用 `sjson.SetBytes` 或 `proxywasm.ReplaceHttpRequestBody` 失败
   - 处理：记录警告日志，保持原始请求体
   - 影响：请求体不被修改

### 错误日志级别

- **Debug**: 正常的处理流程信息（协议检测、值提取、请求头设置）
- **Warn**: 非致命错误（提取失败、参数缺失、设置失败）
- **Error**: 不使用（所有错误都是可恢复的）

### 安全考虑

1. **API 密钥日志**：在日志中不记录实际的 API 密钥值，仅记录"已提取 API 密钥"
2. **输入验证**：对提取的值不进行额外验证，依赖下游服务进行验证
3. **错误信息**：错误日志不包含敏感信息

## 测试策略

### 双重测试方法

本功能将采用单元测试和基于属性的测试相结合的方法：

- **单元测试**：验证特定示例、边缘情况和错误条件
- **基于属性的测试**：验证所有输入的通用属性

两者是互补的，对于全面覆盖都是必要的。

### 单元测试

单元测试专注于：
- 具体示例（如需求 9 中的真实 Gemini API 请求）
- 边缘情况（URL 编码字符、空值、特殊字符）
- 错误条件（无效 JSON、缺失参数、格式错误的路径）
- 与现有功能的集成点

**单元测试用例：**

1. **Gemini 协议检测**
   - 测试以 `/v1/models/` 开头的路径被识别
   - 测试不以 `/v1/models/` 开头的路径不被识别
   - 测试带查询参数的路径正确识别

2. **模型名称提取**
   - 测试标准格式：`/v1/models/gemini-pro:generateContent`
   - 测试带斜杠的格式：`/v1/models/gemini-pro/generate`
   - 测试无操作后缀：`/v1/models/gemini-pro`
   - 测试复杂模型名称：`/v1/models/gemini-3.1-pro-preview:generateContent`

3. **查询参数解析**
   - 测试单个参数：`?key=abc123`
   - 测试多个参数：`?key=abc123&provider=gemini`
   - 测试 URL 编码：`?key=abc%2B123`
   - 测试空值：`?key=&provider=gemini`

4. **请求头设置**
   - 测试模型请求头设置
   - 测试 API 密钥请求头设置
   - 测试提供商请求头设置（显式和默认）

5. **请求体修改**
   - 测试添加 model 属性（provider = "default"）
   - 测试添加 model 属性（provider != "default"）
   - 测试已存在 model 属性时不修改
   - 测试无效 JSON 时不修改

6. **端到端示例**（需求 9）
   - 测试完整的 Gemini API 请求处理流程
   - 验证所有请求头和请求体修改

7. **向后兼容性**
   - 测试非 Gemini 请求使用现有逻辑
   - 测试自定义请求头在 Gemini 请求中仍然生效

### 基于属性的测试

基于属性的测试将使用 Go 的 `testing/quick` 包或第三方库（如 `gopter`）。

**配置：**
- 每个属性测试最少 100 次迭代
- 每个测试必须引用其设计文档属性
- 标签格式：`Feature: gemini-protocol-support, Property {number}: {property_text}`

**属性测试用例：**

1. **属性 1：Gemini 协议检测**
   ```go
   // Feature: gemini-protocol-support, Property 1: Gemini 协议检测
   // 对于任何请求路径，如果路径的 URI 部分以 "/v1/models/" 开头，
   // 则应该被识别为 Gemini 协议
   func TestProperty_GeminiProtocolDetection(t *testing.T) {
       // 生成随机路径，验证检测逻辑
   }
   ```

2. **属性 2：模型名称提取**
   ```go
   // Feature: gemini-protocol-support, Property 2: 模型名称提取
   // 对于任何符合 Gemini 协议格式的路径，提取的模型名称应该正确
   func TestProperty_ModelNameExtraction(t *testing.T) {
       // 生成随机模型名称和操作，构造路径，验证提取结果
   }
   ```

3. **属性 3：提取信息到请求头的映射**
   ```go
   // Feature: gemini-protocol-support, Property 3: 提取信息到请求头的映射
   // 对于任何有效的 Gemini 协议请求，提取的信息应该正确设置到请求头
   func TestProperty_HeaderMapping(t *testing.T) {
       // 生成随机的模型名称、API 密钥、提供商，验证请求头设置
   }
   ```

4. **属性 7：请求体 model 属性添加**
   ```go
   // Feature: gemini-protocol-support, Property 7: 请求体 model 属性添加
   // 对于任何有效的 JSON 请求体，如果不包含 model 属性则添加，
   // 如果已包含则不修改
   func TestProperty_ModelPropertyAddition(t *testing.T) {
       // 生成随机的 JSON 请求体，验证 model 属性处理
   }
   ```

5. **属性 8：model 属性格式化**
   ```go
   // Feature: gemini-protocol-support, Property 8: model 属性格式化
   // 对于任何需要添加 model 属性的请求体，格式应该根据提供商正确
   func TestProperty_ModelPropertyFormatting(t *testing.T) {
       // 生成随机的模型名称和提供商，验证格式化逻辑
   }
   ```

6. **属性 10：URL 解析正确性**
   ```go
   // Feature: gemini-protocol-support, Property 10: URL 解析正确性
   // 对于任何包含查询参数的 URL，应该正确分离和解析
   func TestProperty_URLParsing(t *testing.T) {
       // 生成随机的 URI 和查询参数，验证解析结果
   }
   ```

### 测试数据生成器

为基于属性的测试创建以下生成器：

1. **路径生成器**：生成各种格式的 URL 路径
   - Gemini 协议路径（有效和无效格式）
   - 非 Gemini 协议路径
   - 带和不带查询参数的路径

2. **模型名称生成器**：生成各种模型名称
   - 简单名称：`gemini-pro`
   - 复杂名称：`gemini-3.1-pro-preview`
   - 包含特殊字符的名称（用于边缘情况）

3. **查询参数生成器**：生成查询字符串
   - 单个和多个参数
   - 包含和不包含 URL 编码
   - 空值和特殊字符

4. **JSON 请求体生成器**：生成各种 JSON 结构
   - 有效的 Gemini 请求体（带和不带 model 属性）
   - 空对象和复杂嵌套结构
   - 无效的 JSON（用于错误测试）

### 测试覆盖率目标

- 代码覆盖率：>= 85%
- 分支覆盖率：>= 80%
- 所有正确性属性都有对应的属性测试
- 所有错误场景都有对应的单元测试

### 集成测试

除了单元测试和属性测试，还应该进行集成测试：

1. **与现有功能的集成**
   - 验证 Gemini 协议处理不影响现有的 JSON/Multipart 处理
   - 验证自定义请求头配置在 Gemini 请求中正常工作

2. **端到端测试**
   - 使用真实的 Gemini API 请求格式
   - 验证完整的请求转换流程

3. **性能测试**
   - 验证 Gemini 协议处理不显著增加延迟
   - 测试高并发场景下的稳定性

## 实现注意事项

### 代码组织

建议的代码组织结构：

```go
// main.go
// - 现有的配置和主要处理函数
// - 添加 Gemini 协议相关的辅助函数

// 新增函数（按调用顺序）：
// 1. isGeminiProtocol(path string) bool
// 2. extractModelFromPath(path string) string
// 3. parseQueryParams(queryString string) map[string]string
// 4. processGeminiProtocol(config, path, log)
// 5. addModelToBody(body, modelName, provider, log) []byte

// 修改的函数：
// - onHttpRequestHeaders: 添加 Gemini 协议检测分支
// - onHttpRequestBody: 添加 Gemini 协议请求体处理分支
```

### 性能考虑

1. **字符串操作优化**
   - 使用 `strings.Index` 而不是正则表达式进行路径解析
   - 避免不必要的字符串复制

2. **内存分配**
   - 查询参数映射使用合理的初始容量
   - 重用缓冲区（如果可能）

3. **早期返回**
   - 在检测到非 Gemini 协议时立即返回
   - 在提取失败时避免后续处理

### 向后兼容性

1. **配置兼容性**
   - 不添加新的配置字段
   - 使用现有的配置字段（`ModelToHeader`, `AddProviderHeader`, `DefaultProvider`）

2. **行为兼容性**
   - 非 Gemini 请求的处理逻辑完全不变
   - 自定义请求头处理在所有情况下都执行

3. **API 兼容性**
   - 不修改现有函数签名
   - 新增的函数都是内部辅助函数

### 日志记录最佳实践

1. **日志级别使用**
   - Debug: 正常流程信息（协议检测、值提取、请求头设置）
   - Warn: 可恢复的错误（提取失败、参数缺失）
   - 不使用 Error 级别（所有错误都是可恢复的）

2. **日志内容**
   - 包含足够的上下文信息用于调试
   - 不记录敏感信息（API 密钥值）
   - 使用结构化日志格式

3. **日志示例**
   ```go
   log.Debug("Detected Gemini native protocol")
   log.Debugf("Extracted model name: %s", modelName)
   log.Debug("Extracted API key from query parameter")  // 不记录实际密钥
   log.Debugf("Set header %s=%s", headerName, headerValue)
   log.Warn("Failed to extract model name from Gemini protocol path")
   ```

### 安全考虑

1. **输入验证**
   - 不对提取的值进行额外验证（依赖下游服务）
   - 防止注入攻击（使用安全的字符串操作）

2. **敏感信息处理**
   - API 密钥不记录到日志
   - 错误消息不包含敏感信息

3. **资源限制**
   - 查询参数数量没有限制（依赖 Envoy 的限制）
   - 请求体大小没有限制（依赖 Envoy 的限制）

## 部署和配置

### 配置示例

支持 Gemini 协议的配置示例：

```yaml
modelKey: model
modelToHeader: x-higress-llm-model
addProviderHeader: x-request-llm-provider
defaultProvider: gemini
enableOnPathSuffix:
  - /v1/chat/completions
  - /v1/embeddings
  # Gemini 协议会自动检测，不需要添加到这里
```

### 部署注意事项

1. **配置迁移**
   - 现有配置无需修改即可支持 Gemini 协议
   - 建议设置 `defaultProvider` 为 `gemini` 以获得更好的体验

2. **监控指标**
   - 监控 Gemini 协议请求的数量
   - 监控提取失败的次数
   - 监控请求体修改失败的次数

3. **故障排查**
   - 启用 Debug 日志级别查看详细处理流程
   - 检查请求头是否正确设置
   - 检查请求体是否正确修改

### 文档更新

需要更新以下文档：

1. **README.md**
   - 添加 Gemini 协议支持说明
   - 添加配置示例
   - 添加使用示例

2. **配置文档**
   - 说明 Gemini 协议的自动检测
   - 说明相关配置字段的作用

3. **API 文档**
   - 更新插件功能描述
   - 添加 Gemini 协议处理流程图

## 未来增强

以下功能可以在未来版本中考虑：

1. **更多协议支持**
   - 支持其他 LLM 提供商的原生协议
   - 可配置的协议检测规则

2. **高级路由**
   - 基于模型名称的路由规则
   - 基于提供商的负载均衡

3. **请求转换**
   - Gemini 协议到 OpenAI 协议的转换
   - 请求格式标准化

4. **监控和分析**
   - 详细的请求统计
   - 模型使用分析
   - 性能指标收集
