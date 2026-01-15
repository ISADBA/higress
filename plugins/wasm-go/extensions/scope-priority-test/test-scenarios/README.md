# 作用域优先级测试场景

本目录包含多个测试场景的 YAML 配置文件，用于验证 Higress WASM 插件的作用域优先级机制。

## 测试场景列表

### 01-global-only.yaml
**测试目标**: 验证只有 GLOBAL 配置时的行为

**配置内容**:
- 只有 GLOBAL 默认配置

**预期结果**:
- 所有请求都使用 GLOBAL 配置
- 日志显示 `Matched Scope Type: GLOBAL`

**测试命令**:
```bash
kubectl apply -f 01-global-only.yaml
curl http://your-gateway/any-path
```

---

### 02-consumer-priority.yaml
**测试目标**: 验证 CONSUMER 作用域优先于 GLOBAL

**配置内容**:
- GLOBAL 配置（优先级 1）
- CONSUMER 配置 for `premium-user`（优先级 100）

**预期结果**:
- consumer=premium-user 的请求使用 CONSUMER 配置
- 其他请求使用 GLOBAL 配置

**测试命令**:
```bash
kubectl apply -f 02-consumer-priority.yaml

# 测试 consumer 请求（需要先配置认证插件设置 consumer_name）
curl -H "Authorization: Bearer premium-user-token" http://your-gateway/api/test

# 测试非 consumer 请求
curl http://your-gateway/api/test
```

---

### 03-route-priority.yaml
**测试目标**: 验证 ROUTE 作用域优先于 GLOBAL

**配置内容**:
- GLOBAL 配置（优先级 1）
- ROUTE 配置 for `test-route`（优先级 80）

**预期结果**:
- 匹配 test-route 的请求使用 ROUTE 配置
- 其他请求使用 GLOBAL 配置

**测试命令**:
```bash
kubectl apply -f 03-route-priority.yaml

# 测试匹配 test-route 的请求
curl http://your-gateway/test-route-path

# 测试不匹配的请求
curl http://your-gateway/other-path
```

---

### 04-full-priority-chain.yaml
**测试目标**: 验证完整的作用域优先级链

**配置内容**:
- GLOBAL 配置（优先级 1）
- DOMAIN 配置 for `example.com`（优先级 60）
- SERVICE 配置 for `test-service`（优先级 70）
- ROUTE 配置 for `test-route`（优先级 80）
- CONSUMER 配置 for `premium-user`（优先级 100）

**预期优先级顺序**:
```
CONSUMER (100) > ROUTE (80) > SERVICE (70) > DOMAIN (60) > GLOBAL (1)
```

**测试场景**:

1. **测试 CONSUMER 最高优先级**:
   ```bash
   # 即使同时匹配 route、service、domain，也应使用 CONSUMER 配置
   curl -H "Authorization: Bearer premium-user-token" \
        -H "Host: example.com" \
        http://your-gateway/test-route-path
   # 预期: Matched Scope Type: CONSUMER, priority: 100
   ```

2. **测试 ROUTE 第二优先级**:
   ```bash
   # 匹配 route、service、domain，但无 consumer
   curl -H "Host: example.com" http://your-gateway/test-route-path
   # 预期: Matched Scope Type: ROUTE, priority: 80
   ```

3. **测试 SERVICE 第三优先级**:
   ```bash
   # 匹配 service、domain，但无 consumer 和 route
   curl -H "Host: example.com" http://your-gateway/service-path
   # 预期: Matched Scope Type: SERVICE, priority: 70
   ```

4. **测试 DOMAIN 第四优先级**:
   ```bash
   # 只匹配 domain
   curl -H "Host: example.com" http://your-gateway/other-path
   # 预期: Matched Scope Type: DOMAIN, priority: 60
   ```

5. **测试 GLOBAL 最低优先级**:
   ```bash
   # 不匹配任何规则
   curl -H "Host: other-domain.com" http://your-gateway/other-path
   # 预期: Matched Scope Type: GLOBAL, priority: 1
   ```

---

### 05-consumer-vs-route.yaml
**测试目标**: 重点验证 CONSUMER 优先于 ROUTE

**配置内容**:
- GLOBAL 配置（优先级 1）
- ROUTE 配置 for `test-route`（优先级 80）
- CONSUMER 配置 for `premium-user`（优先级 100）

**测试场景**:

1. **同时匹配 CONSUMER 和 ROUTE**:
   ```bash
   curl -H "Authorization: Bearer premium-user-token" \
        http://your-gateway/test-route-path
   # 预期: Matched Scope Type: CONSUMER (CONSUMER 胜出)
   ```

2. **只匹配 ROUTE**:
   ```bash
   curl http://your-gateway/test-route-path
   # 预期: Matched Scope Type: ROUTE
   ```

3. **都不匹配**:
   ```bash
   curl http://your-gateway/other-path
   # 预期: Matched Scope Type: GLOBAL
   ```

---

## 如何查看日志

部署配置后，查看插件日志以验证作用域优先级：

```bash
# 查看 Higress Gateway 日志
kubectl logs -n higress-system -l app=higress-gateway -f | grep "Scope Priority Test"
```

**日志输出示例**:
```
[INFO] === Scope Priority Test - Request Headers ===
[INFO] Matched Scope Type: CONSUMER
[INFO] Config Message: [TEST-04] Consumer config - PRIORITY 100 - HIGHEST
[INFO] Config Priority: 100
[INFO] Request Method: GET
[INFO] Request Path: /api/test
[INFO] Request Host: example.com
[INFO] Consumer Name: premium-user
[INFO] Route Name: test-route
[INFO] ===========================================
```

## 部署步骤

1. **构建插件镜像**:
   ```bash
   cd plugins/wasm-go/extensions/scope-priority-test
   make build
   make push
   ```

2. **更新配置中的镜像 URL**:
   编辑 YAML 文件，将 `url: oci://your-registry/scope-priority-test:1.0.0` 替换为实际的镜像地址

3. **应用配置**:
   ```bash
   kubectl apply -f test-scenarios/01-global-only.yaml
   ```

4. **发送测试请求并查看日志**:
   ```bash
   # 终端 1: 查看日志
   kubectl logs -n higress-system -l app=higress-gateway -f | grep "Scope Priority Test"
   
   # 终端 2: 发送请求
   curl http://your-gateway/api/test
   ```

## 注意事项

1. **Consumer 认证**: 要测试 CONSUMER 作用域，需要先配置认证插件（如 key-auth、jwt-auth）来设置 `consumer_name` 属性

2. **Route 名称**: `ingress` 字段中的路由名称需要与实际的 Ingress 资源名称匹配

3. **Service 名称**: `service` 字段中的服务名称需要与实际的 Kubernetes Service 名称匹配

4. **Domain 匹配**: `domain` 字段支持通配符，如 `*.example.com`

5. **日志级别**: 确保 Higress Gateway 的日志级别设置为 INFO 或更详细，才能看到插件的日志输出

## 清理

删除测试配置：
```bash
kubectl delete wasmplugin -n higress-system scope-priority-test-global-only
kubectl delete wasmplugin -n higress-system scope-priority-test-consumer-priority
kubectl delete wasmplugin -n higress-system scope-priority-test-route-priority
kubectl delete wasmplugin -n higress-system scope-priority-test-full-chain
kubectl delete wasmplugin -n higress-system scope-priority-test-consumer-vs-route
```
