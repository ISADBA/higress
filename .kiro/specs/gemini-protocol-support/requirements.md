# 需求文档

## 简介

本文档规定了增强 ai-header-modifier 插件以支持 Gemini 原生协议的需求。Gemini 使用独特的 API 风格，其中模型名称和 API 密钥通过 URL 路径和查询参数传递，而不是通过请求头或请求体。此增强功能将使插件能够检测 Gemini 原生协议请求，从 URL 中提取必要信息，并将其转换为 Higress AI 网关使用的标准请求头格式。

## 术语表

- **AI_Header_Modifier**：Higress 插件，从 LLM 请求中提取模型和提供商信息并将其添加到请求头
- **Gemini_原生协议**：Google Gemini API 协议，其中模型和 API 密钥通过 URL 路径和查询参数传递
- **模型名称**：AI 模型的标识符（例如 "gemini-3.1-pro-preview"）
- **API_密钥**：访问 Gemini API 的身份验证密钥
- **提供商**：LLM 服务提供商标识符（例如 "gemini"、"default"）
- **请求路径**：HTTP 请求的 URL 路径组件
- **查询参数**：URL 查询字符串中 "?" 字符后的键值对
- **请求体**：HTTP 请求的 JSON 负载
- **默认提供商**：配置中的 defaultProvider 值，当查询参数中不存在 provider 时使用

## 需求

### 需求 1：检测 Gemini 原生协议

**用户故事：** 作为开发者，我希望插件能够自动检测 Gemini 原生协议请求，以便在无需手动配置的情况下正确处理它们。

#### 验收标准

1. WHEN 请求路径以 "/v1/models/" 开头，THE AI_Header_Modifier SHALL 将请求识别为使用 Gemini_原生协议
2. WHEN 请求路径不以 "/v1/models/" 开头，THE AI_Header_Modifier SHALL 使用现有协议处理逻辑处理请求

### 需求 2：从 URL 路径提取模型名称

**用户故事：** 作为开发者，我希望从 URL 路径中提取模型名称，以便将其用于路由和计费目的。

#### 验收标准

1. WHEN 检测到 Gemini_原生协议请求，THE AI_Header_Modifier SHALL 从 "/v1/models/" 和下一个 "/" 或 ":" 字符之间的路径段中提取模型名称
2. WHEN 成功提取模型名称，THE AI_Header_Modifier SHALL 将 "x-higress-llm-model" 请求头设置为提取的模型名称值
3. WHEN 无法从路径中提取模型名称，THE AI_Header_Modifier SHALL 记录警告消息并继续处理而不设置模型请求头

### 需求 3：从查询参数提取 API 密钥

**用户故事：** 作为开发者，我希望从查询参数中提取 API 密钥并设置到相应的请求头，以便 Gemini 协议请求能够正确进行身份验证和消费者识别。

#### 验收标准

1. WHEN 检测到 Gemini_原生协议请求，THE AI_Header_Modifier SHALL 从 "key" 查询参数中提取 API_密钥
2. WHEN 成功提取 API_密钥，THE AI_Header_Modifier SHALL 将 "x-mse-consumer-apikey" 请求头设置为提取的 API_密钥值
3. WHEN 成功提取 API_密钥且 "x-api-key" 请求头不存在或值为空，THE AI_Header_Modifier SHALL 将 "x-api-key" 请求头设置为提取的 API_密钥值
4. WHEN "x-api-key" 请求头已存在且有值，THE AI_Header_Modifier SHALL 不修改 "x-api-key" 请求头的值
5. WHEN "key" 查询参数不存在，THE AI_Header_Modifier SHALL 记录警告消息并继续处理而不设置这些 API 密钥请求头

### 需求 4：处理提供商参数

**用户故事：** 作为开发者，我希望从查询参数或配置中确定提供商，以便将请求路由到正确的后端服务。

#### 验收标准

1. WHEN Gemini_原生协议请求包含 "provider" 查询参数，THE AI_Header_Modifier SHALL 将 "x-request-llm-provider" 请求头设置为 "provider" 查询参数的值
2. WHEN Gemini_原生协议请求不包含 "provider" 查询参数，THE AI_Header_Modifier SHALL 将 "x-request-llm-provider" 请求头设置为配置的默认提供商值
3. WHEN 默认提供商配置未设置，THE AI_Header_Modifier SHALL 使用 "default" 作为提供商值

### 需求 5：向请求体添加模型属性

**用户故事：** 作为开发者，我希望将模型信息添加到请求体中，以便下游服务接收格式正确的请求。

#### 验收标准

1. WHEN 处理 Gemini_原生协议请求且请求体不包含 "model" 属性，THE AI_Header_Modifier SHALL 向请求体添加 "model" 属性
2. WHEN "x-request-llm-provider" 请求头值为 "default"，THE AI_Header_Modifier SHALL 将 "model" 属性设置为仅包含模型名称
3. WHEN "x-request-llm-provider" 请求头值不为 "default"，THE AI_Header_Modifier SHALL 将 "model" 属性设置为 "{提供商}/{模型名称}" 格式
4. WHEN 请求体已包含 "model" 属性，THE AI_Header_Modifier SHALL 不修改现有的 "model" 属性值
5. WHEN 请求体不是有效的 JSON，THE AI_Header_Modifier SHALL 记录警告消息并跳过添加模型属性

### 需求 6：解析 URL 路径和查询字符串

**用户故事：** 作为开发者，我希望插件能够正确解析带有查询参数的 URL 路径，以便可以提取基于路径和基于查询的信息。

#### 验收标准

1. WHEN 处理请求时，THE AI_Header_Modifier SHALL 在 "?" 字符处将请求路径分离为 URI 组件和查询字符串组件
2. WHEN 解析查询参数时，THE AI_Header_Modifier SHALL 正确提取由 "&" 字符分隔的键值对
3. WHEN 查询参数值包含 URL 编码字符，THE AI_Header_Modifier SHALL 在使用之前解码该值

### 需求 7：保留现有功能

**用户故事：** 作为开发者，我希望现有插件功能保持不变，以便当前集成无需修改即可继续工作。

#### 验收标准

1. WHEN 请求不使用 Gemini_原生协议，THE AI_Header_Modifier SHALL 使用现有的 JSON 和 multipart 请求体处理逻辑处理请求
2. WHEN 处理 Gemini_原生协议请求时，THE AI_Header_Modifier SHALL 仍然执行所有配置的静态请求头、固定源请求头和优先级源请求头
3. THE AI_Header_Modifier SHALL 保持与所有现有配置参数的向后兼容性

### 需求 8：日志记录和调试

**用户故事：** 作为开发者，我希望有详细的 Gemini 协议处理日志记录，以便我可以有效地排查问题。

#### 验收标准

1. WHEN 检测到 Gemini_原生协议，THE AI_Header_Modifier SHALL 记录指示 Gemini 协议检测的调试消息
2. WHEN 提取模型名称时，THE AI_Header_Modifier SHALL 在调试级别记录提取的值
3. WHEN 提取 API_密钥时，THE AI_Header_Modifier SHALL 在调试级别记录密钥已提取而不记录实际密钥值
4. WHEN 设置请求头时，THE AI_Header_Modifier SHALL 在调试级别记录请求头名称和值
5. WHEN 向请求体添加模型属性时，THE AI_Header_Modifier SHALL 在调试级别记录该操作

### 需求 9：Gemini 协议示例验证

**用户故事：** 作为开发者，我希望插件能够正确处理真实的 Gemini API 请求示例，以便确保与 Gemini 服务的兼容性。

#### 验收标准

1. WHEN 接收到路径为 "/v1/models/gemini-3.1-pro-preview:generateContent?key=ak_ezE3snddlIygrxxxxBuaNEtfsd2Ys" 的请求，THE AI_Header_Modifier SHALL 提取模型名称 "gemini-3.1-pro-preview" 并设置到 "x-higress-llm-model" 请求头
2. WHEN 接收到路径为 "/v1/models/gemini-3.1-pro-preview:generateContent?key=ak_ezE3snddlIygrxxxxBuaNEtfsd2Ys" 的请求，THE AI_Header_Modifier SHALL 提取 API 密钥 "ak_ezE3snddlIygrxxxxBuaNEtfsd2Ys" 并设置到 "x-mse-consumer-apikey" 请求头
3. WHEN 接收到路径为 "/v1/models/gemini-3.1-pro-preview:generateContent?key=ak_ezE3snddlIygrxxxxBuaNEtfsd2Ys" 的请求且 "x-api-key" 请求头不存在或为空，THE AI_Header_Modifier SHALL 提取 API 密钥 "ak_ezE3snddlIygrxxxxBuaNEtfsd2Ys" 并设置到 "x-api-key" 请求头
4. WHEN 处理包含 "contents" 数组的 Gemini 请求体且不包含 "model" 属性，THE AI_Header_Modifier SHALL 根据提供商值添加适当格式的 "model" 属性
5. WHEN 路径包含 ":generateContent" 等操作后缀，THE AI_Header_Modifier SHALL 正确提取操作前的模型名称
