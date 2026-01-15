# 需求文档

## 介绍

基于 hello-world 插件创建一个作用域优先级测试插件，用于验证和展示 Higress WASM 插件系统中多种作用域（GLOBAL、DOMAIN、SERVICE、ROUTE、CONSUMER）的配置优先级机制。该插件将完整实现配置解析、HTTP 头处理和 HTTP Body 处理功能，并在处理过程中打印所有可用的元数据信息。

## 术语表

- **Scope**: 作用域，定义插件生效的范围级别（GLOBAL、DOMAIN、SERVICE、ROUTE、CONSUMER）
- **Priority**: 优先级，决定多个作用域插件配置的执行顺序
- **ParseConfigFunc**: 配置解析函数，用于解析插件配置
- **onHttpHeadersFunc**: HTTP 头处理函数，在请求/响应头阶段执行
- **onHttpBodyFunc**: HTTP Body 处理函数，在请求/响应体阶段执行
- **Metadata**: 元数据，包括请求头、响应头、上下文信息等

## 需求

### 需求 1

**用户故事：** 作为一个插件开发者，我希望能够完整实现 ParseConfigFunc 接口，以便正确解析不同作用域的插件配置。

#### 验收标准

1. 当插件启动时，系统应当调用 ParseConfigFunc 解析全局配置
2. 当存在多个作用域的配置规则时，系统应当正确解析每个规则的配置
3. 当配置包含 consumer、domain、service、route 等匹配规则时，系统应当正确识别和存储这些规则
4. 当配置解析失败时，系统应当返回明确的错误信息

### 需求 2

**用户故事：** 作为一个插件开发者，我希望能够在配置中支持多种作用域，以便测试作用域优先级机制。

#### 验收标准

1. 当配置包含 GLOBAL 作用域时，系统应当将其作为默认配置
2. 当配置包含 CONSUMER 作用域规则时，系统应当支持按 consumer 名称匹配
3. 当配置包含 ROUTE 作用域规则时，系统应当支持按路由名称匹配
4. 当配置包含 DOMAIN 作用域规则时，系统应当支持按域名匹配
5. 当配置包含 SERVICE 作用域规则时，系统应当支持按服务名称匹配
6. 当同时存在多个作用域配置时，系统应当按照优先级顺序（CONSUMER > ROUTE > SERVICE > DOMAIN > GLOBAL）选择配置

### 需求 3

**用户故事：** 作为一个插件开发者，我希望能够完整实现 onHttpHeadersFunc 接口，以便在 HTTP 头处理阶段打印所有可用的元数据。

#### 验收标准

1. 当请求到达时，系统应当在请求头阶段调用 onHttpRequestHeaders 函数
2. 当处理请求头时，系统应当打印所有请求头信息（包括伪头部 :method、:path、:authority、:scheme）
3. 当处理请求头时，系统应当打印当前匹配的配置作用域信息
4. 当处理请求头时，系统应当打印 consumer 信息（如果存在）
5. 当处理请求头时，系统应当打印路由名称、服务名称等上下文信息
6. 当响应返回时，系统应当在响应头阶段调用 onHttpResponseHeaders 函数
7. 当处理响应头时，系统应当打印所有响应头信息

### 需求 4

**用户故事：** 作为一个插件开发者，我希望能够完整实现 onHttpBodyFunc 接口，以便在 HTTP Body 处理阶段打印所有可用的元数据。

#### 验收标准

1. 当请求包含 Body 时，系统应当在请求体阶段调用 onHttpRequestBody 函数
2. 当处理请求体时，系统应当打印请求体大小信息
3. 当处理请求体时，系统应当打印请求体内容（如果是文本格式）
4. 当处理请求体时，系统应当打印当前的执行阶段信息
5. 当响应包含 Body 时，系统应当在响应体阶段调用 onHttpResponseBody 函数
6. 当处理响应体时，系统应当打印响应体大小和内容信息

### 需求 5

**用户故事：** 作为一个插件开发者，我希望插件能够打印详细的元数据信息，以便调试和验证作用域优先级机制。

#### 验收标准

1. 当插件执行时，系统应当打印当前使用的配置作用域类型（GLOBAL、CONSUMER、ROUTE 等）
2. 当插件执行时，系统应当打印配置中的自定义字段值
3. 当插件执行时，系统应当打印请求的完整路径、方法、主机信息
4. 当插件执行时，系统应当打印所有可用的 HTTP 头信息
5. 当插件执行时，系统应当打印上下文中的 consumer_name、route_name、cluster_name 等属性
6. 当插件执行时，系统应当使用结构化的日志格式便于阅读

### 需求 6

**用户故事：** 作为一个插件开发者，我希望配置结构清晰且易于扩展，以便支持不同的测试场景。

#### 验收标准

1. 当定义配置结构时，系统应当包含 scopeType 字段标识作用域类型
2. 当定义配置结构时，系统应当包含 message 字段用于自定义消息
3. 当定义配置结构时，系统应当包含 enableRequestHeaders 字段控制是否打印请求头
4. 当定义配置结构时，系统应当包含 enableResponseHeaders 字段控制是否打印响应头
5. 当定义配置结构时，系统应当包含 enableRequestBody 字段控制是否打印请求体
6. 当定义配置结构时，系统应当包含 enableResponseBody 字段控制是否打印响应体
7. 当定义配置结构时，系统应当支持 JSON 格式的配置解析

### 需求 7

**用户故事：** 作为一个插件开发者，我希望插件能够正确使用新的非废弃 API，以便符合最佳实践。

#### 验收标准

1. 当初始化插件时，系统应当使用 wrapper.ProcessRequestHeaders 而不是废弃的 ProcessRequestHeadersBy
2. 当初始化插件时，系统应当使用 wrapper.ProcessRequestBody 而不是废弃的 ProcessRequestBodyBy
3. 当初始化插件时，系统应当使用 wrapper.ProcessResponseHeaders 而不是废弃的 ProcessResponseHeadersBy
4. 当初始化插件时，系统应当使用 wrapper.ProcessResponseBody 而不是废弃的 ProcessResponseBodyBy
5. 当初始化插件时，系统应当使用 wrapper.ParseConfig 而不是废弃的 ParseConfigBy
