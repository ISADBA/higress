# Scope Priority Test Plugin

## 概述

这是一个用于测试和验证 Higress WASM 插件系统中多种作用域配置优先级机制的测试插件。该插件完整实现了配置解析、HTTP 头处理和 HTTP Body 处理功能，并在处理过程中打印所有可用的元数据信息。

## 功能特性

- ✅ 完整实现 `ParseConfigFunc` 接口，支持配置解析
- ✅ 完整实现 `onHttpHeadersFunc` 接口，打印请求/响应头元数据
- ✅ 完整实现 `onHttpBodyFunc` 接口，打印请求/响应体元数据
- ✅ 支持多种作用域配置（GLOBAL、CONSUMER、ROUTE、DOMAIN、SERVICE）
- ✅ 打印详细的元数据信息用于调试和验证
- ✅ 使用非废弃的 wrapper API

## 配置结构

```json
{
  "scopeType": "GLOBAL",
  "message": "This is global config",
  "priority": 1,
  "enableRequestHeaders": true,
  "enableResponseHeaders": true,
  "enableRequestBody": true,
  "enableResponseBody": true
}
```

### 配置字段说明

| 字段 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `scopeType` | string | 否 | "UNKNOWN" | 作用域类型标识，用于日志输出 |
| `message` | string | 否 | "" | 自定义消息，用于区分不同配置 |
| `priority` | int | 否 | 0 | 优先级数值，用于标识配置优先级 |
| `enableRequestHeaders` | bool | 否 | true | 是否打印请求头信息 |
| `enableResponseHeaders` | bool | 否 | true | 是否打印响应头信息 |
| `enableRequestBody` | bool | 否 | true | 是否打印请求体信息 |
| `enableResponseBody` | bool | 否 | true | 是否打印响应体信息 |

## 作用域优先级

插件支持以下作用域，按优先级从高到低排列：

1. **CONSUMER** (最高优先级) - 按 consumer 名称匹配
2. **ROUTE** - 按路由名称匹配
3. **SERVICE** - 按服务名称匹配
4. **DOMAIN** - 按域名匹配
5. **GLOBAL** (最低优先级) - 默认配置

## 配置示例

### 完整配置示例

参见 `config-example.json` 文件，包含所有作用域的配置示例。

### 测试场景配置

#### 场景 1: 只有 GLOBAL 配置

```json
{
  "scopeType": "GLOBAL",
  "message": "Global config only",
  "priority": 1
}
```

#### 场景 2: GLOBAL + CONSUMER 配置

```json
{
  "scopeType": "GLOBAL",
  "message": "Global config",
  "priority": 1,
  "_rules_": [
    {
      "consumer": ["test-user"],
      "_match_consumer_": ["test-user"],
      "config": {
        "scopeType": "CONSUMER",
        "message": "Consumer config for test-user",
        "priority": 100
      }
    }
  ]
}
```

#### 场景 3: GLOBAL + ROUTE 配置

```json
{
  "scopeType": "GLOBAL",
  "message": "Global config",
  "priority": 1,
  "_rules_": [
    {
      "ingress": ["my-route"],
      "_match_route_": ["my-route"],
      "config": {
        "scopeType": "ROUTE",
        "message": "Route config for my-route",
        "priority": 80
      }
    }
  ]
}
```

## 构建和部署

### 构建插件

```bash
cd plugins/wasm-go/extensions/scope-priority-test
go mod tidy
tinygo build -o main.wasm -scheduler=none -target=wasi -gc=custom -tags='custommalloc nottinygc_finalizer' main.go
```

### 部署到 Higress

1. 将编译好的 `main.wasm` 文件上传到 Higress
2. 创建 WasmPlugin 资源，配置插件
3. 应用配置到目标路由或域名

## 日志输出

### 请求头阶段日志示例

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

### 响应头阶段日志示例

```
[INFO] === Scope Priority Test - Response Headers ===
[INFO] Matched Scope Type: CONSUMER
[INFO] Response Status: 200
[INFO] Response Headers Count: 5
[INFO]   :status: 200
[INFO]   content-type: application/json
[INFO]   content-length: 123
[INFO]   date: Mon, 01 Jan 2024 00:00:00 GMT
[INFO]   server: Higress
[INFO] ===========================================
```

### 请求体阶段日志示例

```
[INFO] === Scope Priority Test - Request Body ===
[INFO] Matched Scope Type: CONSUMER
[INFO] Request Body Size: 45 bytes
[INFO] Request Body Preview: {"name":"test","value":"example data"}
[INFO] ==========================================
```

### 响应体阶段日志示例

```
[INFO] === Scope Priority Test - Response Body ===
[INFO] Matched Scope Type: CONSUMER
[INFO] Response Body Size: 123 bytes
[INFO] Response Body Preview: {"status":"success","data":{"id":1,"name":"test"}}
[INFO] ===========================================
```

## 测试验证

### 验证作用域优先级

1. **测试 GLOBAL 作用域**
   - 配置只有 GLOBAL 配置
   - 发送请求，查看日志输出 `Matched Scope Type: GLOBAL`

2. **测试 CONSUMER 优先级**
   - 配置 GLOBAL + CONSUMER
   - 使用带 consumer 标识的请求
   - 查看日志输出 `Matched Scope Type: CONSUMER`

3. **测试 ROUTE 优先级**
   - 配置 GLOBAL + ROUTE
   - 发送请求到匹配的路由
   - 查看日志输出 `Matched Scope Type: ROUTE`

4. **测试完整优先级顺序**
   - 配置所有作用域
   - 分别测试不同场景
   - 验证优先级顺序：CONSUMER > ROUTE > SERVICE > DOMAIN > GLOBAL

### 验证控制开关

1. 设置 `enableRequestHeaders: false`，验证不打印请求头
2. 设置 `enableResponseHeaders: false`，验证不打印响应头
3. 设置 `enableRequestBody: false`，验证不打印请求体
4. 设置 `enableResponseBody: false`，验证不打印响应体

## 注意事项

- 二进制内容（如图片、视频）只打印大小，不打印内容
- Body 内容超过 500 字节时，只打印前 500 字节
- 所有元数据获取失败时，会跳过该项但继续执行
- 插件使用非废弃的 wrapper API，符合最佳实践

## 许可证

Apache License 2.0
