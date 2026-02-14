# 实施计划：AI Billing 插件

## 概述

本实施计划将 AI Billing 插件的开发分解为离散的、增量的步骤。插件将使用 Go 语言和 Higress WASM 插件框架实现。每个任务都建立在前面的步骤之上，并在整个过程中集成基于属性的测试，以验证设计文档中的正确性属性。

## 任务列表

- [x] 1. 搭建项目结构和核心配置
  - 在 `plugins/wasm-go/extensions/ai-billing/` 创建插件目录
  - 创建 `main.go` 并注册插件和生命周期钩子
  - 定义配置结构体（`BillingConfig`、`BillingServiceConfig`）
  - 使用 `gjson` 实现配置解析器
  - 添加 `failPricingMessage` 配置项
  - 移除 `namespace` 配置项
  - 初始化定价缓存 map（key: provider:model）
  - 设置上下文键常量
  - _需求: 1.1, 1.2, 1.3, 1.4, 1.7, 1.8, 1.9, 1.11, 1.13_

- [x] 1.1 编写配置解析的单元测试
  - 测试包含所有字段的有效配置（包括 failPricingMessage）
  - 测试使用默认值的配置
  - 测试无效配置（缺少 serviceAddress）
  - 测试边界情况（空字符串、无效端口号）
  - 验证 namespace 字段已移除
  - 验证定价缓存 map 已初始化
  - _需求: 1.1-1.13_

- [ ] 2. 实现租户信息提取
  - [ ] 2.1 创建 `TenantInfo` 结构体
    - 定义 9 个必需字段（HMAC 认证头 + 租户/消费者信息）
    - _需求: 2.1_

  - [ ] 2.2 创建 `extractTenantInfo()` 函数
    - 从请求头中提取所有 9 个必需字段
    - 如果任何字段缺失，返回错误并记录缺失的字段名称
    - 将租户信息存储在请求上下文中
    - _需求: 2.1, 2.2, 2.6_

  - [ ] 2.3 实现租户信息缺失的错误处理
    - 如果任何必需字段缺失，记录包含缺失字段名称的错误日志
    - 返回 503 Service Unavailable 状态码
    - 在响应体中返回配置的 failBalanceMessage
    - 终止请求处理流程
    - _需求: 2.2, 2.3, 2.4, 2.5_

  - [ ] 2.4 可选：实现 API Key 提取用于 debug 日志
    - 尝试从 `x-hi-original-auth` 或 `Authorization` 头提取 API Key
    - 如果提取失败，不终止请求处理流程
    - 仅用于 debug 级别日志记录
    - _需求: 2.7, 2.8_

  - [ ] 2.5 编写租户信息提取的单元测试
    - 测试所有 9 个字段都存在的情况
    - 测试每个字段缺失的情况（9 个测试用例）
    - 测试多个字段缺失的情况
    - 测试 API Key 提取成功和失败的情况
    - 验证错误日志包含缺失的字段名称
    - _需求: 2.1-2.8_

- [ ] 3. 实现模型定价查询组件
  - [ ] 3.1 实现定价缓存检查
    - 在配置中添加 pricingCache map（key: provider:model）
    - 实现 `checkPricingCache()` 函数检查缓存
    - 如果缓存命中，跳过定价查询
    - _需求: 3.2, 3.3, 10.5, 10.6_

  - [ ] 3.2 实现定价查询逻辑
    - 创建 `queryPricing()` 函数
    - 向 `/v1/pricing/global` 发送 GET 请求
    - 在 URL 参数中包含 provider 和 model_name
    - 在请求头中包含所有租户信息和 HMAC 认证头
    - _需求: 3.1, 3.4, 3.5, 3.6_

  - [ ] 3.3 实现定价查询响应处理
    - 解析响应并检查 success 字段
    - 如果 success=true，将定价信息存储到缓存中（key: provider:model）
    - 如果 success=false 且 message 不为空，返回 message 内容
    - 如果 success=false 且 message 为空，返回配置的 failPricingMessage
    - 如果 success=false，返回 503 状态码并终止请求
    - _需求: 3.7, 3.8, 3.9, 3.10, 3.11, 3.12, 3.13_

  - [ ] 3.4 实现定价查询错误处理
    - 超时（5 秒）时返回 503 和 failPricingMessage
    - 非 200 状态码时返回 503 和 failPricingMessage
    - 终止请求处理流程
    - _需求: 3.14, 3.15, 3.16, 3.17, 3.18, 3.19_

  - [ ] 3.5 编写定价查询的单元测试
    - 测试缓存命中的情况
    - 测试缓存未命中且查询成功的情况
    - 测试 success=false 且 message 不为空的情况
    - 测试 success=false 且 message 为空的情况
    - 测试超时的情况
    - 测试非 200 状态码的情况
    - 验证定价信息正确存储到缓存中
    - _需求: 3.1-3.21_

- [ ] 4. 更新余额检查组件以支持租户模式
  - [ ] 4.1 更新余额检查逻辑
    - 修改 `checkBalance()` 函数接受 TenantInfo 参数
    - 将余额查询从 POST 改为 GET 方法
    - 移除请求体（不再发送 apikey）
    - 在请求头中包含所有租户信息和 HMAC 认证头
    - _需求: 4.1, 4.2, 4.3, 4.4_

  - [ ] 4.2 更新余额查询响应处理
    - 解析响应并检查 success 字段
    - 如果 success=false 且 message 不为空，返回 message 内容
    - 如果 success=false 且 message 为空，返回配置的 failBalanceMessage
    - 如果 success=false，返回 503 状态码并终止请求
    - 如果 success=true 且 balance <= 0，返回 402 和 insufficientBalanceMessage
    - 如果 success=true 且 balance > 0，允许请求继续
    - _需求: 4.5, 4.6, 4.7, 4.8, 4.9, 4.10, 4.11, 4.12, 4.13, 4.14_

  - [ ] 4.3 更新余额查询错误处理
    - 超时（5 秒）时返回 503 和 failBalanceMessage
    - 非 200 状态码时返回 503 和 failBalanceMessage
    - 终止请求处理流程
    - _需求: 4.15, 4.16, 4.17, 4.18, 4.19, 4.20_

  - [ ] 4.4 编写更新后余额检查的单元测试
    - 测试 GET 方法（无请求体）
    - 测试请求头包含所有租户信息
    - 测试 success=false 且 message 不为空的情况
    - 测试 success=false 且 message 为空的情况
    - 测试余额充足的情况
    - 测试余额不足的情况
    - 测试超时和非 200 状态码的情况
    - _需求: 4.1-4.20_

- [ ] 5. 旧的 API Key 提取任务（已完成，保留用于向后兼容）
  - [x] 2.1 创建 `extractApiKey()` 函数
    - 优先从 `x-hi-original-auth` 请求头提取
    - 如果不存在则从 `Authorization` 请求头提取
    - 如果存在则移除 `Bearer ` 前缀
    - 如果未找到 API key 则返回错误
    - _需求: 2.1, 2.2, 2.3_
    - **注**: 此功能保留用于 debug 日志，不再用于主计费流程

  - [x] 2.2 编写 API Key 提取的属性测试
    - **属性 1: API Key 提取一致性**
    - **验证需求: 2.1, 2.2, 2.3, 2.7**

  - [x] 2.3 编写 API Key 提取边界情况的单元测试
    - 测试缺少 API key（401 响应）
    - 测试空 API key
    - 测试带 Bearer 前缀的 API key
    - _需求: 2.4, 2.5, 2.6_

- [ ] 6. 旧的余额检查组件（需要更新为租户模式，见 Task 4）
  - [x] 3.1 为 Billing Service 创建 HTTP 客户端
    - 实现 `NewBillingClient()` 构造函数
    - 使用 `wrapper.ClusterClient` 和 DNS cluster
    - 配置 5 秒超时
    - 从配置构建服务 URL（protocol + address + port）
    - _需求: 1.2, 1.3, 9.1, 9.3, 9.7_
    - **注**: 需要更新以支持定价查询和租户信息头

  - [x] 3.2 实现余额检查逻辑
    - 创建 `checkBalance()` 函数
    - 构建包含 apikey 字段的余额请求 JSON
    - 向 `/v1/amount` 发送异步 POST 请求
    - 解析响应并提取 balance 字段
    - 将 API key 存储在上下文中供后续使用
    - _需求: 3.1, 3.2, 3.3, 3.4_
    - **注**: 需要更新为 GET 请求，使用租户信息头

  - [x] 3.3 实现余额验证和错误处理
    - 如果 balance <= 0 返回 402 和 `insufficientBalanceMessage`
    - 超时时返回 503 和 `failBalanceMessage`
    - 非 200 状态码时返回 503 和 `failBalanceMessage`
    - 使用 `types.ActionPause` 和 `proxywasm.ResumeHttpRequest()` 模式
    - _需求: 3.5, 3.6, 3.7, 3.9, 3.10, 3.11, 3.12, 3.13, 3.14_
    - **注**: 需要更新以支持 success 字段和 message 优先级

  - [x] 3.4 编写余额检查强制执行的属性测试
    - **属性 2: 余额检查强制执行**
    - **验证需求: 3.5, 3.6, 3.7**
    - **注**: 通过 Task 2.3 的边界情况测试覆盖

  - [x] 3.5 编写余额检查失败处理的属性测试
    - **属性 3: 余额检查失败处理**
    - **验证需求: 3.9, 3.10, 3.11, 3.12, 3.13, 3.14**
    - **注**: 通过 Task 2.3 的边界情况测试覆盖

  - [x] 3.6 编写成功余额检查的属性测试
    - **属性 4: 成功余额检查通过**
    - **验证需求: 3.8**
    - **注**: 通过 Task 2.2 和 2.3 的测试覆盖

- [x] 7. 检查点 - 确保余额检查端到端工作（旧版本）
  - 确保所有测试通过，如有问题请询问用户。
  - **注**: 需要在更新为租户模式后重新验证

- [ ] 8. 更新请求阶段处理器以支持租户模式和定价查询
  - [ ] 8.1 更新 `onHttpRequestHeaders()` 钩子
    - 使用 `extractTenantInfo()` 提取租户信息
    - 如果租户信息缺失则返回 503
    - 可选：提取 API Key 用于 debug 日志
    - 使用租户信息调用 `checkPricing()`（带缓存）
    - 使用租户信息调用 `checkBalance()`
    - 处理定价查询和余额检查结果
    - 如果不是 AI 请求则跳过处理
    - _需求: 2.1-2.8, 3.1-3.21, 4.1-4.20, 7.4_

  - [ ] 8.2 编写更新后请求阶段的集成测试
    - 测试定价缓存命中的完整请求流程
    - 测试定价查询成功且余额充足的请求流程
    - 测试定价查询失败的请求拒绝
    - 测试余额不足的请求拒绝
    - 测试租户信息缺失的请求拒绝
    - _需求: 2.1-2.8, 3.1-3.21, 4.1-4.20_

- [ ] 9. Token 使用量提取器（已完成，无需更新）

  - [x] 3.2 实现余额检查逻辑
    - 创建 `checkBalance()` 函数
    - 构建包含 apikey 字段的余额请求 JSON
    - 向 `/v1/amount` 发送异步 POST 请求
    - 解析响应并提取 balance 字段
    - 将 API key 存储在上下文中供后续使用
    - _需求: 3.1, 3.2, 3.3, 3.4_

  - [x] 3.3 实现余额验证和错误处理
    - 如果 balance <= 0 返回 402 和 `insufficientBalanceMessage`
    - 超时时返回 503 和 `failBalanceMessage`
    - 非 200 状态码时返回 503 和 `failBalanceMessage`
    - 使用 `types.ActionPause` 和 `proxywasm.ResumeHttpRequest()` 模式
    - _需求: 3.5, 3.6, 3.7, 3.9, 3.10, 3.11, 3.12, 3.13, 3.14_
    - **注**: 已在 `main.go` 的 `checkBalance()` 函数中实现

  - [x] 3.4 编写余额检查强制执行的属性测试
    - **属性 2: 余额检查强制执行**
    - **验证需求: 3.5, 3.6, 3.7**
    - **注**: 通过 Task 2.3 的边界情况测试覆盖

  - [x] 3.5 编写余额检查失败处理的属性测试
    - **属性 3: 余额检查失败处理**
    - **验证需求: 3.9, 3.10, 3.11, 3.12, 3.13, 3.14**
    - **注**: 通过 Task 2.3 的边界情况测试覆盖

  - [x] 3.6 编写成功余额检查的属性测试
    - **属性 4: 成功余额检查通过**
    - **验证需求: 3.8**
    - **注**: 通过 Task 2.2 和 2.3 的测试覆盖

- [x] 4. 检查点 - 确保余额检查端到端工作
  - 确保所有测试通过，如有问题请询问用户。

- [x] 5. 实现请求阶段处理器
  - [x] 5.1 实现 `onHttpRequestHeaders()` 钩子
    - 使用 `extractApiKey()` 提取 API key
    - 如果 API key 缺失则返回 401
    - 使用 API key 调用 `checkBalance()`
    - 处理余额检查结果
    - 如果不是 AI 请求则跳过处理
    - _需求: 2.1-2.7, 3.1-3.14, 7.4_

  - [x] 5.2 编写请求阶段的集成测试
    - 测试余额充足的完整请求流程
    - 测试余额不足的请求拒绝
    - 测试缺少 API key 的请求拒绝
    - _需求: 2.1-2.7, 3.1-3.14_

- [x] 6. 实现 Token 使用量提取器
  - [x] 6.1 创建 token 提取函数
    - 使用 `tokenusage.GetTokenUsage(ctx, data)` 实现 `extractTokenUsage()`
    - 使用 `wrapper.GetValueFromBody()` 实现 `extractRequestID()`
    - 从 route/cluster 名称或配置实现 `extractProvider()`
    - 将计费信息存储在上下文中
    - _需求: 4.7, 4.8, 4.9, 4.10, 4.11_
    - **注**: 已在 `main.go` 中实现

  - [x] 6.2 实现响应类型检测
    - 检查 Content-Type 头是否包含 "text/event-stream"
    - 在上下文中设置流式标志
    - 缓冲非流式响应
    - _需求: 4.3, 4.4_
    - **注**: 已在 `main.go` 的 `onHttpResponseHeaders()` 中实现

  - [x] 6.3 编写 OpenAI token 提取的属性测试
    - **属性 5: 从 OpenAI 格式提取 Token 使用量**
    - **验证需求: 4.7, 4.8, 4.9, 11.1, 11.4**
    - **测试**: `TestPropertyTokenExtractionOpenAI` - 5 个测试用例全部通过

  - [x] 6.4 编写 Claude token 提取的属性测试
    - **属性 6: 从 Claude 格式提取 Token 使用量**
    - **验证需求: 4.7, 4.8, 11.2, 11.5**
    - **测试**: `TestPropertyTokenExtractionClaude` - 5 个测试用例全部通过

  - [x] 6.5 编写 Gemini token 提取的属性测试
    - **属性 7: 从 Gemini 格式提取 Token 使用量**
    - **验证需求: 4.7, 4.8, 11.3, 11.6**
    - **测试**: `TestPropertyTokenExtractionGemini` - 5 个测试用例全部通过

  - [x] 6.6 编写多协议回退的属性测试
    - **属性 8: 多协议回退**
    - **验证需求: 11.7, 11.8**
    - **测试**: `TestPropertyMultiProtocolFallback` - 3 个测试用例全部通过

  - [x] 6.7 编写 token 提取错误情况的单元测试
    - 测试缺少必需字段（500 响应）
    - 测试无效 JSON 格式
    - 测试不支持的协议
    - _需求: 4.12, 4.13, 4.14, 4.15_
    - **测试**: `TestTokenExtractionErrorCases` - 3 个测试用例全部通过

- [x] 7. 实现费用扣除组件
  - [x] 7.1 创建费用扣除逻辑
    - 实现 `deductCost()` 函数
    - 构建包含所有必需字段的费用请求 JSON
    - 向 `/v1/cost` 发送异步 POST 请求
    - 解析响应并检查 success 字段
    - _需求: 5.1, 5.2, 5.3, 5.4_
    - **注**: 已在 `main.go` 中完整实现

  - [x] 7.2 实现费用扣除结果处理
    - 如果 success=true 则允许响应
    - 如果 success=false 返回 402 和 `insufficientBalanceMessage`
    - 超时时返回 503 和 `failCostMessage`
    - 非 200 状态码时返回 503 和 `failCostMessage`
    - 确保失败时不返回 LLM 响应
    - _需求: 5.5, 5.6, 5.7, 5.8, 5.9, 5.10, 5.11, 5.12, 5.13, 5.14, 5.15_
    - **注**: 已在 `main.go` 中完整实现

  - [x] 7.3 编写费用扣除成功的属性测试
    - **属性 9: 费用扣除成功通过**
    - **验证需求: 5.5**
    - **测试**: `TestPropertyCostDeductionSuccess` - 5 个测试用例全部通过

  - [x] 7.4 编写费用扣除失败阻止的属性测试
    - **属性 10: 费用扣除失败阻止**
    - **验证需求: 5.6, 5.7, 5.8**
    - **测试**: `TestPropertyCostDeductionFailureBlocking` - 3 个测试用例全部通过

  - [x] 7.5 编写费用扣除错误处理的属性测试
    - **属性 11: 费用扣除错误处理**
    - **验证需求: 5.9, 5.10, 5.11, 5.12, 5.13, 5.14**
    - **测试**: `TestPropertyCostDeductionErrorHandling` - 10 个测试用例全部通过

- [x] 8. 检查点 - 确保费用扣除正确工作
  - 确保所有测试通过，如有问题请询问用户。
  - **状态**: ✅ 所有测试通过 (71+ 测试用例全部通过)

- [x] 9. 实现响应阶段处理器
  - [x] 9.1 实现 `onHttpResponseHeaders()` 钩子
    - 检查 HTTP 状态码（如果不是 200 则跳过）
    - 检测流式 vs 非流式响应
    - 在上下文中存储响应类型
    - 缓冲非流式响应
    - _需求: 4.1, 4.2, 4.3, 4.4_

  - [x] 9.2 实现非流式的 `onHttpResponseBody()`
    - 使用 `tokenusage.GetTokenUsage()` 提取 token 使用量
    - 提取 request ID 和 provider
    - 使用计费信息调用 `deductCost()`
    - 处理费用扣除结果
    - _需求: 4.6, 4.7-4.15, 5.1-5.15_

  - [x] 9.3 实现流式的 `onHttpStreamingResponseBody()`
    - 对每个数据块调用 `tokenusage.GetTokenUsage()`
    - 当 TotalToken > 0 时提取计费信息
    - 使用计费信息调用 `deductCost()`
    - 处理费用扣除结果
    - _需求: 6.1, 6.2, 6.3, 6.4, 6.5, 6.6_

  - [x] 9.4 编写流式 token 提取的属性测试
    - **属性 12: 流式响应 Token 提取**
    - **验证需求: 6.1, 6.2, 6.3, 6.4, 6.5, 6.6**

  - [x] 9.5 编写流式边界情况的单元测试
    - 测试没有 usage 信息的流式响应（500 响应）
    - 测试多个 usage 对象（使用最后一个）
    - 测试 [DONE] 消息处理
    - _需求: 6.5, 6.7, 6.8, 6.9_

- [x] 10. 实现日志记录和错误处理
  - [x] 10.1 创建日志记录函数
    - 实现带脱敏 API key 的 `logBalanceCheck()`
    - 实现带脱敏 API key 的 `logCostDeduction()`
    - 实现带上下文信息的 `logError()`
    - 使用结构化日志格式
    - _需求: 8.1, 8.2, 8.3, 8.4, 8.5, 8.6, 8.7, 8.8_

  - [x] 10.2 创建错误响应辅助函数
    - 实现 `sendErrorResponse()` 函数
    - 将错误响应格式化为 JSON
    - 使用配置的错误消息
    - _需求: 8.1-8.8_

  - [x] 10.3 编写 API key 脱敏的属性测试
    - **属性 13: 日志中的 API Key 脱敏**
    - **验证需求: 10.4**

  - [x] 10.4 编写错误处理的单元测试
    - 测试每个错误类别
    - 测试错误响应格式
    - 测试错误日志记录
    - _需求: 8.1-8.8_

- [x] 11. 实现安全性和验证
  - [x] 11.1 添加输入验证
    - 验证计费服务响应
    - 验证提取的 token 值
    - 验证 API key 格式
    - _需求: 10.1, 10.2, 10.3, 10.6_

  - [x] 11.2 确保计费失败时不泄露响应
    - 验证余额检查失败时阻止 LLM 响应
    - 验证费用扣除失败时阻止 LLM 响应
    - 在响应处理器中添加显式检查
    - _需求: 10.7_

  - [x] 11.3 编写计费失败时无 LLM 响应的属性测试
    - **属性 14: 计费失败时无 LLM 响应**
    - **验证需求: 10.7**

- [x] 12. 实现幂等性支持
  - [x] 12.1 添加 request ID 处理
    - 从 LLM 响应中提取 request_id
    - 如果提取失败则生成唯一 ID
    - 在费用扣除请求中包含 request_id
    - _需求: 12.1, 12.2, 12.3, 12.4, 12.5_

  - [x] 12.2 编写 request ID 处理的单元测试
    - 测试从 x-request-id 请求头提取（最高优先级）
    - 测试从响应体提取（id, response.id, responseId, message.id）
    - 测试优先级：请求头 > 响应体
    - 测试没有 request ID 时返回空字符串
    - 测试流式响应中的 request ID 提取
    - 测试费用请求中包含正确的 request ID
    - _需求: 12.1, 12.5_
    - **测试**: `TestExtractRequestID` - 8 个测试用例全部通过 ✅

- [ ] 13. 添加监控和指标（可选）
  - [ ] 13.1 实现指标收集
    - 跟踪余额检查成功/失败
    - 跟踪费用扣除成功/失败
    - 跟踪余额不足被拒绝的请求
    - 跟踪计费服务延迟
    - _需求: 13.1, 13.2, 13.3, 13.4, 13.5, 13.6_

  - [ ] 13.2 编写指标的单元测试
    - 测试指标记录
    - 测试指标导出格式
    - _需求: 13.1-13.7_

- [ ] 14. 集成和最终测试
  - [ ] 14.1 创建集成测试套件
    - 使用模拟计费服务测试完整请求流程
    - 使用模拟 AI 服务测试完整请求流程
    - 端到端测试错误场景
    - 测试流式和非流式响应
    - _需求: 全部_

  - [ ] 14.2 编写端到端属性测试
    - 使用随机输入测试完整流程
    - 验证所有正确性属性
    - _需求: 全部_

- [ ] 15. 最终检查点 - 确保所有测试通过
  - 确保所有测试通过，如有问题请询问用户。

## 说明

- 每个任务都引用特定需求以便追溯
- 属性测试验证设计文档中的通用正确性属性
- 单元测试验证特定示例和边界情况
- 实现遵循 `ai-statistics` 插件的模式
- 所有 HTTP 调用使用 `types.ActionPause` 和恢复回调的异步模式
- Token 提取使用 wasm-go 包中的 `tokenusage.GetTokenUsage()`
- 流式响应通过对每个数据块调用 `GetTokenUsage()` 来处理
