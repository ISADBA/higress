# 设计文档

## 概述

本设计文档描述了为 Higress WASM 插件系统新增 consumer 级别作用域支持的技术实现方案。该功能将允许 WASM 插件在 consumer 级别进行配置和生效，优先级高于 ROUTER 级别，实现更细粒度的流量控制。

## 架构

### 当前架构分析

当前 Higress WASM 插件系统具有两层作用域架构：

#### API 管理层作用域
用于构建 REST API 路径的字符串常量：
- **GLOBAL**: 全局作用域 (`/v1/global/plugin-instances`)
- **DOMAIN**: 域名作用域 (`/v1/domains/{domain}/plugin-instances`)
- **SERVICE**: 服务作用域 (`/v1/services/{service}/plugin-instances`)
- **ROUTE**: 路由作用域 (`/v1/routes/{route}/plugin-instances`)

#### 运行时匹配引擎类别
用于实际执行配置匹配的枚举：
- **Route**: 路由匹配
- **Host**: 主机/域名匹配
- **Service**: 服务匹配
- **RoutePrefix**: 路由前缀匹配

#### 匹配优先级机制
- 规则按定义顺序匹配，第一个匹配成功的规则生效
- 若所有规则都不匹配，则使用全局配置
- 若没有全局配置，返回 nil 表示不启用插件

#### 上下文隔离机制
- **userContext**: 内部状态存储，用于插件内部逻辑
- **userAttribute**: 可观测性数据存储，用于日志和监控

### 新架构设计

#### API 管理层扩展
新增 **CONSUMER** 作用域字符串常量，用于构建 consumer 相关的 API 路径：
```
/v1/consumers/{consumer}/plugin-instances
```

#### 运行时匹配引擎扩展
新增 **Consumer** 匹配类别，用于实际的 consumer 配置匹配。

#### 新的匹配逻辑
Consumer 匹配将作为最高优先级的匹配规则：
1. 首先检查是否存在 consumer 标识
2. 如果存在，优先匹配 consumer 级别的规则
3. 如果 consumer 规则不匹配或不存在，按原有顺序匹配其他规则
4. 最后使用全局配置或返回 nil

## 组件和接口

### 1. Protobuf 定义扩展

#### 1.1 WasmPlugin 扩展
在 `api/extensions/v1alpha1/wasmplugin.proto` 中扩展 `MatchRule` 消息：

```protobuf
message MatchRule {
  repeated string ingress = 1;
  repeated string domain = 2;
  google.protobuf.Struct config = 3;
  google.protobuf.BoolValue config_disable = 4;
  repeated string service = 5;
  RouteType route_type = 6;
  // 新增：consumer 匹配规则
  repeated string consumer = 7;
}
```

### 2. 作用域常量定义

#### 2.1 扩展 API 管理层作用域
在 `plugins/golang-filter/mcp-server/servers/higress/higress-api/tools/plugins/util.go` 中新增：

```go
const (
    ScopeGlobal   = "GLOBAL"
    ScopeDomain   = "DOMAIN"
    ScopeService  = "SERVICE"
    ScopeRoute    = "ROUTE"
    ScopeConsumer = "CONSUMER"  // 新增 API 管理层作用域
)

// 更新有效作用域列表
var ValidScopes = []string{ScopeGlobal, ScopeDomain, ScopeService, ScopeRoute, ScopeConsumer}
```

#### 2.2 扩展 BuildPluginPath 函数
```go
func BuildPluginPath(pluginName, scope, resourceName string) string {
    switch scope {
    case ScopeConsumer:  // 新增
        return fmt.Sprintf("/v1/consumers/%s/plugin-instances/%s", resourceName, pluginName)
    case ScopeGlobal:
        return fmt.Sprintf("/v1/global/plugin-instances/%s", pluginName)
    // ... 其他 case 保持不变
    }
}
```

### 3. Consumer 识别机制

#### 3.1 Consumer 标识提取
Consumer 识别将通过以下方式进行：

1. **JWT Token 方式**: 从 Authorization 头中的 JWT token 提取 consumer 信息
   ```go
   func extractConsumerFromJWT(jwt string) string {
       // 解析 JWT token，提取 consumer_id 或 sub 字段
   }
   ```

2. **API Key 方式**: 从 X-API-Key 头中提取并映射到 consumer
   ```go
   func extractConsumerFromAPIKey(apiKey string) string {
       // 通过 API Key 查找对应的 consumer
   }
   ```

3. **自定义 Header 方式**: 从 X-Consumer-ID 等自定义头中直接获取
   ```go
   func (ctx *CommonHttpCtx[T]) extractConsumer() string {
       return ctx.GetRequestHeader("X-Consumer-ID")
   }
   ```

4. **认证插件集成**: 从认证插件的结果中获取 consumer 信息
   - Basic Auth 插件认证成功后设置 consumer 信息
   - Key Auth 插件认证成功后设置 consumer 信息
   - JWT Auth 插件认证成功后设置 consumer 信息

#### 3.2 Consumer 信息传递和上下文管理
在插件执行过程中，consumer 信息通过以下方式传递和管理：

```go
// Consumer 信息传递
const ConsumerContextKey = "consumer_name"

// 认证插件设置 consumer 信息
func authenticated(consumerName string) types.Action {
    proxywasm.SetProperty([]string{"consumer_name"}, []byte(consumerName))
    return types.ActionContinue
}

// 上下文隔离使用
func (ctx *CommonHttpCtx[T]) storeConsumerInfo(consumerName string) {
    // 内部状态存储（用于插件逻辑）
    ctx.SetContext("consumer_name", consumerName)
    
    // 可观测性数据存储（用于日志和监控）
    ctx.SetUserAttribute("consumer_name", consumerName)
}
```

### 4. RuleMatcher 扩展

#### 4.1 新增 Consumer 匹配类别
在 `plugins/wasm-go/pkg/matcher/rule_matcher.go` 中扩展运行时匹配引擎：

```go
type Category int

const (
    Route Category = iota
    Host
    Service
    RoutePrefix
    Consumer  // 新增运行时匹配类别
)

const (
    // 现有常量...
    MATCH_CONSUMER_KEY = "_match_consumer_"  // 新增
)
```

#### 4.2 扩展 RuleConfig 结构
```go
type RuleConfig[PluginConfig any] struct {
    category     Category
    routes       map[string]struct{}
    services     map[string]struct{}
    routePrefixs map[string]struct{}
    hosts        []HostMatcher
    consumers    map[string]struct{}  // 新增
    config       PluginConfig
}
```

#### 4.3 扩展 GetMatchConfig 方法（实现 Consumer 优先匹配）
```go
func (m RuleMatcher[PluginConfig]) GetMatchConfig() (*PluginConfig, error) {
    // 获取 consumer 信息
    consumerName, err := proxywasm.GetProperty([]string{"consumer_name"})
    if err != nil && err != types.ErrorStatusNotFound {
        return nil, err
    }
    
    // 获取其他匹配信息...
    host, _ := proxywasm.GetHttpRequestHeader(":authority")
    routeName, _ := proxywasm.GetProperty([]string{"route_name"})
    serviceName, _ := proxywasm.GetProperty([]string{"cluster_name"})
    
    // 按规则定义顺序匹配，Consumer 规则优先
    for _, rule := range m.ruleConfig {
        // 1. Consumer 匹配（如果存在 consumer 标识）
        if rule.category == Consumer && string(consumerName) != "" {
            if _, ok := rule.consumers[string(consumerName)]; ok {
                return &rule.config, nil
            }
        }
        
        // 2. Route 匹配
        if rule.category == Route {
            if _, ok := rule.routes[string(routeName)]; ok {
                return &rule.config, nil
            }
        }
        
        // 3. Service 匹配
        if m.serviceMatch(rule, string(serviceName)) {
            return &rule.config, nil
        }
        
        // 4. Host/Domain 匹配
        if rule.category == Host {
            if m.hostMatch(rule, host) {
                return &rule.config, nil
            }
        }
        
        // 5. RoutePrefix 匹配
        if rule.category == RoutePrefix {
            for routePrefix := range rule.routePrefixs {
                if strings.HasPrefix(string(routeName), routePrefix) {
                    return &rule.config, nil
                }
            }
        }
    }
    
    // 6. 返回全局配置或 nil
    if m.hasGlobalConfig {
        return &m.globalConfig, nil
    }
    return nil, nil
}
```
        // 1. 优先匹配 Consumer（最高优先级）
        if rule.category == Consumer && string(consumerName) != "" {
            if _, ok := rule.consumers[string(consumerName)]; ok {
                return &rule.config, nil
            }
        }
        
        // 2. 匹配 Route
        if rule.category == Route {
            if _, ok := rule.routes[string(routeName)]; ok {
                return &rule.config, nil
            }
        }
        
        // 3. 匹配 Service
        if m.serviceMatch(rule, string(serviceName)) {
            return &rule.config, nil
        }
        
        // 4. 匹配 Host/Domain
        if rule.category == Host {
            if m.hostMatch(rule, host) {
                return &rule.config, nil
            }
        }
        
        // 5. 匹配 RoutePrefix
        if rule.category == RoutePrefix {
            for routePrefix := range rule.routePrefixs {
                if strings.HasPrefix(string(routeName), routePrefix) {
                    return &rule.config, nil
                }
            }
        }
    }
    
    // 6. 返回全局配置（最低优先级）
    if m.hasGlobalConfig {
        return &m.globalConfig, nil
    }
    return nil, nil
}
```

### 5. API 路径扩展

#### 5.1 新增 Consumer 相关 API 端点
```
GET    /v1/consumers/{consumer_name}/plugin-instances           # 列出 consumer 的所有插件实例
GET    /v1/consumers/{consumer_name}/plugin-instances/{plugin}  # 获取 consumer 的特定插件配置
POST   /v1/consumers/{consumer_name}/plugin-instances/{plugin}  # 创建 consumer 的插件配置
PUT    /v1/consumers/{consumer_name}/plugin-instances/{plugin}  # 更新 consumer 的插件配置
DELETE /v1/consumers/{consumer_name}/plugin-instances/{plugin}  # 删除 consumer 的插件配置
```

#### 5.2 扩展通用插件工具
在 `plugins/golang-filter/mcp-server/servers/higress/higress-api/tools/plugins/common.go` 中更新：

```go
func handleListPluginInstances(client *higress.HigressClient) common.ToolHandlerFunc {
    return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
        // ... 现有逻辑
        
        var path string
        switch scope {
        case ScopeConsumer:  // 新增
            path = fmt.Sprintf("/v1/consumers/%s/plugin-instances", resourceName)
        case ScopeGlobal:
            path = "/v1/global/plugin-instances"
        // ... 其他 case
        }
        
        // ... 其余逻辑
    }
}
```

## 数据模型

### 1. Consumer 配置存储
Consumer 级别的插件配置将存储在与其他作用域相同的存储后端中，通过作用域标识进行区分。

```yaml
# 配置示例
apiVersion: extensions.higress.io/v1alpha1
kind: WasmPlugin
metadata:
  name: consumer-rate-limit
spec:
  phase: AUTHN
  priority: 800  # 比 ROUTE 级别插件优先级高
  matchRules:
  - consumer:
    - "premium-user-123"
    - "enterprise-user-456"
    config:
      rateLimit:
        requests: 1000
        window: 60s
  - consumer:
    - "basic-user-789"
    config:
      rateLimit:
        requests: 100
        window: 60s
```

### 2. 配置匹配模型
```
匹配逻辑: 按规则定义顺序匹配，第一个匹配成功的规则生效
Consumer 优先: 当存在 consumer 标识时，consumer 规则优先匹配
回退机制: 
1. 检查是否存在 consumer 标识
2. 如果存在，优先匹配 consumer 规则
3. 如果 consumer 规则不匹配，继续按顺序匹配其他规则
4. 若所有规则都不匹配，使用全局配置
5. 若没有全局配置，返回 nil（不启用插件）
```

### 3. Consumer 标识模型
```go
type ConsumerInfo struct {
    Name     string            // Consumer 名称
    Source   string            // 标识来源 (jwt, api_key, header, auth_plugin)
    Metadata map[string]string // 额外元数据
}
```

## 错误处理

### 1. Consumer 识别失败
- 当 consumer 标识解析失败时，记录警告日志并继续按原有优先级处理
- 不影响现有功能的正常运行

### 2. Consumer 配置不存在
- 当请求匹配到 consumer 但该 consumer 没有对应插件配置时，继续按原有优先级查找其他级别的配置

### 3. API 错误处理
- Consumer 不存在时返回 404 错误
- 配置格式错误时返回 400 错误
- 权限不足时返回 403 错误

## 正确性属性

*属性是一个特征或行为，应该在系统的所有有效执行中保持为真——本质上是关于系统应该做什么的正式声明。属性作为人类可读规范和机器可验证正确性保证之间的桥梁。*

### 属性 1: Consumer 作用域有效性
*对于任何* 作用域验证请求，当作用域为 "CONSUMER" 时，系统应当将其识别为有效作用域
**验证需求: 需求 1.1**

### 属性 2: Consumer 配置存储往返一致性
*对于任何* 有效的 consumer 名称和插件配置，创建配置后立即查询应当返回相同的配置内容
**验证需求: 需求 1.2, 3.1, 3.2**

### 属性 3: Consumer 规则优先匹配
*对于任何* 请求，当存在 consumer 标识且存在匹配的 consumer 规则时，系统应当优先选择 consumer 规则的配置，而不是其他类型的规则
**验证需求: 需求 1.4, 2.1, 2.2, 2.3**

### 属性 4: 规则匹配顺序保持
*对于任何* 不包含 consumer 标识或 consumer 规则不匹配的请求，系统应当按照原有的规则定义顺序进行匹配
**验证需求: 需求 2.4**

### 属性 5: Consumer 识别的鲁棒性
*对于任何* 请求，无论 consumer 标识是否存在、有效或无效，系统都应当能够正常处理并返回适当的配置，不会因 consumer 识别失败而中断
**验证需求: 需求 4.1, 4.2, 4.3**

### 属性 6: API 操作的完整性
*对于任何* 有效的 consumer 名称和插件名称，API 的创建、查询、更新、删除操作应当能够正确执行，且删除后的配置不应再被查询到
**验证需求: 需求 3.1, 3.2, 3.3, 3.4, 3.5**

### 属性 7: Protobuf 序列化往返一致性
*对于任何* 包含 consumer 配置的 WasmPlugin 对象，序列化后再反序列化应当产生等价的对象
**验证需求: 需求 5.1, 5.3**

### 属性 8: 向后兼容性保持
*对于任何* 现有的匹配规则和配置，添加 consumer 功能后应当不影响其原有的匹配和执行行为
**验证需求: 需求 5.2**

### 属性测试配置
- 每个属性测试最少 100 次迭代（由于随机化）
- 每个属性测试必须引用其设计文档属性
- 标签格式: **Feature: consumer-scope-wasm-plugin, Property {number}: {property_text}**

### 属性测试配置
- 每个属性测试最少 100 次迭代（由于随机化）
- 每个属性测试必须引用其设计文档属性
- 标签格式: **Feature: consumer-scope-wasm-plugin, Property {number}: {property_text}**