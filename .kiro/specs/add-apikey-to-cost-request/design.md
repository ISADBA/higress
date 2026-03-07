# Design Document: Add ApiKey to Cost Request

## Overview

本设计文档描述了如何在ai-billing插件的计费请求中添加apikey参数。该功能将从HTTP请求头`x-mse-consumer-apikey`中提取apikey值，并将其作为CostRequest结构体的新字段发送到billing-service的`/v1/cost`端点。

这个新增的apikey字段独立于现有的HMAC认证机制（9个必需的请求头字段），用于在计费记录中标识具体的API消费者。

## Architecture

### 当前架构

ai-billing插件当前的工作流程：

```
HTTP Request → onHttpRequestHeaders → extractTenantInfo (9个HMAC头)
                                    → checkPricing
                                    → checkBalance
             ↓
HTTP Response → onHttpResponseHeaders → 检测streaming/non-streaming
              ↓
              → onHttpResponseBody (non-streaming) → deductCost
              → onHttpStreamingResponseBody (streaming) → deductCostAsync
```

### 修改点

本次修改涉及以下几个关键点：

1. **请求头提取阶段** (`onHttpRequestHeaders`): 新增提取`x-mse-consumer-apikey`的逻辑
2. **数据结构** (`CostRequest`): 添加`ApiKey`字段
3. **计费请求构建** (`deductCost` 和 `deductCostAsync`): 在构建CostRequest时包含apikey值

### 修改后的数据流

```
HTTP Request Headers
  ├── x-mse-consumer-apikey (新增提取)
  └── 9个HMAC认证头 (现有)
       ↓
  存储到Context
       ↓
  构建CostRequest时使用
       ↓
  发送到 /v1/cost 端点
```

## Components and Interfaces

### 1. 数据结构修改

#### CostRequest 结构体

**当前定义** (main.go:94-100):
```go
type CostRequest struct {
	Provider     string `json:"provider"`
	ModelName    string `json:"model_name"`
	RequestID    string `json:"request_id"`
	InputTokens  int64  `json:"input_tokens"`
	OutputTokens int64  `json:"output_tokens"`
}
```

**修改后定义**:
```go
type CostRequest struct {
	Provider     string `json:"provider"`
	ModelName    string `json:"model_name"`
	RequestID    string `json:"request_id"`
	InputTokens  int64  `json:"input_tokens"`
	OutputTokens int64  `json:"output_tokens"`
	ApiKey       string `json:"apikey"`  // 新增字段
}
```

### 2. 请求头提取函数

新增一个辅助函数用于提取`x-mse-consumer-apikey`请求头：

```go
// extractConsumerApiKey extracts the consumer API key from x-mse-consumer-apikey header
// Returns empty string if header is not present
func extractConsumerApiKey() string {
	apiKey, err := proxywasm.GetHttpRequestHeader("x-mse-consumer-apikey")
	if err != nil || apiKey == "" {
		return ""
	}
	return apiKey
}
```

**设计决策**:
- 如果请求头不存在，返回空字符串（而非错误）
- 不对apikey值进行任何验证或转换
- 保留原始值，包括空白字符

### 3. Context存储

在`onHttpRequestHeaders`阶段提取并存储apikey到context中：

**新增Context Key**:
```go
const (
	// ... 现有的context keys
	CtxKeyConsumerApiKey = "ai-billing-consumer-apikey"  // 新增
)
```

**存储逻辑**:
```go
// 在 onHttpRequestHeaders 函数中
consumerApiKey := extractConsumerApiKey()
ctx.SetContext(CtxKeyConsumerApiKey, consumerApiKey)
log.Debugf("[%s] consumer apikey extracted: %s", pluginName, maskApiKey(consumerApiKey))
```

### 4. 计费请求构建修改

#### deductCost 函数修改

在构建CostRequest时，从context中获取apikey并添加到请求体：

```go
// 从context获取apikey
consumerApiKey := ""
if key, ok := ctx.GetContext(CtxKeyConsumerApiKey).(string); ok {
	consumerApiKey = key
}

// 构建请求体
requestBody := CostRequest{
	Provider:     billingInfo.Provider,
	ModelName:    billingInfo.Model,
	RequestID:    billingInfo.RequestID,
	InputTokens:  billingInfo.InputTokens,
	OutputTokens: billingInfo.OutputTokens,
	ApiKey:       consumerApiKey,  // 新增
}
```

#### deductCostAsync 函数修改

同样的修改应用到异步计费函数中（用于streaming响应）。

## Data Models

### CostRequest JSON格式

**修改前**:
```json
{
  "provider": "openai",
  "model_name": "gpt-4",
  "request_id": "req-123",
  "input_tokens": 100,
  "output_tokens": 50
}
```

**修改后**:
```json
{
  "provider": "openai",
  "model_name": "gpt-4",
  "request_id": "req-123",
  "input_tokens": 100,
  "output_tokens": 50,
  "apikey": "sk-abc123xyz"
}
```

**当apikey不存在时**:
```json
{
  "provider": "openai",
  "model_name": "gpt-4",
  "request_id": "req-123",
  "input_tokens": 100,
  "output_tokens": 50,
  "apikey": ""
}
```

## Correctness Properties


*属性（Property）是系统在所有有效执行中都应该保持为真的特征或行为——本质上是关于系统应该做什么的形式化陈述。属性是人类可读规范和机器可验证正确性保证之间的桥梁。*

### Property 1: ApiKey提取正确性

*对于任何*包含`x-mse-consumer-apikey`请求头的HTTP请求，提取的apikey值应该等于该请求头的值

**Validates: Requirements 1.1, 1.2**

### Property 2: CostRequest包含正确的apikey

*对于任何*提取的apikey值（包括空字符串），构造的CostRequest序列化后的JSON应该包含一个`apikey`字段，其值等于提取的值

**Validates: Requirements 2.1, 2.3**

### Property 3: CostRequest保留现有字段

*对于任何*CostRequest实例，序列化后的JSON应该包含所有必需字段：`provider`, `model_name`, `request_id`, `input_tokens`, `output_tokens`, `apikey`

**Validates: Requirements 2.2**

### Property 4: JSON序列化round-trip

*对于任何*有效的CostRequest对象，序列化为JSON然后反序列化应该产生一个等价的对象（包括apikey字段）

**Validates: Requirements 3.3**

### Property 5: HMAC认证头保持不变

*对于任何*计费请求，发送到billing-service的HTTP请求应该包含所有9个HMAC认证头，不受apikey字段的影响

**Validates: Requirements 4.2**

## Error Handling

### 错误场景处理

本次修改不引入新的错误场景。现有的错误处理机制保持不变：

1. **请求头缺失**: 如果`x-mse-consumer-apikey`不存在，apikey字段设置为空字符串，不影响计费流程
2. **序列化错误**: 如果CostRequest序列化失败，使用现有的错误处理逻辑（返回500错误）
3. **billing-service错误**: billing-service的响应处理逻辑不变

### 日志记录

添加适当的日志记录以便调试：

```go
log.Debugf("[%s] consumer apikey extracted: %s", pluginName, maskApiKey(consumerApiKey))
```

注意：使用`maskApiKey`函数对apikey进行脱敏处理（只显示前8个字符）。

## Testing Strategy

### 单元测试

单元测试应该覆盖以下场景：

1. **提取函数测试**:
   - 测试`extractConsumerApiKey`在请求头存在时返回正确值
   - 测试请求头不存在时返回空字符串
   - 测试请求头包含空白字符时保留原始值

2. **数据结构测试**:
   - 测试CostRequest的JSON序列化包含所有字段
   - 测试apikey字段的序列化和反序列化

3. **集成测试**:
   - 测试完整的计费流程（从请求头提取到发送计费请求）
   - 验证发送到billing-service的请求体包含apikey字段

### 属性测试

属性测试应该使用property-based testing框架（如Go的`gopter`或`rapid`）：

1. **Property 1测试**: 生成随机的HTTP请求头，验证提取逻辑
2. **Property 2测试**: 生成随机的apikey值，验证CostRequest构造
3. **Property 3测试**: 生成随机的CostRequest，验证所有字段存在
4. **Property 4测试**: 生成随机的CostRequest，验证序列化round-trip
5. **Property 5测试**: 验证HMAC头在添加apikey后仍然正确发送

**配置要求**:
- 每个属性测试至少运行100次迭代
- 测试标签格式: `Feature: add-apikey-to-cost-request, Property {N}: {property_text}`

### 测试覆盖率目标

- 新增代码行覆盖率: 100%
- 分支覆盖率: 100%
- 集成测试: 覆盖streaming和non-streaming两种响应模式

## Implementation Notes

### 修改文件清单

1. **plugins/wasm-go/extensions/ai-billing/main.go**:
   - 修改`CostRequest`结构体定义
   - 添加`CtxKeyConsumerApiKey`常量
   - 添加`extractConsumerApiKey`函数
   - 修改`onHttpRequestHeaders`函数（添加apikey提取逻辑）
   - 修改`deductCost`函数（在CostRequest中包含apikey）
   - 修改`deductCostAsync`函数（在CostRequest中包含apikey）

### 代码修改位置

1. **第94-100行**: CostRequest结构体定义
2. **第18-24行**: Context keys常量定义
3. **第400行附近**: onHttpRequestHeaders函数
4. **第790行附近**: deductCost函数中的CostRequest构造
5. **第884行附近**: deductCostAsync函数中的CostRequest构造

### 向后兼容性考虑

1. **billing-service兼容性**: 
   - 旧版本的billing-service应该能够忽略未知的`apikey`字段（JSON反序列化的标准行为）
   - 新版本的billing-service可以选择使用或忽略该字段

2. **插件升级路径**:
   - 插件可以独立升级，不依赖billing-service的版本
   - 即使billing-service不使用apikey字段，插件也能正常工作

3. **配置兼容性**:
   - 不需要修改插件配置
   - 不引入新的配置参数

## Performance Considerations

### 性能影响分析

1. **请求头提取**: 
   - 新增一次`proxywasm.GetHttpRequestHeader`调用
   - 性能影响: 可忽略（纳秒级）

2. **Context存储**:
   - 新增一次`ctx.SetContext`调用
   - 性能影响: 可忽略（内存中的map操作）

3. **JSON序列化**:
   - CostRequest增加一个字段
   - 性能影响: 可忽略（字符串字段的序列化开销很小）

4. **网络传输**:
   - 请求体大小增加（取决于apikey长度，通常20-50字节）
   - 性能影响: 可忽略（相对于整个HTTP请求）

**结论**: 本次修改对性能的影响可以忽略不计。

## Security Considerations

### 安全性分析

1. **ApiKey敏感信息**:
   - apikey可能包含敏感信息
   - 在日志中使用`maskApiKey`函数进行脱敏
   - 只记录前8个字符，其余用`***`替代

2. **HMAC认证独立性**:
   - apikey字段不参与HMAC签名计算
   - HMAC认证机制保持不变
   - apikey仅用于计费记录的标识，不用于认证

3. **注入攻击防护**:
   - apikey值直接从请求头提取，不进行任何转换
   - JSON序列化会自动处理特殊字符的转义
   - 不存在SQL注入或XSS风险（值只用于JSON传输）

4. **数据泄露风险**:
   - apikey通过HTTPS传输到billing-service
   - 不在响应中返回apikey值
   - 日志中的apikey已脱敏

## Deployment Strategy

### 部署步骤

1. **阶段1: 插件升级**
   - 部署新版本的ai-billing插件
   - 插件开始在计费请求中包含apikey字段
   - 旧版本的billing-service会忽略该字段

2. **阶段2: billing-service升级**（可选）
   - 如果billing-service需要使用apikey字段，可以在此阶段升级
   - 升级后的billing-service可以读取和使用apikey字段

3. **回滚策略**
   - 如果需要回滚，可以直接部署旧版本插件
   - 不需要修改billing-service（向后兼容）

### 验证步骤

1. **功能验证**:
   - 发送包含`x-mse-consumer-apikey`请求头的测试请求
   - 检查billing-service的日志，确认收到的请求包含apikey字段
   - 验证计费流程正常工作

2. **兼容性验证**:
   - 发送不包含`x-mse-consumer-apikey`请求头的测试请求
   - 验证计费流程仍然正常工作（apikey为空字符串）

3. **性能验证**:
   - 对比升级前后的响应时间
   - 确认性能没有明显下降

## References

### 相关文档

- ai-billing插件现有设计文档: `plugins/wasm-go/extensions/ai-billing/design/`
- HMAC认证规范: 内部文档
- billing-service API文档: 内部文档

### 相关代码

- `plugins/wasm-go/extensions/ai-billing/main.go`: 插件主要实现
- `github.com/higress-group/proxy-wasm-go-sdk/proxywasm`: Proxy-Wasm SDK
- `github.com/higress-group/wasm-go/pkg/wrapper`: Higress wrapper库
