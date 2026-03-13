# AI Capability Handler Plugin

AI Capability Handler Plugin 是一个 WASM 插件，用于与 capability-service 集成，实现统一的能力处理框架。该插件负责在请求阶段调用 capability-service 进行各种能力验证和处理，如 API 认证、IP 白名单、参数检查等。

## 功能特性

- **统一能力处理**: 与 capability-service 集成，支持多种访问控制和安全能力
- **请求修改**: 支持根据能力策略修改请求头、查询参数、路径和请求体
- **请求阻断**: 支持根据能力策略阻断不符合条件的请求
- **响应检查**: 支持响应阶段的检查标记（当前仅记录日志）
- **超时处理**: 支持可配置的超时处理策略
- **错误恢复**: 支持在能力服务不可用时的降级处理

## 配置说明

### 基本配置

```yaml
capability_service:
  url: "http://capability-service.dns/v1/capability/verify-request"
  timeout_ms: 1000
  timeout_action: continue  # continue | deny
timeout_deny_message:
  code: 501
  message: "能力处理超时，请联系管理员"
```

### 配置参数

| 参数 | 类型 | 必填 | 说明 | 默认值 |
|------|------|------|------|--------|
| `capability_service.url` | string | 是 | Capability service 完整 URL | - |
| `capability_service.timeout_ms` | int | 否 | 请求超时时间（毫秒） | 1000 |
| `capability_service.timeout_action` | string | 否 | 超时处理策略：continue/deny | continue |
| `timeout_deny_message.code` | int | 否 | 超时拒绝的 HTTP 状态码 | 501 |
| `timeout_deny_message.message` | string | 否 | 超时拒绝的错误消息 | "能力处理超时，请联系管理员" |

## 工作流程

1. **请求拦截**: 插件在请求阶段拦截所有请求
2. **信息提取**: 提取请求的方法、路径、头部、查询参数和请求体
3. **能力调用**: 调用 capability-service 的 `/v1/capability/verify-request` 接口
4. **结果处理**: 根据返回结果进行相应处理：
   - 如果 `allow=false`，使用 `block_info` 阻断请求
   - 如果 `allow=true`，执行 `actions` 修改请求
5. **请求继续**: 处理完成后继续请求流程

## 支持的 Actions

### Replace 操作
- **header**: 替换或新增请求头
- **query**: 替换或新增查询参数
- **path**: 替换请求路径
- **body**: 替换请求体（base64 编码）

### Unset 操作
- **header**: 删除请求头
- **query**: 删除查询参数
- **body**: 清空请求体

## 错误处理

### 超时处理
- `continue`: 忽略能力处理，继续请求（默认）
- `deny`: 拒绝请求，返回超时错误

### 错误恢复
- 网络错误：根据 `timeout_action` 处理
- 解析错误：根据 `timeout_action` 处理
- 服务错误：根据 `timeout_action` 处理

## 使用示例

### 部署配置

```yaml
apiVersion: extensions.higress.io/v1alpha1
kind: WasmPlugin
metadata:
  name: ai-capability-handler
  namespace: higress-system
spec:
  defaultConfig:
    capability_service:
      url: "http://capability-service.default.svc.cluster.local:8080/v1/capability/verify-request"
      timeout_ms: 2000
      timeout_action: deny
    timeout_deny_message:
      code: 503
      message: "服务暂时不可用，请稍后重试"
  matchRules:
  - ingress:
    - default/ai-gateway
  url: oci://registry.cn-hangzhou.aliyuncs.com/higress/ai-capability-handler:1.0.0
```

### 路由级配置

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: ai-gateway
  annotations:
    higress.io/wasm-config: |
      {
        "capability_service": {
          "url": "http://capability-service.production:8080/v1/capability/verify-request",
          "timeout_ms": 3000,
          "timeout_action": "continue"
        }
      }
spec:
  rules:
  - host: api.example.com
    http:
      paths:
      - path: /v1/chat
        pathType: Prefix
        backend:
          service:
            name: ai-service
            port:
              number: 8080
```

## 日志说明

插件会记录以下关键日志：

- 配置解析日志
- 请求处理日志
- 能力服务调用日志
- Action 处理日志
- 错误和警告日志

## 性能考虑

- 异步 HTTP 调用，避免阻塞请求处理
- 支持超时机制，防止长时间等待
- 错误恢复机制，确保服务可用性
- 最小化内存使用，避免内存泄漏

## 限制说明

- 当前 `response_check` 仅支持日志记录，不执行具体检查逻辑
- Body 修改可能影响性能，建议谨慎使用
- 依赖 capability-service 的可用性和性能

## 故障排查

### 常见问题

1. **配置错误**: 检查 capability-service URL 是否正确
2. **网络问题**: 检查插件与 capability-service 的网络连通性
3. **超时问题**: 调整 `timeout_ms` 参数或检查服务性能
4. **Action 失败**: 检查 Action 格式和目标类型是否正确

### 调试方法

1. 查看插件日志：`kubectl logs -n higress-system deployment/higress-gateway`
2. 检查配置：`kubectl get wasmPlugin ai-capability-handler -o yaml`
3. 测试连通性：在网关 Pod 中测试到 capability-service 的连接