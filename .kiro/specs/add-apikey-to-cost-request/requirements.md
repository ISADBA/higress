# Requirements Document

## Introduction

本需求文档描述了在ai-billing插件的计费请求中添加apikey参数的功能。该功能需要从HTTP请求头`x-mse-consumer-apikey`中提取apikey值，并将其作为CostRequest的一个新字段发送到billing-service。

## Glossary

- **AI_Billing_Plugin**: 负责处理AI服务计费的插件系统
- **CostRequest**: 发送到billing-service的计费请求数据结构
- **Billing_Service**: 接收和处理计费请求的后端服务
- **HTTP_Header**: HTTP请求中的头部信息
- **HMAC_Authentication**: 基于哈希的消息认证码，用于验证请求的完整性和真实性

## Requirements

### Requirement 1: 提取请求头中的apikey

**User Story:** 作为系统开发者，我希望从HTTP请求头中提取apikey，以便在计费请求中包含消费者标识信息。

#### Acceptance Criteria

1. WHEN AI_Billing_Plugin处理计费请求时，THE AI_Billing_Plugin SHALL从HTTP请求头`x-mse-consumer-apikey`中读取apikey值
2. WHEN 请求头`x-mse-consumer-apikey`存在时，THE AI_Billing_Plugin SHALL提取其值作为apikey
3. WHEN 请求头`x-mse-consumer-apikey`不存在时，THE AI_Billing_Plugin SHALL将apikey设置为空字符串或null值
4. WHEN 请求头`x-mse-consumer-apikey`包含空白字符时，THE AI_Billing_Plugin SHALL保留原始值不做修改

### Requirement 2: 扩展CostRequest数据结构

**User Story:** 作为系统开发者，我希望CostRequest包含apikey字段，以便将消费者标识信息传递给billing-service。

#### Acceptance Criteria

1. THE CostRequest SHALL包含一个名为`apikey`的字段
2. THE CostRequest SHALL保留现有的所有字段：provider, model_name, request_id, input_tokens, output_tokens
3. WHEN 构造CostRequest时，THE AI_Billing_Plugin SHALL将提取的apikey值赋给apikey字段
4. THE apikey字段 SHALL独立于HMAC认证所需的9个必需字段

### Requirement 3: 发送包含apikey的计费请求

**User Story:** 作为系统开发者，我希望将包含apikey的CostRequest发送到billing-service，以便后端服务能够记录消费者信息。

#### Acceptance Criteria

1. WHEN 发送计费请求到billing-service时，THE AI_Billing_Plugin SHALL在请求体中包含apikey字段
2. THE AI_Billing_Plugin SHALL将CostRequest发送到billing-service的`/v1/cost`端点
3. WHEN CostRequest被序列化时，THE AI_Billing_Plugin SHALL确保apikey字段被正确编码
4. WHEN billing-service响应时，THE AI_Billing_Plugin SHALL正常处理响应，不受apikey字段影响

### Requirement 4: 向后兼容性

**User Story:** 作为系统维护者，我希望新功能不破坏现有的计费流程，以便平滑升级系统。

#### Acceptance Criteria

1. WHEN 旧版本的billing-service接收到包含apikey的请求时，THE Billing_Service SHALL能够正常处理请求（忽略未知字段）
2. WHEN CostRequest包含apikey字段时，THE AI_Billing_Plugin SHALL继续正常执行HMAC认证流程
3. THE AI_Billing_Plugin SHALL保持现有的错误处理机制不变
4. WHEN apikey为空或null时，THE AI_Billing_Plugin SHALL仍然发送计费请求
