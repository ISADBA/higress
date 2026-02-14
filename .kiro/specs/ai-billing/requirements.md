# Requirements Document

## Introduction

AI Billing 插件是 Higress 网关的计费能力核心组件，用于在 AI 代理请求的生命周期中实现定价验证、余额检查和费用扣除。该插件与独立的 billing-service 服务集成，确保所有通过网关的 AI 请求都经过计费控制，防止欠费租户继续使用服务。插件采用 FAIL_CLOSE 策略（非流式响应），即任何计费相关的失败都会导致请求终止，确保计费的严格性和准确性。对于流式响应，计费失败会记录详细日志但不阻断响应。

## Glossary

- **AI_Billing_Plugin**: Higress 网关中的 WASM 插件，负责定价验证、余额检查和计费触发
- **Billing_Service**: 独立的计费服务，提供定价查询、余额查询和费用扣除的 HTTP API
- **Tenant_ID**: 租户标识符，用于关联租户账户和余额信息（替代原 API_Key）
- **Consumer_ID**: 消费者标识符，用于标识具体的消费者
- **Consumer_Name**: 消费者名称，用于日志记录和审计
- **API_Key**: 用户身份标识符，仅用于 debug 级别日志记录（可选）
- **Token**: LLM 请求和响应中的计量单位，包括 input_token 和 output_token
- **Provider**: LLM 服务提供商，如 OpenAI、Claude、Gemini 等
- **Model**: 具体的 LLM 模型名称，如 gpt-4、claude-3 等
- **Request_ID**: LLM 请求的唯一标识符
- **FAIL_CLOSE**: 失败策略，计费相关操作失败时终止请求处理（仅适用于非流式响应）
- **SSE**: Server-Sent Events，流式响应的传输协议
- **HTTP_Status_Code**: HTTP 响应状态码，用于判断请求是否成功
- **Service_Protocol**: 计费服务的通信协议（http 或 https）
- **Service_Port**: 计费服务的端口号
- **HMAC**: Hash-based Message Authentication Code，用于请求签名验证的算法
- **Pricing_Cache**: 定价信息缓存，以 provider:model 为 key 存储在插件内存中

## Requirements

### Requirement 1: 插件配置解析

**User Story:** 作为系统管理员，我希望能够通过 YAML 配置文件配置 AI Billing 插件，以便灵活控制计费行为和服务地址。

#### Acceptance Criteria

1. WHEN 插件启动时，THE AI_Billing_Plugin SHALL 解析 WasmPlugin 配置中的 billingService.serviceAddress 字段
2. WHEN 插件启动时，THE AI_Billing_Plugin SHALL 解析 billingService.protocol 字段（http 或 https）
3. WHEN 插件启动时，THE AI_Billing_Plugin SHALL 解析 billingService.port 字段
4. WHEN 插件启动时，THE AI_Billing_Plugin SHALL 解析 failPricingMessage 字段作为定价查询失败时的提示信息
5. WHEN 插件启动时，THE AI_Billing_Plugin SHALL 解析 failBalanceMessage 字段作为余额查询失败时的提示信息
6. WHEN 插件启动时，THE AI_Billing_Plugin SHALL 解析 insufficientBalanceMessage 字段作为余额不足时的提示信息
7. WHEN 插件启动时，THE AI_Billing_Plugin SHALL 解析 failCostMessage 字段作为计费失败时的提示信息
8. WHEN 配置中的 serviceAddress 为空，THE AI_Billing_Plugin SHALL 返回配置错误并拒绝启动
9. WHERE protocol 未配置，THE AI_Billing_Plugin SHALL 默认使用 http 协议
10. WHERE port 未配置，THE AI_Billing_Plugin SHALL 默认使用 8888 端口
11. WHERE failPricingMessage 未配置，THE AI_Billing_Plugin SHALL 使用默认提示信息 "503 Pricing Information Unavailable"
12. WHERE failBalanceMessage 未配置，THE AI_Billing_Plugin SHALL 使用默认提示信息 "503 Billing Service Balance Unavailable"
13. WHERE insufficientBalanceMessage 未配置，THE AI_Billing_Plugin SHALL 使用默认提示信息 "余额不足"
14. WHERE failCostMessage 未配置，THE AI_Billing_Plugin SHALL 使用默认提示信息 "503 Billing Service Cost Unavailable"
15. THE AI_Billing_Plugin SHALL 对非流式响应使用 FAIL_CLOSE 策略（计费失败则请求失败）
16. THE AI_Billing_Plugin SHALL 对流式响应使用异步计费策略（计费失败记录日志但不阻断响应）

### Requirement 2: 租户信息提取

**User Story:** 作为开发者，我希望插件能够从请求头中提取租户信息和 HMAC 认证信息，以便进行基于租户的计费和安全验证。

#### Acceptance Criteria

1. WHEN 请求到达时，THE AI_Billing_Plugin SHALL 从请求头中提取以下必需字段：
   - x-internal-auth-sign-version（HMAC 签名版本）
   - x-internal-auth-ts（时间戳）
   - x-internal-auth-nonce（随机数）
   - x-internal-auth-sign（签名）
   - x-consumer-id（消费者 ID）
   - x-mse-consumer-name（消费者名称）
   - x-mse-tenant-id（租户 ID）
   - x-domain-resource-id（域资源 ID）
   - x-router-resource-id（路由资源 ID）
2. IF 任何必需字段缺失，THEN THE AI_Billing_Plugin SHALL 记录包含缺失字段名称的错误日志
3. IF 任何必需字段缺失，THEN THE AI_Billing_Plugin SHALL 返回 503 Service Unavailable 状态码
4. IF 任何必需字段缺失，THEN THE AI_Billing_Plugin SHALL 在响应体中返回配置的 failBalanceMessage
5. IF 任何必需字段缺失，THEN THE AI_Billing_Plugin SHALL 终止请求处理流程
6. WHEN 租户信息提取成功，THE AI_Billing_Plugin SHALL 将其存储在请求上下文中供后续使用
7. THE AI_Billing_Plugin MAY 尝试提取 API Key（从 x-hi-original-auth 或 Authorization 头）用于 debug 日志
8. WHEN API Key 提取失败，THE AI_Billing_Plugin SHALL NOT 终止请求处理流程（API Key 仅用于日志）

### Requirement 3: 模型定价查询

**User Story:** 作为 SaaS 运营者，我希望在计费前验证模型定价信息已配置，以便确保计费的准确性和避免未配置模型的使用。

#### Acceptance Criteria

1. WHEN 租户信息提取成功，THE AI_Billing_Plugin SHALL 从请求头或上下文中提取 provider 和 model_name 信息
2. WHEN provider 和 model_name 提取成功，THE AI_Billing_Plugin SHALL 检查内存缓存中是否存在该 provider:model 的定价信息
3. IF 缓存中存在定价信息，THEN THE AI_Billing_Plugin SHALL 跳过定价查询并继续执行余额检查
4. IF 缓存中不存在定价信息，THEN THE AI_Billing_Plugin SHALL 向 Billing_Service 发送 GET 请求到 /v1/pricing/global 端点
5. WHEN 发送定价查询请求，THE AI_Billing_Plugin SHALL 在 URL 参数中包含 provider 和 model_name
6. WHEN 发送定价查询请求，THE AI_Billing_Plugin SHALL 在请求头中包含所有提取的租户信息和 HMAC 认证头
7. WHEN Billing_Service 返回定价信息，THE AI_Billing_Plugin SHALL 解析 success 字段
8. IF success 为 true，THEN THE AI_Billing_Plugin SHALL 将定价信息存储到内存缓存中（key 为 provider:model）
9. IF success 为 true，THEN THE AI_Billing_Plugin SHALL 继续执行余额检查
10. IF success 为 false 且 message 不为空，THEN THE AI_Billing_Plugin SHALL 在响应体中返回 message 内容
11. IF success 为 false 且 message 为空，THEN THE AI_Billing_Plugin SHALL 在响应体中返回配置的 failPricingMessage
12. IF success 为 false，THEN THE AI_Billing_Plugin SHALL 返回 503 Service Unavailable 状态码
13. IF success 为 false，THEN THE AI_Billing_Plugin SHALL 终止请求处理流程
14. WHEN 定价查询请求超时（超过 5 秒），THE AI_Billing_Plugin SHALL 返回 503 Service Unavailable 状态码
15. WHEN 定价查询请求超时，THE AI_Billing_Plugin SHALL 在响应体中返回配置的 failPricingMessage
16. WHEN 定价查询请求超时，THE AI_Billing_Plugin SHALL 终止请求处理流程
17. IF Billing_Service 返回非 200 状态码，THEN THE AI_Billing_Plugin SHALL 返回 503 Service Unavailable 状态码
18. IF Billing_Service 返回非 200 状态码，THEN THE AI_Billing_Plugin SHALL 在响应体中返回配置的 failPricingMessage
19. IF Billing_Service 返回非 200 状态码，THEN THE AI_Billing_Plugin SHALL 终止请求处理流程
20. THE AI_Billing_Plugin SHALL 使用 provider:model 作为缓存 key 存储定价信息
21. THE AI_Billing_Plugin SHALL 在插件实例的生命周期内保持定价缓存（不设置过期时间）

### Requirement 4: 租户余额检查

### Requirement 4: 租户余额检查

**User Story:** 作为 SaaS 运营者，我希望在请求转发到 AI 服务之前检查租户余额，以便阻止欠费租户继续使用服务。

#### Acceptance Criteria

1. WHEN 定价查询成功，THE AI_Billing_Plugin SHALL 向 Billing_Service 发送 GET 请求到 /v1/amount 端点
2. WHEN 发送余额查询请求，THE AI_Billing_Plugin SHALL 使用配置的 protocol 和 port 构建完整的服务 URL
3. WHEN 发送余额查询请求，THE AI_Billing_Plugin SHALL 在请求头中包含所有提取的租户信息和 HMAC 认证头
4. WHEN 发送余额查询请求，THE AI_Billing_Plugin SHALL NOT 在请求体中包含任何数据（使用 GET 方法）
5. WHEN Billing_Service 返回余额信息，THE AI_Billing_Plugin SHALL 解析 success 字段
6. WHEN Billing_Service 返回余额信息，THE AI_Billing_Plugin SHALL 解析 balance 字段
7. IF success 为 false 且 message 不为空，THEN THE AI_Billing_Plugin SHALL 在响应体中返回 message 内容
8. IF success 为 false 且 message 为空，THEN THE AI_Billing_Plugin SHALL 在响应体中返回配置的 failBalanceMessage
9. IF success 为 false，THEN THE AI_Billing_Plugin SHALL 返回 503 Service Unavailable 状态码
10. IF success 为 false，THEN THE AI_Billing_Plugin SHALL 终止请求处理流程
11. IF success 为 true 且 balance 小于或等于 0，THEN THE AI_Billing_Plugin SHALL 返回 402 Payment Required 状态码
12. IF success 为 true 且 balance 小于或等于 0，THEN THE AI_Billing_Plugin SHALL 在响应体中返回配置的 insufficientBalanceMessage
13. IF success 为 true 且 balance 小于或等于 0，THEN THE AI_Billing_Plugin SHALL 终止请求处理流程，不继续执行后续插件
14. IF success 为 true 且 balance 大于 0，THEN THE AI_Billing_Plugin SHALL 允许请求继续转发到上游服务
15. WHEN 余额查询请求超时（超过 5 秒），THE AI_Billing_Plugin SHALL 返回 503 Service Unavailable 状态码
16. WHEN 余额查询请求超时，THE AI_Billing_Plugin SHALL 在响应体中返回配置的 failBalanceMessage
17. WHEN 余额查询请求超时，THE AI_Billing_Plugin SHALL 终止请求处理流程，不继续执行后续插件
18. IF Billing_Service 返回非 200 状态码，THEN THE AI_Billing_Plugin SHALL 返回 503 Service Unavailable 状态码
19. IF Billing_Service 返回非 200 状态码，THEN THE AI_Billing_Plugin SHALL 在响应体中返回配置的 failBalanceMessage
20. IF Billing_Service 返回非 200 状态码，THEN THE AI_Billing_Plugin SHALL 终止请求处理流程，不继续执行后续插件

### Requirement 5: LLM 响应信息提取

**User Story:** 作为开发者，我希望插件能够从 LLM 响应中提取 token 使用量和模型信息，以便进行准确计费。

#### Acceptance Criteria

1. WHEN 响应头返回时，THE AI_Billing_Plugin SHALL 检查 HTTP_Status_Code 是否为 200
2. IF HTTP_Status_Code 不是 200，THEN THE AI_Billing_Plugin SHALL 跳过计费流程并允许响应返回
3. WHEN 响应头的 Content-Type 包含 "text/event-stream"，THE AI_Billing_Plugin SHALL 标记为流式响应
4. WHEN 响应头的 Content-Type 包含 "application/json"，THE AI_Billing_Plugin SHALL 标记为非流式响应
5. WHEN 处理流式响应体，THE AI_Billing_Plugin SHALL 解析 SSE 格式的数据块
6. WHEN 处理非流式响应体，THE AI_Billing_Plugin SHALL 解析完整的 JSON 响应体
7. WHEN 解析响应体，THE AI_Billing_Plugin SHALL 提取 input_tokens 字段（或 prompt_tokens）
8. WHEN 解析响应体，THE AI_Billing_Plugin SHALL 提取 output_tokens 字段（或 completion_tokens）
9. WHEN 解析响应体，THE AI_Billing_Plugin SHALL 提取 model 字段
10. WHEN 解析响应体，THE AI_Billing_Plugin SHALL 提取 provider 信息（从上下文或配置中获取）
11. WHEN 解析响应体，THE AI_Billing_Plugin SHALL 提取 request_id 字段（或 id 字段）
12. IF 任何必需字段（input_tokens、output_tokens、model）缺失，THEN THE AI_Billing_Plugin SHALL 记录错误日志
13. IF 任何必需字段（input_tokens、output_tokens、model）缺失，THEN THE AI_Billing_Plugin SHALL 返回 500 Internal Server Error 状态码
14. IF 任何必需字段（input_tokens、output_tokens、model）缺失，THEN THE AI_Billing_Plugin SHALL 在响应体中返回 "Failed to extract billing information" 消息
15. IF 任何必需字段（input_tokens、output_tokens、model）缺失，THEN THE AI_Billing_Plugin SHALL 终止请求处理流程

### Requirement 6: 租户费用扣除

**User Story:** 作为 SaaS 运营者，我希望在 LLM 请求成功返回后自动扣除租户费用，以便实现按量计费。

#### Acceptance Criteria

1. WHEN LLM 响应处理完成且所有必需信息已提取，THE AI_Billing_Plugin SHALL 向 Billing_Service 发送 POST 请求到 /v1/cost 端点
2. WHEN 发送计费请求，THE AI_Billing_Plugin SHALL 使用配置的 protocol 和 port 构建完整的服务 URL
3. WHEN 发送计费请求，THE AI_Billing_Plugin SHALL 在请求头中包含所有提取的租户信息和 HMAC 认证头
4. WHEN 发送计费请求，THE AI_Billing_Plugin SHALL 在请求体中包含 provider、model_name、request_id、input_tokens、output_tokens 字段
5. WHEN 发送计费请求，THE AI_Billing_Plugin SHALL NOT 在请求体中包含 apikey、consumer_id、consumer_name 字段（这些信息在请求头中）
6. WHEN Billing_Service 返回计费结果，THE AI_Billing_Plugin SHALL 解析 success 字段
7. IF 响应为非流式类型 且 success 为 true，THEN THE AI_Billing_Plugin SHALL 允许响应正常返回给客户端
8. IF 响应为非流式类型 且 success 为 false 且 message 不为空，THEN THE AI_Billing_Plugin SHALL 在响应体中返回 message 内容
9. IF 响应为非流式类型 且 success 为 false 且 message 为空，THEN THE AI_Billing_Plugin SHALL 在响应体中返回配置的 insufficientBalanceMessage
10. IF 响应为非流式类型 且 success 为 false，THEN THE AI_Billing_Plugin SHALL 返回 402 Payment Required 状态码
11. IF 响应为非流式类型 且 success 为 false，THEN THE AI_Billing_Plugin SHALL 终止请求处理流程，不返回 LLM 的原始响应
12. IF 响应为流式类型 且 success 为 false，THEN THE AI_Billing_Plugin SHALL 记录详细的错误日志（包括 tenant_id、consumer_id、request_id、错误原因）
13. IF 响应为流式类型 且 success 为 false，THEN THE AI_Billing_Plugin SHALL NOT 阻断响应（流已经开始返回给客户端）
14. WHEN 计费请求超时（超过 5 秒）且响应为非流式类型，THE AI_Billing_Plugin SHALL 返回 503 Service Unavailable 状态码
15. WHEN 计费请求超时且响应为非流式类型，THE AI_Billing_Plugin SHALL 在响应体中返回配置的 failCostMessage
16. WHEN 计费请求超时且响应为非流式类型，THE AI_Billing_Plugin SHALL 终止请求处理流程，不返回 LLM 的原始响应
17. WHEN 计费请求超时且响应为流式类型，THE AI_Billing_Plugin SHALL 记录详细的错误日志
18. WHEN 计费请求超时且响应为流式类型，THE AI_Billing_Plugin SHALL NOT 阻断响应
19. IF Billing_Service 返回非 200 状态码且响应为非流式类型，THEN THE AI_Billing_Plugin SHALL 返回 503 Service Unavailable 状态码
20. IF Billing_Service 返回非 200 状态码且响应为非流式类型，THEN THE AI_Billing_Plugin SHALL 在响应体中返回配置的 failCostMessage
21. IF Billing_Service 返回非 200 状态码且响应为非流式类型，THEN THE AI_Billing_Plugin SHALL 终止请求处理流程，不返回 LLM 的原始响应
22. IF Billing_Service 返回非 200 状态码且响应为流式类型，THEN THE AI_Billing_Plugin SHALL 记录详细的错误日志
23. IF Billing_Service 返回非 200 状态码且响应为流式类型，THEN THE AI_Billing_Plugin SHALL NOT 阻断响应
24. WHEN 计费失败（无论流式或非流式），THE AI_Billing_Plugin SHALL 记录详细的错误信息（包括 tenant_id、consumer_id、request_id、错误原因）

### Requirement 7: 流式响应处理

**User Story:** 作为开发者，我希望插件能够正确处理流式响应，以便在流式场景下也能准确计费。

#### Acceptance Criteria

1. WHEN 响应为流式类型，THE AI_Billing_Plugin SHALL 缓冲所有数据块直到流结束
2. WHEN 处理流式数据块，THE AI_Billing_Plugin SHALL 解析每个 "data: " 前缀的 SSE 消息
3. WHEN 遇到 "data: [DONE]" 消息，THE AI_Billing_Plugin SHALL 标记流结束
4. WHEN 流结束时，THE AI_Billing_Plugin SHALL 从缓冲的数据中提取最终的 token 使用量
5. WHEN 流式响应中包含多个 usage 对象，THE AI_Billing_Plugin SHALL 使用最后一个 usage 对象的值
6. WHEN 流式响应处理完成，THE AI_Billing_Plugin SHALL 触发计费流程
7. IF 流式响应中没有 usage 信息，THEN THE AI_Billing_Plugin SHALL 返回 500 Internal Server Error 状态码
8. IF 流式响应中没有 usage 信息，THEN THE AI_Billing_Plugin SHALL 在响应体中返回 "Failed to extract billing information from stream" 消息
9. IF 流式响应中没有 usage 信息，THEN THE AI_Billing_Plugin SHALL 终止请求处理流程

### Requirement 8: 插件优先级和执行顺序

### Requirement 8: 插件优先级和执行顺序

**User Story:** 作为系统架构师，我希望 AI Billing 插件在正确的时机执行，以便确保在认证之后、AI 代理之前进行定价验证和余额检查。

#### Acceptance Criteria

1. THE AI_Billing_Plugin SHALL 配置 priority 为 200
2. WHEN 多个插件同时配置，THE AI_Billing_Plugin SHALL 在认证插件（如 jwt-auth、key-auth、ext-auth）之后执行
3. WHEN 多个插件同时配置，THE AI_Billing_Plugin SHALL 在 ai-proxy 插件之前执行
4. THE AI_Billing_Plugin SHALL 依赖上游认证插件提供租户信息和 HMAC 认证头
5. THE AI_Billing_Plugin SHALL 在请求头处理阶段（ProcessRequestHeaders）执行定价查询和余额检查
6. THE AI_Billing_Plugin SHALL 在响应体处理阶段（ProcessResponseBody/ProcessStreamingResponseBody）执行计费
7. WHEN 定价查询或余额检查失败，THE AI_Billing_Plugin SHALL 阻止请求继续传递到 ai-proxy 插件

### Requirement 9: 错误处理和日志记录

**User Story:** 作为运维人员，我希望插件能够记录详细的日志信息，以便排查计费问题和监控系统健康状态。

#### Acceptance Criteria

1. WHEN 记录 info 级别日志，THE AI_Billing_Plugin SHALL 包含 tenant_id、consumer_id、consumer_name 信息
2. WHEN 记录 debug 级别日志，THE AI_Billing_Plugin MAY 包含脱敏的 apikey 信息（仅前 8 个字符后跟 "***"）
3. WHEN 租户信息提取失败，THE AI_Billing_Plugin SHALL 记录包含缺失字段名称的错误日志
4. WHEN 定价查询失败，THE AI_Billing_Plugin SHALL 记录包含 tenant_id、consumer_id、provider、model_name、错误原因的错误日志
5. WHEN 余额查询失败，THE AI_Billing_Plugin SHALL 记录包含 tenant_id、consumer_id、consumer_name、错误原因的错误日志
6. WHEN 计费请求失败，THE AI_Billing_Plugin SHALL 记录包含 tenant_id、consumer_id、consumer_name、request_id、token 使用量、错误原因的错误日志
7. WHEN 响应信息提取失败，THE AI_Billing_Plugin SHALL 记录包含原始响应体片段（最多 500 字符）的错误日志
8. WHEN 计费成功，THE AI_Billing_Plugin SHALL 记录包含 tenant_id、consumer_id、consumer_name、request_id、input_tokens、output_tokens、cost 的信息日志
9. WHEN Billing_Service 不可达，THE AI_Billing_Plugin SHALL 记录包含服务地址和网络错误的错误日志
10. THE AI_Billing_Plugin SHALL 使用结构化日志格式便于日志分析和监控
11. WHEN 记录 apikey 到 debug 日志，THE AI_Billing_Plugin SHALL 仅记录前 8 个字符后跟 "***"

### Requirement 9: 性能优化

### Requirement 10: 性能优化

**User Story:** 作为系统架构师，我希望插件具有高性能，以便在高并发场景下不成为系统瓶颈。

#### Acceptance Criteria

1. WHEN 发送定价查询请求，THE AI_Billing_Plugin SHALL 使用异步 HTTP 调用避免阻塞
2. WHEN 发送余额查询请求，THE AI_Billing_Plugin SHALL 使用异步 HTTP 调用避免阻塞
3. WHEN 发送计费请求，THE AI_Billing_Plugin SHALL 使用异步 HTTP 调用避免阻塞
4. THE AI_Billing_Plugin SHALL 设置合理的 HTTP 超时时间（定价查询 5 秒，余额查询 5 秒，计费 5 秒）
5. THE AI_Billing_Plugin SHALL 在内存中缓存模型定价信息，以 provider:model 为 key
6. THE AI_Billing_Plugin SHALL 在缓存命中时跳过定价查询，直接执行余额检查
7. WHEN 处理流式响应，THE AI_Billing_Plugin SHALL 仅缓冲必要的数据（usage 相关信息）
8. THE AI_Billing_Plugin SHALL 避免在热路径上进行复杂的字符串操作或内存分配
9. WHEN 解析 JSON 响应，THE AI_Billing_Plugin SHALL 使用高效的 JSON 解析库（如 gjson）
10. THE AI_Billing_Plugin SHALL 复用 HTTP 连接以减少连接开销

### Requirement 11: 安全性保障

### Requirement 11: 安全性保障

**User Story:** 作为安全工程师，我希望插件能够防止计费绕过和数据泄露，以便保护系统和租户数据安全。

#### Acceptance Criteria

1. THE AI_Billing_Plugin SHALL 确保所有通过网关的 AI 请求都经过定价验证和余额检查
2. THE AI_Billing_Plugin SHALL 确保所有成功的 AI 响应都触发计费流程
3. WHEN 租户信息缺失，THE AI_Billing_Plugin SHALL 拒绝请求
4. THE AI_Billing_Plugin SHALL 不在 info 级别日志中记录完整的 API_Key
5. THE AI_Billing_Plugin MAY 在 debug 级别日志中记录脱敏的 API_Key（仅前 8 个字符）
6. WHEN 与 Billing_Service 通信，THE AI_Billing_Plugin SHALL 支持使用 HTTPS 协议
7. THE AI_Billing_Plugin SHALL 验证 Billing_Service 返回的响应格式，防止注入攻击
8. WHEN 计费失败（非流式响应），THE AI_Billing_Plugin SHALL 确保不返回 LLM 的原始响应给客户端
9. THE AI_Billing_Plugin SHALL 透传所有 HMAC 认证头到 Billing_Service，不进行修改或验证
10. THE AI_Billing_Plugin SHALL 依赖 Billing_Service 进行 HMAC 签名验证

### Requirement 12: 多协议支持

**User Story:** 作为开发者，我希望插件能够支持多种 LLM 协议格式，以便兼容不同的 AI 服务提供商。

#### Acceptance Criteria

1. THE AI_Billing_Plugin SHALL 支持 OpenAI 协议格式的 token 使用量提取
2. THE AI_Billing_Plugin SHALL 支持 Claude/Anthropic 协议格式的 token 使用量提取
3. THE AI_Billing_Plugin SHALL 支持 Google Gemini 协议格式的 token 使用量提取
4. WHEN 响应格式为 OpenAI，THE AI_Billing_Plugin SHALL 从 usage.prompt_tokens 和 usage.completion_tokens 提取数据
5. WHEN 响应格式为 Claude，THE AI_Billing_Plugin SHALL 从 usage.input_tokens 和 usage.output_tokens 提取数据
6. WHEN 响应格式为 Gemini，THE AI_Billing_Plugin SHALL 从 usageMetadata.promptTokenCount 和 usageMetadata.candidatesTokenCount 提取数据
7. IF 无法识别响应格式，THEN THE AI_Billing_Plugin SHALL 尝试所有已知格式并使用第一个成功的结果
8. IF 所有协议格式都无法提取到 token 信息，THEN THE AI_Billing_Plugin SHALL 返回错误并终止请求

### Requirement 13: 计费幂等性

**User Story:** 作为 SaaS 运营者，我希望避免重复计费，以便确保计费的准确性和用户信任。

#### Acceptance Criteria

1. WHEN 发送计费请求，THE AI_Billing_Plugin SHALL 在请求中包含唯一的 request_id
2. THE Billing_Service SHALL 使用 request_id 实现计费幂等性（防止重复扣费）
3. IF 同一 request_id 的计费请求被发送多次，THEN THE Billing_Service SHALL 仅扣费一次
4. THE AI_Billing_Plugin SHALL 不在插件层面实现重试逻辑（依赖 Billing_Service 的幂等性）
5. WHEN request_id 无法从响应中提取，THE AI_Billing_Plugin SHALL 生成一个唯一的 ID 作为 request_id

### Requirement 14: 监控和指标

### Requirement 14: 监控和指标

**User Story:** 作为运维人员，我希望插件能够暴露关键指标，以便监控计费系统的健康状态和性能。

#### Acceptance Criteria

1. THE AI_Billing_Plugin SHALL 记录定价查询的成功率指标
2. THE AI_Billing_Plugin SHALL 记录定价缓存命中率指标
3. THE AI_Billing_Plugin SHALL 记录余额检查的成功率指标
4. THE AI_Billing_Plugin SHALL 记录计费请求的成功率指标
5. THE AI_Billing_Plugin SHALL 记录余额不足被拒绝的请求数量
6. THE AI_Billing_Plugin SHALL 记录计费失败的请求数量
7. THE AI_Billing_Plugin SHALL 记录与 Billing_Service 通信的延迟指标
8. THE AI_Billing_Plugin SHALL 记录因定价查询失败而被拒绝的请求数量
9. THE AI_Billing_Plugin SHALL 记录因余额检查失败而被拒绝的请求数量
10. THE AI_Billing_Plugin SHALL 支持通过 Prometheus 格式导出指标（如果 Higress 支持）
