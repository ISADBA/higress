# 实施计划: 作用域优先级测试插件

## 概述

基于 hello-world 插件创建作用域优先级测试插件，实现完整的配置解析、HTTP 头处理和 HTTP Body 处理功能。每个任务都基于前一个任务构建，最终将所有组件集成在一起。

## 任务

- [x] 1. 创建插件目录结构和基础文件
  - 复制 hello-world 插件作为基础
  - 重命名为 scope-priority-test
  - 更新 go.mod 文件中的模块路径
  - _需求: 7.1_

- [ ] 2. 定义配置结构
  - [ ] 2.1 创建 ScopePriorityTestConfig 结构体
    - 添加 ScopeType 字段（string）
    - 添加 Message 字段（string）
    - 添加 Priority 字段（int）
    - 添加 EnableRequestHeaders 字段（bool）
    - 添加 EnableResponseHeaders 字段（bool）
    - 添加 EnableRequestBody 字段（bool）
    - 添加 EnableResponseBody 字段（bool）
    - 添加 JSON 标签
    - _需求: 6.1, 6.2, 6.3, 6.4, 6.5, 6.6_

  - [ ]* 2.2 编写配置结构单元测试
    - 测试结构体字段的 JSON 序列化和反序列化
    - 测试默认值处理
    - _需求: 6.7_

- [ ] 3. 实现配置解析函数
  - [ ] 3.1 实现 parseConfig 函数
    - 解析 scopeType 字段，默认值为 "UNKNOWN"
    - 解析 message 字段
    - 解析 priority 字段
    - 解析所有 enable 开关，默认值为 true
    - 添加错误处理逻辑
    - _需求: 1.1, 1.2, 1.3, 1.4, 6.7_

  - [ ]* 3.2 编写配置解析单元测试
    - 测试完整配置的解析
    - 测试部分字段缺失时的默认值
    - 测试无效配置的错误处理
    - _需求: 1.4_

  - [ ]* 3.3 编写配置解析属性测试
    - **属性 1: 配置解析正确性**
    - **验证需求: 需求 1.1, 1.2, 1.3**

- [ ] 4. 实现辅助函数
  - [ ] 4.1 实现 printContextInfo 函数
    - 获取并打印 consumer_name 属性
    - 获取并打印 route_name 属性
    - 获取并打印 cluster_name 属性
    - 获取并打印 x-request-id 头
    - 添加错误处理，失败时记录警告但继续执行
    - _需求: 3.4, 3.5, 5.5_

  - [ ]* 4.2 编写辅助函数单元测试
    - 测试各种属性存在和不存在的情况
    - 测试错误处理逻辑
    - _需求: 5.5_

- [ ] 5. 实现 HTTP 请求头处理函数
  - [ ] 5.1 实现 onHttpRequestHeaders 函数
    - 打印配置信息（scopeType、message、priority）
    - 检查 enableRequestHeaders 开关
    - 使用 ctx.Method()、ctx.Path()、ctx.Host()、ctx.Scheme() 获取伪头部
    - 使用 proxywasm.GetHttpRequestHeaders() 获取所有请求头
    - 打印所有请求头信息
    - 调用 printContextInfo() 打印上下文信息
    - 使用结构化的日志格式（带分隔线）
    - _需求: 3.1, 3.2, 3.3, 3.4, 3.5, 5.1, 5.2, 5.3, 5.4, 5.6_

  - [ ]* 5.2 编写请求头处理单元测试
    - 测试 enableRequestHeaders 开关的作用
    - 测试日志输出格式
    - _需求: 3.2, 3.3_

  - [ ]* 5.3 编写元数据打印属性测试
    - **属性 3: 元数据打印完整性**
    - **验证需求: 需求 3.2, 3.3, 3.7, 5.4**

- [ ] 6. 实现 HTTP 响应头处理函数
  - [ ] 6.1 实现 onHttpResponseHeaders 函数
    - 检查 enableResponseHeaders 开关
    - 打印配置信息（scopeType）
    - 使用 proxywasm.GetHttpResponseHeader(":status") 获取状态码
    - 使用 proxywasm.GetHttpResponseHeaders() 获取所有响应头
    - 打印所有响应头信息
    - 使用结构化的日志格式（带分隔线）
    - _需求: 3.6, 3.7, 5.1, 5.4, 5.6_

  - [ ]* 6.2 编写响应头处理单元测试
    - 测试 enableResponseHeaders 开关的作用
    - 测试日志输出格式
    - _需求: 3.7_

  - [ ]* 6.3 编写控制开关属性测试
    - **属性 4: 控制开关有效性**
    - **验证需求: 需求 6.3, 6.4, 6.5, 6.6**

- [ ] 7. 实现 HTTP 请求体处理函数
  - [ ] 7.1 实现 onHttpRequestBody 函数
    - 检查 enableRequestBody 开关
    - 打印配置信息（scopeType）
    - 打印请求体大小
    - 使用 ctx.IsBinaryRequestBody() 检查是否为二进制内容
    - 如果是文本内容，打印前 500 字节
    - 使用结构化的日志格式（带分隔线）
    - _需求: 4.1, 4.2, 4.3, 4.4, 5.1, 5.6_

  - [ ]* 7.2 编写请求体处理单元测试
    - 测试 enableRequestBody 开关的作用
    - 测试文本和二进制内容的处理
    - 测试大 Body 的截断逻辑
    - _需求: 4.2, 4.3_

- [ ] 8. 实现 HTTP 响应体处理函数
  - [ ] 8.1 实现 onHttpResponseBody 函数
    - 检查 enableResponseBody 开关
    - 打印配置信息（scopeType）
    - 打印响应体大小
    - 使用 ctx.IsBinaryResponseBody() 检查是否为二进制内容
    - 如果是文本内容，打印前 500 字节
    - 使用结构化的日志格式（带分隔线）
    - _需求: 4.5, 4.6, 5.1, 5.6_

  - [ ]* 8.2 编写响应体处理单元测试
    - 测试 enableResponseBody 开关的作用
    - 测试文本和二进制内容的处理
    - 测试大 Body 的截断逻辑
    - _需求: 4.6_

- [ ] 9. 集成所有组件
  - [ ] 9.1 更新 init 函数
    - 使用 wrapper.SetCtx 初始化插件
    - 使用 wrapper.ParseConfig 注册配置解析函数（非废弃 API）
    - 使用 wrapper.ProcessRequestHeaders 注册请求头处理函数（非废弃 API）
    - 使用 wrapper.ProcessResponseHeaders 注册响应头处理函数（非废弃 API）
    - 使用 wrapper.ProcessRequestBody 注册请求体处理函数（非废弃 API）
    - 使用 wrapper.ProcessResponseBody 注册响应体处理函数（非废弃 API）
    - _需求: 7.1, 7.2, 7.3, 7.4, 7.5_

  - [ ] 9.2 验证所有函数签名正确
    - 确保 parseConfig 签名为 ParseConfigFunc[ScopePriorityTestConfig]
    - 确保 onHttpRequestHeaders 签名为 onHttpHeadersFunc[ScopePriorityTestConfig]
    - 确保 onHttpResponseHeaders 签名为 onHttpHeadersFunc[ScopePriorityTestConfig]
    - 确保 onHttpRequestBody 签名为 onHttpBodyFunc[ScopePriorityTestConfig]
    - 确保 onHttpResponseBody 签名为 onHttpBodyFunc[ScopePriorityTestConfig]
    - _需求: 7.1, 7.2, 7.3, 7.4, 7.5_

- [ ] 10. 创建测试配置文件
  - [ ] 10.1 创建完整的配置示例
    - 创建包含 GLOBAL 配置的示例
    - 创建包含 CONSUMER 规则的示例
    - 创建包含 ROUTE 规则的示例
    - 创建包含 DOMAIN 规则的示例
    - 创建包含 SERVICE 规则的示例
    - 保存为 config-example.json
    - _需求: 2.1, 2.2, 2.3, 2.4, 2.5_

  - [ ] 10.2 创建测试场景配置
    - 创建只有 GLOBAL 的配置
    - 创建 GLOBAL + CONSUMER 的配置
    - 创建 GLOBAL + ROUTE 的配置
    - 创建所有作用域的配置
    - 保存为 test-scenarios/ 目录
    - _需求: 2.6_

- [ ] 11. 编写 README 文档
  - [ ] 11.1 创建 README.md
    - 说明插件的用途
    - 说明配置结构和字段含义
    - 提供配置示例
    - 说明如何构建和部署
    - 说明如何查看日志输出
    - 提供测试场景说明
    - _需求: 所有需求_

- [ ] 12. 检查点 - 编译和基础测试
  - 确保代码能够成功编译
  - 确保所有单元测试通过
  - 如有问题请询问用户

- [ ]* 13. 编写集成测试
  - [ ]* 13.1 测试 GLOBAL 作用域
    - 配置只有 GLOBAL 配置
    - 验证 GLOBAL 配置生效
    - _需求: 2.1_

  - [ ]* 13.2 测试 CONSUMER 作用域优先级
    - 配置 GLOBAL + CONSUMER
    - 验证 CONSUMER 配置优先生效
    - _需求: 2.2, 2.6_

  - [ ]* 13.3 测试 ROUTE 作用域优先级
    - 配置 GLOBAL + ROUTE
    - 验证 ROUTE 配置优先生效
    - _需求: 2.3, 2.6_

  - [ ]* 13.4 测试完整优先级顺序
    - 配置所有作用域
    - 验证优先级顺序：CONSUMER > ROUTE > SERVICE > DOMAIN > GLOBAL
    - _需求: 2.6_

  - [ ]* 13.5 编写作用域优先级属性测试
    - **属性 2: 作用域优先级一致性**
    - **验证需求: 需求 2.6**

- [ ]* 14. 编写错误处理测试
  - [ ]* 14.1 测试配置解析错误处理
    - 测试无效 JSON 配置
    - 测试缺失字段的默认值
    - 验证错误不会中断请求处理
    - _需求: 1.4_

  - [ ]* 14.2 测试元数据获取错误处理
    - 测试 Property 不存在的情况
    - 测试 Header 不存在的情况
    - 验证错误不会中断请求处理
    - _需求: 5.5_

  - [ ]* 14.3 编写错误处理属性测试
    - **属性 5: 错误处理鲁棒性**
    - **验证需求: 需求 1.4**

- [ ] 15. 最终检查点 - 完整测试
  - 确保所有测试通过
  - 验证日志输出格式正确
  - 验证所有作用域优先级正确
  - 如有问题请询问用户

## 注意事项

- 标记为 `*` 的任务是可选的，可以跳过以实现更快的 MVP
- 每个任务都引用了具体的需求以便追溯
- 检查点确保增量验证
- 属性测试验证通用正确性属性
- 单元测试验证特定示例和边界情况
- 使用非废弃的 wrapper API（ProcessRequestHeaders 而不是 ProcessRequestHeadersBy）
