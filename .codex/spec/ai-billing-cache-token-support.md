# AI Billing 插件支持 Cache Token 计费方案

## 目标

调整 `ai-billing` 插件，使其在调用 `/v1/cost` 时，能够把以下字段传给 billing-service：

- `cache_read_tokens`
- `cache_write_tokens`

从而支持带缓存语义的计费场景。实现方式上，`ai-billing` 复用 `tokenusage` 的 usage 解析能力，再映射为本地的归一化计费结构 `BillingTokenUsage`，而不是在 `ai-billing` 中自行解析各家 provider 的 cache usage 字段。

本次变更刻意收敛范围：

- 范围内：把响应中的 cache token 用量从 `tokenusage` 传递到 billing-service
- 范围外：reasoning token 计费、图片计费、音频计费、按次计费重构

## 现状分析

### 1. tokenusage 已具备部分提取能力

当前 `tokenusage` 已经能从多个主流协议中提取基础 usage 信息，并且已经具备部分 cache token 提取能力。

目前在 cache 维度上，代码里已明确可见的是 Anthropic 风格字段：

- `AnthropicCacheCreationInputToken`
- `AnthropicCacheReadInputToken`

同时，它还会把这两个值写入 `InputTokenDetails`，对应 key 为：

- `cache_creation_input_tokens`
- `cache_read_input_tokens`

这说明“缓存 token 的解析”在底层已经具备基础能力，不需要在 `ai-billing` 里重新做一套 provider-specific 解析。

### 2. ai-billing 当前没有把 cache token 往下游传

当前 `ai-billing` 在构造 `BillingInfo` 和 `CostRequest` 时，只保留了这些字段：

- `provider`
- `model_name`
- `request_id`
- `input_tokens`
- `output_tokens`
- `apikey`
- `apikey_id`

因此，即使上游响应里已经提取到了 cache token，用量在进入 `/v1/cost` 前也被丢掉了，billing-service 无法基于缓存 token 做计费。

### 3. 当前流式计费逻辑是否有问题

结论：当前流式计费主链路本身没有明显设计错误，本次不需要重做流式计费逻辑，只需要做 cache token 的适配和测试补强。

具体判断如下：

- 当前流式模式下，`ai-billing` 会在收到包含 usage 的 chunk 后，把提取出的账单信息写入 context。
- 在流结束时，插件从 context 取出 `BillingInfo`，再调用 `deductCostAsync` 发起异步扣费。
- 对于“usage 只在最终 chunk 或最终 SSE 事件中出现”的场景，这个模式是成立的。

本次需要做的不是重构流式逻辑，而是保证：

- 流式场景下写入 context 的 `BillingInfo` 增加 `CacheReadTokens` 和 `CacheWriteTokens`
- 最终异步扣费请求能把这两个字段传出去

需要额外验证的点：

- 如果某个 provider 的 usage 是多次增量上报，最终写入 context 的值必须以最后一次有效 usage 为准
- 如果 cache token 只在最后一个事件中出现，当前覆盖式写入逻辑也应该能够正确拿到最终值

因此，本次对流式链路的判断是：

- 不是“当前实现有结构性错误”
- 而是“当前实现尚未适配 cache 计费字段”

## 目标行为

当上游响应中包含缓存 token 用量时，`ai-billing` 应该在 `/v1/cost` 请求体中带上：

- `cache_read_tokens`
- `cache_write_tokens`

本次不再让 `ai-billing` 直接依赖 provider-specific 字段，而是通过本地归一化结构 `BillingTokenUsage` 做一次中间映射，再生成 `/v1/cost` 请求体。

如果响应中没有缓存 token 信息，则这两个字段默认按 `0` 处理。

## 第一阶段支持范围

### 1. 支持原则

这次的目标不是只支持某一个 provider 或某一个协议，而是：

- 对所有“有缓存计费能力、且当前 `tokenusage` 已经能够稳定解析出 cache token 用量”的模型，尽量支持

因此，本次支持范围的判断标准不是 provider 白名单，也不是 model 名字白名单，而是：

- 响应里是否存在可识别的 cache token 字段
- 当前 `tokenusage` 是否已经对这些字段建立了稳定映射
- `ai-billing` 是否能够把这些用量映射为 `BillingTokenUsage` 并透传给 `/v1/cost`

### 2. 当前已明确可验证的 cache 协议

基于当前代码现状，已经明确可验证的 cache usage 协议是：

- Anthropic Messages 风格响应

对应字段为：

- `usage.input_tokens`
- `usage.output_tokens`
- `usage.cache_creation_input_tokens`
- `usage.cache_read_input_tokens`

这些字段与当前 `tokenusage` 的提取逻辑已经一一对应，因此本次改造完成后，这类协议可以直接支持 cache 计费。

但本次方案不是“只支持 Anthropic cache”，而是：

- 当前先按 `tokenusage` 已有能力落地
- 后续只要 `tokenusage` 新增了其他 provider 的 cache usage 映射，`ai-billing` 侧应能直接复用 `BillingTokenUsage` 归一化逻辑接入

### 3. 目标支持范围的表述

本次文档采用如下目标描述：

- 凡是“有缓存的模型”，如果其响应协议已经被 `tokenusage` 支持并能提取出独立的 cache token 用量，就应纳入支持范围

换句话说：

- 当前“已验证支持”的是 Anthropic Messages 风格
- 当前“设计目标”不是只支持 Anthropic，而是支持所有已具备 cache usage 解析能力的协议

因此，后续如果 `tokenusage` 新增了其他 provider 的 cache usage 提取逻辑，而字段语义清晰、不会引起双重计费，那么 `ai-billing` 侧原则上不需要重做设计，只需要复用同样的透传机制。

### 4. 支持的模型范围

本次不做 model 名字白名单，也不建议在插件中硬编码某些模型是否支持 cache 计费。

建议采用“能力驱动”的判断方式：

- 只要请求最终返回了可识别的 cache token 字段，就支持 cache 计费
- 如果某个模型没有返回 cache token 字段，就自然退化为普通 token 计费

当前仓库中可视为典型示例的模型名包括：

- `claude-3-opus`
- `claude-3`
- `claude-sonnet-4-5-20250929`

这些只作为样例，不作为硬编码依据。

### 5. 当前尚未纳入支持的场景

以下场景当前仍不纳入本次“已实现支持”的范围：

- 响应协议中没有独立 cache token 字段
- `tokenusage` 还没有对 cache token 字段做提取映射
- 字段语义不清晰，无法确认是否会和 `input_tokens` 产生重复计费

也就是说，本次的边界不是“只支持 Claude”，而是：

- 先支持所有当前已经能被可靠解析的 cache usage 协议
- 对尚未支持的 provider，不猜字段语义，不做冒进适配

## 设计方案

### 0. 核心设计原则

本次方案明确采用两层模型：

1. `tokenusage.TokenUsage`
   负责协议解析，尽量保留 provider-specific 原始语义。

2. `BillingTokenUsage`
   负责计费归一化，向 `ai-billing` 和 `/v1/cost` 暴露稳定字段。

也就是说：

- `tokenusage` 负责识别 OpenAI / Anthropic / Gemini / 其他兼容协议里的 usage 字段
- `ai-billing` 只消费归一化后的 `BillingTokenUsage`
- `ai-billing` 不自行解析 provider-specific cache usage 字段

这套设计的目标是：

- 避免在 `ai-billing` 中重复维护协议解析逻辑
- 避免把计费逻辑绑定到 Anthropic 专属字段名
- 为后续更多有 cache 能力的 provider 预留统一接入点

### 1. 新增 BillingTokenUsage 归一化结构

建议在 `ai-billing` 内部新增本地结构 `BillingTokenUsage`，作为计费用量的统一视图：

```go
type BillingTokenUsage struct {
    InputTokens      int64
    OutputTokens     int64
    CacheReadTokens  int64
    CacheWriteTokens int64

    Model    string
    Provider string
    RequestID string

    RawInputTokenDetails  map[string]int64
    RawOutputTokenDetails map[string]int64
}
```

说明：

- `InputTokens`、`OutputTokens`、`CacheReadTokens`、`CacheWriteTokens` 是当前计费主字段
- `Model`、`Provider`、`RequestID` 是计费请求上下文
- `RawInputTokenDetails`、`RawOutputTokenDetails` 不是本次 `/v1/cost` 的必传字段，但建议保留在内部结构中，方便后续扩展 reasoning、image、tool-use 等计费维度

本次 `BillingTokenUsage` 的定位不是“大而全账单对象”，而是“面向计费的最小归一化结构”。

### 2. 从 TokenUsage 映射到 BillingTokenUsage

建议新增一个归一化函数，例如：

```go
func buildBillingTokenUsage(usage tokenusage.TokenUsage) BillingTokenUsage
```

第一阶段的映射规则建议如下：

```go
func buildBillingTokenUsage(usage tokenusage.TokenUsage) BillingTokenUsage {
    b := BillingTokenUsage{
        InputTokens:           usage.InputToken,
        OutputTokens:          usage.OutputToken,
        RawInputTokenDetails:  usage.InputTokenDetails,
        RawOutputTokenDetails: usage.OutputTokenDetails,
    }

    // 先消费当前 tokenusage 已有的 provider-specific 能力
    b.CacheReadTokens = usage.AnthropicCacheReadInputToken
    b.CacheWriteTokens = usage.AnthropicCacheCreationInputToken

    return b
}
```

这个函数是本次方案的关键。

它的意义在于：

- provider-specific 的解析逻辑留在 `tokenusage`
- provider-neutral 的计费语义在 `BillingTokenUsage`
- `ai-billing` 业务逻辑以后只依赖 `BillingTokenUsage`

后续如果 `tokenusage` 新增例如 Gemini 或其他 provider 的 cache token 提取能力，只需要增强这个映射函数，而不需要改 `deductCost`、`deductCostAsync`、`BillingInfo` 等主链路逻辑。

### 3. 调整 BillingInfo，使其承载 BillingTokenUsage 的结果

给 `BillingInfo` 增加两个字段：

```go
type BillingInfo struct {
    InputTokens      int64
    OutputTokens     int64
    CacheReadTokens  int64
    CacheWriteTokens int64
    Model            string
    Provider         string
    RequestID        string
}
```

原因：

- `BillingInfo` 是响应解析阶段与扣费阶段之间的内部传递对象。
- 当前非流式和流式两条链路都依赖这个结构体。
- 它可以承接 `BillingTokenUsage` 的结果，保证对现有主链路改动最小。

### 4. 按最新 billing-service 协议更新 CostRequest

将 `ai-billing` 内部使用的 `CostRequest` 调整为与 `/v1/cost` 最新请求体结构一致：

```go
type CostRequest struct {
    Provider         string  `json:"provider"`
    ModelName        string  `json:"model_name"`
    RequestID        string  `json:"request_id"`
    InputTokens      int64   `json:"input_tokens"`
    OutputTokens     int64   `json:"output_tokens"`
    CacheReadTokens  int64   `json:"cache_read_tokens,omitempty"`
    CacheWriteTokens int64   `json:"cache_write_tokens,omitempty"`
    SpecialCost      string  `json:"special_cost,omitempty"`
    ConsumerID       *int64  `json:"consumer_id,omitempty"`
    ConsumerName     *string `json:"consumer_name,omitempty"`
    Apikey           string  `json:"apikey,omitempty"`
    ApikeyID         *int64  `json:"apikey_id,omitempty"`
}
```

说明：

- Go 的 JSON tag 应使用标准 `omitempty`，不能写成 `optional`。
- `ConsumerID`、`ConsumerName` 本次先不主动在 body 中赋值，因为当前租户和消费者身份主要通过 header 透传。
- `SpecialCost` 本次不启用，只保留结构兼容性。
- 如果要与最新协议严格对齐，`ApikeyID` 建议在插件内也改成 `*int64`。

本次确定采用以下约定：

- `cache_read_tokens` / `cache_write_tokens` 使用 `omitempty`
- `ApikeyID` 本次一并调整为 `*int64`
- 不在本次把 `BillingInfo` 扩展为通用多维计费结构

### 5. 在非流式链路中通过 BillingTokenUsage 归一化赋值

在非流式响应处理中，执行：

```go
usage := tokenusage.GetTokenUsage(ctx, body)
```

后，先执行归一化：

```go
billingUsage := buildBillingTokenUsage(usage)
```

再构造 `BillingInfo`：

- `InputTokens: billingUsage.InputTokens`
- `OutputTokens: billingUsage.OutputTokens`
- `CacheReadTokens: billingUsage.CacheReadTokens`
- `CacheWriteTokens: billingUsage.CacheWriteTokens`

这样非流式请求在调用 `/v1/cost` 时，就能把缓存 token 一并传出。

### 6. 在流式链路中通过 BillingTokenUsage 归一化赋值

流式处理逻辑中，一旦 `usage.TotalToken > 0`，当前会把提取到的账单信息写入上下文。

这里同样应先执行：

```go
billingUsage := buildBillingTokenUsage(usage)
```

再把归一化后的值写入 `BillingInfo`：

- `InputTokens: billingUsage.InputTokens`
- `OutputTokens: billingUsage.OutputTokens`
- `CacheReadTokens: billingUsage.CacheReadTokens`
- `CacheWriteTokens: billingUsage.CacheWriteTokens`

这样在流结束时调用 `deductCostAsync` 时，缓存 token 也能被带上。

### 7. 在 /v1/cost 请求体中传递 cache token

在以下两个函数中：

- `deductCost`
- `deductCostAsync`

构造 `CostRequest` 时增加：

- `CacheReadTokens: billingInfo.CacheReadTokens`
- `CacheWriteTokens: billingInfo.CacheWriteTokens`

这是本次需求真正生效的关键点。

### 8. 补充日志字段

当前日志里主要输出 `inputTokens` 和 `outputTokens`。为了方便联调和排障，建议把以下字段补进关键信息日志：

- `cacheReadTokens=%d`
- `cacheWriteTokens=%d`

另外建议新增一条 debug 日志，用于输出归一化前后的信息来源，例如：

- 当前 `BillingTokenUsage` 是从哪些 `tokenusage` 字段映射得来
- 是否命中了 provider-specific cache usage 映射

建议更新的位置：

- 提取账单信息成功时的日志
- 发送扣费请求前的日志
- 扣费成功后的日志

原因：

- 方便确认 provider 响应里已有 cache token，但是否真正透传给 billing-service
- 方便区分“解析到了 cache token”还是“billing-service 没算进去”

## 兼容性分析

### 1. 与 billing-service 的兼容性

如果 billing-service 已经支持最新 `/v1/cost` 请求体，那么这次改动是兼容的：

- 有 cache token 的模型会新增相关字段
- 没有 cache token 的模型不会改变实际计费结果

### 2. 与其他 provider 的兼容性

对于不返回缓存 token 的 provider：

- `CacheReadTokens = 0`
- `CacheWriteTokens = 0`

因此不会引入行为变化。

进一步说明如下：

- 如果 provider 不支持 cache 计费字段，插件仍然照常提取 `input_tokens` / `output_tokens`
- 由于 `cache_*` 字段为 `omitempty`，序列化后的 `/v1/cost` 请求体可以自然省略这些字段
- 对 billing-service 而言，这类请求应继续按原有 token 计费逻辑处理

这意味着本次改造对“不支持 cache 的模型或供应商”应当是良好兼容的，不会强制它们进入新计费分支。

### 3. 与流式处理逻辑的兼容性

当前流式模式会把最新一次提取到的 `BillingInfo` 保存到 context 中，流结束时再发起异步扣费。

这套机制仍然成立，只需要确保最终保存的 `BillingInfo` 中包含最新的 cache token 值。

潜在风险：

- 如果某些 provider 的 usage 信息是分段上报，而且 cache token 只出现在最后一个事件中，那么最终上下文里保存的值必须是最后一次解析结果。

缓解方式：

- 保持当前“每次解析到有效 usage 就覆盖 context 中 BillingInfo”的行为
- 用流式测试覆盖该场景

## 风险与边界

### 1. `usage.TotalToken == 0` 时仍会被视为提取失败

当前非流式逻辑里，`usage.TotalToken == 0` 会被直接认为提取账单信息失败。

本次不调整这条规则。

后续可能需要重新评估：

- 某些 provider 是否可能返回 cache token 明细，但不返回 total token

如果存在这种情况，再单独调整“提取成功”的判断逻辑。

### 2. 需要避免 input_tokens 与 cache token 双重计费

这是本次方案里必须重点澄清的点，不能只停留在提醒层面。

根据当前 `tokenusage` 设计，可得出两个重要事实：

- `input_tokens` 来源于 Anthropic 响应中的 `usage.input_tokens`
- `cache_creation_input_tokens` 与 `cache_read_input_tokens` 是独立提取的附加字段

同时，`tokenusage` 在计算 `TotalToken` 的 fallback 值时，会执行：

- `input + output + cache_creation + cache_read`

这说明在当前设计语义下：

- `input_tokens` 被视为“基础输入 token”
- cache read / write token 被视为“独立的可计费维度”
- `total_token` 只是统计口径，不应直接拿来做多维计费公式推导

因此，本次需要明确以下计费约束：

1. `ai-billing` 继续向 billing-service 传递原始的 `input_tokens`、`output_tokens`、`cache_read_tokens`、`cache_write_tokens`。
2. `ai-billing` 不应基于 `TotalToken` 做任何计费换算。
3. billing-service 在计算 Claude/Anthropic cache 计费时，不得再根据 `total_token` 反推出 cache 用量。
4. billing-service 必须把 `input_tokens`、`cache_read_tokens`、`cache_write_tokens` 视为三个并列维度，而不是包含关系。

推荐的计费公式约束如下：

```text
cost =
  input_tokens * input_token_price +
  output_tokens * output_token_price +
  cache_read_tokens * cache_read_token_price +
  cache_write_tokens * cache_write_token_price
```

不推荐的做法：

- 使用 `total_token * 某单价` 再叠加 cache 单价
- 把 `input_tokens` 理解为“已包含 cache token 的总输入”

为了把这个风险落到实处，本次联调和测试中要新增以下校验：

- 对包含 `cache_creation_input_tokens` / `cache_read_input_tokens` 的 Anthropic 响应，验证 `/v1/cost` 请求体里的四个 token 维度是否分别正确
- 与 billing-service 联调时，校验最终 cost 是否严格按四维公式计算，而不是参考 `total_token`

结论：

- 从插件侧看，只要不把 `TotalToken` 用于计费，就不会直接造成双重计费
- 真正需要明确约束的是 billing-service 的计费公式和字段语义

## 实施步骤

1. 在 `ai-billing` 内新增 `BillingTokenUsage` 结构。
2. 新增 `buildBillingTokenUsage(usage tokenusage.TokenUsage)` 归一化函数。
3. 修改 `BillingInfo` 定义，使其承接 `BillingTokenUsage` 的主字段。
4. 修改 `CostRequest` 定义，对齐最新 `/v1/cost` 协议，并将 `ApikeyID` 调整为 `*int64`。
5. 在非流式响应路径中，先把 `TokenUsage` 映射为 `BillingTokenUsage`，再构造 `BillingInfo`。
6. 在流式响应路径中，同样先做归一化，再把结果写入 context。
7. 在 `deductCost` 和 `deductCostAsync` 中，把 cache token 写入 `/v1/cost` 请求体。
8. 更新关键日志，输出 cache token 信息和归一化命中情况。
9. 补充或更新测试，覆盖非流式、流式、无 cache 字段三类场景。
10. 联调时验证 billing-service 不会对 `input_tokens` 与 cache token 产生双重计费。

## 测试方案

### 1. 非流式测试

新增一个响应样例，包含：

- `usage.input_tokens`
- `usage.output_tokens`
- `usage.cache_creation_input_tokens`
- `usage.cache_read_input_tokens`

断言生成的 `/v1/cost` 请求体中包含：

- `cache_read_tokens`
- `cache_write_tokens`

并且值正确。

### 2. 流式测试

新增一个流式响应样例，在最终 usage chunk 中包含 cache token 字段。

断言最终异步调用 `/v1/cost` 时，请求体中包含：

- `cache_read_tokens`
- `cache_write_tokens`

### 3. 回归测试

保留现有不带 cache token 的 provider 用例，验证：

- 请求体仍然合法
- 现有逻辑不被破坏
- 无 cache token 时不会产生异常

### 4. 双重计费校验

增加联调校验项：

- 对同一个 Anthropic cache 响应样例，记录 `/v1/cost` 发出的 `input_tokens`、`output_tokens`、`cache_read_tokens`、`cache_write_tokens`
- 对照 billing-service 返回的 `cost`，确认其计算逻辑符合四维加和公式
- 明确 billing-service 没有使用 `total_token` 参与本次 cache 计费公式

## 待确认问题

1. billing-service 是否要求 `cache_*` 字段即使为 0 也必须显式出现在 JSON 中？
当前约定：默认使用 `omitempty`，只有在 billing-service 明确要求时才调整。

2. billing-service 是否已经确认在 cache 计费公式中完全不参考 `total_token`？
这个问题需要在联调前明确，否则仍然存在字段语义歧义。

3. 是否需要在后续版本把支持范围扩展到其他 provider 的 cache 计费协议？
本次不处理，后续按 provider 单独评估。

## 推荐范围控制

本次建议作为一次聚焦的兼容性改造合入：

- 不改数据库模型
- 不改 billing-service 定价策略
- 不引入 reasoning/image/audio 等新计费维度

成功标准：

对于 Anthropic/Claude 风格、且响应中包含 cache usage 的请求，`ai-billing` 在非流式和流式两种模式下，都能把 `cache_read_tokens` 和 `cache_write_tokens` 正确传给 `/v1/cost`。
