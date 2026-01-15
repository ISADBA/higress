# 需求文档

## 介绍

为 Higress WASM 插件系统新增 consumer 级别的作用域支持，使得 WASM 插件可以在 consumer 级别进行配置和生效，优先级高于 ROUTER 级别。这将允许用户在 consumer 级别配置 IP 白名单、限流、配额等 WASM 插件，实现更细粒度的流量控制。

## 术语表

- **Consumer**: 消费者，代表 API 的使用方，通常通过认证信息（如 API Key、JWT 等）进行标识
- **WASM_Plugin**: WebAssembly 插件，用于扩展 Envoy 代理功能的插件
- **Scope**: 作用域，定义插件生效的范围级别
- **Priority**: 优先级，决定多个作用域插件的执行顺序
- **Match_Rule**: 匹配规则，定义插件在特定条件下的配置

## 需求

### 需求 1

**用户故事：** 作为一个 API 网关管理员，我希望能够在 consumer 级别配置 WASM 插件，以便为不同的 API 消费者提供个性化的流量控制策略。

#### 验收标准

1. 当系统定义 consumer 作用域时，系统应当将其添加到有效作用域列表中
2. 当用户为特定 consumer 配置 WASM 插件时，系统应当存储该配置并与 consumer 标识关联
3. 当请求匹配到特定 consumer 时，系统应当应用该 consumer 级别的 WASM 插件配置
4. 当 consumer 级别和其他级别都有插件配置时，系统应当优先执行 consumer 级别的配置

### 需求 2

**用户故事：** 作为一个开发者，我希望 consumer 级别的插件优先级高于 ROUTER 级别，以便实现更精确的流量控制。

#### 验收标准

1. 当同时存在 consumer 级别和 ROUTER 级别的插件配置时，系统应当优先执行 consumer 级别的配置
2. 当同时存在 consumer 级别和 DOMAIN 级别的插件配置时，系统应当优先执行 consumer 级别的配置
3. 当同时存在 consumer 级别和 GLOBAL 级别的插件配置时，系统应当优先执行 consumer 级别的配置
4. 当只存在非 consumer 级别的插件配置时，系统应当按照原有优先级顺序执行（ROUTER > DOMAIN > GLOBAL）

### 需求 3

**用户故事：** 作为一个 API 网关管理员，我希望能够通过 API 管理 consumer 级别的 WASM 插件配置，以便进行动态配置管理。

#### 验收标准

1. 当用户调用创建 consumer 插件配置 API 时，系统应当创建新的 consumer 级别插件配置
2. 当用户调用查询 consumer 插件配置 API 时，系统应当返回指定 consumer 的插件配置信息
3. 当用户调用更新 consumer 插件配置 API 时，系统应当更新指定 consumer 的插件配置
4. 当用户调用删除 consumer 插件配置 API 时，系统应当删除指定 consumer 的插件配置
5. 当用户调用列出 consumer 插件实例 API 时，系统应当返回指定 consumer 的所有插件实例

### 需求 4

**用户故事：** 作为一个开发者，我希望 consumer 识别机制能够正确工作，以便插件能够准确匹配到对应的 consumer。

#### 验收标准

1. 当请求包含有效的 consumer 标识信息时，系统应当正确识别出对应的 consumer
2. 当请求不包含 consumer 标识信息时，系统应当按照原有的作用域优先级处理
3. 当请求包含无效的 consumer 标识信息时，系统应当按照原有的作用域优先级处理
4. 当 consumer 标识信息解析失败时，系统应当记录错误日志并按照原有的作用域优先级处理

### 需求 5

**用户故事：** 作为一个系统集成者，我希望新的 consumer 作用域能够与现有的 protobuf 定义兼容，以便保持 API 的向后兼容性。

#### 验收标准

1. 当扩展 WasmPlugin protobuf 定义时，系统应当保持与现有字段的兼容性
2. 当添加 consumer 相关的匹配规则时，系统应当不影响现有的匹配规则功能
3. 当序列化和反序列化包含 consumer 配置的 protobuf 消息时，系统应当正确处理所有字段
4. 当旧版本客户端访问新版本 API 时，系统应当正常工作（忽略不识别的 consumer 字段）

### 需求 6

**用户故事：** 作为一个运维人员，我希望能够监控和调试 consumer 级别的插件执行情况，以便排查问题和优化性能。

#### 验收标准

1. 当 consumer 级别插件执行时，系统应当记录相应的日志信息
2. 当 consumer 级别插件执行失败时，系统应当记录详细的错误信息
3. 当插件优先级选择发生时，系统应当记录选择的作用域和原因
4. 当 consumer 识别过程发生时，系统应当记录识别结果和过程信息

### 需求 7

**用户故事：** 作为一个 API 网关用户，我希望能够在 consumer 级别配置常见的安全和流控插件，以便实现精细化的访问控制。

#### 验收标准

1. 当在 consumer 级别配置 IP 白名单插件时，系统应当仅对该 consumer 的请求应用 IP 白名单检查
2. 当在 consumer 级别配置限流插件时，系统应当为该 consumer 单独计算和应用限流规则
3. 当在 consumer 级别配置配额插件时，系统应当为该 consumer 单独跟踪和管理配额使用情况
4. 当在 consumer 级别配置认证插件时，系统应当为该 consumer 应用特定的认证规则