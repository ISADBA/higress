# 状态码上下文传递修复

## 问题描述

在之前的实现中，虽然在 `onHttpResponseHeaders` 中正确检测了非 200 状态码并记录日志，但 `onHttpStreamingResponseBody` 仍然会被调用并尝试提取计费信息，导致调用 `sendErrorResponse()` 破坏 HTTP/2 流。

## 根本原因

在响应体处理阶段（`onHttpResponseBody` 和 `onHttpStreamingResponseBody`），无法可靠地通过 `proxywasm.GetHttpResponseHeader(":status")` 获取 HTTP 状态码。这是因为：

1. HTTP 伪头部（pseudo-headers）如 `:status` 在响应头阶段之后可能不再可用
2. Envoy 的 WASM ABI 在不同阶段对头部的访问有不同的限制

## 解决方案

使用 Context 在不同阶段之间传递状态码：

1. 在 `onHttpResponseHeaders` 中：
   - 获取并解析 `:status` 头部
   - 将状态码（int 类型）存储到 context 中：`ctx.SetContext(CtxKeyStatusCode, statusCodeInt)`

2. 在 `onHttpResponseBody` 和 `onHttpStreamingResponseBody` 中：
   - 从 context 读取状态码：`ctx.GetContext(CtxKeyStatusCode).(int)`
   - 如果状态码不是 200，提前返回，跳过计费处理

## 代码变更

### 1. 添加 Context Key

```go
const (
    // ... 其他 keys
    CtxKeyStatusCode = "ai-billing-status-code"
)
```

### 2. 在 onHttpResponseHeaders 中存储状态码

```go
// 解析状态码
statusCodeStr, err := proxywasm.GetHttpResponseHeader(":status")
statusCodeInt := 200 // default to 200
if err == nil {
    if code, parseErr := strconv.Atoi(statusCodeStr); parseErr == nil {
        statusCodeInt = code
    }
}

// 存储到 context
ctx.SetContext(CtxKeyStatusCode, statusCodeInt)
```

### 3. 在响应体处理函数中读取状态码

```go
// onHttpResponseBody 和 onHttpStreamingResponseBody 中
if statusCode, ok := ctx.GetContext(CtxKeyStatusCode).(int); ok && statusCode != 200 {
    log.Debugf("[%s] skipping response body processing for non-200 response: status=%d", pluginName, statusCode)
    return data // 或 types.ActionContinue
}
```

## 测试验证

所有测试通过，包括：
- 非 200 响应测试（4xx, 5xx, 3xx）
- 200 响应保留测试（流式、非流式、计费流程）
- 请求拒绝场景测试

## 经验教训

在 Envoy WASM 插件开发中：
1. 不要假设所有 HTTP 头部在所有阶段都可用
2. 使用 Context 在不同阶段之间传递关键信息
3. 伪头部（如 `:status`, `:method`, `:path`）在响应体阶段可能不可用
