# 功能说明

`ai-header-modifier` 插件从 LLM 请求体中提取模型信息并添加到请求头中,用于路由和流量管理。该插件支持从 JSON 和 multipart/form-data 格式的请求体中提取模型名称,并可选地提取提供商信��。

## 功能特性

- **模型提取**: 从请求体中提取模型名称并添加到指定的请求头
- **提供商提取**: 支持从 `provider/model` 格式的模型名称中提取提供商信息
- **请求体重写**: 提取提供商后,自动重写请求体,将模型名称从 `provider/model` 改为 `model`
- **多格式支持**: 支持 JSON 和 multipart/form-data 两种请求体格式
- **路径过滤**: 仅对指定路径后缀的请求进行处理
- **通配符支持**: 支持使用 `*` 通配符匹配所有路径

## 使用场景

### 场景 1: 基于模型的路由

在 AI 网关中,不同的模型可能需要路由到不同的后端服务。通过将模型信息添加到请求头,可以使用 Higress 的路由规则进行灵活的流量分发。

```yaml
modelKey: model
modelToHeader: x-higress-llm-model
enableOnPathSuffix:
  - /v1/chat/completions
  - /v1/embeddings
```

### 场景 2: 多提供商路由

当使用统一的 API 格式支持多个 LLM 提供商时,可以通过提取提供商信息实现基于提供商的路由。

```yaml
modelKey: model
modelToHeader: x-higress-llm-model
addProviderHeader: x-higress-llm-provider
enableOnPathSuffix:
  - /v1/chat/completions
```

请求示例:
```json
{
  "model": "openai/gpt-4",
  "messages": [{"role": "user", "content": "Hello"}]
}
```

处理后:
- 添加请求头: `x-higress-llm-model: openai/gpt-4`
- 添加请求头: `x-higress-llm-provider: openai`
- 请求体中的 model 字段被重写为: `"model": "gpt-4"`

### 场景 3: 全路径处理

使用通配符 `*` 对所有请求进行处理:

```yaml
modelKey: model
modelToHeader: x-model
enableOnPathSuffix:
  - "*"
```

## 配置字段

| 名称 | 数据类型 | 填写要求 | 默认值 | 描述 |
|------|---------|---------|--------|------|
| modelKey | string | 选填 | "model" | 请求体中模型字段的键名 |
| modelToHeader | string | 选填(至少配置一个) | - | 将模型值添加到此请求头 |
| addProviderHeader | string | 选填(至少配置一个) | - | 将提取的提供商信息添加到此请求头 |
| enableOnPathSuffix | array of string | 选填 | [默认路径列表] | 启用插件的路径后缀列表,支持通配符 "*" |

**注意**: `modelToHeader` 和 `addProviderHeader` 至少需要配置一个。

### 默认路径列表

如果不配置 `enableOnPathSuffix`,插件将对以下路径后缀的请求进行处理:

- `/completions`
- `/embeddings`
- `/images/generations`
- `/audio/speech`
- `/fine_tuning/jobs`
- `/moderations`
- `/image-synthesis`
- `/video-synthesis`
- `/rerank`
- `/messages`

## 配置示例

### 示例 1: 仅提取模型名称

```yaml
modelKey: model
modelToHeader: x-higress-llm-model
enableOnPathSuffix:
  - /v1/chat/completions
  - /v1/embeddings
```

### 示例 2: 提取模型和提供商

```yaml
modelKey: model
modelToHeader: x-higress-llm-model
addProviderHeader: x-higress-llm-provider
enableOnPathSuffix:
  - /v1/chat/completions
  - /v1/embeddings
```

### 示例 3: 自定义模型字段名

```yaml
modelKey: llm_model
modelToHeader: x-model
enableOnPathSuffix:
  - "*"
```

### 示例 4: 仅提取提供商

```yaml
modelKey: model
addProviderHeader: x-provider
enableOnPathSuffix:
  - /v1/chat/completions
```

## 处理流程

1. **请求头处理阶段**:
   - 检查请求是否有请求体
   - 验证请求路径是否匹配配置的后缀
   - 检测 Content-Type (JSON 或 multipart/form-data)
   - 缓冲请求体以便后续处理

2. **请求体处理阶段**:
   - 根据 Content-Type 选择相应的处理方式
   - 从请求体中提取模型值
   - 添加配置的请求头
   - 如果配置了 `addProviderHeader` 且模型值包含 `/`,则提取提供商并重写请求体

## 注意事项

1. **性能考虑**: 插件需要缓冲完整的请求体,对于大型请求可能会增加内存使用
2. **错误处理**: 如果处理过程中出现错误(如无效的 JSON),插件会记录警告并继续,不会阻塞请求
3. **提供商格式**: 提供商提取仅在模型值包含 `/` 时生效,格式为 `provider/model`
4. **多个斜杠**: 如果模型值包含多个 `/` (如 `provider/namespace/model`),仅第一个 `/` 前的部分被视为提供商
5. **查询参数**: 路径匹配会自动忽略查询参数

## 兼容性

- 支持 JSON 格式的请求体 (Content-Type: application/json)
- 支持 multipart/form-data 格式的请求体
- 兼容所有主流 LLM API 格式 (OpenAI, Anthropic, Google, 等)

## 自定义 Header 管理功能

除了 AI 模型提取功能外，插件还支持灵活的自定义 header 管理，适用于 MSE（微服务引擎）元数据注入等场景。

### 功能类型

#### 1. 静态 Header (Static Headers)

添加配置的固定值 header，适用于网关实例标识、环境标签等场景。

```yaml
staticHeaders:
  - key: "x-mse-gateway-instance-id"
    value: "gateway-001"
  - key: "x-environment"
    value: "production"
```

#### 2. 固定源映射 (Fixed Source Headers)

从固定来源（Envoy 属性或 pseudo-headers）读取值并写入目标 header。

```yaml
fixedSourceHeaders:
  - source: "authority"          # 来源：:authority pseudo-header
    target: "x-mse-domain-name"  # 目标 header
  - source: "route_name"         # 来源：Envoy 路由名称
    target: "x-mse-router-name"
  - source: "cluster_name"       # 来源：Envoy 集群名称
    target: "x-mse-service-name"
  - source: "consumer_name"      # 来源：认证的消费者名称
    target: "x-mse-consumer-name"
```

**支持的来源：**
- `authority` - `:authority` pseudo-header（域名）
- `route_name` - Envoy 路由名称
- `cluster_name` - Envoy 集群/服务名称
- `consumer_name` - 认证的消费者名称

#### 3. 优先级源列表 (Priority Source Headers)

从多个候选 header 中提取值（按优先级顺序），适用于 API key 提取等场景。

```yaml
prioritySourceHeaders:
  - target: "x-mse-consumer-apikey"
    sources:
      - "authorization"      # 优先级 1
      - "x-api-key"         # 优先级 2
      - "api-key"           # 优先级 3
    stripPrefix: "Bearer "  # 可选：去除前缀
```

**特性：**
- 按优先级顺序尝试每个源 header
- 使用第一个找到的非空值
- 支持前缀去除（如从 Authorization header 中去除 "Bearer "）

### 使用场景

#### 场景 3: MSE 元数据注入

为所有请求添加 MSE 相关的元数据 header，用于可观测性、计费、路由等。

```yaml
staticHeaders:
  - key: "x-mse-gateway-instance-id"
    value: "gateway-001"

fixedSourceHeaders:
  - source: "authority"
    target: "x-mse-domain-name"
  - source: "route_name"
    target: "x-mse-router-name"
  - source: "cluster_name"
    target: "x-mse-service-name"

prioritySourceHeaders:
  - target: "x-mse-consumer-apikey"
    sources: ["authorization", "x-api-key"]
    stripPrefix: "Bearer "
```

#### 场景 4: AI + MSE 混合使用

同时使用 AI 模型提取和 MSE 元数据注入功能。

```yaml
# AI 配置
modelKey: "model"
modelToHeader: "x-higress-llm-model"
addProviderHeader: "x-higress-llm-provider"
enableOnPathSuffix:
  - "/v1/chat/completions"

# MSE 配置
staticHeaders:
  - key: "x-mse-gateway-instance-id"
    value: "gateway-001"

fixedSourceHeaders:
  - source: "authority"
    target: "x-mse-domain-name"
  - source: "route_name"
    target: "x-mse-router-name"

prioritySourceHeaders:
  - target: "x-mse-consumer-apikey"
    sources: ["authorization", "x-api-key"]
    stripPrefix: "Bearer "
```

**行为说明：**
- MSE headers 对所有请求生效（不受 `enableOnPathSuffix` 限制）
- AI 模型提取仅对匹配路径的请求生效
- LLM 请求：同时添加 AI headers 和 MSE headers
- 非 LLM 请求：仅添加 MSE headers

### 配置示例文件

插件目录提供了多个配置示例文件：

- `example-config.yaml` - 完整配置示例（包含所有功能和注释）
- `example-config-minimal.yaml` - 最小化配置示例（仅 MSE 功能）
- `example-config-combined.yaml` - AI + MSE 混合配置示例
- `CONFIGURATION_EXAMPLES.md` - 详细的配置说明文档

### API Key 提取示例

#### 示例 1: 从 Authorization Bearer Token 提取

**请求：**
```
Authorization: Bearer sk-1234567890abcdef
```

**配置：**
```yaml
prioritySourceHeaders:
  - target: "x-mse-consumer-apikey"
    sources: ["authorization"]
    stripPrefix: "Bearer "
```

**结果：**
```
x-mse-consumer-apikey: sk-1234567890abcdef
```

#### 示例 2: 多个候选 header

**请求：**
```
x-api-key: sk-xyz789
```

**配置：**
```yaml
prioritySourceHeaders:
  - target: "x-mse-consumer-apikey"
    sources: ["authorization", "x-api-key", "api-key"]
```

**结果：**
```
x-mse-consumer-apikey: sk-xyz789
```

#### 示例 3: 优先级选择

**请求：**
```
authorization: sk-priority1
x-api-key: sk-priority2
```

**结果：** 使用 `sk-priority1`（优先级 1）

