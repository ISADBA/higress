# AI Billing 插件

## 功能说明

AI Billing 插件是 Higress 网关的计费能力核心组件，用于在 AI 代理请求的生命周期中实现余额检查和费用扣除。

### 核心功能

1. **请求前余额检查**：在请求转发到 AI 服务之前，验证用户是否有足够的余额
2. **响应后费用扣除**：在 LLM 请求成功返回后，根据 token 使用量自动扣除费用
3. **多协议支持**：支持 OpenAI、Claude、Gemini 等多种 LLM 协议格式
4. **流式响应处理**：正确处理 SSE 流式响应场景下的计费
5. **严格的失败处理**：任何计费相关的失败都会终止请求，不返回 LLM 响应

### 失败策略

插件采用严格的 FAIL_CLOSE 策略：
- 余额检查失败 → 返回 503 或 402，终止请求
- 费用扣除失败 → 返回 503 或 402，不返回 LLM 响应
- 确保计费的准确性和系统的安全性

## 配置说明

### 基本配置

```yaml
billingService:
  serviceAddress: billing-service  # 必需：计费服务名称
  namespace: higress-system        # 可选：Kubernetes 命名空间，默认 higress-system
  protocol: http                   # 可选：http 或 https，默认 http
  port: 8888                       # 可选：端口号，默认 8888
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
      namespace: higress-system
      protocol: http
      port: 8888
    failBalanceMessage: "503 Billing Service Balance Unavailable"
    insufficientBalanceMessage: "余额不足"
    failCostMessage: "503 Billing Service Cost Unavailable"
  priority: 200
  url: oci://higress-registry.cn-hangzhou.cr.aliyuncs.com/plugins/ai-billing:1.0.0
```

## 工作流程

### 1. 请求阶段

1. 从请求头提取 API Key（优先 `x-hi-original-auth`，其次 `Authorization`）
2. 向计费服务发送余额查询请求（POST /v1/amount）
3. 如果余额不足或查询失败，返回错误并终止请求
4. 如果余额充足，允许请求继续转发到 AI 服务

### 2. 响应阶段

1. 检测响应类型（流式或非流式）
2. 从响应中提取 token 使用量，从请求头 `x-higress-llm-model` 提取模型信息，从请求头或响应中提取请求 ID
3. 向计费服务发送费用扣除请求（POST /v1/cost）
4. 如果扣费失败，返回错误且不返回 LLM 响应
5. 如果扣费成功，返回 LLM 响应给客户端

## API 接口

### 余额查询接口

**请求**：
```
POST /v1/amount
Content-Type: application/json

{
  "apikey": "user-api-key"
}
```

**响应**：
```json
{
  "balance": "100.50",
  "uid": 12345,
  "updated_at": 1234567890
}
```

### 费用扣除接口

**请求**：
```
POST /v1/cost
Content-Type: application/json

{
  "apikey": "user-api-key",
  "input_tokens": 100,
  "output_tokens": 200,
  "model_name": "gpt-4",
  "provider": "openai",
  "request_id": "req-123"
}
```

**响应**：
```json
{
  "billing_event_id": 789,
  "cost": "0.05",
  "cost_actual": "0.045",
  "discount_ratio": "0.9",
  "remaining_balance": "100.45",
  "success": true
}
```

## 错误码

| 状态码 | 说明 | 触发条件 | 解决方案 |
|--------|------|----------|----------|
| 401 | 缺少 API Key | 请求头中没有 `x-hi-original-auth` 或 `Authorization` | 在请求头中添加有效的 API Key |
| 402 | 余额不足或扣费失败 | 用户余额 <= 0 或计费服务返回 success=false | 充值账户余额 |
| 500 | 内部错误 | 提取计费信息失败或计费配置错误（如缺少定价数据） | 检查 LLM 响应格式或联系管理员修复计费配置 |
| 503 | 计费服务不可用 | 计费服务超时、不可达或返回错误 | 等待服务恢复或联系运维人员 |

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

#### 1. 计费配置错误（500）

**错误信息示例**：
```json
{
  "error": {
    "message": "Billing configuration error: pricing not found for provider: ai-route-qwen.internal, model: qwen-flash",
    "type": "billing_error"
  }
}
```

**原因**：计费服务中没有配置该 provider/model 的定价信息

**解决方案**：
1. 检查计费服务的定价配置
2. 确保 provider 名称与计费服务中的配置一致
3. 为缺失的 provider/model 添加定价数据

#### 2. 余额不足（402）

**错误信息示例**：
```json
{
  "error": {
    "message": "余额不足",
    "type": "billing_error"
  }
}
```

**原因**：用户账户余额不足以支付本次请求

**解决方案**：为账户充值

#### 3. 计费服务不可用（503）

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
- 余额检查结果（API key 脱敏，仅显示前 8 个字符）
- 费用扣除结果（包含 token 使用量、费用等）
- 错误信息（包含详细的错误原因）

日志格式示例：
```
[ai-billing] balance check: apikey=abc12345*** balance=100.50
[ai-billing] cost deduction: apikey=abc12345*** requestId=req-123 inputTokens=100 outputTokens=200 cost=0.05 success=true
```

## 安全性

1. **API Key 保护**：日志中仅记录 API key 的前 8 个字符
2. **HTTPS 支持**：支持使用 HTTPS 与计费服务通信
3. **响应保护**：计费失败时不返回 LLM 响应，防止免费使用
4. **超时保护**：所有 HTTP 调用设置 5 秒超时，防止资源耗尽

## 性能优化

1. **异步调用**：所有 HTTP 调用使用异步模式，避免阻塞
2. **连接复用**：HTTP 连接自动复用，减少开销
3. **高效解析**：使用 gjson 进行快速 JSON 解析
4. **流式处理**：流式响应不缓冲全部数据，仅提取必要信息

## 兼容性

支持的 LLM 协议格式：
- OpenAI（包括流式和非流式）
- Claude/Anthropic
- Google Gemini

## 版本

当前版本：1.0.15-alpha

### 版本历史

- **1.0.15-alpha**: 修改模型名称提取方式，从请求头 `x-higress-llm-model` 获取，如果不存在则回退到响应体中的模型信息
- **1.0.12-alpha**: 改进 400 错误处理，区分配置错误和服务错误
- **1.0.11-alpha**: 修复流式响应计费问题
- **1.0.0-alpha**: 初始版本
