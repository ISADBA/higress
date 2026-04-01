# Claude Provider 自定义 baseURL 支持

## 需求
为 ai-proxy 的 Claude provider 添加自定义 baseURL 支持，以适配：
- 私有部署的 Claude API
- 代理服务
- 其他兼容 Claude API 的服务

## 实现方案

### 1. 配置层修改 (`provider/provider.go`)

#### 1.1 添加配置字段
在 `ProviderConfig` 结构体中添加（约第 458 行附近）：
```go
claudeCustomUrl string `required:"false" yaml:"claudeCustomUrl" json:"claudeCustomUrl"`
```

#### 1.2 解析配置
在 `FromJson` 方法中添加（约第 674 行附近）：
```go
c.claudeCustomUrl = json.Get("claudeCustomUrl").String()
```

#### 1.3 添加 Getter
添加方法：
```go
func (c *ProviderConfig) GetClaudeCustomUrl() string {
    return c.claudeCustomUrl
}
```

### 2. Provider 层修改 (`provider/claude.go`)

#### 2.1 添加字段
在 `claudeProvider` 结构体中添加（约第 307 行）：
```go
customDomain string
```

#### 2.2 修改 CreateClaudeProvider
在 `CreateClaudeProvider` 函数中解析自定义 URL：
```go
func CreateClaudeProvider(config ProviderConfig) (Provider, error) {
    customDomain := ""
    if customUrl := config.GetClaudeCustomUrl(); customUrl != "" {
        customDomain = strings.TrimPrefix(strings.TrimPrefix(customUrl, "http://"), "https://")
        if idx := strings.Index(customDomain, "/"); idx != -1 {
            customDomain = customDomain[:idx]
        }
    }
    
    config.setDefaultCapabilities(c.DefaultCapabilities())
    return &claudeProvider{
        config:       config,
        customDomain: customDomain,
        contextCache: createContextCache(&config),
    }, nil
}
```

#### 2.3 修改 TransformRequestHeaders
修改第 334 行，使用自定义域名：
```go
if c.customDomain != "" {
    util.OverwriteRequestHostHeader(headers, c.customDomain)
} else {
    util.OverwriteRequestHostHeader(headers, claudeDomain)
}
```

## 配置示例

### 基础配置
```yaml
provider:
  type: claude
  claudeCustomUrl: "https://custom-claude-api.example.com"
  apiTokens:
    - "sk-ant-xxx"
```

### 完整配置示例
```yaml
provider:
  type: claude
  claudeCustomUrl: "https://api.example.com"  # 自定义 Claude API 地址
  apiTokens:
    - "sk-ant-api03-xxx"
  modelMapping:
    "*": "claude-3-5-sonnet-20241022"
  timeout: 120000
```

### 使用场景
- **代理服务**: `claudeCustomUrl: "https://proxy.example.com"`
- **私有部署**: `claudeCustomUrl: "https://internal-claude.company.com"`
- **镜像服务**: `claudeCustomUrl: "https://mirror.example.com"`

## 与 OpenAI 实现的差异

Claude 的实现比 OpenAI 更简单：
- OpenAI 需要处理 `customPath` 和 `isDirectCustomPath`（支持直接指定完整路径）
- Claude 只需替换 host，path 由 `OverwriteRequestPathHeaderByCapability` 统一处理
- 不需要额外的 path 拼接逻辑

## 改动文件清单

1. `plugins/wasm-go/extensions/ai-proxy/provider/provider.go` - 3 处修改
2. `plugins/wasm-go/extensions/ai-proxy/provider/claude.go` - 2 处修改
