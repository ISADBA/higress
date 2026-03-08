# AI Billing 非 200 响应处理 Bugfix 设计文档

## 概述

当前 ai-billing 插件在处理 AI 提供商（Gemini、OpenAI 等）返回的非 200 HTTP 响应时存在缺陷。虽然插件正确地跳过了计费，但破坏了 HTTP/2 流，导致客户端收到不友好的 `INTERNAL_ERROR (err 2)` 错误，而不是提供商的原始错误消息和状态码。

本次修复采用混合方案：对非 200 响应进行分级日志记录（4xx 用 Info，5xx 用 Warn，其他用 Debug），同时将响应体直接透传给客户端，不进行任何拦截或修改。

## 术语表

- **Bug_Condition (C)**: 触发 bug 的条件 - 当 AI 提供商返回非 200 HTTP 状态码时
- **Property (P)**: 期望的行为 - 非 200 响应应该被透传给客户端，同时记录适当级别的日志
- **Preservation**: 必须保持不变的现有行为 - 200 响应的正常计费流程
- **onHttpResponseHeaders**: `plugins/wasm-go/extensions/ai-billing/main.go` 中的函数（第 598-625 行），负责处理响应头阶段的逻辑
- **TenantInfo**: 包含租户和消费者信息的结构体，包括 ConsumerID、ConsumerName、TenantID 等字段
- **Context**: HTTP 上下文，用于在请求/响应处理的不同阶段之间传递信息

## Bug 详情

### 故障条件

当 AI 提供商返回非 200 HTTP 状态码时，bug 会显现。`onHttpResponseHeaders` 函数虽然正确地跳过了计费，但没有提供足够的上下文信息用于调试，并且后续的响应处理逻辑可能导致 HTTP/2 流被破坏。

**形式化规范：**
```
FUNCTION isBugCondition(input)
  INPUT: input of type HttpContext
  OUTPUT: boolean
  
  statusCode := getHttpResponseHeader(":status")
  RETURN statusCode != "200"
         AND statusCode IN ['4xx', '5xx', 'other non-200']
         AND (logContextInsufficient(statusCode) 
              OR responseStreamBroken(input))
END FUNCTION
```

### 示例

- **示例 1**: Gemini 返回 421 状态码（Misdirected Request）
  - 当前行为：客户端收到 `curl: (92) HTTP/2 stream 1 was not closed cleanly: INTERNAL_ERROR (err 2)`
  - 日志：`[ai-billing] skipping billing for non-200 response: status=421`（缺少 consumer、provider、model 信息）
  - 期望行为：客户端收到 421 状态码和 Gemini 的原始错误消息，日志包含完整上下文

- **示例 2**: OpenAI 返回 429 状态码（Too Many Requests）
  - 当前行为：客户端收到 HTTP/2 INTERNAL_ERROR
  - 日志：`[ai-billing] skipping billing for non-200 response: status=429`
  - 期望行为：客户端收到 429 状态码和 OpenAI 的速率限制错误消息，日志记录为 Info 级别并包含 consumer、provider、model

- **示例 3**: AI 提供商返回 500 状态码（Internal Server Error）
  - 当前行为：客户端收到 HTTP/2 INTERNAL_ERROR
  - 日志：`[ai-billing] skipping billing for non-200 response: status=500`
  - 期望行为：客户端收到 500 状态码和提供商的错误详情，日志记录为 Warn 级别并包含完整上下文

- **边缘情况**: AI 提供商返回 301 状态码（重定向）
  - 期望行为：客户端收到 301 状态码，日志记录为 Debug 级别并包含 status

## 期望行为

### 保持不变的行为

**不变的行为：**
- 200 响应的正常计费流程必须继续正常工作
- 从 200 响应中提取计费信息的逻辑必须保持不变
- 向 cost 服务发送计费请求的流程必须保持不变
- 流式响应和非流式响应的检测和处理逻辑必须保持不变

**范围：**
所有不涉及非 200 HTTP 状态码的输入都应该完全不受此修复的影响。这包括：
- 200 状态码的成功响应
- 流式响应（text/event-stream）的处理
- 非流式响应的缓冲和解析
- 计费信息的提取和发送

## 假设的根本原因

基于 bug 描述，最可能的问题是：

1. **日志信息不足**: 当前代码只记录状态码，没有记录 consumer、provider、model 等关键上下文信息
   - 第 611 行：`log.Debugf("[%s] skipping billing for non-200 response: status=%s", pluginName, statusCode)`
   - 缺少从 context 提取 TenantInfo 和其他上下文信息的逻辑

2. **响应流处理不当**: 虽然代码返回了 `types.ActionContinue`，但可能在后续的响应体处理阶段出现问题
   - 对于非 200 响应，不应该调用 `ctx.BufferResponseBody()`
   - 应该让响应体直接透传，不进行任何拦截

3. **日志级别不合理**: 所有非 200 响应都使用 Debug 级别，不利于生产环境的问题排查
   - 4xx 错误应该使用 Info 级别（客户端错误，需要关注）
   - 5xx 错误应该使用 Warn 级别（服务端错误，需要警告）
   - 其他非 200 应该使用 Debug 级别

4. **安全风险**: 如果在日志中包含 tenant 信息，可能导致敏感信息泄露
   - 日志应该只包含 consumer、provider、model、status
   - 不应该包含 TenantID、DomainResourceID、RouterResourceID 等敏感信息

## 正确性属性

Property 1: 故障条件 - 非 200 响应透传和分级日志

_对于任何_ HTTP 响应，如果状态码不是 200（isBugCondition 返回 true），修复后的 onHttpResponseHeaders 函数应该：
1. 跳过计费逻辑
2. 根据状态码类型记录不同级别的日志（4xx 用 Info，5xx 用 Warn，其他用 Debug）
3. 日志包含 consumer、provider、model、status 信息（但不包含 tenant 敏感信息）
4. 直接返回 `types.ActionContinue`，让响应体透传给客户端
5. 不调用 `BufferResponseBody()` 或 `sendErrorResponse()`

**验证需求：2.1, 2.2, 2.3, 2.4**

Property 2: 保持不变 - 200 响应的正常计费流程

_对于任何_ HTTP 响应，如果状态码是 200（isBugCondition 返回 false），修复后的代码应该产生与原始代码完全相同的行为，保持正常的计费流程，包括：
1. 检测流式/非流式响应
2. 对非流式响应调用 `BufferResponseBody()`
3. 在响应体阶段提取计费信息
4. 向 cost 服务发送计费请求

**验证需求：3.1, 3.2, 3.3, 3.4**

## 修复实现

### 需要的修改

假设我们的根本原因分析是正确的：

**文件**: `plugins/wasm-go/extensions/ai-billing/main.go`

**函数**: `onHttpResponseHeaders`（第 598-625 行）

**具体修改**:

1. **添加上下文信息提取逻辑**:
   - 从 context 提取 TenantInfo（使用 `ctx.GetContext(CtxKeyTenantInfo)`）
   - 从 TenantInfo 提取 ConsumerID 和 ConsumerName
   - 调用 `extractProvider(ctx)` 提取 provider
   - 调用 `extractModel(ctx)` 提取 model

2. **实现分级日志记录**:
   - 解析状态码，判断是 4xx、5xx 还是其他
   - 4xx 错误：使用 `log.Infof()` 记录
   - 5xx 错误：使用 `log.Warnf()` 记录
   - 其他非 200：使用 `log.Debugf()` 记录
   - 日志格式：`[ai-billing] skipping billing for non-200 response: consumer=%s, provider=%s, model=%s, status=%s`

3. **确保响应透传**:
   - 对于非 200 响应，直接返回 `types.ActionContinue`
   - 不调用 `ctx.BufferResponseBody()`
   - 不调用 `sendErrorResponse()`
   - 让响应体在后续阶段直接透传给客户端

4. **安全加固**:
   - 确保日志中不包含 TenantID、DomainResourceID、RouterResourceID 等敏感信息
   - 只记录 ConsumerID/ConsumerName、Provider、Model、Status

5. **代码结构优化**:
   ```go
   func onHttpResponseHeaders(ctx wrapper.HttpContext, config BillingConfig) types.Action {
       // 1. 检查请求阶段是否被拒绝（保持不变）
       if denied, ok := ctx.GetContext(CtxKeyRequestDenied).(bool); ok && denied {
           log.Debugf("[%s] request was denied, skipping response processing", pluginName)
           return types.ActionContinue
       }

       log.Debugf("[%s] processing response headers", pluginName)

       // 2. 检查 HTTP 状态码
       statusCode, err := proxywasm.GetHttpResponseHeader(":status")
       if err != nil || statusCode != "200" {
           // 3. 提取上下文信息
           consumer := "unknown"
           if tenantInfo, ok := ctx.GetContext(CtxKeyTenantInfo).(*TenantInfo); ok && tenantInfo != nil {
               if tenantInfo.ConsumerName != "" {
                   consumer = tenantInfo.ConsumerName
               } else if tenantInfo.ConsumerID != "" {
                   consumer = tenantInfo.ConsumerID
               }
           }
           provider := extractProvider(ctx)
           model := extractModel(ctx)

           // 4. 根据状态码类型记录不同级别日志
           statusInt, _ := strconv.Atoi(statusCode)
           if statusInt >= 400 && statusInt < 500 {
               log.Infof("[%s] skipping billing for 4xx response: consumer=%s, provider=%s, model=%s, status=%s",
                   pluginName, consumer, provider, model, statusCode)
           } else if statusInt >= 500 && statusInt < 600 {
               log.Warnf("[%s] skipping billing for 5xx response: consumer=%s, provider=%s, model=%s, status=%s",
                   pluginName, consumer, provider, model, statusCode)
           } else {
               log.Debugf("[%s] skipping billing for non-200 response: consumer=%s, provider=%s, model=%s, status=%s",
                   pluginName, consumer, provider, model, statusCode)
           }

           // 5. 直接返回，让响应透传
           return types.ActionContinue
       }

       // 6. 200 响应：继续正常计费流程（保持不变）
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

## 测试策略

### 验证方法

测试策略遵循两阶段方法：首先，在未修复的代码上运行测试以暴露 bug 的反例；然后，验证修复后的代码正确工作并保持现有行为。

### 探索性故障条件检查

**目标**: 在实施修复之前，在未修复的代码上暴露 bug 的反例。确认或反驳根本原因分析。如果反驳，我们需要重新假设。

**测试计划**: 编写测试来模拟 AI 提供商返回非 200 状态码的场景，并断言：
1. 日志中缺少 consumer、provider、model 信息
2. 客户端可能收到 HTTP/2 INTERNAL_ERROR（如果能复现）

在未修复的代码上运行这些测试以观察失败并理解根本原因。

**测试用例**:
1. **4xx 错误测试**: 模拟 Gemini 返回 421 状态码（未修复代码上会失败）
   - 验证日志只包含 status，缺少 consumer/provider/model
   - 验证日志级别是 Debug 而不是 Info

2. **5xx 错误测试**: 模拟 OpenAI 返回 500 状态码（未修复代码上会失败）
   - 验证日志只包含 status，缺少上下文信息
   - 验证日志级别是 Debug 而不是 Warn

3. **其他非 200 测试**: 模拟返回 301 状态码（未修复代码上会失败）
   - 验证日志缺少上下文信息
   - 验证日志级别是 Debug

4. **响应透传测试**: 验证非 200 响应是否能正确透传给客户端（可能在未修复代码上失败）
   - 模拟 429 状态码和错误消息体
   - 验证客户端是否收到原始状态码和消息体

**期望的反例**:
- 日志中缺少 consumer、provider、model 信息
- 可能的原因：未从 context 提取 TenantInfo，未调用 extractProvider/extractModel

### 修复检查

**目标**: 验证对于所有满足 bug 条件的输入，修复后的函数产生期望的行为。

**伪代码:**
```
FOR ALL input WHERE isBugCondition(input) DO
  result := onHttpResponseHeaders_fixed(input)
  ASSERT expectedBehavior(result)
END FOR
```

**期望行为验证**:
- 对于 4xx 状态码：日志级别是 Info，包含完整上下文
- 对于 5xx 状态码：日志级别是 Warn，包含完整上下文
- 对于其他非 200：日志级别是 Debug，包含完整上下文
- 响应直接透传给客户端，不被拦截或修改

### 保持不变检查

**目标**: 验证对于所有不满足 bug 条件的输入，修复后的函数产生与原始函数相同的结果。

**伪代码:**
```
FOR ALL input WHERE NOT isBugCondition(input) DO
  ASSERT onHttpResponseHeaders_original(input) = onHttpResponseHeaders_fixed(input)
END FOR
```

**测试方法**: 建议使用基于属性的测试进行保持不变检查，因为：
- 它自动生成许多测试用例覆盖输入域
- 它能捕获手动单元测试可能遗漏的边缘情况
- 它为所有非 bug 输入提供强有力的保证，确保行为不变

**测试计划**: 首先在未修复的代码上观察 200 响应的行为，然后编写基于属性的测试来捕获该行为。

**测试用例**:
1. **流式响应保持不变**: 观察未修复代码对 200 + text/event-stream 的处理，然后验证修复后行为相同
   - 验证 `isStreaming` 上下文变量被正确设置
   - 验证不调用 `BufferResponseBody()`

2. **非流式响应保持不变**: 观察未修复代码对 200 + application/json 的处理，然后验证修复后行为相同
   - 验证 `isStreaming` 上下文变量被正确设置为 false
   - 验证调用 `BufferResponseBody()`

3. **计费流程保持不变**: 验证 200 响应后续的计费逻辑不受影响
   - 验证 `onHttpResponseBody` 能正常提取计费信息
   - 验证能正常向 cost 服务发送计费请求

4. **请求拒绝场景保持不变**: 验证请求阶段被拒绝的场景不受影响
   - 验证 `CtxKeyRequestDenied` 为 true 时直接返回 ActionContinue

### 单元测试

- 测试 4xx 状态码的日志记录（Info 级别，包含完整上下文）
- 测试 5xx 状态码的日志记录（Warn 级别，包含完整上下文）
- 测试其他非 200 状态码的日志记录（Debug 级别，包含完整上下文）
- 测试边缘情况（无 TenantInfo、无 provider、无 model）
- 测试 200 响应的流式/非流式检测逻辑保持不变

### 基于属性的测试

- 生成随机的非 200 状态码，验证日志级别和内容正确
- 生成随机的 200 响应配置（流式/非流式），验证行为与原始代码相同
- 生成随机的上下文信息（有/无 TenantInfo），验证日志记录正确处理各种情况

### 集成测试

- 测试完整的请求-响应流程，包含 4xx 错误响应
- 测试完整的请求-响应流程，包含 5xx 错误响应
- 测试完整的请求-响应流程，包含 200 成功响应（验证计费正常）
- 测试客户端能否正确接收非 200 响应的原始状态码和消息体
