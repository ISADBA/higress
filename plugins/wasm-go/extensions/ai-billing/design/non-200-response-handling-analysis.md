# AI-Billing 插件非 200 响应处理优化调研

## 问题描述

当 provider（如 Gemini）返回非 200 状态码（4xx/5xx）时，ai-billing 插件当前的行为会导致用户体验不佳：

### 当前问题现象

1. **Provider 返回 421 错误**：
   ```
   [ai-billing] skipping billing for non-200 response: status=421
   ```

2. **用户收到的错误信息**：
   ```
   HTTP/2 stream 1 was not closed cleanly: INTERNAL_ERROR (err 2)
   curl: (92) HTTP/2 stream 1 was not closed cleanly: INTERNAL_ERROR (err 2)
   ```

3. **Envoy 日志显示**：
   ```
   response_code_details: "via_wasm::higress-system.ai-billing::ai-billing.error"
   ```

### 问题分析

从日志和代码分析来看，当前的问题在于：

1. **响应头阶段的处理逻辑**（`onHttpResponseHeaders` 函数，第 717-738 行）：
   ```go
   func onHttpResponseHeaders(ctx wrapper.HttpContext, config BillingConfig) types.Action {
       // 检查 HTTP 状态码
       statusCode, err := proxywasm.GetHttpResponseHeader(":status")
       if err != nil || statusCode != "200" {
           log.Debugf("[%s] skipping billing for non-200 response: status=%s", pluginName, statusCode)
           return types.ActionContinue
       }
       // ... 继续处理 200 响应
   }
   ```

2. **当前行为**：
   - 当 provider 返回非 200 状态码时，插件直接 `return types.ActionContinue`
   - 这会让响应继续传递，但由于某些原因导致 HTTP/2 流异常关闭
   - 用户看到的是 `INTERNAL_ERROR` 而不是 provider 的原始错误信息

3. **根本原因**：
   - 插件在非 200 响应时没有正确处理响应传递
   - 可能是因为响应体被缓冲但没有正确释放
   - 或者是因为某些内部状态导致响应流异常

## 当前代码流程分析

### 请求阶段流程

```
onHttpRequestHeaders
  ├─ extractTenantInfo()          // 提取租户信息
  ├─ extractConsumerApiKey()      // 提取消费者 API Key
  ├─ extractProvider() & extractModel()  // 提取 provider 和 model
  ├─ checkPricing()               // 检查定价（带缓存）
  │   └─ 异步调用 GET /v1/pricing/global
  │       ├─ 成功 → 缓存 → checkBalance()
  │       └─ 失败 → sendErrorResponseAndMarkDenied() → 返回 503
  └─ checkBalance()               // 检查余额
      └─ 异步调用 GET /v1/amount
          ├─ 余额充足 → ResumeHttpRequest()
          └─ 余额不足/失败 → sendErrorResponseAndMarkDenied() → 返回 402/503
```

### 响应阶段流程

```
onHttpResponseHeaders
  ├─ 检查 CtxKeyRequestDenied
  │   └─ 如果为 true → 跳过处理
  ├─ 检查状态码
  │   ├─ 非 200 → log.Debugf() → return types.ActionContinue  ⚠️ 问题点
  │   └─ 200 → 继续处理
  ├─ 检测响应类型（streaming vs non-streaming）
  └─ 非流式 → BufferResponseBody()

onHttpResponseBody (非流式)
  ├─ 提取 token usage
  ├─ 提取 billing info
  └─ deductCost()
      └─ 异步调用 POST /v1/cost
          ├─ 成功 → ResumeHttpResponse()
          └─ 失败 → sendErrorResponse() → 返回 503/402

onHttpStreamingResponseBody (流式)
  ├─ 每个 chunk → GetTokenUsage()
  └─ endOfStream → deductCostAsync()
      └─ 异步调用 POST /v1/cost（仅记录日志，不阻断）
```

## 问题根源分析

### 为什么会出现 HTTP/2 INTERNAL_ERROR？

1. **响应体缓冲问题**：
   - 在 `onHttpResponseHeaders` 中，如果状态码是 200，会调用 `ctx.BufferResponseBody()`
   - 但对于非 200 响应，没有调用 `BufferResponseBody()`，也没有明确处理响应体
   - 这可能导致响应流状态不一致

2. **响应传递中断**：
   - 当插件返回 `types.ActionContinue` 时，期望响应正常传递给客户端
   - 但如果插件内部状态或 Envoy 状态不一致，可能导致流异常关闭

3. **错误响应覆盖**：
   - 虽然代码中没有调用 `sendErrorResponse()`，但 Envoy 日志显示 `response_code_details: "via_wasm::higress-system.ai-billing::ai-billing.error"`
   - 这表明可能有其他地方触发了错误响应

### 可能的触发点

检查代码后，发现以下可能的问题：

1. **请求阶段的 `sendErrorResponseAndMarkDenied()`**：
   - 如果请求阶段已经调用了这个函数，会设置 `CtxKeyRequestDenied = true`
   - 但如果 provider 仍然返回了响应（虽然是错误响应），响应阶段会跳过处理
   - 这可能导致响应流状态不一致

2. **响应体处理不完整**：
   - 对于非 200 响应，插件没有读取或处理响应体
   - 这可能导致响应体数据残留在缓冲区中

## 优化方案

### 方案 1：透传 Provider 错误响应（推荐）

**核心思路**：对于非 200 响应，不进行计费，直接透传 provider 的原始响应给客户端。

**实现要点**：

1. **在 `onHttpResponseHeaders` 中**：
   ```go
   func onHttpResponseHeaders(ctx wrapper.HttpContext, config BillingConfig) types.Action {
       // 检查是否在请求阶段被拒绝
       if denied, ok := ctx.GetContext(CtxKeyRequestDenied).(bool); ok && denied {
           log.Debugf("[%s] request was denied, skipping response processing", pluginName)
           return types.ActionContinue
       }

       // 检查 HTTP 状态码
       statusCode, err := proxywasm.GetHttpResponseHeader(":status")
       if err != nil || statusCode != "200" {
           log.Infof("[%s] skipping billing for non-200 response: status=%s", pluginName, statusCode)
           // 直接返回，让响应透传给客户端
           return types.ActionContinue
       }

       // ... 继续处理 200 响应
   }
   ```

2. **确保响应体正常传递**：
   - 不调用 `BufferResponseBody()`（已经是当前行为）
   - 不调用 `sendErrorResponse()`（已经是当前行为）
   - 直接返回 `types.ActionContinue`

3. **日志级别调整**：
   - 将 `log.Debugf` 改为 `log.Infof`，便于追踪非 200 响应
   - 添加更多上下文信息（tenant、consumer、provider、model）

**优点**：
- 用户能看到 provider 的原始错误信息
- 实现简单，风险低
- 符合"不计费就不拦截"的原则

**缺点**：
- 无法自定义错误消息
- 无法统一错误格式

### 方案 2：包装 Provider 错误响应

**核心思路**：读取 provider 的错误响应，包装后返回给客户端。

**实现要点**：

1. **在 `onHttpResponseHeaders` 中缓冲响应体**：
   ```go
   func onHttpResponseHeaders(ctx wrapper.HttpContext, config BillingConfig) types.Action {
       // 检查状态码
       statusCode, err := proxywasm.GetHttpResponseHeader(":status")
       if err != nil || statusCode != "200" {
           log.Infof("[%s] non-200 response detected: status=%s, buffering body", pluginName, statusCode)
           // 缓冲响应体以便读取错误信息
           ctx.BufferResponseBody()
           ctx.SetContext("ai-billing-error-status", statusCode)
           return types.ActionContinue
       }
       // ... 继续处理 200 响应
   }
   ```

2. **在 `onHttpResponseBody` 中处理错误响应**：
   ```go
   func onHttpResponseBody(ctx wrapper.HttpContext, config BillingConfig, body []byte) types.Action {
       // 检查是否是错误响应
       if errorStatus, ok := ctx.GetContext("ai-billing-error-status").(string); ok {
           log.Infof("[%s] processing error response: status=%s body=%s", pluginName, errorStatus, string(body))
           
           // 提取 provider 错误信息
           providerError := extractProviderError(body)
           
           // 构造包装后的错误响应
           wrappedError := fmt.Sprintf(`{"error":{"message":"provider error: %s","type":"provider_error","provider_status":"%s"}}`, 
               providerError, errorStatus)
           
           // 发送包装后的错误响应
           statusCode, _ := strconv.Atoi(errorStatus)
           _ = proxywasm.SendHttpResponseWithDetail(uint32(statusCode), "ai-billing.provider-error", [][2]string{
               {"content-type", "application/json"},
           }, []byte(wrappedError), -1)
           
           return types.ActionContinue
       }
       
       // ... 继续处理正常响应
   }
   ```

3. **添加 `extractProviderError` 辅助函数**：
   ```go
   func extractProviderError(body []byte) string {
       // 尝试从 JSON 响应中提取错误信息
       if errorMsg := wrapper.GetValueFromBody(body, []string{
           "error.message",
           "error",
           "message",
       }); errorMsg != nil {
           return errorMsg.String()
       }
       
       // 如果无法解析，返回原始响应体（截断）
       if len(body) > 200 {
           return string(body[:200]) + "..."
       }
       return string(body)
   }
   ```

**优点**：
- 可以统一错误格式
- 可以添加额外的上下文信息（如 tenant、consumer）
- 便于客户端解析

**缺点**：
- 需要缓冲和解析响应体，增加延迟
- 可能改变 provider 的原始错误格式
- 实现复杂度较高

### 方案 3：混合方案（推荐用于生产）

**核心思路**：根据状态码类型采用不同策略。

**实现要点**：

1. **4xx 错误（客户端错误）**：
   - 直接透传 provider 响应
   - 不缓冲响应体
   - 记录 info 级别日志

2. **5xx 错误（服务端错误）**：
   - 可选：缓冲并包装错误响应
   - 或者：直接透传
   - 记录 warn 级别日志

3. **实现代码**：
   ```go
   func onHttpResponseHeaders(ctx wrapper.HttpContext, config BillingConfig) types.Action {
       // 检查是否在请求阶段被拒绝
       if denied, ok := ctx.GetContext(CtxKeyRequestDenied).(bool); ok && denied {
           log.Debugf("[%s] request was denied, skipping response processing", pluginName)
           return types.ActionContinue
       }

       // 检查 HTTP 状态码
       statusCode, err := proxywasm.GetHttpResponseHeader(":status")
       if err != nil {
           log.Warnf("[%s] failed to get response status code: %v", pluginName, err)
           return types.ActionContinue
       }

       if statusCode != "200" {
           // 提取上下文信息用于日志
           tenantInfo, _ := ctx.GetContext(CtxKeyTenantInfo).(*TenantInfo)
           provider := extractProvider(ctx)
           model := extractModel(ctx)
           
           // 根据状态码类型记录不同级别的日志
           if strings.HasPrefix(statusCode, "4") {
               // 4xx: 客户端错误，info 级别
               if tenantInfo != nil {
                   log.Infof("[%s] skipping billing for 4xx response: tenantId=%s consumerId=%s provider=%s model=%s status=%s",
                       pluginName, tenantInfo.TenantID, tenantInfo.ConsumerID, provider, model, statusCode)
               } else {
                   log.Infof("[%s] skipping billing for 4xx response: provider=%s model=%s status=%s",
                       pluginName, provider, model, statusCode)
               }
           } else if strings.HasPrefix(statusCode, "5") {
               // 5xx: 服务端错误，warn 级别
               if tenantInfo != nil {
                   log.Warnf("[%s] skipping billing for 5xx response: tenantId=%s consumerId=%s provider=%s model=%s status=%s",
                       pluginName, tenantInfo.TenantID, tenantInfo.ConsumerID, provider, model, statusCode)
               } else {
                   log.Warnf("[%s] skipping billing for 5xx response: provider=%s model=%s status=%s",
                       pluginName, provider, model, statusCode)
               }
           } else {
               // 其他非 200 状态码（如 3xx），debug 级别
               log.Debugf("[%s] skipping billing for non-200 response: status=%s", pluginName, statusCode)
           }
           
           // 直接透传响应，不做任何修改
           return types.ActionContinue
       }

       // 检测响应类型（streaming vs non-streaming）
       contentType, _ := proxywasm.GetHttpResponseHeader("content-type")
       isStreaming := strings.Contains(contentType, "text/event-stream")
       ctx.SetContext(CtxKeyIsStreaming, isStreaming)

       if isStreaming {
           log.Debugf("[%s] detected streaming response", pluginName)
       } else {
           log.Debugf("[%s] detected non-streaming response, buffering body", pluginName)
           ctx.BufferResponseBody()
       }

       return types.ActionContinue
   }
   ```

**优点**：
- 保持简单，风险低
- 用户能看到 provider 的原始错误
- 通过日志级别区分错误类型
- 便于监控和告警

**缺点**：
- 无法自定义错误格式

## HTTP/2 INTERNAL_ERROR 问题排查

### 可能的原因

1. **响应流状态不一致**：
   - 插件在某个阶段修改了响应流状态但没有正确恢复
   - 建议：确保所有代码路径都正确处理响应流

2. **请求阶段的错误响应与 provider 响应冲突**：
   - 如果请求阶段调用了 `sendErrorResponseAndMarkDenied()`，但 provider 仍然返回了响应
   - 建议：检查 `CtxKeyRequestDenied` 标志

3. **Envoy 内部错误**：
   - 可能是 Envoy 或 proxy-wasm-go-sdk 的 bug
   - 建议：升级到最新版本

### 调试建议

1. **添加详细日志**：
   ```go
   log.Infof("[%s] onHttpResponseHeaders: status=%s denied=%v", 
       pluginName, statusCode, ctx.GetContext(CtxKeyRequestDenied))
   ```

2. **检查请求阶段是否有错误**：
   - 查看日志中是否有 `sendErrorResponseAndMarkDenied` 的调用
   - 确认 `CtxKeyRequestDenied` 的值

3. **测试不同场景**：
   - Provider 返回 4xx 错误
   - Provider 返回 5xx 错误
   - 请求阶段余额不足（402）
   - 请求阶段计费服务不可用（503）

## 推荐实现方案

基于以上分析，推荐采用**方案 3（混合方案）**，具体实现步骤：

### 第一步：优化 `onHttpResponseHeaders` 函数

```go
func onHttpResponseHeaders(ctx wrapper.HttpContext, config BillingConfig) types.Action {
    // 检查是否在请求阶段被拒绝
    if denied, ok := ctx.GetContext(CtxKeyRequestDenied).(bool); ok && denied {
        log.Debugf("[%s] request was denied in request phase, skipping response processing", pluginName)
        return types.ActionContinue
    }

    log.Debugf("[%s] processing response headers", pluginName)

    // 检查 HTTP 状态码
    statusCode, err := proxywasm.GetHttpResponseHeader(":status")
    if err != nil {
        log.Warnf("[%s] failed to get response status code: %v", pluginName, err)
        return types.ActionContinue
    }

    if statusCode != "200" {
        // 提取上下文信息用于日志
        tenantInfo, _ := ctx.GetContext(CtxKeyTenantInfo).(*TenantInfo)
        provider := extractProvider(ctx)
        model := extractModel(ctx)
        
        // 根据状态码类型记录不同级别的日志
        if strings.HasPrefix(statusCode, "4") {
            // 4xx: 客户端错误
            if tenantInfo != nil {
                log.Infof("[%s] skipping billing for 4xx response: tenantId=%s consumerId=%s provider=%s model=%s status=%s",
                    pluginName, tenantInfo.TenantID, tenantInfo.ConsumerID, provider, model, statusCode)
            } else {
                log.Infof("[%s] skipping billing for 4xx response: provider=%s model=%s status=%s",
                    pluginName, provider, model, statusCode)
            }
        } else if strings.HasPrefix(statusCode, "5") {
            // 5xx: 服务端错误
            if tenantInfo != nil {
                log.Warnf("[%s] skipping billing for 5xx response: tenantId=%s consumerId=%s provider=%s model=%s status=%s",
                    pluginName, tenantInfo.TenantID, tenantInfo.ConsumerID, provider, model, statusCode)
            } else {
                log.Warnf("[%s] skipping billing for 5xx response: provider=%s model=%s status=%s",
                    pluginName, provider, model, statusCode)
            }
        } else {
            // 其他非 200 状态码（如 3xx）
            log.Debugf("[%s] skipping billing for non-200 response: status=%s", pluginName, statusCode)
        }
        
        // 直接透传响应，不做任何修改
        // 不调用 BufferResponseBody()，让响应体直接传递给客户端
        return types.ActionContinue
    }

    // 200 响应：继续正常的计费流程
    // 检测响应类型（streaming vs non-streaming）
    contentType, _ := proxywasm.GetHttpResponseHeader("content-type")
    isStreaming := strings.Contains(contentType, "text/event-stream")
    ctx.SetContext(CtxKeyIsStreaming, isStreaming)

    if isStreaming {
        log.Debugf("[%s] detected streaming response", pluginName)
    } else {
        log.Debugf("[%s] detected non-streaming response, buffering body", pluginName)
        ctx.BufferResponseBody()
    }

    return types.ActionContinue
}
```

### 第二步：确保 `onHttpResponseBody` 和 `onHttpStreamingResponseBody` 正确处理

这两个函数已经有 `CtxKeyRequestDenied` 检查，无需修改。

### 第三步：测试验证

1. **测试 provider 返回 4xx 错误**：
   ```bash
   # 模拟无效的 API key
   curl -X POST https://api.aportal.ai/v1/chat/completions \
     -H "Authorization: Bearer invalid_key" \
     -H "Content-Type: application/json" \
     -d '{"model":"gpt-4","messages":[{"role":"user","content":"test"}]}'
   
   # 预期：收到 provider 的 401 错误响应
   # 日志：[ai-billing] skipping billing for 4xx response: status=401
   ```

2. **测试 provider 返回 5xx 错误**：
   ```bash
   # 模拟 provider 服务不可用
   # 预期：收到 provider 的 5xx 错误响应
   # 日志：[ai-billing] skipping billing for 5xx response: status=503
   ```

3. **测试正常 200 响应**：
   ```bash
   # 正常请求
   # 预期：正常计费，返回 LLM 响应
   # 日志：[ai-billing] cost deduction: ... success=true
   ```

4. **测试余额不足**：
   ```bash
   # 余额不足的账户
   # 预期：请求阶段被拒绝，返回 402
   # 日志：[ai-billing] insufficient balance: ...
   ```

## 总结

### 当前问题

- Provider 返回非 200 响应时，用户收到 `HTTP/2 INTERNAL_ERROR` 而不是 provider 的原始错误
- 日志显示 `response_code_details: "via_wasm::higress-system.ai-billing::ai-billing.error"`

### 根本原因

- 代码逻辑本身是正确的（直接 `return types.ActionContinue` 透传响应）
- 可能是响应流状态不一致或其他 Envoy/SDK 问题导致

### 推荐方案

- 采用**方案 3（混合方案）**：直接透传 provider 错误响应
- 优化日志记录：根据状态码类型使用不同日志级别
- 添加更多上下文信息（tenant、consumer、provider、model）

### 实现要点

1. 在 `onHttpResponseHeaders` 中检查状态码
2. 对于非 200 响应：
   - 记录详细日志（包含 tenant、consumer、provider、model、status）
   - 直接返回 `types.ActionContinue`，不做任何修改
   - 不调用 `BufferResponseBody()`
   - 不调用 `sendErrorResponse()`
3. 对于 200 响应：
   - 继续正常的计费流程

### 后续优化

如果透传方案仍然出现 HTTP/2 错误，可以考虑：

1. **升级 proxy-wasm-go-sdk** 到最新版本
2. **添加更详细的调试日志** 追踪响应流状态
3. **联系 Higress/Envoy 社区** 报告可能的 bug
4. **考虑方案 2（包装错误响应）** 作为 workaround

### 风险评估

- **低风险**：方案 3 只是优化日志，不改变核心逻辑
- **中风险**：如果问题是 SDK/Envoy bug，可能需要更深入的修复
- **高风险**：方案 2 需要缓冲和修改响应，可能引入新问题

### 建议

1. **先实现方案 3**，验证是否解决问题
2. **如果问题仍然存在**，添加更详细的调试日志
3. **如果确认是 SDK/Envoy 问题**，考虑升级或报告 bug
4. **如果需要快速 workaround**，可以尝试方案 2
