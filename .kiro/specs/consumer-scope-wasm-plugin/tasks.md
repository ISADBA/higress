# 实施计划: Consumer 级别 WASM 插件作用域

## 概述

将设计转换为一系列代码生成任务，实现 consumer 级别的 WASM 插件作用域支持。每个任务都基于前一个任务构建，最终将所有组件集成在一起。专注于涉及编写、修改或测试代码的任务。

## 任务

- [x] 1. 扩展 Protobuf 定义
  - 在 `api/extensions/v1alpha1/wasmplugin.proto` 中的 `MatchRule` 消息添加 `consumer` 字段
  - 重新生成 protobuf Go 代码
  - _需求: 5.1_

- [x] 2. 扩展 API 管理层作用域支持
  - [x] 2.1 更新作用域常量定义
    - 在 `plugins/golang-filter/mcp-server/servers/higress/higress-api/tools/plugins/util.go` 中添加 `ScopeConsumer` 常量
    - 更新 `ValidScopes` 数组包含新的 consumer 作用域
    - 扩展 `BuildPluginPath` 函数支持 consumer 路径构建
    - _需求: 1.1, 3.1_

  - [ ]* 2.2 编写作用域验证属性测试
    - **属性 1: Consumer 作用域有效性**
    - **验证需求: 需求 1.1**

- [x] 3. 扩展通用插件管理工具
  - [x] 3.1 更新插件实例列表处理
    - 在 `plugins/golang-filter/mcp-server/servers/higress/higress-api/tools/plugins/common.go` 中扩展 `handleListPluginInstances` 函数
    - 添加 consumer 作用域的 API 路径处理
    - _需求: 3.5_

  - [x] 3.2 更新插件配置 CRUD 操作
    - 扩展 `handleGetPluginConfig`、`handleDeletePluginConfig` 函数支持 consumer 作用域
    - 更新 JSON schema 包含 consumer 作用域选项
    - _需求: 3.1, 3.2, 3.4_

  - [ ]* 3.3 编写 API 操作属性测试
    - **属性 6: API 操作的完整性**
    - **验证需求: 需求 3.1, 3.2, 3.3, 3.4, 3.5**

- [x] 4. 检查点 - 确保所有测试通过
  - 确保所有测试通过，如有问题请询问用户。

- [x] 5. 扩展运行时匹配引擎
  - [x] 5.1 扩展匹配类别和常量
    - 在 `plugins/wasm-go/pkg/matcher/rule_matcher.go` 中添加 `Consumer` 类别
    - 添加 `MATCH_CONSUMER_KEY` 常量
    - _需求: 1.3, 2.1_

  - [x] 5.2 扩展 RuleConfig 结构
    - 在 `RuleConfig` 结构中添加 `consumers` 字段
    - 实现 `parseConsumerMatchConfig` 方法
    - _需求: 1.3_

  - [x] 5.3 实现 Consumer 优先匹配逻辑
    - 修改 `GetMatchConfig` 方法实现 consumer 规则优先匹配
    - 确保在存在 consumer 标识时优先匹配 consumer 规则
    - 保持原有匹配顺序的向后兼容性
    - _需求: 1.4, 2.1, 2.2, 2.3, 2.4_

  - [ ]* 5.4 编写 Consumer 匹配优先级属性测试
    - **属性 3: Consumer 规则优先匹配**
    - **验证需求: 需求 1.4, 2.1, 2.2, 2.3**

  - [ ]* 5.5 编写向后兼容性属性测试
    - **属性 4: 规则匹配顺序保持**
    - **验证需求: 需求 2.4**

- [x] 6. 实现 Consumer 识别机制
  - [x] 6.1 扩展 Consumer 信息提取
    - 在 `plugins/wasm-go/pkg/wrapper/plugin_wrapper.go` 中实现 consumer 信息提取逻辑
    - 支持从 JWT、API Key、自定义 Header 中提取 consumer 信息
    - _需求: 4.1_

  - [x] 6.2 实现 Consumer 信息传递和上下文管理
    - 实现 consumer 信息在插件执行过程中的传递机制
    - 正确使用 userContext 和 userAttribute 进行上下文隔离
    - _需求: 4.1_

  - [ ]* 6.3 编写 Consumer 识别鲁棒性属性测试
    - **属性 5: Consumer 识别的鲁棒性**
    - **验证需求: 需求 4.1, 4.2, 4.3**

- [x] 7. 实现配置存储和序列化
  - [x] 7.1 实现 Consumer 配置存储逻辑
    - 确保 consumer 配置能够正确存储和检索
    - 实现配置与 consumer 标识的关联
    - _需求: 1.2_

  - [ ]* 7.2 编写配置存储属性测试
    - **属性 2: Consumer 配置存储往返一致性**
    - **验证需求: 需求 1.2, 3.1, 3.2**

  - [ ]* 7.3 编写 Protobuf 序列化属性测试
    - **属性 7: Protobuf 序列化往返一致性**
    - **验证需求: 需求 5.1, 5.3**

- [x] 8. 检查点 - 确保所有测试通过
  - 确保所有测试通过，如有问题请询问用户。

- [x] 9. 集成和兼容性验证
  - [x] 9.1 验证与现有认证插件的集成
    - 测试与 basic-auth、key-auth、jwt-auth 插件的集成
    - 确保认证插件能够正确设置 consumer 信息
    - _需求: 4.1_

  - [x] 9.2 实现向后兼容性保证
    - 确保现有配置和插件不受影响
    - 验证没有 consumer 标识时的原有行为
    - _需求: 5.2_

  - [ ]* 9.3 编写向后兼容性属性测试
    - **属性 8: 向后兼容性保持**
    - **验证需求: 需求 5.2**

- [x] 10. 最终集成和验证
  - [x] 10.1 集成所有组件
    - 确保 API 管理层、运行时匹配引擎、consumer 识别机制协同工作
    - 验证完整的端到端流程
    - _需求: 1.1, 1.2, 1.3, 1.4_

  - [x] 10.2 创建完整的配置示例
    - 创建包含 consumer 规则的完整 WasmPlugin 配置示例
    - 验证配置的正确性和功能性
    - _需求: 7.1, 7.2, 7.3, 7.4_

- [x] 11. 最终检查点 - 确保所有测试通过
  - 确保所有测试通过，如有问题请询问用户。

## 注意事项

- 标记为 `*` 的任务是可选的，可以跳过以实现更快的 MVP
- 每个任务都引用了具体的需求以便追溯
- 检查点确保增量验证
- 属性测试验证通用正确性属性
- 单元测试验证特定示例和边界情况