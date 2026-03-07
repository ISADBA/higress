# 需求文档

## 介绍

为 Gemini provider 添加自定义 URL 支持，允许用户通过配置参数指定自定义的 Gemini 服务地址，以便使用代理服务或镜像服务，而不是默认的官方域名 (generativelanguage.googleapis.com)。

该功能参考了 OpenAI 和 vLLM provider 中已有的 `openaiCustomUrl` 和 `vllmCustomUrl` 实现模式，为 Gemini provider 提供相同的灵活性。

## 术语表

- **Gemini_Provider**: AI Proxy 插件中处理 Google Gemini API 请求的提供商实现
- **Custom_URL**: 用户配置的自定义服务地址，用于替代默认的 Gemini 官方域名
- **Domain**: URL 中的主机部分，例如 "custom.gemini.com"
- **Path_Prefix**: URL 中域名后的路径前缀部分，例如 "/v1/provider"
- **Standard_Path**: 标准的 Gemini API 路径，例如 "/v1/models/gemini-pro:generateContent"
- **ProviderConfig**: AI Proxy 插件中的提供商配置结构体
- **TransformRequestHeaders**: 转换请求头的处理函数
- **Host_Header**: HTTP 请求头中的 Host 字段，指示请求的目标服务器域名
- **421_Error**: HTTP 状态码 421 (Misdirected Request)，表示请求被定向到无法生成响应的服务器，通常由 Host 头与实际服务器不匹配导致
- **Protocol**: 协议模式，可以是 "openai"（将 OpenAI 格式转换为 Gemini 格式）或 "original"（透传原生 Gemini 协议）

## 配置示例

### 示例 1：使用原生 Gemini 协议 + 自定义 URL

```yaml
providers:
  - id: gemini-native
    type: gemini
    protocol: "original"  # 透传原生 Gemini 协议，不做 OpenAI 到 Gemini 的转换
    apiTokens:
      - "your-gemini-api-key"
    geminiCustomUrl: "custom.gemini.com"  # 自定义域名
```

**客户端请求格式**（原生 Gemini 协议）：
```
POST /v1/models/gemini-3.1-pro-preview:generateContent?key=your-api-key
Content-Type: application/json

{
  "contents": [{
    "parts": [{"text": "Hello"}]
  }]
}
```

### 示例 2：使用 OpenAI 协议 + 自定义 URL

```yaml
providers:
  - id: gemini-openai
    type: gemini
    protocol: "openai"  # 或不配置，默认为 openai，会将 OpenAI 格式转换为 Gemini 格式
    apiTokens:
      - "your-gemini-api-key"
    geminiCustomUrl: "custom.gemini.com"  # 自定义域名
```

**客户端请求格式**（OpenAI 协议）：
```
POST /v1/chat/completions
Content-Type: application/json

{
  "model": "gemini-3.1-pro-preview",
  "messages": [
    {"role": "user", "content": "Hello"}
  ]
}
```

### 示例 3：使用默认官方域名（不配置 geminiCustomUrl）

```yaml
providers:
  - id: gemini-default
    type: gemini
    protocol: "original"  # 可选，默认为 openai
    apiTokens:
      - "your-gemini-api-key"
    # 不配置 geminiCustomUrl，使用默认域名 generativelanguage.googleapis.com
```

### 示例 4：带自定义路径前缀

```yaml
providers:
  - id: gemini-proxy
    type: gemini
    protocol: "original"
    apiTokens:
      - "your-gemini-api-key"
    geminiCustomUrl: "https://proxy.example.com/gemini/api"  # 带路径前缀
```

**效果**：
- 请求将发送到 `proxy.example.com`
- Host 头设置为 `proxy.example.com`
- 路径将拼接为：自定义路径前缀 + 标准 API 路径
- 例如：`/gemini/api/v1/models/gemini-pro:generateContent`

### 路径处理规则说明

系统采用统一的"域名替换 + 路径拼接"策略：

1. **仅配置域名**：`aaa.xxx.com`
   - 用户请求：`/v1/models/xxxx:generateContent`
   - 最终请求：`aaa.xxx.com/v1/models/xxxx:generateContent`
   - （只替换域名，保持原路径）

2. **配置域名+端口**：`aaa.xxx.com:8080`
   - 用户请求：`/v1/models/xxxx:generateContent`
   - 最终请求：`aaa.xxx.com:8080/v1/models/xxxx:generateContent`
   - （替换域名和端口，保持原路径）

3. **配置域名+路径前缀**：`aaa.xxx.com/v1/provider`
   - 用户请求：`/v1/models/xxxx:generateContent`
   - 最终请求：`aaa.xxx.com/v1/provider/v1/models/xxxx:generateContent`
   - （替换域名，拼接：自定义路径前缀 + 原路径）

## 需求

### 需求 1: 配置参数支持

**用户故事:** 作为 Higress 用户，我希望能够配置自定义的 Gemini 服务 URL，以便使用代理服务或镜像服务访问 Gemini API。

#### 验收标准

1. THE ProviderConfig SHALL 添加 `geminiCustomUrl` 字段用于存储自定义 URL 配置
2. WHEN 用户在配置中提供 `geminiCustomUrl` 参数时，THE System SHALL 解析并存储该配置值
3. THE ProviderConfig SHALL 提供 `GetGeminiCustomUrl()` 方法返回配置的自定义 URL
4. WHEN `geminiCustomUrl` 未配置时，THE System SHALL 使用默认域名 "generativelanguage.googleapis.com"

### 需求 2: URL 解析和处理

**用户故事:** 作为开发者，我希望系统能够正确解析自定义 URL，提取域名和路径前缀信息，以便正确构建请求。

#### 验收标准

1. WHEN 解析自定义 URL 时，THE System SHALL 移除 "http://" 和 "https://" 协议前缀
2. WHEN 自定义 URL 包含路径时，THE System SHALL 将 URL 分割为域名和路径前缀两部分
3. WHEN 自定义 URL 仅包含域名时，THE System SHALL 将路径前缀设置为 "/"
4. WHEN 路径前缀不为 "/" 时，THE System SHALL 将标准 API 路径拼接到路径前缀后
5. WHEN 路径前缀为 "/" 时，THE System SHALL 直接使用标准 API 路径

### 需求 3: 请求头转换和 Host 头匹配

**用户故事:** 作为系统，我需要根据自定义 URL 配置正确设置请求头，特别是 Host 头必须与目标服务器匹配，以便避免 421 错误并将请求路由到正确的服务地址。

#### 验收标准

1. WHEN 配置了自定义域名时，THE TransformRequestHeaders SHALL 使用自定义域名设置请求的 Host 头
2. WHEN 未配置自定义域名时，THE TransformRequestHeaders SHALL 使用默认域名 "generativelanguage.googleapis.com" 设置 Host 头
3. THE TransformRequestHeaders SHALL 确保 Host 头与实际请求的目标域名一致，以避免 421_Error
4. WHEN 路径前缀不为 "/" 时，THE TransformRequestHeaders SHALL 使用路径前缀与标准路径拼接后的完整路径
5. WHEN 路径前缀为 "/" 时，THE TransformRequestHeaders SHALL 直接使用标准 API 路径
6. THE TransformRequestHeaders SHALL 保持 API Key 头的设置不变

### 需求 4: Provider 初始化

**用户故事:** 作为系统，我需要在创建 Gemini provider 实例时正确处理自定义 URL 配置，以便后续请求使用正确的配置。

#### 验收标准

1. WHEN `geminiCustomUrl` 为空时，THE CreateProvider SHALL 创建使用默认配置的 provider 实例
2. WHEN `geminiCustomUrl` 不为空时，THE CreateProvider SHALL 解析 URL 并提取域名和路径前缀
3. WHEN 创建 provider 实例时，THE CreateProvider SHALL 存储解析后的 customDomain 和 customPath 字段
4. THE CreateProvider SHALL 正确设置 capabilities 映射

### 需求 5: 421 错误规避

**用户故事:** 作为用户，当我使用自定义域名时，我希望系统能够正确设置 Host 头，以便避免因 Host 头不匹配导致的 421 错误。

#### 验收标准

1. WHEN 使用自定义域名时，THE System SHALL 将 Host 头设置为自定义域名而非官方域名
2. IF Host 头设置为官方域名但请求发送到自定义域名，THEN THE System SHALL 可能收到 421_Error
3. THE System SHALL 确保 Host 头始终与实际请求的目标服务器域名一致
4. WHEN 自定义服务器验证 Host 头时，THE System SHALL 提供正确的域名以通过验证
5. THE System SHALL 避免强制覆盖 Host 头为官方域名当使用自定义域名时

### 需求 6: 配置验证

**用户故事:** 作为用户，我希望系统能够验证我的配置是否正确，以便及早发现配置错误。

#### 验收标准

1. WHEN 配置了 `geminiCustomUrl` 时，THE System SHALL 验证 URL 格式的基本有效性
2. WHEN URL 格式无效时，THE System SHALL 记录警告日志
3. THE System SHALL 允许 `geminiCustomUrl` 为空值（使用默认配置）
4. THE System SHALL 在配置解析阶段处理 `geminiCustomUrl` 参数

### 需求 7: 向后兼容性

**用户故事:** 作为现有用户，我希望在不配置自定义 URL 的情况下，系统行为保持不变，以便平滑升级。

#### 验收标准

1. WHEN `geminiCustomUrl` 未配置时，THE System SHALL 使用默认域名 "generativelanguage.googleapis.com"
2. WHEN `geminiCustomUrl` 未配置时，THE System SHALL 使用标准的 Gemini API 路径
3. THE System SHALL 保持现有的 API Key 认证机制不变
4. THE System SHALL 保持现有的请求和响应处理逻辑不变

### 需求 8: 测试覆盖

**用户故事:** 作为开发者，我需要完整的测试用例来验证自定义 URL 功能的正确性，以便确保功能稳定可靠。

#### 验收标准

1. THE Test_Suite SHALL 包含仅配置域名的测试用例
2. THE Test_Suite SHALL 包含配置域名和端口的测试用例
3. THE Test_Suite SHALL 包含配置域名和路径前缀的测试用例
4. THE Test_Suite SHALL 包含请求头转换的测试用例，验证 Host 头与目标域名匹配
5. THE Test_Suite SHALL 包含未配置自定义 URL 时的默认行为测试用例
6. THE Test_Suite SHALL 验证与 OpenAI 和 vLLM provider 的实现一致性
7. THE Test_Suite SHALL 包含验证 Host 头正确设置以避免 421_Error 的测试用例
8. THE Test_Suite SHALL 包含路径拼接逻辑的测试用例，验证路径前缀与标准路径的正确拼接

