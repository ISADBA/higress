# AI Billing 插件

## 功能说明

AI Billing 插件是 Higress 网关的计费能力核心组件，用于在 AI 代理请求的生命周期中实现定价验证、余额检查和费用扣除。插件采用基于租户的计费模式，与独立的 billing-service 服务集成，通过 HMAC 签名验证确保请求的安全性。

### 核心功能

1. **租户信息提取**：从请求头中提取租户信息和 HMAC 认证头（由上游认证插件提供）
2. **模型定价查询**：在余额检查前验证模型定价信息已配置，支持内存缓存
3. **请求前余额检查**：在请求转发到 AI 服务之前，验证租户是否有足够的余额
4. **响应后费用扣除**：在 LLM 请求成功返回后，根据 token 使用量自动扣除费用
5. **多协议支持**：支持 OpenAI、Claude、Gemini 等多种 LLM 协议格式
6. **流式响应处理**：正确处理 SSE 流式响应场景下的计费
7. **差异化失败处理**：非流式响应采用 FAIL_CLOSE 策略，流式响应计费失败仅记录日志不阻断响应

### 失败策略

插件对不同类型的响应采用不同的失败策略：

**非流式响应（FAIL_CLOSE）**：
- 定价查询失败 → 返回 503，终止请求
- 余额检查失败 → 返回 503 或 402，终止请求
- 费用扣除失败 → 返回 503 或 402，不返回 LLM 响应

**流式响应（异步计费）**：
- 定价查询失败 → 返回 503，终止请求（在流开始前）
- 余额检查失败 → 返回 503 或 402，终止请求（在流开始前）
- 费用扣除失败 → 记录详细日志，不阻断响应（流已经开始返回给客户端）

## 配置说明

### 基本配置

```yaml
billingService:
  serviceAddress: billing-service  # 必需：计费服务名称
  protocol: http                   # 可选：http 或 https，默认 http
  port: 8888                       # 可选：端口号，默认 8888
failPricingMessage: "503 Pricing Information Unavailable"      # 可选：定价查询失败提示
failBalanceMessage: "503 Billing Service Balance Unavailable"  # 可选：余额查询失败提示
insufficientBalanceMessage: "余额不足"                          # 可选：余额不足提示
failCostMessage: "503 Billing Service Cost Unavailable"        # 可选：计费失败提示
```

### 配置示例

```yaml
apiVersion: extensions.higress.io/v1alpha1
kind: WasmPlugin
metadata:
  name: ai-billing
  namespace: higress-system
spec:
  defaultConfig:
    billingService:
      serviceAddress: billing-service
      protocol: http
      port: 8888
    failPricingMessage: "503 Pricing Information Unavailable"
    failBalanceMessage: "503 Billing Service Balance Unavailable"
    insufficientBalanceMessage: "余额不足"
    failCostMessage: "503 Billing Service Cost Unavailable"
  priority: 200
  url: oci://higress-registry.cn-hangzhou.cr.aliyuncs.com/plugins/ai-billing:2.0.0-tenant
```

## 插件依赖关系

AI Billing 插件依赖上游认证插件提供租户信息和 HMAC 认证头。推荐的插件执行顺序：

```
1. 认证插件 (ext-auth/jwt-auth/key-auth)
   ↓ 提供租户信息和 HMAC 认证头
2. ai-billing (priority=200)
   ↓ 定价验证、余额检查、计费
3. ai-proxy
   ↓ 转发到 AI 服务
```

### 必需的请求头

AI Billing 插件需要以下 9 个请求头（由上游认证插件提供）：

**HMAC 认证头**：
- `x-internal-auth-sign-version` - HMAC 签名版本
- `x-internal-auth-ts` - 时间戳
- `x-internal-auth-nonce` - 随机数
- `x-internal-auth-sign` - 签名

**租户/消费者信息**：
- `x-consumer-id` - 消费者 ID
- `x-mse-consumer-name` - 消费者名称
- `x-mse-tenant-id` - 租户 ID
- `x-domain-resource-id` - 域资源 ID
- `x-router-resource-id` - 路由资源 ID

如果任何必需字段缺失，插件将返回 503 错误并终止请求。

## 工作流程

### 1. 请求阶段

1. 从请求头提取租户信息和 HMAC 认证头（9 个必需字段）
2. 可选：提取 API Key 用于 debug 日志
3. 从请求头或上下文中提取 provider 和 model_name
4. 检查定价缓存，如果缓存未命中则向计费服务查询定价（GET /v1/pricing/global）
5. 向计费服务发送余额查询请求（GET /v1/amount）
6. 如果余额不足或查询失败，返回错误并终止请求
7. 如果余额充足，允许请求继续转发到 AI 服务

### 2. 响应阶段

1. 检测响应类型（流式或非流式）
2. 从响应中提取 token 使用量，从请求头 `x-higress-llm-model` 提取模型信息，从请求头或响应中提取请求 ID
3. 向计费服务发送费用扣除请求（POST /v1/cost）
4. **非流式响应**：如果扣费失败，返回错误且不返回 LLM 响应；如果扣费成功，返回 LLM 响应给客户端
5. **流式响应**：异步计费，如果扣费失败仅记录详细日志，不阻断响应

## API 接口

### 定价查询接口（新增）

**请求**：
```
GET /v1/pricing/global?provider=openai&model_name=gpt-4
Headers:
  x-internal-auth-sign-version: v1
  x-internal-auth-ts: 1234567890
  x-internal-auth-nonce: abc123
  x-internal-auth-sign: signature
  x-consumer-id: 123
  x-mse-consumer-name: user1
  x-mse-tenant-id: 1001
  x-domain-resource-id: domain-1
  x-router-resource-id: router-1
```

**响应**：
```json
{
  "success": true,
  "message": "",
  "data": {
    "provider": "openai",
    "model_name": "gpt-4"
  }
}
```

### 余额查询接口（更新为 GET 方法）

**请求**：
```
GET /v1/amount
Headers:
  x-internal-auth-sign-version: v1
  x-internal-auth-ts: 1234567890
  x-internal-auth-nonce: abc123
  x-internal-auth-sign: signature
  x-consumer-id: 123
  x-mse-consumer-name: user1
  x-mse-tenant-id: 1001
  x-domain-resource-id: domain-1
  x-router-resource-id: router-1
```

**响应**：
```json
{
  "success": true,
  "message": "",
  "balance": "100.50",
  "uid": 12345,
  "updated_at": 1234567890
}
```

### 费用扣除接口（更新请求格式）

**请求**：
```
POST /v1/cost
Headers:
  Content-Type: application/json
  x-internal-auth-sign-version: v1
  x-internal-auth-ts: 1234567890
  x-internal-auth-nonce: abc123
  x-internal-auth-sign: signature
  x-consumer-id: 123
  x-mse-consumer-name: user1
  x-mse-tenant-id: 1001
  x-domain-resource-id: domain-1
  x-router-resource-id: router-1

Body:
{
  "provider": "openai",
  "model_name": "gpt-4",
  "request_id": "req-123",
  "input_tokens": 100,
  "output_tokens": 200
}
```

**响应**：
```json
{
  "success": true,
  "message": "",
  "billing_event_id": 789,
  "cost": "0.05",
  "cost_actual": "0.045",
  "discount_ratio": "0.9",
  "remaining_balance": "100.45"
}
```

## 错误码

| 状态码 | 说明 | 触发条件 | 解决方案 |
|--------|------|----------|----------|
| 402 | 余额不足或扣费失败 | 租户余额 <= 0 或计费服务返回 success=false | 充值账户余额 |
| 500 | 内部错误 | 提取计费信息失败 | 检查 LLM 响应格式 |
| 503 | 计费服务不可用 | 租户信息缺失、定价查询失败、余额查询失败、计费服务超时或返回错误 | 检查认证插件配置、定价配置、服务健康状态 |

### 错误响应格式

所有错误响应都采用统一的 JSON 格式：

```json
{
  "error": {
    "message": "错误描述信息",
    "type": "billing_error"
  }
}
```

### 常见错误场景

#### 1. 租户信息缺失（503）

**错误信息示例**：
```json
{
  "error": {
    "message": "503 Billing Service Balance Unavailable",
    "type": "billing_error"
  }
}
```

**原因**：请求头中缺少必需的租户信息或 HMAC 认证头

**解决方案**：
1. 确保上游认证插件（ext-auth/jwt-auth/key-auth）已正确配置
2. 验证认证插件是否在 ai-billing 之前执行
3. 检查认证插件是否正确设置了所有 9 个必需的请求头

#### 2. 定价信息未配置（503）

**错误信息示例**：
```json
{
  "error": {
    "message": "Pricing not found for provider: openai, model: gpt-4",
    "type": "billing_error"
  }
}
```

**原因**：计费服务中没有配置该 provider/model 的定价信息

**解决方案**：
1. 检查计费服务的定价配置
2. 确保 provider 名称与计费服务中的配置一致
3. 为缺失的 provider/model 添加定价数据

#### 3. 余额不足（402）

**错误信息示例**：
```json
{
  "error": {
    "message": "余额不足",
    "type": "billing_error"
  }
}
```

**原因**：租户账户余额不足以支付本次请求

**解决方案**：为租户账户充值

#### 4. 计费服务不可用（503）

**错误信息示例**：
```json
{
  "error": {
    "message": "503 Billing Service Cost Unavailable",
    "type": "billing_error"
  }
}
```

**原因**：计费服务宕机、网络不通或响应超时

**解决方案**：
1. 检查计费服务健康状态
2. 验证网络连接
3. 检查服务发现配置

## 日志说明

插件会记录以下关键信息：

**Info 级别日志**（包含租户信息）：
```
[ai-billing] tenant info extracted: tenantId=1001 consumerId=123 consumerName=user1
[ai-billing] pricing cache hit: tenantId=1001 provider=openai model=gpt-4
[ai-billing] balance check: tenantId=1001 consumerId=123 consumerName=user1 balance=100.50
[ai-billing] cost deduction: tenantId=1001 consumerId=123 consumerName=user1 requestId=req-123 inputTokens=100 outputTokens=200 cost=0.05 success=true
```

**Debug 级别日志**（可包含脱敏的 API Key）：
```
[ai-billing] API key extracted: abc12345***
```

**流式响应计费失败日志**（详细错误信息）：
```
[ai-billing] async cost deduction failed: tenantId=1001 consumerId=123 consumerName=user1 requestId=req-123 reason=billing_service_returned_failure
```

## 安全性

1. **HMAC 签名验证**：所有请求都包含 HMAC 签名，由 billing-service 验证，插件只负责透传
2. **API Key 保护**：日志中仅记录 API key 的前 8 个字符（仅用于 debug 日志）
3. **HTTPS 支持**：支持使用 HTTPS 与计费服务通信
4. **响应保护**：非流式响应计费失败时不返回 LLM 响应，防止免费使用
5. **超时保护**：所有 HTTP 调用设置 5 秒超时，防止资源耗尽

## 性能优化

1. **定价缓存**：在插件内存中缓存定价信息（key: provider:model），减少不必要的 HTTP 调用
2. **异步调用**：所有 HTTP 调用使用异步模式，避免阻塞
3. **连接复用**：HTTP 连接自动复用，减少开销
4. **高效解析**：使用 gjson 进行快速 JSON 解析
5. **流式处理**：流式响应不缓冲全部数据，仅提取必要信息

## 兼容性

支持的 LLM 协议格式：
- OpenAI（包括流式和非流式）
- Claude/Anthropic
- Google Gemini

## 版本

当前版本：2.0.0-tenant

### 版本历史

- **2.0.0-tenant**: 重构为基于租户的计费模式
  - 添加租户信息提取和 HMAC 认证头支持
  - 添加模型定价查询功能（带内存缓存）
  - 更新余额查询为 GET 方法，使用租户信息头
  - 更新费用扣除请求格式，使用租户信息头
  - 支持流式响应的异步计费策略
  - 优先使用 billing-service 返回的错误消息
- **1.0.15-alpha**: 修改模型名称提取方式，从请求头 `x-higress-llm-model` 获取
- **1.0.12-alpha**: 改进 400 错误处理，区分配置错误和服务错误
- **1.0.11-alpha**: 修复流式响应计费问题
- **1.0.0-alpha**: 初始版本（基于 API Key 的计费模式）

## 迁移指南

### 从 1.x 迁移到 2.0

1. **配置更新**：
   - 添加 `failPricingMessage` 配置项
   - 移除 `namespace` 配置项（不再需要）

2. **认证插件配置**：
   - 确保上游认证插件（ext-auth/jwt-auth/key-auth）已配置
   - 验证认证插件设置了所有 9 个必需的请求头

3. **计费服务更新**：
   - 更新计费服务以支持新的 API 接口：
     - GET /v1/pricing/global（新增）
     - GET /v1/amount（从 POST 改为 GET）
     - POST /v1/cost（更新请求格式）
   - 配置模型定价信息
   - 实现 HMAC 签名验证

4. **测试验证**：
   - 测试定价查询功能
   - 测试租户信息提取
   - 测试流式和非流式响应的计费
   - 验证错误处理和日志记录
