# AI Billing 插件设计文档

## 概述

### 插件目的
AI Billing 插件是 Higress 网关的计费能力核心组件，用于在 AI 代理请求的生命周期中实现余额检查和费用扣除。

### 解决的问题
- **计费控制**：防止欠费用户继续使用 AI 服务
- **按量计费**：根据实际 token 使用量自动扣除费用
- **计费准确性**：采用 FAIL_CLOSE 策略，确保所有计费相关的失败都会导致请求终止
- **多协议支持**：兼容 OpenAI、Claude、Gemini 等多种 LLM 协议格式

### 目标用户
- SaaS 运营者：需要对 AI 服务进行计费管理
- 系统管理员：需要配置和监控计费系统
- 开发者：需要集成计费功能到 AI 网关

## 功能设计

### 核心功能 1：请求前余额检查

**功能描述**：
在请求转发到 AI 服务之前，验证用户是否有足够的余额。

**实现方式**：
1. 在 `onHttpRequestHeaders` 阶段执行
2. 从请求头提取 API Key（优先 `x-hi-original-auth`，其次 `Authorization`）
3. 向计费服务发送异步 POST 请求到 `/v1/amount` 端点
4. 解析响应中的 `balance` 字段
5. 如果 balance <= 0，返回 402 Payment Required
6. 如果 balance > 0，允许请求继续

**关键代码逻辑**：
```go
func checkBalance(ctx wrapper.HttpContext, config BillingConfig, apiKey string) types.Action {
    // 构建请求
    requestBody := BalanceRequest{ApiKey: apiKey}
    bodyBytes, _ := json.Marshal(requestBody)
    
    // 异步 HTTP 调用
    config.billingClient.Post(url, headers, bodyBytes, func(statusCode int, responseHeaders http.Header, responseBody []byte) {
        if statusCode != 200 {
            sendErrorResponse(503, config.FailBalanceMessage)
            return
        }
        
        var balanceResp BalanceResponse
        json.Unmarshal(responseBody, &balanceResp)
        balance, _ := strconv.ParseFloat(balanceResp.Balance, 64)
        
        if balance <= 0 {
            sendErrorResponse(402, config.InsufficientBalanceMessage)
            return
        }
        
        proxywasm.ResumeHttpRequest()
    }, 5000)
    
    return types.ActionPause
}
```

### 核心功能 2：响应后费用扣除

**功能描述**：
在 LLM 请求成功返回后，根据 token 使用量自动扣除费用。

**实现方式**：
1. 在 `onHttpResponseHeaders` 阶段检测响应类型（流式/非流式）
2. 在 `onHttpResponseBody` 或 `onHttpStreamingResponseBody` 阶段提取 token 使用量
3. 使用 `tokenusage.GetTokenUsage()` 自动识别多种协议格式
4. 提取 request ID、provider、model 等信息
5. 向计费服务发送异步 POST 请求到 `/v1/cost` 端点
6. 检查响应中的 `success` 字段
7. 如果 success=false，返回 402 且不返回 LLM 响应
8. 如果 success=true，返回 LLM 响应给客户端

**关键代码逻辑**：
```go
func deductCost(ctx wrapper.HttpContext, config BillingConfig, billingInfo *BillingInfo, apiKey string) types.Action {
    // 构建请求
    requestBody := CostRequest{
        ApiKey:       apiKey,
        InputTokens:  billingInfo.InputTokens,
        OutputTokens: billingInfo.OutputTokens,
        ModelName:    billingInfo.Model,
        Provider:     billingInfo.Provider,
        RequestID:    billingInfo.RequestID,
    }
    bodyBytes, _ := json.Marshal(requestBody)
    
    // 异步 HTTP 调用
    config.billingClient.Post(url, headers, bodyBytes, func(statusCode int, responseHeaders http.Header, responseBody []byte) {
        if statusCode != 200 {
            sendErrorResponse(503, config.FailCostMessage)
            return
        }
        
        var costResp CostResponse
        json.Unmarshal(responseBody, &costResp)
        
        if !costResp.Success {
            sendErrorResponse(402, config.InsufficientBalanceMessage)
            return
        }
        
        proxywasm.ResumeHttpResponse()
    }, 5000)
    
    return types.ActionPause
}
```

### 核心功能 3：流式响应处理

**功能描述**：
正确处理 SSE 流式响应场景下的计费。

**实现方式**：
1. 在响应头阶段检测 `Content-Type: text/event-stream`
2. 对每个数据块调用 `tokenusage.GetTokenUsage()`
3. 当 `TotalToken > 0` 时，表示找到了 usage 信息
4. 在流结束时（`endOfStream=true`）触发费用扣除
5. 如果扣费失败，不返回任何数据给客户端

**关键代码逻辑**：
```go
func onHttpStreamingResponseBody(ctx wrapper.HttpContext, config BillingConfig, data []byte, endOfStream bool) []byte {
    // 对每个数据块调用 GetTokenUsage
    usage := tokenusage.GetTokenUsage(ctx, data)
    if usage.TotalToken > 0 {
        billingInfo := &BillingInfo{
            InputTokens:  usage.InputToken,
            OutputTokens: usage.OutputToken,
            Model:        usage.Model,
        }
        ctx.SetContext(CtxKeyBillingInfo, billingInfo)
    }
    
    if !endOfStream {
        return data
    }
    
    // 流结束时扣费
    billingInfo, _ := ctx.GetContext(CtxKeyBillingInfo).(*BillingInfo)
    action := deductCost(ctx, config, billingInfo, apiKey)
    if action == types.ActionPause {
        return nil  // 等待扣费完成
    }
    
    return data
}
```

### 核心功能 4：多协议支持

**功能描述**：
支持 OpenAI、Claude、Gemini 等多种 LLM 协议格式的 token 提取。

**实现方式**：
使用 `tokenusage.GetTokenUsage()` 函数，该函数内部已实现多协议支持：
- OpenAI: `usage.prompt_tokens` 和 `usage.completion_tokens`
- Claude: `usage.input_tokens` 和 `usage.output_tokens`
- Gemini: `usageMetadata.promptTokenCount` 和 `usageMetadata.candidatesTokenCount`

**关键代码逻辑**：
```go
// 使用 wasm-go 包提供的函数，自动处理多种格式
usage := tokenusage.GetTokenUsage(ctx, data)
if usage.TotalToken > 0 {
    // 成功提取到 token 信息
    inputTokens := usage.InputToken
    outputTokens := usage.OutputToken
    model := usage.Model
}
```

### 核心功能 5：Request ID 提取

**功能描述**：
从请求头或响应体中提取 request ID，用于计费幂等性。

**实现方式**：
1. 优先从请求头 `x-request-id` 提取
2. 如果请求头中没有，从响应体中提取（尝试多个字段名）
3. 如果都没有，生成一个唯一 ID

**关键代码逻辑**：
```go
func extractRequestID(ctx wrapper.HttpContext, data []byte) string {
    // 优先从请求头提取
    if requestID, err := proxywasm.GetHttpRequestHeader("x-request-id"); err == nil && requestID != "" {
        return requestID
    }
    
    // 从响应体提取
    if requestID := wrapper.GetValueFromBody(data, []string{
        "id",
        "response.id",
        "responseId",
        "message.id",
    }); requestID != nil {
        return requestID.String()
    }
    
    // 生成唯一 ID
    return generateUniqueID()
}
```

## 配置参数

| 参数 | 类型 | 必需 | 描述 | 默认值 |
|------|------|------|------|--------|
| billingService.serviceAddress | string | 是 | 计费服务地址 | - |
| billingService.protocol | string | 否 | 通信协议（http/https） | http |
| billingService.port | int | 否 | 服务端口 | 8888 |
| failBalanceMessage | string | 否 | 余额查询失败提示 | "503 Billing Service Balance Unavailable" |
| insufficientBalanceMessage | string | 否 | 余额不足提示 | "余额不足" |
| failCostMessage | string | 否 | 计费失败提示 | "503 Billing Service Cost Unavailable" |

## 技术实现

### 技术选型

**编程语言**：Go 1.24.1

**核心依赖**：
- `github.com/higress-group/proxy-wasm-go-sdk`: WASM 插件 SDK
- `github.com/higress-group/wasm-go`: Higress WASM 工具包
- `github.com/tidwall/gjson`: 高效 JSON 解析
- `github.com/stretchr/testify`: 单元测试框架

**关键技术**：
1. **异步 HTTP 调用**：使用 `wrapper.ClusterClient` 进行非阻塞 HTTP 调用
2. **上下文管理**：使用 `ctx.SetContext()` 在请求生命周期中传递数据
3. **流式处理**：使用 `onHttpStreamingResponseBody` 处理 SSE 流
4. **Token 提取**：使用 `tokenusage.GetTokenUsage()` 自动识别多种协议

### 性能考虑

1. **异步调用**：所有 HTTP 调用使用异步模式，避免阻塞主请求流程
2. **连接复用**：HTTP 客户端自动复用连接，减少连接开销
3. **高效解析**：使用 `gjson` 进行快速 JSON 解析，无需完整反序列化
4. **流式优化**：流式响应不缓冲全部数据，仅提取必要的 usage 信息
5. **超时控制**：所有 HTTP 调用设置 5 秒超时，防止资源耗尽

### 安全考虑

1. **API Key 保护**：日志中仅记录 API key 的前 8 个字符
2. **HTTPS 支持**：支持使用 HTTPS 与计费服务通信
3. **输入验证**：验证计费服务返回的响应格式
4. **响应保护**：计费失败时不返回 LLM 响应，防止免费使用
5. **超时保护**：防止长时间等待导致的资源耗尽

## 测试计划

### 单元测试

1. **配置解析测试**
   - 有效配置（包含所有字段）
   - 默认值配置
   - 无效配置（缺少必需字段）
   - 边界情况（空字符串、无效端口）

2. **API Key 提取测试**
   - 从 `x-hi-original-auth` 提取
   - 从 `Authorization` 提取
   - 带 `Bearer ` 前缀
   - 缺少 API key
   - 空 API key

3. **余额检查测试**
   - 余额充足
   - 余额不足
   - 超时
   - 非 200 状态码
   - 无效响应格式

4. **Token 提取测试**
   - OpenAI 格式（流式和非流式）
   - Claude 格式
   - Gemini 格式
   - 缺少必需字段
   - 无效 JSON 格式

5. **费用扣除测试**
   - 扣费成功
   - 扣费失败（success=false）
   - 超时
   - 非 200 状态码
   - 无效响应格式

6. **错误处理测试**
   - 每个错误类别
   - 错误响应格式
   - 错误日志记录（API key 脱敏）

### 属性测试

使用 `gopter` 库进行基于属性的测试，每个测试运行 100 次迭代：

1. **API Key 提取一致性**：验证所有有效请求都能正确提取 API key
2. **余额检查强制执行**：验证余额不足时总是拒绝请求
3. **余额检查失败处理**：验证所有失败情况都正确处理
4. **成功余额检查通过**：验证余额充足时总是允许请求
5. **Token 提取（多协议）**：验证所有协议格式都能正确提取
6. **费用扣除成功通过**：验证扣费成功时总是返回响应
7. **费用扣除失败阻止**：验证扣费失败时总是阻止响应
8. **流式响应处理**：验证流式场景下的正确计费
9. **API Key 脱敏**：验证日志中 API key 总是被脱敏
10. **无响应泄露**：验证计费失败时从不返回 LLM 响应

### 集成测试

1. 使用模拟计费服务测试完整请求流程
2. 使用模拟 AI 服务测试完整请求流程
3. 端到端测试错误场景
4. 测试流式和非流式响应

### 性能测试

1. **延迟开销**：余额检查 + 费用扣除应 < 50ms
2. **吞吐量**：应能处理 1000+ 请求/秒
3. **内存使用**：每个请求应 < 10MB
4. **连接池效率**：验证连接复用的有效性

## 限制和注意事项

### 已知限制

1. **计费服务依赖**：插件依赖外部计费服务，如果计费服务不可用，所有请求都会被拒绝
2. **超时设置**：HTTP 调用超时固定为 5 秒，不可配置
3. **重试逻辑**：插件不实现重试，依赖计费服务的幂等性
4. **协议支持**：仅支持 OpenAI、Claude、Gemini 三种协议格式

### 使用建议

1. **计费服务高可用**：确保计费服务具有高可用性，避免影响 AI 服务
2. **监控告警**：监控余额检查和费用扣除的成功率，及时发现问题
3. **日志分析**：定期分析日志，识别异常模式
4. **性能测试**：在生产环境部署前进行充分的性能测试
5. **插件优先级**：设置 priority=200，确保在认证之后、ai-proxy 之前执行

### 故障处理

1. **计费服务故障**：所有请求会被拒绝（FAIL_CLOSE 策略）
2. **网络超时**：返回 503 错误，不影响其他请求
3. **响应解析失败**：返回 500 或 503 错误，记录详细日志
4. **余额不足**：返回 402 错误，提示用户充值

## 工作流程图

```
┌─────────────┐
│   Client    │
└──────┬──────┘
       │ HTTP Request
       ▼
┌─────────────────────────────────────┐
│     AI Billing Plugin               │
│                                     │
│  1. Extract API Key                 │
│     ├─ x-hi-original-auth          │
│     └─ Authorization                │
│                                     │
│  2. Check Balance                   │
│     POST /v1/amount                 │
│     ├─ balance <= 0 → 402          │
│     ├─ error → 503                  │
│     └─ balance > 0 → Continue       │
└──────┬──────────────────────────────┘
       │ Forward Request
       ▼
┌─────────────┐
│ AI Service  │
└──────┬──────┘
       │ LLM Response
       ▼
┌─────────────────────────────────────┐
│     AI Billing Plugin               │
│                                     │
│  3. Extract Token Usage             │
│     ├─ tokenusage.GetTokenUsage()  │
│     ├─ Extract Request ID           │
│     └─ Extract Provider             │
│                                     │
│  4. Deduct Cost                     │
│     POST /v1/cost                   │
│     ├─ success=false → 402         │
│     ├─ error → 503                  │
│     └─ success=true → Return        │
└──────┬──────────────────────────────┘
       │ LLM Response
       ▼
┌─────────────┐
│   Client    │
└─────────────┘
```

## 版本历史

- **v1.0.0** (2025-01-16): 初始版本
  - 实现余额检查功能
  - 实现费用扣除功能
  - 支持流式和非流式响应
  - 支持多种 LLM 协议格式
  - 实现 FAIL_CLOSE 策略
