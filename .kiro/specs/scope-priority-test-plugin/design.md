# 设计文档

## 概述

本设计文档描述了作用域优先级测试插件的技术实现方案。该插件基于 hello-world 插件进行扩展，完整实现配置解析、HTTP 头处理和 HTTP Body 处理功能，用于验证和展示 Higress WASM 插件系统中多种作用域的配置优先级机制。

## 架构

### 插件架构

```
scope-priority-test-plugin
├── 配置解析层 (ParseConfig)
│   └── 解析 JSON 配置，支持多作用域规则
├── HTTP 头处理层 (onHttpRequestHeaders/onHttpResponseHeaders)
│   ├── 获取匹配的配置
│   ├── 打印请求/响应头信息
│   └── 打印元数据信息
└── HTTP Body 处理层 (onHttpRequestBody/onHttpResponseBody)
    ├── 获取请求/响应体
    ├── 打印 Body 信息
    └── 打印执行阶段信息
```

### 作用域优先级机制

插件将利用 Higress 现有的作用域匹配机制，按以下优先级顺序匹配配置：

1. **CONSUMER** (最高优先级) - 按 consumer 名称匹配
2. **ROUTE** - 按路由名称匹配
3. **SERVICE** - 按服务名称匹配
4. **DOMAIN** - 按域名匹配
5. **GLOBAL** (最低优先级) - 默认配置

## 组件和接口

### 1. 配置结构定义

```go
type ScopePriorityTestConfig struct {
    // 作用域类型标识（用于日志输出）
    ScopeType string `json:"scopeType"`
    
    // 自定义消息（用于区分不同配置）
    Message string `json:"message"`
    
    // 控制开关
    EnableRequestHeaders  bool `json:"enableRequestHeaders"`
    EnableResponseHeaders bool `json:"enableResponseHeaders"`
    EnableRequestBody     bool `json:"enableRequestBody"`
    EnableResponseBody    bool `json:"enableResponseBody"`
    
    // 额外的测试字段
    Priority int `json:"priority"`
}
```

### 2. 配置解析函数

```go
func parseConfig(json gjson.Result, config *ScopePriorityTestConfig) error {
    // 解析 scopeType
    config.ScopeType = json.Get("scopeType").String()
    if config.ScopeType == "" {
        config.ScopeType = "UNKNOWN"
    }
    
    // 解析 message
    config.Message = json.Get("message").String()
    
    // 解析控制开关（默认全部启用）
    config.EnableRequestHeaders = json.Get("enableRequestHeaders").Bool()
    if !json.Get("enableRequestHeaders").Exists() {
        config.EnableRequestHeaders = true
    }
    
    config.EnableResponseHeaders = json.Get("enableResponseHeaders").Bool()
    if !json.Get("enableResponseHeaders").Exists() {
        config.EnableResponseHeaders = true
    }
    
    config.EnableRequestBody = json.Get("enableRequestBody").Bool()
    if !json.Get("enableRequestBody").Exists() {
        config.EnableRequestBody = true
    }
    
    config.EnableResponseBody = json.Get("enableResponseBody").Bool()
    if !json.Get("enableResponseBody").Exists() {
        config.EnableResponseBody = true
    }
    
    // 解析优先级
    config.Priority = int(json.Get("priority").Int())
    
    return nil
}
```

### 3. HTTP 请求头处理函数

```go
func onHttpRequestHeaders(ctx wrapper.HttpContext, config ScopePriorityTestConfig) types.Action {
    // 打印配置信息
    proxywasm.LogInfof("=== Scope Priority Test - Request Headers ===")
    proxywasm.LogInfof("Matched Scope Type: %s", config.ScopeType)
    proxywasm.LogInfof("Config Message: %s", config.Message)
    proxywasm.LogInfof("Config Priority: %d", config.Priority)
    
    if config.EnableRequestHeaders {
        // 打印伪头部
        method := ctx.Method()
        path := ctx.Path()
        host := ctx.Host()
        scheme := ctx.Scheme()
        
        proxywasm.LogInfof("Request Method: %s", method)
        proxywasm.LogInfof("Request Path: %s", path)
        proxywasm.LogInfof("Request Host: %s", host)
        proxywasm.LogInfof("Request Scheme: %s", scheme)
        
        // 打印所有请求头
        headers, _ := proxywasm.GetHttpRequestHeaders()
        proxywasm.LogInfof("Request Headers Count: %d", len(headers))
        for _, header := range headers {
            proxywasm.LogInfof("  %s: %s", header[0], header[1])
        }
        
        // 打印上下文信息
        printContextInfo()
    }
    
    proxywasm.LogInfof("===========================================")
    
    return types.ActionContinue
}
```

### 4. HTTP 响应头处理函数

```go
func onHttpResponseHeaders(ctx wrapper.HttpContext, config ScopePriorityTestConfig) types.Action {
    if !config.EnableResponseHeaders {
        return types.ActionContinue
    }
    
    proxywasm.LogInfof("=== Scope Priority Test - Response Headers ===")
    proxywasm.LogInfof("Matched Scope Type: %s", config.ScopeType)
    
    // 打印响应状态码
    status, _ := proxywasm.GetHttpResponseHeader(":status")
    proxywasm.LogInfof("Response Status: %s", status)
    
    // 打印所有响应头
    headers, _ := proxywasm.GetHttpResponseHeaders()
    proxywasm.LogInfof("Response Headers Count: %d", len(headers))
    for _, header := range headers {
        proxywasm.LogInfof("  %s: %s", header[0], header[1])
    }
    
    proxywasm.LogInfof("===========================================")
    
    return types.ActionContinue
}
```

### 5. HTTP 请求体处理函数

```go
func onHttpRequestBody(ctx wrapper.HttpContext, config ScopePriorityTestConfig, body []byte) types.Action {
    if !config.EnableRequestBody {
        return types.ActionContinue
    }
    
    proxywasm.LogInfof("=== Scope Priority Test - Request Body ===")
    proxywasm.LogInfof("Matched Scope Type: %s", config.ScopeType)
    proxywasm.LogInfof("Request Body Size: %d bytes", len(body))
    
    // 如果是文本内容，打印前 500 字节
    if len(body) > 0 && !ctx.IsBinaryRequestBody() {
        maxLen := 500
        if len(body) < maxLen {
            maxLen = len(body)
        }
        proxywasm.LogInfof("Request Body Preview: %s", string(body[:maxLen]))
    }
    
    proxywasm.LogInfof("==========================================")
    
    return types.ActionContinue
}
```

### 6. HTTP 响应体处理函数

```go
func onHttpResponseBody(ctx wrapper.HttpContext, config ScopePriorityTestConfig, body []byte) types.Action {
    if !config.EnableResponseBody {
        return types.ActionContinue
    }
    
    proxywasm.LogInfof("=== Scope Priority Test - Response Body ===")
    proxywasm.LogInfof("Matched Scope Type: %s", config.ScopeType)
    proxywasm.LogInfof("Response Body Size: %d bytes", len(body))
    
    // 如果是文本内容，打印前 500 字节
    if len(body) > 0 && !ctx.IsBinaryResponseBody() {
        maxLen := 500
        if len(body) < maxLen {
            maxLen = len(body)
        }
        proxywasm.LogInfof("Response Body Preview: %s", string(body[:maxLen]))
    }
    
    proxywasm.LogInfof("===========================================")
    
    return types.ActionContinue
}
```

### 7. 辅助函数 - 打印上下文信息

```go
func printContextInfo() {
    // 打印 consumer 信息
    consumerName, err := proxywasm.GetProperty([]string{"consumer_name"})
    if err == nil && len(consumerName) > 0 {
        proxywasm.LogInfof("Consumer Name: %s", string(consumerName))
    }
    
    // 打印路由信息
    routeName, err := proxywasm.GetProperty([]string{"route_name"})
    if err == nil && len(routeName) > 0 {
        proxywasm.LogInfof("Route Name: %s", string(routeName))
    }
    
    // 打印服务/集群信息
    clusterName, err := proxywasm.GetProperty([]string{"cluster_name"})
    if err == nil && len(clusterName) > 0 {
        proxywasm.LogInfof("Cluster Name: %s", string(clusterName))
    }
    
    // 打印请求 ID
    requestID, _ := proxywasm.GetHttpRequestHeader("x-request-id")
    if requestID != "" {
        proxywasm.LogInfof("Request ID: %s", requestID)
    }
}
```

## 数据模型

### 配置示例

```json
{
  "_plugin_id_": "scope-priority-test",
  "scopeType": "GLOBAL",
  "message": "This is global config",
  "priority": 1,
  "enableRequestHeaders": true,
  "enableResponseHeaders": true,
  "enableRequestBody": true,
  "enableResponseBody": true,
  "_rules_": [
    {
      "consumer": ["premium-user"],
      "_match_consumer_": ["premium-user"],
      "config": {
        "scopeType": "CONSUMER",
        "message": "This is consumer-level config for premium-user",
        "priority": 100
      }
    },
    {
      "ingress": ["test-route"],
      "_match_route_": ["test-route"],
      "config": {
        "scopeType": "ROUTE",
        "message": "This is route-level config for test-route",
        "priority": 80
      }
    },
    {
      "domain": ["example.com"],
      "_match_domain_": ["example.com"],
      "config": {
        "scopeType": "DOMAIN",
        "message": "This is domain-level config for example.com",
        "priority": 60
      }
    },
    {
      "service": ["test-service.default.svc.cluster.local"],
      "_match_service_": ["test-service"],
      "config": {
        "scopeType": "SERVICE",
        "message": "This is service-level config",
        "priority": 70
      }
    }
  ]
}
```

### 日志输出示例

```
[INFO] === Scope Priority Test - Request Headers ===
[INFO] Matched Scope Type: CONSUMER
[INFO] Config Message: This is consumer-level config for premium-user
[INFO] Config Priority: 100
[INFO] Request Method: GET
[INFO] Request Path: /api/test
[INFO] Request Host: example.com
[INFO] Request Scheme: https
[INFO] Request Headers Count: 8
[INFO]   :method: GET
[INFO]   :path: /api/test
[INFO]   :authority: example.com
[INFO]   :scheme: https
[INFO]   user-agent: curl/7.68.0
[INFO]   accept: */*
[INFO]   x-request-id: 12345-67890
[INFO] Consumer Name: premium-user
[INFO] Route Name: test-route
[INFO] Cluster Name: test-service
[INFO] Request ID: 12345-67890
[INFO] ===========================================
```

## 错误处理

### 1. 配置解析错误
- 当配置格式不正确时，记录错误日志并使用默认配置
- 当必填字段缺失时，使用合理的默认值

### 2. 元数据获取错误
- 当获取 Property 失败时，记录警告但继续执行
- 当获取 Header 失败时，跳过该 Header 的打印

### 3. Body 处理错误
- 当 Body 为二进制格式时，只打印大小不打印内容
- 当 Body 过大时，只打印前 500 字节

## 测试策略

### 单元测试
- 测试配置解析函数的正确性
- 测试不同配置字段的默认值处理
- 测试错误配置的处理

### 集成测试
- 测试 GLOBAL 作用域配置的应用
- 测试 CONSUMER 作用域配置的优先级
- 测试 ROUTE 作用域配置的优先级
- 测试 DOMAIN 作用域配置的优先级
- 测试 SERVICE 作用域配置的优先级
- 测试多个作用域同时存在时的优先级顺序

### 手动测试场景
1. 只配置 GLOBAL，验证全局配置生效
2. 配置 GLOBAL + CONSUMER，验证 CONSUMER 优先
3. 配置 GLOBAL + ROUTE，验证 ROUTE 优先
4. 配置 GLOBAL + CONSUMER + ROUTE，验证 CONSUMER 最优先
5. 配置所有作用域，验证完整的优先级顺序

## 正确性属性

*属性是一个特征或行为，应该在系统的所有有效执行中保持为真——本质上是关于系统应该做什么的正式声明。属性作为人类可读规范和机器可验证正确性保证之间的桥梁。*

### 属性 1: 配置解析正确性
*对于任何* 有效的 JSON 配置，解析后的配置对象应当包含所有指定的字段值，未指定的字段应当使用默认值
**验证需求: 需求 1.1, 1.2, 1.3**

### 属性 2: 作用域优先级一致性
*对于任何* 请求，当存在多个作用域的配置时，系统应当始终选择优先级最高的匹配配置（CONSUMER > ROUTE > SERVICE > DOMAIN > GLOBAL）
**验证需求: 需求 2.6**

### 属性 3: 元数据打印完整性
*对于任何* 启用了打印功能的配置，系统应当打印所有可用的元数据信息，不应遗漏任何请求头或响应头
**验证需求: 需求 3.2, 3.3, 3.7, 5.4**

### 属性 4: 控制开关有效性
*对于任何* 配置，当某个 enable 开关设置为 false 时，对应的打印功能应当被禁用，不产生任何日志输出
**验证需求: 需求 6.3, 6.4, 6.5, 6.6**

### 属性 5: 错误处理鲁棒性
*对于任何* 配置解析错误或元数据获取错误，系统应当记录错误信息但继续执行，不应中断请求处理流程
**验证需求: 需求 1.4**

## 测试配置

- 每个属性测试最少 100 次迭代（由于随机化）
- 每个属性测试必须引用其设计文档属性
- 标签格式: **Feature: scope-priority-test-plugin, Property {number}: {property_text}**
