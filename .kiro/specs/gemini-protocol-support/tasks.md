# 实现计划：Gemini 协议支持

## 概述

本实现计划将为 ai-header-modifier 插件添加 Google Gemini 原生 API 协议支持。实现将在现有插件基础上添加 Gemini 协议检测和处理逻辑，包括从 URL 路径提取模型名称、从查询参数提取 API 密钥和提供商信息，以及修改请求体添加 model 属性。所有新增功能将保持与现有功能的完全向后兼容性。

## 任务

- [x] 1. 实现 Gemini 协议检测和 URL 解析函数
  - [x] 1.1 实现 `isGeminiProtocol` 函数
    - 检测请求路径是否以 `/v1/models/` 开头
    - 正确处理带查询参数的路径
    - _需求：1.1, 1.2_
  
  - [ ]* 1.2 为 `isGeminiProtocol` 编写基于属性的测试
    - **属性 1：Gemini 协议检测**
    - **验证需求：1.1**
  
  - [x] 1.3 实现 `extractModelFromPath` 函数
    - 从路径中提取模型名称（支持 `:operation` 和 `/operation` 格式）
    - 正确处理查询参数
    - 处理无效路径格式的容错
    - _需求：2.1, 2.3_
  
  - [ ]* 1.4 为 `extractModelFromPath` 编写基于属性的测试
    - **属性 2：模型名称提取**
    - **验证需求：2.1**
  
  - [x] 1.5 实现 `parseQueryParams` 函数
    - 解析查询字符串为键值对映射
    - 处理 URL 编码字符（简单的 `+` 到空格转换）
    - 处理空查询字符串和无效格式
    - _需求：6.1, 6.2, 6.3_
  
  - [ ]* 1.6 为 `parseQueryParams` 编写基于属性的测试
    - **属性 10：URL 解析正确性**
    - **验证需求：6.1, 6.2**

- [x] 2. 实现 Gemini 协议请求头处理
  - [x] 2.1 实现 `processGeminiProtocol` 函数
    - 调用 `extractModelFromPath` 提取模型名称
    - 调用 `parseQueryParams` 解析查询参数
    - 从查询参数提取 API 密钥（`key` 参数）
    - 设置 `x-mse-consumer-apikey` 请求头为提取的 API 密钥
    - 检查 `x-api-key` 请求头是否存在或为空
    - 如果 `x-api-key` 不存在或为空，设置 `x-api-key` 请求头为提取的 API 密钥
    - 从查询参数或配置提取提供商（`provider` 参数或 `defaultProvider`）
    - 设置 `x-higress-llm-model` 请求头
    - 设置 `x-request-llm-provider` 请求头
    - 添加适当的日志记录（Debug 和 Warn 级别）
    - 处理提取失败和参数缺失的容错
    - _需求：2.1, 2.2, 2.3, 3.1, 3.2, 3.3, 3.4, 3.5, 4.1, 4.2, 4.3, 8.1, 8.2, 8.3, 8.4_
  
  - [ ]* 2.2 为 `processGeminiProtocol` 编写基于属性的测试
    - **属性 3：提取信息到请求头的映射**
    - **验证需求：2.2, 3.2, 4.1, 4.2**
  
  - [ ]* 2.3 为 `processGeminiProtocol` 编写单元测试
    - 测试完整的 Gemini API 请求示例（需求 9）
    - 测试 API 密钥同时设置到 x-mse-consumer-apikey 和 x-api-key
    - 测试当 x-api-key 已存在时不覆盖
    - 测试 API 密钥缺失的容错性
    - 测试模型名称提取失败的容错性
    - 测试提供商参数的各种组合
    - _需求：9.1, 9.2, 9.3, 9.4, 9.5_

- [x] 3. 实现 Gemini 协议请求体处理
  - [x] 3.1 实现 `addModelToBody` 函数
    - 验证请求体是否为有效 JSON
    - 检查是否已存在 `model` 属性
    - 根据提供商格式化 model 值（`default` 或 `{provider}/{model}`）
    - 使用 `sjson.SetBytes` 添加 model 属性
    - 添加适当的日志记录
    - 处理无效 JSON 的容错
    - _需求：5.1, 5.2, 5.3, 5.4, 5.5, 8.5_
  
  - [ ]* 3.2 为 `addModelToBody` 编写基于属性的测试
    - **属性 7：请求体 model 属性添加**
    - **属性 8：model 属性格式化**
    - **验证需求：5.1, 5.2, 5.3, 5.4**
  
  - [ ]* 3.3 为 `addModelToBody` 编写单元测试
    - 测试添加 model 属性（provider = "default"）
    - 测试添加 model 属性（provider != "default"）
    - 测试已存在 model 属性时不修改
    - 测试无效 JSON 时不修改
    - _需求：5.1, 5.2, 5.3, 5.4, 5.5_

- [x] 4. 检查点 - 确保所有辅助函数测试通过
  - 确保所有测试通过，如有问题请询问用户

- [x] 5. 修改 `onHttpRequestHeaders` 函数集成 Gemini 协议检测
  - [x] 5.1 在现有逻辑中添加 Gemini 协议检测分支
    - 在自定义请求头处理后添加 Gemini 协议检测
    - 调用 `isGeminiProtocol` 检测协议
    - 如果是 Gemini 协议，调用 `processGeminiProtocol`
    - 检查是否有请求体
    - 设置上下文标记 `isGemini` 为 true
    - 设置 mode 为 `ModeJSON`
    - 移除 content-length 请求头
    - 返回 `types.HeaderStopIteration`
    - 如果不是 Gemini 协议，继续现有的路径后缀检查逻辑
    - _需求：1.1, 1.2, 7.1, 7.2_
  
  - [ ]* 5.2 为 `onHttpRequestHeaders` 编写集成测试
    - 测试 Gemini 协议请求的完整处理流程
    - 测试非 Gemini 协议请求使用现有逻辑
    - 测试自定义请求头在 Gemini 请求中仍然生效
    - _需求：7.1, 7.2, 7.3_

- [x] 6. 修改 `onHttpRequestBody` 函数集成 Gemini 请求体处理
  - [x] 6.1 在现有逻辑中添加 Gemini 请求体处理分支
    - 从上下文检查 `isGemini` 标记
    - 如果是 Gemini 协议，从请求头获取模型名称和提供商
    - 调用 `addModelToBody` 修改请求体
    - 使用 `proxywasm.ReplaceHttpRequestBody` 替换请求体
    - 如果不是 Gemini 协议，继续现有的 JSON/Multipart 处理逻辑
    - _需求：5.1, 5.2, 5.3, 5.4, 5.5, 7.1_
  
  - [ ]* 6.2 为 `onHttpRequestBody` 编写集成测试
    - 测试 Gemini 协议请求体修改
    - 测试非 Gemini 协议请求体处理不受影响
    - _需求：5.1, 7.1_

- [x] 7. 检查点 - 确保所有集成测试通过
  - 确保所有测试通过，如有问题请询问用户

- [x] 8. 编写端到端测试和容错测试
  - [ ]* 8.1 编写端到端测试
    - 测试完整的 Gemini API 请求处理流程（需求 9）
    - 验证所有请求头和请求体修改
    - _需求：9.1, 9.2, 9.3, 9.4_
  
  - [ ]* 8.2 编写容错性基于属性的测试
    - **属性 4：提取失败的容错性**
    - **属性 5：API 密钥缺失的容错性**
    - **属性 9：无效 JSON 的容错性**
    - **验证需求：2.3, 3.3, 5.5**
  
  - [ ]* 8.3 编写向后兼容性测试
    - **属性 11：自定义请求头处理的独立性**
    - 测试非 Gemini 请求使用现有逻辑
    - 测试自定义请求头配置在 Gemini 请求中正常工作
    - _需求：7.1, 7.2, 7.3_

- [x] 9. 更新文档
  - [x] 9.1 更新 README.md
    - 添加 Gemini 协议支持说明
    - 添加配置示例
    - 添加使用示例（包括 Gemini API 请求格式）
  
  - [x] 9.2 创建设计文档目录和文件
    - 根据插件开发标准，创建 `design/` 目录
    - 将 `.kiro/specs/gemini-protocol-support/design.md` 复制到 `plugins/wasm-go/extensions/ai-header-modifier/design/gemini-protocol-support.md`
    - 将 `.kiro/specs/gemini-protocol-support/requirements.md` 复制到 `plugins/wasm-go/extensions/ai-header-modifier/design/gemini-requirements.md`

- [x] 10. 最终检查点 - 确保所有测试通过
  - 运行所有测试确保功能正常
  - 确保代码覆盖率达到目标（>= 85%）
  - 如有问题请询问用户

## 注意事项

- 标记 `*` 的任务为可选任务，可以跳过以加快 MVP 开发
- 每个任务都引用了具体的需求以确保可追溯性
- 检查点任务确保增量验证
- 基于属性的测试验证通用正确性属性
- 单元测试验证特定示例和边缘情况
- 所有新增功能保持与现有功能的完全向后兼容性
