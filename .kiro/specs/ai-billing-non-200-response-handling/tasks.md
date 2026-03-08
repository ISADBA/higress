# 实现计划

- [x] 1. 编写 bug 条件探索性测试
  - **Property 1: Fault Condition** - 非 200 响应日志信息不足和响应透传问题
  - **关键**: 此测试必须在未修复的代码上失败 - 失败确认 bug 存在
  - **不要在测试失败时尝试修复测试或代码**
  - **注意**: 此测试编码了期望行为 - 在实现修复后测试通过时将验证修复
  - **目标**: 暴露反例以证明 bug 存在
  - **限定范围的 PBT 方法**: 对于确定性 bug，将属性限定到具体的失败案例以确保可重现性
  - 测试实现细节来自设计文档中的故障条件
  - 测试断言应该匹配设计文档中的期望行为属性
  - 在未修复的代码上运行测试
  - **期望结果**: 测试失败（这是正确的 - 证明 bug 存在）
  - 记录发现的反例以理解根本原因
  - 当测试编写完成、运行并记录失败时标记任务完成
  - _Requirements: 2.1, 2.2, 2.3, 2.4_

  - [x] 1.1 测试 4xx 错误的日志信息不足
    - 模拟 Gemini 返回 421 状态码
    - 设置 context 包含 TenantInfo（ConsumerName="test-consumer"）
    - 设置 provider="gemini", model="gemini-pro"
    - 断言日志只包含 status，缺少 consumer/provider/model 信息
    - 断言日志级别是 Debug 而不是 Info
    - 在未修复代码上运行 - 期望失败

  - [x] 1.2 测试 5xx 错误的日志信息不足
    - 模拟 OpenAI 返回 500 状态码
    - 设置 context 包含 TenantInfo（ConsumerID="consumer-123"）
    - 设置 provider="openai", model="gpt-4"
    - 断言日志只包含 status，缺少上下文信息
    - 断言日志级别是 Debug 而不是 Warn
    - 在未修复代码上运行 - 期望失败

  - [x] 1.3 测试其他非 200 状态码的日志信息不足
    - 模拟返回 301 状态码（重定向）
    - 设置 context 包含 TenantInfo
    - 断言日志缺少 consumer/provider/model 信息
    - 在未修复代码上运行 - 期望失败

  - [x] 1.4 测试响应透传问题
    - 模拟 429 状态码和错误消息体
    - 验证响应是否能正确透传给客户端
    - 验证客户端是否收到原始状态码和消息体
    - 在未修复代码上运行 - 可能失败（如果存在 HTTP/2 流问题）

- [x] 2. 编写保持不变属性测试（在实现修复之前）
  - **Property 2: Preservation** - 200 响应的正常计费流程
  - **重要**: 遵循观察优先方法
  - 在未修复的代码上观察非 bug 输入的行为
  - 编写基于属性的测试来捕获设计文档中保持不变需求的观察行为模式
  - 基于属性的测试生成许多测试用例以提供更强保证
  - 在未修复的代码上运行测试
  - **期望结果**: 测试通过（这确认了要保持的基线行为）
  - 当测试编写完成、运行并在未修复代码上通过时标记任务完成
  - _Requirements: 3.1, 3.2, 3.3, 3.4_

  - [x] 2.1 观察并测试流式响应处理保持不变
    - 观察未修复代码对 200 + text/event-stream 的处理
    - 编写基于属性的测试：对于所有 200 + 流式响应
    - 验证 `isStreaming` 上下文变量被正确设置为 true
    - 验证不调用 `BufferResponseBody()`
    - 在未修复代码上运行 - 期望通过

  - [x] 2.2 观察并测试非流式响应处理保持不变
    - 观察未修复代码对 200 + application/json 的处理
    - 编写基于属性的测试：对于所有 200 + 非流式响应
    - 验证 `isStreaming` 上下文变量被正确设置为 false
    - 验证调用 `BufferResponseBody()`
    - 在未修复代码上运行 - 期望通过

  - [x] 2.3 观察并测试计费流程保持不变
    - 观察未修复代码的 200 响应后续计费逻辑
    - 验证 `onHttpResponseBody` 能正常提取计费信息
    - 验证能正常向 cost 服务发送计费请求
    - 在未修复代码上运行 - 期望通过

  - [x] 2.4 观察并测试请求拒绝场景保持不变
    - 观察未修复代码对 `CtxKeyRequestDenied` 为 true 的处理
    - 验证直接返回 ActionContinue，跳过响应处理
    - 在未修复代码上运行 - 期望通过

- [x] 3. 修复非 200 响应处理

  - [x] 3.1 实现修复
    - 修改 `plugins/wasm-go/extensions/ai-billing/main.go` 中的 `onHttpResponseHeaders` 函数（第 598-625 行）
    - 添加上下文信息提取逻辑：
      - 从 context 提取 TenantInfo（使用 `ctx.GetContext(CtxKeyTenantInfo)`）
      - 从 TenantInfo 提取 ConsumerID 和 ConsumerName
      - 调用 `extractProvider(ctx)` 提取 provider
      - 调用 `extractModel(ctx)` 提取 model
    - 实现分级日志记录：
      - 解析状态码，判断是 4xx、5xx 还是其他
      - 4xx 错误：使用 `log.Infof()` 记录
      - 5xx 错误：使用 `log.Warnf()` 记录
      - 其他非 200：使用 `log.Debugf()` 记录
      - 日志格式：`[ai-billing] skipping billing for non-200 response: consumer=%s, provider=%s, model=%s, status=%s`
    - 确保响应透传：
      - 对于非 200 响应，直接返回 `types.ActionContinue`
      - 不调用 `ctx.BufferResponseBody()`
      - 不调用 `sendErrorResponse()`
    - 安全加固：
      - 确保日志中不包含 TenantID、DomainResourceID、RouterResourceID 等敏感信息
      - 只记录 ConsumerID/ConsumerName、Provider、Model、Status
    - _Bug_Condition: isBugCondition(input) where statusCode != "200"_
    - _Expected_Behavior: expectedBehavior(result) from design - 分级日志记录 + 响应透传_
    - _Preservation: 200 响应的正常计费流程保持不变_
    - _Requirements: 2.1, 2.2, 2.3, 2.4, 3.1, 3.2, 3.3, 3.4_

  - [x] 3.2 验证 bug 条件探索性测试现在通过
    - **Property 1: Expected Behavior** - 非 200 响应日志信息完整和响应透传正常
    - **重要**: 重新运行任务 1 中的相同测试 - 不要编写新测试
    - 任务 1 中的测试编码了期望行为
    - 当此测试通过时，确认期望行为得到满足
    - 运行任务 1 中的 bug 条件探索性测试
    - **期望结果**: 测试通过（确认 bug 已修复）
    - _Requirements: 2.1, 2.2, 2.3, 2.4_

  - [x] 3.3 验证保持不变测试仍然通过
    - **Property 2: Preservation** - 200 响应的正常计费流程
    - **重要**: 重新运行任务 2 中的相同测试 - 不要编写新测试
    - 运行任务 2 中的保持不变属性测试
    - **期望结果**: 测试通过（确认没有回归）
    - 确认修复后所有测试仍然通过（没有回归）

- [x] 4. 检查点 - 确保所有测试通过
  - 确保所有测试通过，如有问题请询问用户
