# Implementation Tasks

## Task 1: 在 ProviderConfig 中添加 geminiCustomUrl 字段支持

**Status**: completed

**Description**: 
在 `provider.go` 的 `ProviderConfig` 结构体中添加 `geminiCustomUrl` 字段，并实现相关的 getter 方法和 JSON 解析逻辑。

**Requirements**: 需求 1

**Files to modify**:
- `plugins/wasm-go/extensions/ai-proxy/provider/provider.go`

**Implementation details**:
1. 在 `ProviderConfig` 结构体中添加 `geminiCustomUrl` 字段（约第 315 行）
   ```go
   // @Title zh-CN Gemini 自定义后端 URL
   // @Description zh-CN 仅适用于 Gemini 服务。自定义的 Gemini 服务地址，用于代理或镜像服务
   geminiCustomUrl string `required:"false" yaml:"geminiCustomUrl" json:"geminiCustomUrl"`
   ```

2. 在 `GetVllmCustomUrl` 方法后添加 getter 方法（约第 480 行）
   ```go
   func (c *ProviderConfig) GetGeminiCustomUrl() string {
       return c.geminiCustomUrl
   }
   ```

3. 在 `FromJson` 方法中添加解析逻辑（约第 505 行）
   ```go
   c.geminiCustomUrl = json.Get("geminiCustomUrl").String()
   ```

**Acceptance criteria**:
- ProviderConfig 包含 geminiCustomUrl 字段
- GetGeminiCustomUrl() 方法返回正确的配置值
- FromJson 方法能够正确解析 geminiCustomUrl 参数

---

## Task 2: 扩展 geminiProvider 结构体以支持自定义域名和路径

**Status**: completed

**Description**: 
在 `gemini.go` 的 `geminiProvider` 结构体中添加 `customDomain` 和 `customPath` 字段，用于存储解析后的自定义域名和路径前缀。

**Requirements**: 需求 2, 需求 4

**Files to modify**:
- `plugins/wasm-go/extensions/ai-proxy/provider/gemini.go`

**Implementation details**:
在 `geminiProvider` 结构体中添加新字段（约第 70 行）：
```go
type geminiProvider struct {
    config             ProviderConfig
    contextCache       *contextCache
    client             wrapper.HttpClient
    
    // 新增字段
    customDomain       string  // 自定义域名，如 "custom.gemini.com"
    customPath         string  // 自定义路径，如 "/custom/prefix" 或 "/"
}
```

**Acceptance criteria**:
- geminiProvider 结构体包含 customDomain 和 customPath 字段
- 字段类型为 string

---

## Task 3: 实现 CreateProvider 方法的自定义 URL 解析逻辑

**Status**: completed

**Description**: 
修改 `geminiProviderInitializer.CreateProvider` 方法，添加自定义 URL 的解析逻辑，包括协议前缀移除、域名和路径分割、以及 provider 实例的创建。

**Requirements**: 需求 2, 需求 4

**Dependencies**: Task 1, Task 2

**Files to modify**:
- `plugins/wasm-go/extensions/ai-proxy/provider/gemini.go`

**Implementation details**:
修改 `CreateProvider` 方法（约第 63 行）：

```go
func (g *geminiProviderInitializer) CreateProvider(config ProviderConfig) (Provider, error) {
    // 如果未配置自定义 URL，使用默认配置
    if config.GetGeminiCustomUrl() == "" {
        config.setDefaultCapabilities(g.DefaultCapabilities())
        return &geminiProvider{
            config:       config,
            contextCache: createContextCache(&config),
            client: wrapper.NewClusterClient(wrapper.RouteCluster{
                Host: geminiDomain,
            }),
        }, nil
    }
    
    // 解析自定义 URL
    customUrl := strings.TrimPrefix(strings.TrimPrefix(config.GetGeminiCustomUrl(), "http://"), "https://")
    pairs := strings.SplitN(customUrl, "/", 2)
    customPath := "/"
    if len(pairs) == 2 {
        customPath += pairs[1]
    }
    
    // 设置 capabilities
    config.setDefaultCapabilities(g.DefaultCapabilities())
    
    log.Debugf("ai-proxy: gemini provider customDomain:%s, customPath:%s",
        pairs[0], customPath)
    
    return &geminiProvider{
        config:             config,
        contextCache:       createContextCache(&config),
        client: wrapper.NewClusterClient(wrapper.RouteCluster{
            Host: pairs[0],
        }),
        customDomain:       pairs[0],
        customPath:         customPath,
    }, nil
}
```

**Acceptance criteria**:
- 未配置 geminiCustomUrl 时，使用默认配置创建 provider
- 配置了 geminiCustomUrl 时，正确解析并提取域名和路径前缀
- 协议前缀（http://, https://）被正确移除
- customDomain 和 customPath 字段被正确设置
- 添加调试日志记录解析结果

---

## Task 4: 修改 TransformRequestHeaders 方法以支持自定义 Host 头

**Status**: completed

**Description**: 
修改 `geminiProvider.TransformRequestHeaders` 方法，根据是否配置了自定义域名来设置正确的 Host 头，避免 421 错误。

**Requirements**: 需求 3, 需求 5

**Dependencies**: Task 2, Task 3

**Files to modify**:
- `plugins/wasm-go/extensions/ai-proxy/provider/gemini.go`

**Implementation details**:
修改 `TransformRequestHeaders` 方法（约第 91 行）：

```go
func (g *geminiProvider) TransformRequestHeaders(ctx wrapper.HttpContext, apiName ApiName, headers http.Header) {
    // 设置 Host 头
    if g.customDomain != "" {
        util.OverwriteRequestHostHeader(headers, g.customDomain)
    } else {
        util.OverwriteRequestHostHeader(headers, geminiDomain)
    }
    
    // 设置 API Key 头（保持不变）
    headers.Set(geminiApiKeyHeader, g.config.GetApiTokenInUse(ctx))
    util.OverwriteRequestAuthorizationHeader(headers, "")
}
```

**Acceptance criteria**:
- 配置了自定义域名时，Host 头设置为自定义域名
- 未配置自定义域名时，Host 头设置为默认域名
- API Key 头的设置逻辑保持不变
- Host 头与实际请求的目标域名一致

---

## Task 5: 修改 getRequestPath 方法以支持自定义路径拼接

**Status**: completed

**Description**: 
修改 `geminiProvider.getRequestPath` 方法，实现自定义路径前缀与标准 API 路径的拼接逻辑。

**Requirements**: 需求 2, 需求 3

**Dependencies**: Task 2, Task 3

**Files to modify**:
- `plugins/wasm-go/extensions/ai-proxy/provider/gemini.go`

**Implementation details**:
1. 确保导入 `path` 包（如果尚未导入）

2. 修改 `getRequestPath` 方法（约第 300 行）：

```go
func (g *geminiProvider) getRequestPath(apiName ApiName, model string, stream bool) string {
    action := ""
    if g.config.apiVersion == "" {
        g.config.apiVersion = geminiDefaultApiVersion
    }
    
    switch apiName {
    case ApiNameModels:
        standardPath := fmt.Sprintf("/%s/%s", g.config.apiVersion, geminiModelsPath)
        // 如果有自定义路径，拼接自定义前缀和标准路径
        if g.customPath != "/" {
            return path.Join(g.customPath, standardPath)
        }
        return standardPath
        
    case ApiNameEmbeddings:
        action = geminiEmbeddingPath
    case ApiNameChatCompletion:
        if stream {
            action = geminiChatCompletionStreamPath
        } else {
            action = geminiChatCompletionPath
        }
    case ApiNameImageGeneration:
        action = geminiImageGenerationPath
    case ApiNameGeminiGenerateContent:
        action = geminiChatCompletionPath
    case ApiNameGeminiStreamGenerateContent:
        action = geminiChatCompletionStreamPath
    }
    
    // 构建标准路径: /{version}/models/{model}:{action}
    standardPath := fmt.Sprintf("/%s/models/%s:%s", g.config.apiVersion, model, action)
    
    // 如果有自定义路径，拼接自定义前缀和标准路径
    if g.customPath != "/" {
        return path.Join(g.customPath, standardPath)
    }
    
    return standardPath
}
```

**Acceptance criteria**:
- customPath 为 "/" 时，直接使用标准路径
- customPath 不为 "/" 时，使用 path.Join 拼接自定义前缀和标准路径
- 所有 API 类型（Models, Embeddings, ChatCompletion, ImageGeneration）都正确处理路径拼接
- 路径格式正确，没有多余的斜杠

---

## Task 6: 编写单元测试

**Status**: completed

**Description**: 
为自定义 URL 功能编写全面的单元测试，覆盖各种配置场景和边界情况。

**Requirements**: 需求 8

**Dependencies**: Task 1, Task 2, Task 3, Task 4, Task 5

**Files to create/modify**:
- `plugins/wasm-go/extensions/ai-proxy/provider/gemini_test.go`

**Implementation details**:
创建以下测试用例：

1. **基本功能测试**
   - TestGeminiProvider_NoCustomUrl: 未配置自定义 URL
   - TestGeminiProvider_CustomDomain: 基本自定义域名
   - TestGeminiProvider_UrlWithProtocol: 带协议前缀的 URL

2. **路径处理测试**
   - TestGeminiProvider_PathPrefix: 带路径前缀配置
   - TestGeminiProvider_DomainOnly: 仅域名配置

3. **Host 头设置测试**
   - TestGeminiProvider_CustomDomainHostHeader: 自定义域名的 Host 头
   - TestGeminiProvider_DefaultDomainHostHeader: 默认域名的 Host 头

4. **API Key 设置测试**
   - TestGeminiProvider_ApiKeyUnchanged: API Key 设置不变性

5. **边界情况测试**
   - TestGeminiProvider_EmptyCustomUrl: 空字符串配置
   - TestGeminiProvider_ProtocolOnly: 仅包含协议的 URL

**Acceptance criteria**:
- 所有测试用例通过
- 代码覆盖率 ≥ 80%
- 测试覆盖所有配置场景（仅域名、域名+端口、域名+路径）
- 测试验证 Host 头与目标域名匹配
- 测试验证路径拼接逻辑正确

---

## Task 7: 编写集成测试（可选）

**Status**: pending

**Description**: 
编写端到端的集成测试，验证完整的请求流程。

**Requirements**: 需求 8

**Dependencies**: Task 6

**Files to create/modify**:
- `plugins/wasm-go/extensions/ai-proxy/provider/gemini_integration_test.go`

**Implementation details**:
创建集成测试用例：
- TestIntegration_CustomUrlEndToEnd: 完整请求流程测试
  - 设置测试服务器
  - 验证 Host 头
  - 验证路径
  - 验证 API Key

**Acceptance criteria**:
- 集成测试通过
- 验证端到端的请求流程
- 验证所有请求头和路径设置正确

---

## Task 8: 更新文档和配置示例

**Status**: completed

**Description**: 
更新相关文档，添加 geminiCustomUrl 配置参数的说明和使用示例。

**Requirements**: 需求 7

**Dependencies**: Task 1, Task 2, Task 3, Task 4, Task 5

**Files to create/modify**:
- `plugins/wasm-go/extensions/ai-proxy/README.md` (如果存在)
- 相关配置示例文件

**Implementation details**:
1. 在文档中添加 geminiCustomUrl 参数说明
2. 提供配置示例：
   - 仅域名配置
   - 带路径前缀配置
   - 与 protocol 参数组合使用
3. 说明与 OpenAI/vLLM customUrl 的一致性

**Acceptance criteria**:
- 文档清晰说明 geminiCustomUrl 参数的用途
- 提供完整的配置示例
- 说明向后兼容性

---

## Task 9: 代码审查和优化

**Status**: completed

**Description**: 
进行代码审查，确保代码质量、性能和安全性。

**Requirements**: 所有需求

**Dependencies**: Task 1, Task 2, Task 3, Task 4, Task 5, Task 6

**Implementation details**:
1. 检查代码风格和命名规范
2. 验证错误处理逻辑
3. 检查日志记录是否充分
4. 验证向后兼容性
5. 性能分析（URL 解析只在创建时执行一次）
6. 安全性检查（Host 头注入防护）

**Acceptance criteria**:
- 代码符合项目规范
- 错误处理完善
- 日志记录充分
- 向后兼容性得到保证
- 无明显性能问题
- 无安全漏洞

---

## Implementation Order

建议按以下顺序实施任务：

1. Task 1: 添加配置字段支持
2. Task 2: 扩展 provider 结构体
3. Task 3: 实现 URL 解析逻辑
4. Task 4: 修改 Host 头设置
5. Task 5: 实现路径拼接
6. Task 6: 编写单元测试
7. Task 7: 编写集成测试（可选）
8. Task 8: 更新文档
9. Task 9: 代码审查和优化

## Notes

- 所有代码修改应保持向后兼容性
- 使用 `path.Join` 进行路径拼接，确保路径格式正确
- 添加充分的调试日志，便于问题排查
- 参考 OpenAI 和 vLLM provider 的实现，保持一致性
- protocol 参数的处理逻辑已在现有代码中实现，本次变更不涉及修改
