# Implementation Plan: Add ApiKey to Cost Request

## Overview

本实施计划将ai-billing插件的计费请求功能扩展，添加从HTTP请求头`x-mse-consumer-apikey`提取apikey并包含在CostRequest中的能力。实施将分为数据结构修改、提取逻辑实现、集成和测试四个主要部分。

## Tasks

- [x] 1. 修改CostRequest数据结构
  - 在`CostRequest`结构体中添加`ApiKey`字段
  - 添加JSON标签`json:"apikey"`
  - _Requirements: 2.1, 2.2_

- [x] 2. 实现apikey提取逻辑
  - [x] 2.1 添加Context Key常量
    - 在常量定义区域添加`CtxKeyConsumerApiKey = "ai-billing-consumer-apikey"`
    - _Requirements: 1.1_
  
  - [x] 2.2 实现extractConsumerApiKey函数
    - 创建函数从`x-mse-consumer-apikey`请求头提取值
    - 如果请求头不存在，返回空字符串
    - 保留原始值，不做任何转换
    - _Requirements: 1.1, 1.2, 1.3, 1.4_
  
  - [ ]* 2.3 编写extractConsumerApiKey的单元测试
    - 测试请求头存在时返回正确值
    - 测试请求头不存在时返回空字符串
    - 测试包含空白字符的值被保留
    - _Requirements: 1.2, 1.3, 1.4_

- [x] 3. 集成apikey提取到请求处理流程
  - [x] 3.1 修改onHttpRequestHeaders函数
    - 调用`extractConsumerApiKey()`提取apikey
    - 使用`ctx.SetContext(CtxKeyConsumerApiKey, consumerApiKey)`存储到context
    - 添加debug日志记录提取的apikey（使用maskApiKey脱敏）
    - _Requirements: 1.1, 1.2_
  
  - [ ]* 3.2 编写属性测试：ApiKey提取正确性
    - **Property 1: ApiKey提取正确性**
    - **Validates: Requirements 1.1, 1.2**
    - 生成随机HTTP请求头，验证提取逻辑正确性
    - _Requirements: 1.1, 1.2_

- [x] 4. 修改deductCost函数
  - [x] 4.1 从context获取apikey
    - 使用`ctx.GetContext(CtxKeyConsumerApiKey)`获取apikey值
    - 处理类型断言，默认为空字符串
    - _Requirements: 2.3_
  
  - [x] 4.2 在CostRequest构造中包含apikey
    - 在构建`requestBody`时添加`ApiKey: consumerApiKey`
    - 确保所有现有字段保持不变
    - _Requirements: 2.3, 3.1_
  
  - [ ]* 4.3 编写属性测试：CostRequest包含正确的apikey
    - **Property 2: CostRequest包含正确的apikey**
    - **Validates: Requirements 2.1, 2.3**
    - 生成随机apikey值，验证CostRequest构造正确性
    - _Requirements: 2.1, 2.3_

- [x] 5. 修改deductCostAsync函数
  - [x] 5.1 从context获取apikey
    - 使用`ctx.GetContext(CtxKeyConsumerApiKey)`获取apikey值
    - 处理类型断言，默认为空字符串
    - _Requirements: 2.3_
  
  - [x] 5.2 在CostRequest构造中包含apikey
    - 在构建`requestBody`时添加`ApiKey: consumerApiKey`
    - 确保所有现有字段保持不变
    - _Requirements: 2.3, 3.1_

- [x] 6. Checkpoint - 确保所有测试通过
  - 运行所有单元测试和属性测试
  - 验证代码编译无错误
  - 如有问题，请询问用户

- [ ]* 7. 编写集成测试和额外的属性测试
  - [ ]* 7.1 编写属性测试：CostRequest保留现有字段
    - **Property 3: CostRequest保留现有字段**
    - **Validates: Requirements 2.2**
    - 验证序列化后的JSON包含所有必需字段
    - _Requirements: 2.2_
  
  - [ ]* 7.2 编写属性测试：JSON序列化round-trip
    - **Property 4: JSON序列化round-trip**
    - **Validates: Requirements 3.3**
    - 验证序列化和反序列化的正确性
    - _Requirements: 3.3_
  
  - [ ]* 7.3 编写属性测试：HMAC认证头保持不变
    - **Property 5: HMAC认证头保持不变**
    - **Validates: Requirements 4.2**
    - 验证添加apikey后HMAC头仍然正确发送
    - _Requirements: 4.2_
  
  - [ ]* 7.4 编写集成测试
    - 测试完整的计费流程（non-streaming模式）
    - 测试完整的计费流程（streaming模式）
    - 验证发送到billing-service的请求包含apikey字段
    - _Requirements: 3.1, 3.2_

- [x] 8. Final checkpoint - 最终验证
  - 确保所有测试通过
  - 验证代码符合Go编码规范
  - 检查日志输出是否正确（apikey已脱敏）
  - 如有问题，请询问用户

## Notes

- 标记为`*`的任务是可选的测试任务，可以跳过以加快MVP开发
- 每个任务都引用了具体的需求编号以便追溯
- Checkpoint任务确保增量验证
- 属性测试验证通用正确性属性
- 单元测试验证特定示例和边界情况
- 集成测试验证端到端流程
