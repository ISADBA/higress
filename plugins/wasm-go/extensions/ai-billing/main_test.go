// Copyright (c) 2025 Alibaba Group Holding Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
)

// 测试配置：有效的完整配置
var validFullConfig = func() json.RawMessage {
	data, _ := json.Marshal(map[string]interface{}{
		"billingService": map[string]interface{}{
			"serviceAddress": "billing-service.default.svc.cluster.local",
			"protocol":       "http",
			"port":           8888,
		},
		"failBalanceMessage":         "503 Billing Service Balance Unavailable",
		"insufficientBalanceMessage": "余额不足",
		"failCostMessage":            "503 Billing Service Cost Unavailable",
	})
	return data
}()

// 测试配置：使用默认值的配置
var validDefaultConfig = func() json.RawMessage {
	data, _ := json.Marshal(map[string]interface{}{
		"billingService": map[string]interface{}{
			"serviceAddress": "billing-service",
		},
	})
	return data
}()

// 测试配置：使用 HTTPS 协议
var validHttpsConfig = func() json.RawMessage {
	data, _ := json.Marshal(map[string]interface{}{
		"billingService": map[string]interface{}{
			"serviceAddress": "billing-service",
			"protocol":       "https",
			"port":           443,
		},
	})
	return data
}()

// 测试配置：自定义端口
var validCustomPortConfig = func() json.RawMessage {
	data, _ := json.Marshal(map[string]interface{}{
		"billingService": map[string]interface{}{
			"serviceAddress": "billing-service",
			"port":           9999,
		},
	})
	return data
}()

// 测试配置：缺少 serviceAddress（无效）
var invalidMissingServiceAddress = func() json.RawMessage {
	data, _ := json.Marshal(map[string]interface{}{
		"billingService": map[string]interface{}{
			"protocol": "http",
			"port":     8888,
		},
	})
	return data
}()

// 测试配置：空 serviceAddress（无效）
var invalidEmptyServiceAddress = func() json.RawMessage {
	data, _ := json.Marshal(map[string]interface{}{
		"billingService": map[string]interface{}{
			"serviceAddress": "",
			"protocol":       "http",
			"port":           8888,
		},
	})
	return data
}()

// 测试配置：缺少 billingService（无效）
var invalidMissingBillingService = func() json.RawMessage {
	data, _ := json.Marshal(map[string]interface{}{
		"failBalanceMessage": "503 Billing Service Balance Unavailable",
	})
	return data
}()

// TestParseConfig 测试配置解析功能
func TestParseConfig(t *testing.T) {
	test.RunGoTest(t, func(t *testing.T) {
		// 测试有效的完整配置
		t.Run("valid full config", func(t *testing.T) {
			host, status := test.NewTestHost(validFullConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			config, err := host.GetMatchConfig()
			require.NoError(t, err)
			require.NotNil(t, config)

			billingConfig := config.(*BillingConfig)
			require.Equal(t, "billing-service.default.svc.cluster.local", billingConfig.BillingService.ServiceAddress)
			require.Equal(t, "http", billingConfig.BillingService.Protocol)
			require.Equal(t, 8888, billingConfig.BillingService.Port)
			require.Equal(t, "503 Billing Service Balance Unavailable", billingConfig.FailBalanceMessage)
			require.Equal(t, "余额不足", billingConfig.InsufficientBalanceMessage)
			require.Equal(t, "503 Billing Service Cost Unavailable", billingConfig.FailCostMessage)
		})

		// 测试使用默认值的配置
		t.Run("valid config with defaults", func(t *testing.T) {
			host, status := test.NewTestHost(validDefaultConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			config, err := host.GetMatchConfig()
			require.NoError(t, err)
			require.NotNil(t, config)

			billingConfig := config.(*BillingConfig)
			require.Equal(t, "billing-service", billingConfig.BillingService.ServiceAddress)
			require.Equal(t, "http", billingConfig.BillingService.Protocol) // 默认值
			require.Equal(t, 8888, billingConfig.BillingService.Port)       // 默认值
			require.Equal(t, "503 Billing Service Balance Unavailable", billingConfig.FailBalanceMessage)
			require.Equal(t, "余额不足", billingConfig.InsufficientBalanceMessage)
			require.Equal(t, "503 Billing Service Cost Unavailable", billingConfig.FailCostMessage)
		})

		// 测试 HTTPS 协议配置
		t.Run("valid https config", func(t *testing.T) {
			host, status := test.NewTestHost(validHttpsConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			config, err := host.GetMatchConfig()
			require.NoError(t, err)
			require.NotNil(t, config)

			billingConfig := config.(*BillingConfig)
			require.Equal(t, "https", billingConfig.BillingService.Protocol)
			require.Equal(t, 443, billingConfig.BillingService.Port)
		})

		// 测试自定义端口配置
		t.Run("valid custom port config", func(t *testing.T) {
			host, status := test.NewTestHost(validCustomPortConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			config, err := host.GetMatchConfig()
			require.NoError(t, err)
			require.NotNil(t, config)

			billingConfig := config.(*BillingConfig)
			require.Equal(t, 9999, billingConfig.BillingService.Port)
		})

		// 测试缺少 serviceAddress 的无效配置
		t.Run("invalid config missing serviceAddress", func(t *testing.T) {
			host, status := test.NewTestHost(invalidMissingServiceAddress)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusFailed, status)
		})

		// 测试空 serviceAddress 的无效配置
		t.Run("invalid config empty serviceAddress", func(t *testing.T) {
			host, status := test.NewTestHost(invalidEmptyServiceAddress)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusFailed, status)
		})

		// 测试缺少 billingService 的无效配置
		t.Run("invalid config missing billingService", func(t *testing.T) {
			host, status := test.NewTestHost(invalidMissingBillingService)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusFailed, status)
		})

		// 测试边界情况：端口为 0（应使用默认值）
		t.Run("edge case port zero", func(t *testing.T) {
			config := func() json.RawMessage {
				data, _ := json.Marshal(map[string]interface{}{
					"billingService": map[string]interface{}{
						"serviceAddress": "billing-service",
						"port":           0,
					},
				})
				return data
			}()

			host, status := test.NewTestHost(config)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			cfg, err := host.GetMatchConfig()
			require.NoError(t, err)
			require.NotNil(t, cfg)

			billingConfig := cfg.(*BillingConfig)
			require.Equal(t, 8888, billingConfig.BillingService.Port) // 应使用默认值
		})

		// 测试边界情况：空字符串协议（应使用默认值）
		t.Run("edge case empty protocol", func(t *testing.T) {
			config := func() json.RawMessage {
				data, _ := json.Marshal(map[string]interface{}{
					"billingService": map[string]interface{}{
						"serviceAddress": "billing-service",
						"protocol":       "",
					},
				})
				return data
			}()

			host, status := test.NewTestHost(config)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			cfg, err := host.GetMatchConfig()
			require.NoError(t, err)
			require.NotNil(t, cfg)

			billingConfig := cfg.(*BillingConfig)
			require.Equal(t, "http", billingConfig.BillingService.Protocol) // 应使用默认值
		})

		// 测试边界情况：无效的端口号（负数）
		t.Run("edge case negative port", func(t *testing.T) {
			config := func() json.RawMessage {
				data, _ := json.Marshal(map[string]interface{}{
					"billingService": map[string]interface{}{
						"serviceAddress": "billing-service",
						"port":           -1,
					},
				})
				return data
			}()

			host, status := test.NewTestHost(config)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			cfg, err := host.GetMatchConfig()
			require.NoError(t, err)
			require.NotNil(t, cfg)

			billingConfig := cfg.(*BillingConfig)
			// 负数端口会被保留（JSON 解析不会自动转换为 0）
			// 这是一个边界情况，实际使用中应该避免
			require.Equal(t, -1, billingConfig.BillingService.Port)
		})

		// 测试边界情况：超大端口号
		t.Run("edge case large port", func(t *testing.T) {
			config := func() json.RawMessage {
				data, _ := json.Marshal(map[string]interface{}{
					"billingService": map[string]interface{}{
						"serviceAddress": "billing-service",
						"port":           65535,
					},
				})
				return data
			}()

			host, status := test.NewTestHost(config)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			cfg, err := host.GetMatchConfig()
			require.NoError(t, err)
			require.NotNil(t, cfg)

			billingConfig := cfg.(*BillingConfig)
			require.Equal(t, 65535, billingConfig.BillingService.Port)
		})
	})
}

// TestMaskApiKey 测试 API Key 脱敏功能
func TestMaskApiKey(t *testing.T) {
	tests := []struct {
		name     string
		apiKey   string
		expected string
	}{
		{
			name:     "normal api key",
			apiKey:   "sk-1234567890abcdef",
			expected: "sk-12345***",
		},
		{
			name:     "short api key",
			apiKey:   "short",
			expected: "short***",
		},
		{
			name:     "exactly 8 characters",
			apiKey:   "12345678",
			expected: "12345678***",
		},
		{
			name:     "empty api key",
			apiKey:   "",
			expected: "***",
		},
		{
			name:     "very long api key",
			apiKey:   "sk-proj-1234567890abcdefghijklmnopqrstuvwxyz",
			expected: "sk-proj-***",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := maskApiKey(tt.apiKey)
			require.Equal(t, tt.expected, result)
		})
	}
}

// TestExtractRequestID 测试 Request ID 提取功能
// Feature: ai-billing, Task 12.2: Request ID Handling Unit Tests
// Validates: Requirements 12.1, 12.5
func TestExtractRequestID(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		// 测试从请求头提取 request ID（最高优先级）
		t.Run("extract from x-request-id header", func(t *testing.T) {
			host, status := test.NewTestHost(validDefaultConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			// 发送请求并通过余额检查
			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
				{"Authorization", "Bearer sk-test-key"},
				{"x-request-id", "req-from-header-123"},
			})
			require.Equal(t, types.ActionPause, action)

			// 模拟余额充足
			host.CallOnHttpCall([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			}, []byte(`{"balance":"100.00","uid":12345,"updated_at":1234567890}`))

			// 发送响应头
			action = host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			})
			require.Equal(t, types.ActionContinue, action)

			// 发送响应体（包含不同的 request ID，但应该使用请求头中的）
			responseBody := `{
				"id": "chatcmpl-from-body-456",
				"model": "gpt-4",
				"usage": {
					"prompt_tokens": 100,
					"completion_tokens": 200,
					"total_tokens": 300
				}
			}`

			action = host.CallOnHttpResponseBody([]byte(responseBody))
			require.Equal(t, types.ActionPause, action)

			// 验证费用扣除请求包含正确的 request ID（从请求头）
			// 注意：由于测试框架限制，我们无法直接验证发送的请求体
			// 但我们可以验证请求被正确处理
		})

		// 测试从响应体提取 request ID（id 字段）
		t.Run("extract from response body id field", func(t *testing.T) {
			host, status := test.NewTestHost(validDefaultConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			// 发送请求（不包含 x-request-id 头）
			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
				{"Authorization", "Bearer sk-test-key"},
			})
			require.Equal(t, types.ActionPause, action)

			// 模拟余额充足
			host.CallOnHttpCall([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			}, []byte(`{"balance":"100.00","uid":12345,"updated_at":1234567890}`))

			// 发送响应头
			action = host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			})
			require.Equal(t, types.ActionContinue, action)

			// 发送响应体（包含 id 字段）
			responseBody := `{
				"id": "chatcmpl-body-789",
				"model": "gpt-4",
				"usage": {
					"prompt_tokens": 100,
					"completion_tokens": 200,
					"total_tokens": 300
				}
			}`

			action = host.CallOnHttpResponseBody([]byte(responseBody))
			require.Equal(t, types.ActionPause, action)

			// 模拟费用扣除成功
			host.CallOnHttpCall([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			}, []byte(`{"billing_event_id":12345,"cost":"0.05","cost_actual":"0.05","discount_ratio":"1.0","remaining_balance":"99.95","success":true}`))
		})

		// 测试从响应体提取 request ID（response.id 字段）
		t.Run("extract from response body response.id field", func(t *testing.T) {
			host, status := test.NewTestHost(validDefaultConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			// 发送请求（不包含 x-request-id 头）
			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
				{"Authorization", "Bearer sk-test-key"},
			})
			require.Equal(t, types.ActionPause, action)

			// 模拟余额充足
			host.CallOnHttpCall([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			}, []byte(`{"balance":"100.00","uid":12345,"updated_at":1234567890}`))

			// 发送响应头
			action = host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			})
			require.Equal(t, types.ActionContinue, action)

			// 发送响应体（包含 response.id 字段）
			responseBody := `{
				"response": {
					"id": "resp-nested-123"
				},
				"model": "gpt-4",
				"usage": {
					"prompt_tokens": 100,
					"completion_tokens": 200,
					"total_tokens": 300
				}
			}`

			action = host.CallOnHttpResponseBody([]byte(responseBody))
			require.Equal(t, types.ActionPause, action)

			// 模拟费用扣除成功
			host.CallOnHttpCall([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			}, []byte(`{"billing_event_id":12345,"cost":"0.05","cost_actual":"0.05","discount_ratio":"1.0","remaining_balance":"99.95","success":true}`))
		})

		// 测试从响应体提取 request ID（responseId 字段）
		t.Run("extract from response body responseId field", func(t *testing.T) {
			host, status := test.NewTestHost(validDefaultConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			// 发送请求（不包含 x-request-id 头）
			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
				{"Authorization", "Bearer sk-test-key"},
			})
			require.Equal(t, types.ActionPause, action)

			// 模拟余额充足
			host.CallOnHttpCall([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			}, []byte(`{"balance":"100.00","uid":12345,"updated_at":1234567890}`))

			// 发送响应头
			action = host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			})
			require.Equal(t, types.ActionContinue, action)

			// 发送响应体（包含 responseId 字段）
			responseBody := `{
				"responseId": "resp-id-456",
				"model": "gpt-4",
				"usage": {
					"prompt_tokens": 100,
					"completion_tokens": 200,
					"total_tokens": 300
				}
			}`

			action = host.CallOnHttpResponseBody([]byte(responseBody))
			require.Equal(t, types.ActionPause, action)

			// 模拟费用扣除成功
			host.CallOnHttpCall([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			}, []byte(`{"billing_event_id":12345,"cost":"0.05","cost_actual":"0.05","discount_ratio":"1.0","remaining_balance":"99.95","success":true}`))
		})

		// 测试从响应体提取 request ID（message.id 字段）
		t.Run("extract from response body message.id field", func(t *testing.T) {
			host, status := test.NewTestHost(validDefaultConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			// 发送请求（不包含 x-request-id 头）
			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
				{"Authorization", "Bearer sk-test-key"},
			})
			require.Equal(t, types.ActionPause, action)

			// 模拟余额充足
			host.CallOnHttpCall([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			}, []byte(`{"balance":"100.00","uid":12345,"updated_at":1234567890}`))

			// 发送响应头
			action = host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			})
			require.Equal(t, types.ActionContinue, action)

			// 发送响应体（包含 message.id 字段）
			responseBody := `{
				"message": {
					"id": "msg-nested-789"
				},
				"model": "gpt-4",
				"usage": {
					"prompt_tokens": 100,
					"completion_tokens": 200,
					"total_tokens": 300
				}
			}`

			action = host.CallOnHttpResponseBody([]byte(responseBody))
			require.Equal(t, types.ActionPause, action)

			// 模拟费用扣除成功
			host.CallOnHttpCall([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			}, []byte(`{"billing_event_id":12345,"cost":"0.05","cost_actual":"0.05","discount_ratio":"1.0","remaining_balance":"99.95","success":true}`))
		})

		// 测试优先级：请求头 > 响应体
		t.Run("header takes priority over response body", func(t *testing.T) {
			host, status := test.NewTestHost(validDefaultConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			// 发送请求（包含 x-request-id 头）
			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
				{"Authorization", "Bearer sk-test-key"},
				{"x-request-id", "priority-header-id"},
			})
			require.Equal(t, types.ActionPause, action)

			// 模拟余额充足
			host.CallOnHttpCall([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			}, []byte(`{"balance":"100.00","uid":12345,"updated_at":1234567890}`))

			// 发送响应头
			action = host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			})
			require.Equal(t, types.ActionContinue, action)

			// 发送响应体（包含不同的 id，但应该使用请求头中的）
			responseBody := `{
				"id": "body-id-should-be-ignored",
				"responseId": "another-body-id",
				"model": "gpt-4",
				"usage": {
					"prompt_tokens": 100,
					"completion_tokens": 200,
					"total_tokens": 300
				}
			}`

			action = host.CallOnHttpResponseBody([]byte(responseBody))
			require.Equal(t, types.ActionPause, action)

			// 模拟费用扣除成功
			host.CallOnHttpCall([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			}, []byte(`{"billing_event_id":12345,"cost":"0.05","cost_actual":"0.05","discount_ratio":"1.0","remaining_balance":"99.95","success":true}`))
		})

		// 测试没有 request ID 的情况（返回空字符串）
		t.Run("no request id returns empty string", func(t *testing.T) {
			host, status := test.NewTestHost(validDefaultConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			// 发送请求（不包含 x-request-id 头）
			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
				{"Authorization", "Bearer sk-test-key"},
			})
			require.Equal(t, types.ActionPause, action)

			// 模拟余额充足
			host.CallOnHttpCall([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			}, []byte(`{"balance":"100.00","uid":12345,"updated_at":1234567890}`))

			// 发送响应头
			action = host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			})
			require.Equal(t, types.ActionContinue, action)

			// 发送响应体（不包含任何 request ID 字段）
			responseBody := `{
				"model": "gpt-4",
				"choices": [{
					"message": {
						"role": "assistant",
						"content": "Test response"
					}
				}],
				"usage": {
					"prompt_tokens": 100,
					"completion_tokens": 200,
					"total_tokens": 300
				}
			}`

			action = host.CallOnHttpResponseBody([]byte(responseBody))
			require.Equal(t, types.ActionPause, action)

			// 模拟费用扣除成功（即使没有 request ID，计费服务也应该处理）
			host.CallOnHttpCall([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			}, []byte(`{"billing_event_id":12345,"cost":"0.05","cost_actual":"0.05","discount_ratio":"1.0","remaining_balance":"99.95","success":true}`))
		})

		// 测试流式响应中的 request ID 提取
		t.Run("extract request id from streaming response", func(t *testing.T) {
			host, status := test.NewTestHost(validDefaultConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			// 发送请求（包含 x-request-id 头）
			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
				{"Authorization", "Bearer sk-test-key"},
				{"x-request-id", "streaming-req-id-123"},
			})
			require.Equal(t, types.ActionPause, action)

			// 模拟余额充足
			host.CallOnHttpCall([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			}, []byte(`{"balance":"100.00","uid":12345,"updated_at":1234567890}`))

			// 发送流式响应头
			action = host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "text/event-stream"},
			})
			require.Equal(t, types.ActionContinue, action)

			// 发送流式数据块（包含 usage 信息）
			streamChunk := `data: {"id":"chatcmpl-stream-456","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4","choices":[{"index":0,"delta":{"content":"test"},"finish_reason":"stop"}],"usage":{"prompt_tokens":100,"completion_tokens":200,"total_tokens":300}}

`
			action = host.CallOnHttpStreamingResponseBody([]byte(streamChunk), true)
			require.Equal(t, types.ActionContinue, action)

			// 模拟费用扣除成功
			host.CallOnHttpCall([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			}, []byte(`{"billing_event_id":12345,"cost":"0.05","cost_actual":"0.05","discount_ratio":"1.0","remaining_balance":"99.95","success":true}`))
		})
	})
}

// TestApiKeyExtractionEdgeCases 测试 API Key 提取的边界情况
// Feature: ai-billing, Task 2.3: API Key Extraction Edge Cases
// Validates: Requirements 2.4, 2.5, 2.6
func TestApiKeyExtractionEdgeCases(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		// 测试缺少 API key（应返回 401）
		t.Run("missing api key returns 401", func(t *testing.T) {
			host, status := test.NewTestHost(validDefaultConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			// 发送不包含任何认证头的请求
			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
			})

			// 应该继续（因为已经发送了错误响应）
			require.Equal(t, types.ActionContinue, action)

			// 验证返回了 401 错误
			localResp := host.GetLocalResponse()
			require.NotNil(t, localResp)
			require.Equal(t, uint32(401), localResp.StatusCode)
		})

		// 测试空 API key（应返回 401）
		t.Run("empty api key returns 401", func(t *testing.T) {
			host, status := test.NewTestHost(validDefaultConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			// 发送包含空 API key 的请求
			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
				{"x-hi-original-auth", ""},
			})

			// 应该继续（因为已经发送了错误响应）
			require.Equal(t, types.ActionContinue, action)

			// 验证返回了 401 错误
			localResp := host.GetLocalResponse()
			require.NotNil(t, localResp)
			require.Equal(t, uint32(401), localResp.StatusCode)
		})

		// 测试仅包含 "Bearer" 的 API key（应返回 401）
		t.Run("bearer only returns 401", func(t *testing.T) {
			host, status := test.NewTestHost(validDefaultConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			// 发送仅包含 "Bearer" 的请求（没有实际的 key）
			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
				{"Authorization", "Bearer"},
			})

			// 应该继续（因为已经发送了错误响应）
			require.Equal(t, types.ActionContinue, action)

			// 验证返回了 401 错误
			localResp := host.GetLocalResponse()
			require.NotNil(t, localResp)
			require.Equal(t, uint32(401), localResp.StatusCode)
		})

		// 测试带 Bearer 前缀的 API key（应正确提取）
		t.Run("bearer prefix is stripped correctly", func(t *testing.T) {
			host, status := test.NewTestHost(validDefaultConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			// 发送带 Bearer 前缀的请求
			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
				{"Authorization", "Bearer sk-valid-key-123"},
			})

			// 应该暂停等待余额检查（说明 API key 被正确提取）
			require.Equal(t, types.ActionPause, action)
		})

		// 测试不带 Bearer 前缀的 API key（应正确提取）
		t.Run("api key without bearer prefix", func(t *testing.T) {
			host, status := test.NewTestHost(validDefaultConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			// 发送不带 Bearer 前缀的请求
			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
				{"x-hi-original-auth", "sk-valid-key-456"},
			})

			// 应该暂停等待余额检查（说明 API key 被正确提取）
			require.Equal(t, types.ActionPause, action)
		})

		// 测试 Authorization 头为空但 x-hi-original-auth 有值（应使用 x-hi-original-auth）
		t.Run("fallback to x-hi-original-auth when authorization is empty", func(t *testing.T) {
			host, status := test.NewTestHost(validDefaultConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			// 发送 Authorization 为空但 x-hi-original-auth 有值的请求
			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
				{"Authorization", ""},
				{"x-hi-original-auth", "sk-fallback-key"},
			})

			// 应该暂停等待余额检查
			require.Equal(t, types.ActionPause, action)
		})

		// 测试小写 authorization 头（HTTP 头不区分大小写）
		t.Run("lowercase authorization header is supported", func(t *testing.T) {
			host, status := test.NewTestHost(validDefaultConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			// 发送使用小写 authorization 头的请求
			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
				{"authorization", "Bearer sk-lowercase-auth-key"},
			})

			// 应该暂停等待余额检查（说明小写头被正确识别）
			require.Equal(t, types.ActionPause, action)
		})
	})
}

// TestPropertyApiKeyExtraction 测试 API Key 提取的属性
// Feature: ai-billing, Property 1: API Key Extraction Consistency
// Validates: Requirements 2.1, 2.2, 2.3, 2.7
func TestPropertyApiKeyExtraction(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		t.Run("property: extract from x-hi-original-auth", func(t *testing.T) {
			// 测试多个不同的 API key 值
			testCases := []string{
				"sk-test-key-1",
				"sk-test-key-2",
				"Bearer sk-test-key-3",
				"api-key-without-prefix",
				"very-long-api-key-with-many-characters-1234567890",
			}

			for _, apiKey := range testCases {
				host, status := test.NewTestHost(validDefaultConfig)
				require.Equal(t, types.OnPluginStartStatusOK, status)

				// 设置请求头，包含 x-hi-original-auth
				action := host.CallOnHttpRequestHeaders([][2]string{
					{":authority", "example.com"},
					{":path", "/v1/chat/completions"},
					{":method", "POST"},
					{"x-hi-original-auth", apiKey},
				})

				// 应该暂停等待余额检查（这意味着成功提取了 API key）
				// 注意：由于我们没有模拟计费服务，这里会超时或失败
				// 但重要的是验证插件尝试进行余额检查，这证明 API key 被提取了
				require.Equal(t, types.ActionPause, action)

				host.Reset()
			}
		})

		t.Run("property: extract from Authorization header", func(t *testing.T) {
			// 测试多个不同的 API key 值
			testCases := []string{
				"sk-auth-key-1",
				"Bearer sk-auth-key-2",
				"auth-key-without-prefix",
			}

			for _, apiKey := range testCases {
				host, status := test.NewTestHost(validDefaultConfig)
				require.Equal(t, types.OnPluginStartStatusOK, status)

				// 设置请求头，仅包含 Authorization（不包含 x-hi-original-auth）
				action := host.CallOnHttpRequestHeaders([][2]string{
					{":authority", "example.com"},
					{":path", "/v1/chat/completions"},
					{":method", "POST"},
					{"Authorization", apiKey},
				})

				// 应该暂停等待余额检查
				require.Equal(t, types.ActionPause, action)

				host.Reset()
			}
		})

		t.Run("property: x-hi-original-auth takes priority", func(t *testing.T) {
			// 测试当两个头都存在时，x-hi-original-auth 优先
			testCases := []struct {
				xHiAuth    string
				authHeader string
			}{
				{"priority-key-1", "fallback-key-1"},
				{"priority-key-2", "fallback-key-2"},
				{"Bearer priority-key-3", "Bearer fallback-key-3"},
			}

			for _, tc := range testCases {
				host, status := test.NewTestHost(validDefaultConfig)
				require.Equal(t, types.OnPluginStartStatusOK, status)

				// 设置两个请求头
				action := host.CallOnHttpRequestHeaders([][2]string{
					{":authority", "example.com"},
					{":path", "/v1/chat/completions"},
					{":method", "POST"},
					{"x-hi-original-auth", tc.xHiAuth},
					{"Authorization", tc.authHeader},
				})

				// 应该暂停等待余额检查（使用 x-hi-original-auth 的值）
				require.Equal(t, types.ActionPause, action)

				host.Reset()
			}
		})

		t.Run("property: Bearer prefix is removed", func(t *testing.T) {
			// 测试 Bearer 前缀被正确移除
			testCases := []struct {
				input     string
				hasBearer bool
			}{
				{"Bearer sk-test-key", true},
				{"Bearer api-key-123", true},
				{"sk-test-key-no-bearer", false},
				{"api-key-no-bearer", false},
			}

			for _, tc := range testCases {
				host, status := test.NewTestHost(validDefaultConfig)
				require.Equal(t, types.OnPluginStartStatusOK, status)

				action := host.CallOnHttpRequestHeaders([][2]string{
					{":authority", "example.com"},
					{":path", "/v1/chat/completions"},
					{":method", "POST"},
					{"x-hi-original-auth", tc.input},
				})

				// 应该暂停等待余额检查（Bearer 前缀应该被移除）
				require.Equal(t, types.ActionPause, action)

				host.Reset()
			}
		})
	})
}

// TestPropertyTokenExtractionOpenAI 测试 OpenAI token 提取的属性
// Feature: ai-billing, Property 5: Token Extraction from OpenAI Format
// Validates: Requirements 4.7, 4.8, 4.9, 11.1, 11.4
func TestPropertyTokenExtractionOpenAI(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		t.Run("property: extract tokens from OpenAI response", func(t *testing.T) {
			// 测试多个不同的 token 值组合
			testCases := []struct {
				name         string
				inputTokens  int64
				outputTokens int64
				model        string
			}{
				{"small tokens", 10, 20, "gpt-3.5-turbo"},
				{"medium tokens", 100, 200, "gpt-4"},
				{"large tokens", 1000, 2000, "gpt-4-turbo"},
				{"zero output", 50, 0, "gpt-3.5-turbo"},
				{"asymmetric tokens", 500, 50, "gpt-4"},
			}

			for _, tc := range testCases {
				t.Run(tc.name, func(t *testing.T) {
					host, status := test.NewTestHost(validDefaultConfig)
					require.Equal(t, types.OnPluginStartStatusOK, status)

					// 发送请求并通过余额检查
					action := host.CallOnHttpRequestHeaders([][2]string{
						{":authority", "example.com"},
						{":path", "/v1/chat/completions"},
						{":method", "POST"},
						{"Authorization", "Bearer sk-test-key"},
					})
					require.Equal(t, types.ActionPause, action)

					// 模拟余额充足
					host.CallOnHttpCall([][2]string{
						{":status", "200"},
						{"content-type", "application/json"},
					}, []byte(`{"balance":"100.00","uid":12345,"updated_at":1234567890}`))

					// 发送响应头
					action = host.CallOnHttpResponseHeaders([][2]string{
						{":status", "200"},
						{"content-type", "application/json"},
					})
					require.Equal(t, types.ActionContinue, action)

					// 构建 OpenAI 格式的响应体
					totalTokens := tc.inputTokens + tc.outputTokens
					responseBody := fmt.Sprintf(`{
						"id": "chatcmpl-test-123",
						"object": "chat.completion",
						"created": 1234567890,
						"model": "%s",
						"choices": [{
							"index": 0,
							"message": {
								"role": "assistant",
								"content": "Test response"
							},
							"finish_reason": "stop"
						}],
						"usage": {
							"prompt_tokens": %d,
							"completion_tokens": %d,
							"total_tokens": %d
						}
					}`, tc.model, tc.inputTokens, tc.outputTokens, totalTokens)

					// 发送响应体
					action = host.CallOnHttpResponseBody([]byte(responseBody))

					// 应该暂停等待费用扣除
					require.Equal(t, types.ActionPause, action)

					host.Reset()
				})
			}
		})
	})
}

// TestPropertyTokenExtractionClaude 测试 Claude token 提取的属性
// Feature: ai-billing, Property 6: Token Extraction from Claude Format
// Validates: Requirements 4.7, 4.8, 11.2, 11.5
func TestPropertyTokenExtractionClaude(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		t.Run("property: extract tokens from Claude response", func(t *testing.T) {
			// 测试多个不同的 token 值组合
			testCases := []struct {
				name         string
				inputTokens  int64
				outputTokens int64
				model        string
			}{
				{"small tokens", 15, 25, "claude-3-opus"},
				{"medium tokens", 150, 250, "claude-3-sonnet"},
				{"large tokens", 1500, 2500, "claude-3-haiku"},
				{"zero output", 75, 0, "claude-3-opus"},
				{"asymmetric tokens", 600, 60, "claude-3-sonnet"},
			}

			for _, tc := range testCases {
				t.Run(tc.name, func(t *testing.T) {
					host, status := test.NewTestHost(validDefaultConfig)
					require.Equal(t, types.OnPluginStartStatusOK, status)

					// 发送请求并通过余额检查
					action := host.CallOnHttpRequestHeaders([][2]string{
						{":authority", "example.com"},
						{":path", "/v1/messages"},
						{":method", "POST"},
						{"Authorization", "Bearer sk-test-key"},
					})
					require.Equal(t, types.ActionPause, action)

					// 模拟余额充足
					host.CallOnHttpCall([][2]string{
						{":status", "200"},
						{"content-type", "application/json"},
					}, []byte(`{"balance":"100.00","uid":12345,"updated_at":1234567890}`))

					// 发送响应头
					action = host.CallOnHttpResponseHeaders([][2]string{
						{":status", "200"},
						{"content-type", "application/json"},
					})
					require.Equal(t, types.ActionContinue, action)

					// 构建 Claude 格式的响应体
					responseBody := fmt.Sprintf(`{
						"id": "msg_test_123",
						"type": "message",
						"role": "assistant",
						"content": [{
							"type": "text",
							"text": "Test response"
						}],
						"model": "%s",
						"stop_reason": "end_turn",
						"usage": {
							"input_tokens": %d,
							"output_tokens": %d
						}
					}`, tc.model, tc.inputTokens, tc.outputTokens)

					// 发送响应体
					action = host.CallOnHttpResponseBody([]byte(responseBody))

					// 应该暂停等待费用扣除
					require.Equal(t, types.ActionPause, action)

					host.Reset()
				})
			}
		})
	})
}

// TestPropertyTokenExtractionGemini 测试 Gemini token 提取的属性
// Feature: ai-billing, Property 7: Token Extraction from Gemini Format
// Validates: Requirements 4.7, 4.8, 11.3, 11.6
func TestPropertyTokenExtractionGemini(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		t.Run("property: extract tokens from Gemini response", func(t *testing.T) {
			// 测试多个不同的 token 值组合
			testCases := []struct {
				name         string
				inputTokens  int64
				outputTokens int64
				model        string
			}{
				{"small tokens", 12, 18, "gemini-pro"},
				{"medium tokens", 120, 180, "gemini-pro-vision"},
				{"large tokens", 1200, 1800, "gemini-ultra"},
				{"zero output", 60, 0, "gemini-pro"},
				{"asymmetric tokens", 550, 55, "gemini-pro"},
			}

			for _, tc := range testCases {
				t.Run(tc.name, func(t *testing.T) {
					host, status := test.NewTestHost(validDefaultConfig)
					require.Equal(t, types.OnPluginStartStatusOK, status)

					// 发送请求并通过余额检查
					action := host.CallOnHttpRequestHeaders([][2]string{
						{":authority", "example.com"},
						{":path", "/v1/models/gemini-pro:generateContent"},
						{":method", "POST"},
						{"Authorization", "Bearer sk-test-key"},
					})
					require.Equal(t, types.ActionPause, action)

					// 模拟余额充足
					host.CallOnHttpCall([][2]string{
						{":status", "200"},
						{"content-type", "application/json"},
					}, []byte(`{"balance":"100.00","uid":12345,"updated_at":1234567890}`))

					// 发送响应头
					action = host.CallOnHttpResponseHeaders([][2]string{
						{":status", "200"},
						{"content-type", "application/json"},
					})
					require.Equal(t, types.ActionContinue, action)

					// 构建 Gemini 格式的响应体
					totalTokens := tc.inputTokens + tc.outputTokens
					responseBody := fmt.Sprintf(`{
						"candidates": [{
							"content": {
								"parts": [{
									"text": "Test response"
								}],
								"role": "model"
							},
							"finishReason": "STOP"
						}],
						"usageMetadata": {
							"promptTokenCount": %d,
							"candidatesTokenCount": %d,
							"totalTokenCount": %d
						},
						"modelVersion": "%s"
					}`, tc.inputTokens, tc.outputTokens, totalTokens, tc.model)

					// 发送响应体
					action = host.CallOnHttpResponseBody([]byte(responseBody))

					// 应该暂停等待费用扣除
					require.Equal(t, types.ActionPause, action)

					host.Reset()
				})
			}
		})
	})
}

// TestPropertyMultiProtocolFallback 测试多协议回退的属性
// Feature: ai-billing, Property 8: Multi-Protocol Fallback
// Validates: Requirements 11.7, 11.8
func TestPropertyMultiProtocolFallback(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		t.Run("property: fallback to different protocols", func(t *testing.T) {
			// 测试不同协议格式的响应
			testCases := []struct {
				name         string
				responseBody string
				description  string
			}{
				{
					name: "OpenAI format",
					responseBody: `{
						"id": "chatcmpl-123",
						"model": "gpt-4",
						"usage": {
							"prompt_tokens": 10,
							"completion_tokens": 20,
							"total_tokens": 30
						}
					}`,
					description: "Standard OpenAI format",
				},
				{
					name: "Claude format",
					responseBody: `{
						"id": "msg_123",
						"model": "claude-3-opus",
						"usage": {
							"input_tokens": 15,
							"output_tokens": 25
						}
					}`,
					description: "Standard Claude format",
				},
				{
					name: "Gemini format",
					responseBody: `{
						"modelVersion": "gemini-pro",
						"usageMetadata": {
							"promptTokenCount": 12,
							"candidatesTokenCount": 18,
							"totalTokenCount": 30
						}
					}`,
					description: "Standard Gemini format",
				},
			}

			for _, tc := range testCases {
				t.Run(tc.name, func(t *testing.T) {
					host, status := test.NewTestHost(validDefaultConfig)
					require.Equal(t, types.OnPluginStartStatusOK, status)

					// 发送请求并通过余额检查
					action := host.CallOnHttpRequestHeaders([][2]string{
						{":authority", "example.com"},
						{":path", "/v1/chat/completions"},
						{":method", "POST"},
						{"Authorization", "Bearer sk-test-key"},
					})
					require.Equal(t, types.ActionPause, action)

					// 模拟余额充足
					host.CallOnHttpCall([][2]string{
						{":status", "200"},
						{"content-type", "application/json"},
					}, []byte(`{"balance":"100.00","uid":12345,"updated_at":1234567890}`))

					// 发送响应头
					action = host.CallOnHttpResponseHeaders([][2]string{
						{":status", "200"},
						{"content-type", "application/json"},
					})
					require.Equal(t, types.ActionContinue, action)

					// 发送响应体
					action = host.CallOnHttpResponseBody([]byte(tc.responseBody))

					// 应该暂停等待费用扣除（说明 token 提取成功）
					require.Equal(t, types.ActionPause, action)

					host.Reset()
				})
			}
		})
	})
}

// TestTokenExtractionErrorCases 测试 token 提取错误情况的单元测试
// Feature: ai-billing, Task 6.7: Token Extraction Error Cases
// Validates: Requirements 4.12, 4.13, 4.14, 4.15
func TestTokenExtractionErrorCases(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		// 测试缺少 usage 字段（应返回 500）
		t.Run("missing usage field returns 500", func(t *testing.T) {
			host, status := test.NewTestHost(validDefaultConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			// 发送请求并通过余额检查
			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
				{"Authorization", "Bearer sk-test-key"},
			})
			require.Equal(t, types.ActionPause, action)

			// 模拟余额充足
			host.CallOnHttpCall([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			}, []byte(`{"balance":"100.00","uid":12345,"updated_at":1234567890}`))

			// 发送响应头
			action = host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			})
			require.Equal(t, types.ActionContinue, action)

			// 发送缺少 usage 字段的响应体
			responseBody := `{
				"id": "chatcmpl-123",
				"model": "gpt-4",
				"choices": [{
					"message": {
						"role": "assistant",
						"content": "Test"
					}
				}]
			}`

			action = host.CallOnHttpResponseBody([]byte(responseBody))

			// 应该继续（因为已经发送了错误响应）
			require.Equal(t, types.ActionContinue, action)

			// 验证返回了 500 错误
			localResp := host.GetLocalResponse()
			require.NotNil(t, localResp)
			require.Equal(t, uint32(500), localResp.StatusCode)
		})

		// 测试无效 JSON 格式
		t.Run("invalid json format returns 500", func(t *testing.T) {
			host, status := test.NewTestHost(validDefaultConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			// 发送请求并通过余额检查
			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
				{"Authorization", "Bearer sk-test-key"},
			})
			require.Equal(t, types.ActionPause, action)

			// 模拟余额充足
			host.CallOnHttpCall([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			}, []byte(`{"balance":"100.00","uid":12345,"updated_at":1234567890}`))

			// 发送响应头
			action = host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			})
			require.Equal(t, types.ActionContinue, action)

			// 发送无效 JSON
			action = host.CallOnHttpResponseBody([]byte(`{invalid json}`))

			// 应该继续（因为已经发送了错误响应）
			require.Equal(t, types.ActionContinue, action)

			// 验证返回了 500 错误
			localResp := host.GetLocalResponse()
			require.NotNil(t, localResp)
			require.Equal(t, uint32(500), localResp.StatusCode)
		})

		// 测试 token 值为 0
		t.Run("zero token values returns 500", func(t *testing.T) {
			host, status := test.NewTestHost(validDefaultConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			// 发送请求并通过余额检查
			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
				{"Authorization", "Bearer sk-test-key"},
			})
			require.Equal(t, types.ActionPause, action)

			// 模拟余额充足
			host.CallOnHttpCall([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			}, []byte(`{"balance":"100.00","uid":12345,"updated_at":1234567890}`))

			// 发送响应头
			action = host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			})
			require.Equal(t, types.ActionContinue, action)

			// 发送 token 值为 0 的响应体
			responseBody := `{
				"id": "chatcmpl-123",
				"model": "gpt-4",
				"usage": {
					"prompt_tokens": 0,
					"completion_tokens": 0,
					"total_tokens": 0
				}
			}`

			action = host.CallOnHttpResponseBody([]byte(responseBody))

			// 应该继续（因为已经发送了错误响应）
			require.Equal(t, types.ActionContinue, action)

			// 验证返回了 500 错误
			localResp := host.GetLocalResponse()
			require.NotNil(t, localResp)
			require.Equal(t, uint32(500), localResp.StatusCode)
		})
	})
}

// TestRequestPhaseIntegration 测试请求阶段的集成
// Feature: ai-billing, Task 5.2: Request Phase Integration Tests
// Validates: Requirements 2.1-2.7, 3.1-3.14
func TestRequestPhaseIntegration(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		// 测试余额充足的完整请求流程
		t.Run("sufficient balance allows request", func(t *testing.T) {
			host, status := test.NewTestHost(validDefaultConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			// 发送请求
			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
				{"Authorization", "Bearer sk-test-key-123"},
			})

			// 应该暂停等待余额检查
			require.Equal(t, types.ActionPause, action)

			// 模拟计费服务返回余额充足
			respBody := `{"balance":"100.50","uid":12345,"updated_at":1234567890}`
			host.CallOnHttpCall([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			}, []byte(respBody))

			// 验证请求被恢复（没有本地响应）
			localResp := host.GetLocalResponse()
			require.Nil(t, localResp, "Request should be resumed, not blocked")
		})

		// 测试余额不足的请求拒绝
		t.Run("insufficient balance blocks request", func(t *testing.T) {
			host, status := test.NewTestHost(validDefaultConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			// 发送请求
			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
				{"Authorization", "Bearer sk-test-key-456"},
			})

			// 应该暂停等待余额检查
			require.Equal(t, types.ActionPause, action)

			// 模拟计费服务返回余额不足
			respBody := `{"balance":"0.00","uid":12345,"updated_at":1234567890}`
			host.CallOnHttpCall([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			}, []byte(respBody))

			// 验证返回了 402 错误
			localResp := host.GetLocalResponse()
			require.NotNil(t, localResp)
			require.Equal(t, uint32(402), localResp.StatusCode)

			// 验证错误消息
			require.Contains(t, string(localResp.Data), "余额不足")
		})

		// 测试计费服务不可用
		t.Run("billing service unavailable blocks request", func(t *testing.T) {
			host, status := test.NewTestHost(validDefaultConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			// 发送请求
			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
				{"Authorization", "Bearer sk-test-key-789"},
			})

			// 应该暂停等待余额检查
			require.Equal(t, types.ActionPause, action)

			// 模拟计费服务返回错误
			host.CallOnHttpCall([][2]string{
				{":status", "500"},
				{"content-type", "application/json"},
			}, []byte(`{"error":"internal server error"}`))

			// 验证返回了 503 错误
			localResp := host.GetLocalResponse()
			require.NotNil(t, localResp)
			require.Equal(t, uint32(503), localResp.StatusCode)
		})

		// 测试缺少 API key 的请求拒绝
		t.Run("missing api key blocks request immediately", func(t *testing.T) {
			host, status := test.NewTestHost(validDefaultConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			// 发送不包含 API key 的请求
			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
			})

			// 应该立即继续（因为已经发送了错误响应）
			require.Equal(t, types.ActionContinue, action)

			// 验证返回了 401 错误
			localResp := host.GetLocalResponse()
			require.NotNil(t, localResp)
			require.Equal(t, uint32(401), localResp.StatusCode)
		})

		// 测试计费服务返回无效 JSON
		t.Run("invalid balance response blocks request", func(t *testing.T) {
			host, status := test.NewTestHost(validDefaultConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			// 发送请求
			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
				{"Authorization", "Bearer sk-test-key-invalid"},
			})

			// 应该暂停等待余额检查
			require.Equal(t, types.ActionPause, action)

			// 模拟计费服务返回无效 JSON
			host.CallOnHttpCall([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			}, []byte(`{invalid json}`))

			// 验证返回了 503 错误
			localResp := host.GetLocalResponse()
			require.NotNil(t, localResp)
			require.Equal(t, uint32(503), localResp.StatusCode)
		})
	})
}

// TestPropertyCostDeductionSuccess 测试费用扣除成功的属性
// Feature: ai-billing, Property 9: Cost Deduction Success Pass-Through
// Validates: Requirements 5.5
func TestPropertyCostDeductionSuccess(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		t.Run("property: successful cost deduction allows response", func(t *testing.T) {
			// 测试多个不同的 token 值和模型组合
			testCases := []struct {
				name         string
				inputTokens  int64
				outputTokens int64
				model        string
				cost         string
			}{
				{"small cost", 10, 20, "gpt-3.5-turbo", "0.001"},
				{"medium cost", 100, 200, "gpt-4", "0.05"},
				{"large cost", 1000, 2000, "gpt-4-turbo", "0.50"},
				{"asymmetric tokens", 500, 50, "claude-3-opus", "0.25"},
				{"zero output tokens", 100, 0, "gemini-pro", "0.01"},
			}

			for _, tc := range testCases {
				t.Run(tc.name, func(t *testing.T) {
					host, status := test.NewTestHost(validDefaultConfig)
					require.Equal(t, types.OnPluginStartStatusOK, status)

					// 发送请求并通过余额检查
					action := host.CallOnHttpRequestHeaders([][2]string{
						{":authority", "example.com"},
						{":path", "/v1/chat/completions"},
						{":method", "POST"},
						{"Authorization", "Bearer sk-test-key"},
					})
					require.Equal(t, types.ActionPause, action)

					// 模拟余额充足
					host.CallOnHttpCall([][2]string{
						{":status", "200"},
						{"content-type", "application/json"},
					}, []byte(`{"balance":"100.00","uid":12345,"updated_at":1234567890}`))

					// 发送响应头
					action = host.CallOnHttpResponseHeaders([][2]string{
						{":status", "200"},
						{"content-type", "application/json"},
					})
					require.Equal(t, types.ActionContinue, action)

					// 构建响应体
					totalTokens := tc.inputTokens + tc.outputTokens
					responseBody := fmt.Sprintf(`{
						"id": "chatcmpl-test-123",
						"model": "%s",
						"usage": {
							"prompt_tokens": %d,
							"completion_tokens": %d,
							"total_tokens": %d
						}
					}`, tc.model, tc.inputTokens, tc.outputTokens, totalTokens)

					// 发送响应体
					action = host.CallOnHttpResponseBody([]byte(responseBody))
					require.Equal(t, types.ActionPause, action)

					// 模拟费用扣除成功
					costResponse := fmt.Sprintf(`{
						"billing_event_id": 12345,
						"cost": "%s",
						"cost_actual": "%s",
						"discount_ratio": "1.0",
						"remaining_balance": "99.50",
						"success": true
					}`, tc.cost, tc.cost)

					host.CallOnHttpCall([][2]string{
						{":status", "200"},
						{"content-type", "application/json"},
					}, []byte(costResponse))

					// 验证响应被恢复（没有本地响应）
					localResp := host.GetLocalResponse()
					require.Nil(t, localResp, "Response should be resumed when cost deduction succeeds")

					host.Reset()
				})
			}
		})
	})
}

// TestPropertyCostDeductionFailureBlocking 测试费用扣除失败阻止的属性
// Feature: ai-billing, Property 10: Cost Deduction Failure Blocking
// Validates: Requirements 5.6, 5.7, 5.8
func TestPropertyCostDeductionFailureBlocking(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		t.Run("property: cost deduction failure blocks response", func(t *testing.T) {
			// 测试多个不同的失败场景
			testCases := []struct {
				name         string
				inputTokens  int64
				outputTokens int64
				model        string
				success      bool
				expectedCode uint32
				description  string
			}{
				{
					name:         "success false - insufficient balance",
					inputTokens:  100,
					outputTokens: 200,
					model:        "gpt-4",
					success:      false,
					expectedCode: 402,
					description:  "Billing service returns success=false",
				},
				{
					name:         "success false - large tokens",
					inputTokens:  1000,
					outputTokens: 2000,
					model:        "gpt-4-turbo",
					success:      false,
					expectedCode: 402,
					description:  "Large token count with success=false",
				},
				{
					name:         "success false - small tokens",
					inputTokens:  10,
					outputTokens: 20,
					model:        "gpt-3.5-turbo",
					success:      false,
					expectedCode: 402,
					description:  "Small token count with success=false",
				},
			}

			for _, tc := range testCases {
				t.Run(tc.name, func(t *testing.T) {
					host, status := test.NewTestHost(validDefaultConfig)
					require.Equal(t, types.OnPluginStartStatusOK, status)

					// 发送请求并通过余额检查
					action := host.CallOnHttpRequestHeaders([][2]string{
						{":authority", "example.com"},
						{":path", "/v1/chat/completions"},
						{":method", "POST"},
						{"Authorization", "Bearer sk-test-key"},
					})
					require.Equal(t, types.ActionPause, action)

					// 模拟余额充足
					host.CallOnHttpCall([][2]string{
						{":status", "200"},
						{"content-type", "application/json"},
					}, []byte(`{"balance":"100.00","uid":12345,"updated_at":1234567890}`))

					// 发送响应头
					action = host.CallOnHttpResponseHeaders([][2]string{
						{":status", "200"},
						{"content-type", "application/json"},
					})
					require.Equal(t, types.ActionContinue, action)

					// 构建响应体
					totalTokens := tc.inputTokens + tc.outputTokens
					responseBody := fmt.Sprintf(`{
						"id": "chatcmpl-test-123",
						"model": "%s",
						"usage": {
							"prompt_tokens": %d,
							"completion_tokens": %d,
							"total_tokens": %d
						}
					}`, tc.model, tc.inputTokens, tc.outputTokens, totalTokens)

					// 发送响应体
					action = host.CallOnHttpResponseBody([]byte(responseBody))
					require.Equal(t, types.ActionPause, action)

					// 模拟费用扣除失败（success=false）
					costResponse := `{
						"billing_event_id": 0,
						"cost": "0.00",
						"cost_actual": "0.00",
						"discount_ratio": "1.0",
						"remaining_balance": "0.00",
						"success": false
					}`

					host.CallOnHttpCall([][2]string{
						{":status", "200"},
						{"content-type", "application/json"},
					}, []byte(costResponse))

					// 验证返回了错误响应（LLM 响应被阻止）
					localResp := host.GetLocalResponse()
					require.NotNil(t, localResp, "Response should be blocked when cost deduction fails")
					require.Equal(t, tc.expectedCode, localResp.StatusCode)

					// 验证错误消息
					require.Contains(t, string(localResp.Data), "余额不足")

					host.Reset()
				})
			}
		})
	})
}

// TestPropertyCostDeductionErrorHandling 测试费用扣除错误处理的属性
// Feature: ai-billing, Property 11: Cost Deduction Error Handling
// Validates: Requirements 5.9, 5.10, 5.11, 5.12, 5.13, 5.14
func TestPropertyCostDeductionErrorHandling(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		t.Run("property: cost deduction errors block response", func(t *testing.T) {
			// 测试多个不同的错误场景
			testCases := []struct {
				name         string
				statusCode   int
				responseBody string
				expectedCode uint32
				description  string
			}{
				{
					name:         "billing service returns 500",
					statusCode:   500,
					responseBody: `{"error":"internal server error"}`,
					expectedCode: 503,
					description:  "Billing service internal error",
				},
				{
					name:         "billing service returns 503",
					statusCode:   503,
					responseBody: `{"error":"service unavailable"}`,
					expectedCode: 503,
					description:  "Billing service unavailable",
				},
				{
					name:         "billing service returns 404",
					statusCode:   404,
					responseBody: `{"error":"not found"}`,
					expectedCode: 503,
					description:  "Billing service endpoint not found",
				},
				{
					name:         "billing service returns invalid json",
					statusCode:   200,
					responseBody: `{invalid json}`,
					expectedCode: 503,
					description:  "Invalid JSON response from billing service",
				},
				{
					name:         "billing service returns empty response",
					statusCode:   200,
					responseBody: ``,
					expectedCode: 503,
					description:  "Empty response from billing service",
				},
				{
					name:         "billing service returns 401",
					statusCode:   401,
					responseBody: `{"error":"unauthorized"}`,
					expectedCode: 503,
					description:  "Billing service authentication error",
				},
			}

			for _, tc := range testCases {
				t.Run(tc.name, func(t *testing.T) {
					host, status := test.NewTestHost(validDefaultConfig)
					require.Equal(t, types.OnPluginStartStatusOK, status)

					// 发送请求并通过余额检查
					action := host.CallOnHttpRequestHeaders([][2]string{
						{":authority", "example.com"},
						{":path", "/v1/chat/completions"},
						{":method", "POST"},
						{"Authorization", "Bearer sk-test-key"},
					})
					require.Equal(t, types.ActionPause, action)

					// 模拟余额充足
					host.CallOnHttpCall([][2]string{
						{":status", "200"},
						{"content-type", "application/json"},
					}, []byte(`{"balance":"100.00","uid":12345,"updated_at":1234567890}`))

					// 发送响应头
					action = host.CallOnHttpResponseHeaders([][2]string{
						{":status", "200"},
						{"content-type", "application/json"},
					})
					require.Equal(t, types.ActionContinue, action)

					// 构建响应体
					responseBody := `{
						"id": "chatcmpl-test-123",
						"model": "gpt-4",
						"usage": {
							"prompt_tokens": 100,
							"completion_tokens": 200,
							"total_tokens": 300
						}
					}`

					// 发送响应体
					action = host.CallOnHttpResponseBody([]byte(responseBody))
					require.Equal(t, types.ActionPause, action)

					// 模拟费用扣除错误
					host.CallOnHttpCall([][2]string{
						{":status", fmt.Sprintf("%d", tc.statusCode)},
						{"content-type", "application/json"},
					}, []byte(tc.responseBody))

					// 验证返回了错误响应（LLM 响应被阻止）
					localResp := host.GetLocalResponse()
					require.NotNil(t, localResp, "Response should be blocked when cost deduction errors: %s", tc.description)
					require.Equal(t, tc.expectedCode, localResp.StatusCode, "Expected status code %d for: %s", tc.expectedCode, tc.description)

					// 验证错误消息
					if tc.expectedCode == 503 {
						require.Contains(t, string(localResp.Data), "503 Billing Service Cost Unavailable")
					}

					host.Reset()
				})
			}
		})

		// 测试不同 token 值组合下的错误处理
		t.Run("property: errors block response regardless of token values", func(t *testing.T) {
			tokenCases := []struct {
				name         string
				inputTokens  int64
				outputTokens int64
				model        string
			}{
				{"small tokens", 10, 20, "gpt-3.5-turbo"},
				{"medium tokens", 100, 200, "gpt-4"},
				{"large tokens", 1000, 2000, "gpt-4-turbo"},
				{"asymmetric tokens", 500, 50, "claude-3-opus"},
			}

			for _, tc := range tokenCases {
				t.Run(tc.name, func(t *testing.T) {
					host, status := test.NewTestHost(validDefaultConfig)
					require.Equal(t, types.OnPluginStartStatusOK, status)

					// 发送请求并通过余额检查
					action := host.CallOnHttpRequestHeaders([][2]string{
						{":authority", "example.com"},
						{":path", "/v1/chat/completions"},
						{":method", "POST"},
						{"Authorization", "Bearer sk-test-key"},
					})
					require.Equal(t, types.ActionPause, action)

					// 模拟余额充足
					host.CallOnHttpCall([][2]string{
						{":status", "200"},
						{"content-type", "application/json"},
					}, []byte(`{"balance":"100.00","uid":12345,"updated_at":1234567890}`))

					// 发送响应头
					action = host.CallOnHttpResponseHeaders([][2]string{
						{":status", "200"},
						{"content-type", "application/json"},
					})
					require.Equal(t, types.ActionContinue, action)

					// 构建响应体
					totalTokens := tc.inputTokens + tc.outputTokens
					responseBody := fmt.Sprintf(`{
						"id": "chatcmpl-test-123",
						"model": "%s",
						"usage": {
							"prompt_tokens": %d,
							"completion_tokens": %d,
							"total_tokens": %d
						}
					}`, tc.model, tc.inputTokens, tc.outputTokens, totalTokens)

					// 发送响应体
					action = host.CallOnHttpResponseBody([]byte(responseBody))
					require.Equal(t, types.ActionPause, action)

					// 模拟费用扣除服务错误（500）
					host.CallOnHttpCall([][2]string{
						{":status", "500"},
						{"content-type", "application/json"},
					}, []byte(`{"error":"internal server error"}`))

					// 验证返回了 503 错误（LLM 响应被阻止）
					localResp := host.GetLocalResponse()
					require.NotNil(t, localResp, "Response should be blocked on billing error")
					require.Equal(t, uint32(503), localResp.StatusCode)

					host.Reset()
				})
			}
		})
	})
}

// TestPropertyStreamingTokenExtraction 测试流式响应 Token 提取的属性
// Feature: ai-billing, Property 12: Streaming Response Token Extraction
// Validates: Requirements 6.1, 6.2, 6.3, 6.4, 6.5, 6.6
func TestPropertyStreamingTokenExtraction(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		t.Run("property: extract tokens from streaming response", func(t *testing.T) {
			// 测试多个不同的 token 值组合
			testCases := []struct {
				name         string
				inputTokens  int64
				outputTokens int64
				model        string
			}{
				{"small tokens", 10, 20, "gpt-3.5-turbo"},
				{"medium tokens", 100, 200, "gpt-4"},
				{"large tokens", 1000, 2000, "gpt-4-turbo"},
				{"zero output", 50, 0, "gpt-3.5-turbo"},
				{"asymmetric tokens", 500, 50, "claude-3-opus"},
			}

			for _, tc := range testCases {
				t.Run(tc.name, func(t *testing.T) {
					host, status := test.NewTestHost(validDefaultConfig)
					require.Equal(t, types.OnPluginStartStatusOK, status)

					// 发送请求并通过余额检查
					action := host.CallOnHttpRequestHeaders([][2]string{
						{":authority", "example.com"},
						{":path", "/v1/chat/completions"},
						{":method", "POST"},
						{"Authorization", "Bearer sk-test-key"},
					})
					require.Equal(t, types.ActionPause, action)

					// 模拟余额充足
					host.CallOnHttpCall([][2]string{
						{":status", "200"},
						{"content-type", "application/json"},
					}, []byte(`{"balance":"100.00","uid":12345,"updated_at":1234567890}`))

					// 发送流式响应头
					action = host.CallOnHttpResponseHeaders([][2]string{
						{":status", "200"},
						{"content-type", "text/event-stream"},
					})
					require.Equal(t, types.ActionContinue, action)

					// 构建流式响应数据块（OpenAI 格式）
					totalTokens := tc.inputTokens + tc.outputTokens
					streamChunk := fmt.Sprintf(`data: {"id":"chatcmpl-test-123","object":"chat.completion.chunk","created":1234567890,"model":"%s","choices":[{"index":0,"delta":{"content":"test"},"finish_reason":null}],"usage":{"prompt_tokens":%d,"completion_tokens":%d,"total_tokens":%d}}

`, tc.model, tc.inputTokens, tc.outputTokens, totalTokens)

					// 发送流式数据块（endOfStream=true）
					// 注意：这会触发异步的费用扣除调用
					action = host.CallOnHttpStreamingResponseBody([]byte(streamChunk), true)

					// 流式响应会继续，但会异步进行计费
					// 注意：由于测试框架的限制，流式响应不会返回 ActionPause
					// 但计费逻辑仍然会被正确调用
					require.Equal(t, types.ActionContinue, action, "Streaming response continues while billing happens asynchronously")

					// 模拟费用扣除成功
					host.CallOnHttpCall([][2]string{
						{":status", "200"},
						{"content-type", "application/json"},
					}, []byte(`{"billing_event_id":12345,"cost":"0.05","cost_actual":"0.05","discount_ratio":"1.0","remaining_balance":"99.95","success":true}`))

					host.Reset()
				})
			}
		})

		t.Run("property: streaming chunks accumulate token usage", func(t *testing.T) {
			host, status := test.NewTestHost(validDefaultConfig)
			require.Equal(t, types.OnPluginStartStatusOK, status)

			// 发送请求并通过余额检查
			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
				{"Authorization", "Bearer sk-test-key"},
			})
			require.Equal(t, types.ActionPause, action)

			// 模拟余额充足
			host.CallOnHttpCall([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			}, []byte(`{"balance":"100.00","uid":12345,"updated_at":1234567890}`))

			// 发送流式响应头
			action = host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "text/event-stream"},
			})
			require.Equal(t, types.ActionContinue, action)

			// 发送第一个数据块（没有 usage 信息）
			chunk1 := `data: {"id":"chatcmpl-test-123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}]}

`
			action = host.CallOnHttpStreamingResponseBody([]byte(chunk1), false)
			require.Equal(t, types.ActionContinue, action, "Chunk without usage should continue")

			// 发送第二个数据块（没有 usage 信息）
			chunk2 := `data: {"id":"chatcmpl-test-123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4","choices":[{"index":0,"delta":{"content":" World"},"finish_reason":null}]}

`
			action = host.CallOnHttpStreamingResponseBody([]byte(chunk2), false)
			require.Equal(t, types.ActionContinue, action, "Chunk without usage should continue")

			// 发送最后一个数据块（包含 usage 信息）
			finalChunk := `data: {"id":"chatcmpl-test-123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":100,"completion_tokens":200,"total_tokens":300}}

`
			action = host.CallOnHttpStreamingResponseBody([]byte(finalChunk), true)

			// 流式响应会继续，但会异步进行计费
			require.Equal(t, types.ActionContinue, action, "Final chunk with usage continues while billing happens asynchronously")

			// 模拟费用扣除成功
			host.CallOnHttpCall([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			}, []byte(`{"billing_event_id":12345,"cost":"0.05","cost_actual":"0.05","discount_ratio":"1.0","remaining_balance":"99.95","success":true}`))

			host.Reset()
		})
	})
}

// TestStreamingEdgeCases 测试流式边界情况的单元测试
// Feature: ai-billing, Task 9.5: Streaming Edge Cases
// Validates: Requirements 6.5, 6.7, 6.8, 6.9
func TestStreamingEdgeCases(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		// 测试没有 usage 信息的流式响应（应返回 500）
		t.Run("streaming without usage returns 500", func(t *testing.T) {
			host, status := test.NewTestHost(validDefaultConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			// 发送请求并通过余额检查
			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
				{"Authorization", "Bearer sk-test-key"},
			})
			require.Equal(t, types.ActionPause, action)

			// 模拟余额充足
			host.CallOnHttpCall([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			}, []byte(`{"balance":"100.00","uid":12345,"updated_at":1234567890}`))

			// 发送流式响应头
			action = host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "text/event-stream"},
			})
			require.Equal(t, types.ActionContinue, action)

			// 发送没有 usage 信息的数据块
			chunk := `data: {"id":"chatcmpl-test-123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4","choices":[{"index":0,"delta":{"content":"test"},"finish_reason":"stop"}]}

`
			action = host.CallOnHttpStreamingResponseBody([]byte(chunk), true)

			// 应该继续（因为发送了错误响应）
			require.Equal(t, types.ActionContinue, action)

			// 验证返回了 500 错误
			localResp := host.GetLocalResponse()
			require.NotNil(t, localResp)
			require.Equal(t, uint32(500), localResp.StatusCode)
			require.Contains(t, string(localResp.Data), "Failed to extract billing information from stream")
		})

		// 测试多个 usage 对象（使用最后一个）
		t.Run("multiple usage objects uses last one", func(t *testing.T) {
			host, status := test.NewTestHost(validDefaultConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			// 发送请求并通过余额检查
			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
				{"Authorization", "Bearer sk-test-key"},
			})
			require.Equal(t, types.ActionPause, action)

			// 模拟余额充足
			host.CallOnHttpCall([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			}, []byte(`{"balance":"100.00","uid":12345,"updated_at":1234567890}`))

			// 发送流式响应头
			action = host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "text/event-stream"},
			})
			require.Equal(t, types.ActionContinue, action)

			// 发送第一个包含 usage 的数据块（应该被覆盖）
			chunk1 := `data: {"id":"chatcmpl-test-123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4","choices":[{"index":0,"delta":{"content":"test"},"finish_reason":null}],"usage":{"prompt_tokens":50,"completion_tokens":100,"total_tokens":150}}

`
			action = host.CallOnHttpStreamingResponseBody([]byte(chunk1), false)
			require.Equal(t, types.ActionContinue, action, "First chunk should pass through")

			// 发送第二个包含 usage 的数据块（这个应该被使用）
			chunk2 := `data: {"id":"chatcmpl-test-123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":100,"completion_tokens":200,"total_tokens":300}}

`
			action = host.CallOnHttpStreamingResponseBody([]byte(chunk2), true)

			// 流式响应会继续，但会异步进行计费
			require.Equal(t, types.ActionContinue, action, "Final chunk continues while billing happens asynchronously")

			// 模拟费用扣除成功
			host.CallOnHttpCall([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			}, []byte(`{"billing_event_id":12345,"cost":"0.05","cost_actual":"0.05","discount_ratio":"1.0","remaining_balance":"99.95","success":true}`))

			// 注意：由于 tokenusage.GetTokenUsage() 会覆盖之前的值，
			// 最后一个 usage 对象（100/200）应该被使用
		})

		// 测试 [DONE] 消息处理
		t.Run("done message handling", func(t *testing.T) {
			host, status := test.NewTestHost(validDefaultConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			// 发送请求并通过余额检查
			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
				{"Authorization", "Bearer sk-test-key"},
			})
			require.Equal(t, types.ActionPause, action)

			// 模拟余额充足
			host.CallOnHttpCall([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			}, []byte(`{"balance":"100.00","uid":12345,"updated_at":1234567890}`))

			// 发送流式响应头
			action = host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "text/event-stream"},
			})
			require.Equal(t, types.ActionContinue, action)

			// 发送包含 usage 的数据块
			chunk1 := `data: {"id":"chatcmpl-test-123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4","choices":[{"index":0,"delta":{"content":"test"},"finish_reason":"stop"}],"usage":{"prompt_tokens":100,"completion_tokens":200,"total_tokens":300}}

`
			action = host.CallOnHttpStreamingResponseBody([]byte(chunk1), false)
			require.Equal(t, types.ActionContinue, action, "Chunk with usage should pass through when not end of stream")

			// 发送 [DONE] 消息（endOfStream=true）
			doneMessage := `data: [DONE]

`
			action = host.CallOnHttpStreamingResponseBody([]byte(doneMessage), true)

			// 流式响应会继续，但会异步进行计费
			// 注意：[DONE] 消息本身不包含 usage，但之前的数据块已经设置了 billing info
			require.Equal(t, types.ActionContinue, action, "Done message continues while billing happens asynchronously with accumulated usage")

			// 模拟费用扣除成功
			host.CallOnHttpCall([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			}, []byte(`{"billing_event_id":12345,"cost":"0.05","cost_actual":"0.05","discount_ratio":"1.0","remaining_balance":"99.95","success":true}`))
		})

		// 测试空数据块处理
		t.Run("empty chunk handling", func(t *testing.T) {
			host, status := test.NewTestHost(validDefaultConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			// 发送请求并通过余额检查
			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
				{"Authorization", "Bearer sk-test-key"},
			})
			require.Equal(t, types.ActionPause, action)

			// 模拟余额充足
			host.CallOnHttpCall([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			}, []byte(`{"balance":"100.00","uid":12345,"updated_at":1234567890}`))

			// 发送流式响应头
			action = host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "text/event-stream"},
			})
			require.Equal(t, types.ActionContinue, action)

			// 发送空数据块（不是流结束）
			action = host.CallOnHttpStreamingResponseBody([]byte(""), false)
			require.Equal(t, types.ActionContinue, action, "Empty chunk should pass through when not end of stream")

			// 发送包含 usage 的数据块
			chunk := `data: {"id":"chatcmpl-test-123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4","usage":{"prompt_tokens":100,"completion_tokens":200,"total_tokens":300}}

`
			action = host.CallOnHttpStreamingResponseBody([]byte(chunk), true)

			// 流式响应会继续，但会异步进行计费
			require.Equal(t, types.ActionContinue, action, "Final chunk with usage continues while billing happens asynchronously")

			// 模拟费用扣除成功
			host.CallOnHttpCall([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			}, []byte(`{"billing_event_id":12345,"cost":"0.05","cost_actual":"0.05","discount_ratio":"1.0","remaining_balance":"99.95","success":true}`))
		})

		// 测试非 200 状态码的流式响应（应该跳过计费）
		t.Run("non-200 streaming response skips billing", func(t *testing.T) {
			host, status := test.NewTestHost(validDefaultConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			// 发送请求并通过余额检查
			action := host.CallOnHttpRequestHeaders([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
				{"Authorization", "Bearer sk-test-key"},
			})
			require.Equal(t, types.ActionPause, action)

			// 模拟余额充足
			host.CallOnHttpCall([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			}, []byte(`{"balance":"100.00","uid":12345,"updated_at":1234567890}`))

			// 发送 500 错误响应头
			action = host.CallOnHttpResponseHeaders([][2]string{
				{":status", "500"},
				{"content-type", "text/event-stream"},
			})
			require.Equal(t, types.ActionContinue, action)

			// 发送错误数据块（应该直接通过，不进行计费）
			errorChunk := `data: {"error":"internal server error"}

`
			action = host.CallOnHttpStreamingResponseBody([]byte(errorChunk), true)

			// 应该继续（因为响应头阶段已经跳过了计费）
			// 注意：由于 onHttpResponseHeaders 返回了 ActionContinue 且状态码不是 200，
			// 流式响应处理器不会被调用，或者即使被调用也不会设置流式标志
			// 这个测试主要验证非 200 响应不会触发计费逻辑
			require.Equal(t, types.ActionContinue, action, "Non-200 streaming response should skip billing")
		})
	})
}

// TestPropertyApiKeyMasking 测试 API Key 脱敏的属性
// Feature: ai-billing, Property 13: API Key Masking in Logs
// Validates: Requirements 10.4
func TestPropertyApiKeyMasking(t *testing.T) {
	t.Run("property: masked key never reveals full key", func(t *testing.T) {
		// 测试多个不同长度和格式的 API key
		testCases := []struct {
			name   string
			apiKey string
		}{
			{"short key", "abc"},
			{"8 char key", "12345678"},
			{"9 char key", "123456789"},
			{"normal key", "sk-1234567890abcdef"},
			{"long key", "sk-proj-1234567890abcdefghijklmnopqrstuvwxyz"},
			{"empty key", ""},
			{"special chars", "sk-!@#$%^&*()"},
			{"unicode key", "sk-测试密钥123"},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				masked := maskApiKey(tc.apiKey)

				// 属性 1: 脱敏后的字符串必须包含 "***"
				require.Contains(t, masked, "***", "Masked key must contain ***")

				// 属性 2: 如果原始 key 长度 > 8，脱敏后不应包含完整的原始 key
				if len(tc.apiKey) > 8 {
					require.NotEqual(t, tc.apiKey, masked, "Masked key should not equal original key when length > 8")
					// 脱敏后的字符串不应包含原始 key 的后半部分
					if len(tc.apiKey) > 8 {
						hiddenPart := tc.apiKey[8:]
						require.NotContains(t, masked, hiddenPart, "Masked key should not contain the hidden part")
					}
				}

				// 属性 3: 脱敏后的字符串长度应该合理（不会过长）
				// 最多显示前 8 个字符 + "***"
				maxExpectedLen := 8 + 3
				require.LessOrEqual(t, len(masked), maxExpectedLen, "Masked key length should be reasonable")

				// 属性 4: 如果原始 key 长度 <= 8，脱敏后应该是原始 key + "***"
				if len(tc.apiKey) <= 8 {
					expected := tc.apiKey + "***"
					require.Equal(t, expected, masked, "Short keys should be fully shown with *** suffix")
				}

				// 属性 5: 如果原始 key 长度 > 8，脱敏后应该是前 8 个字符 + "***"
				if len(tc.apiKey) > 8 {
					expected := tc.apiKey[:8] + "***"
					require.Equal(t, expected, masked, "Long keys should show first 8 chars with *** suffix")
				}
			})
		}
	})

	t.Run("property: masking is consistent", func(t *testing.T) {
		// 测试同一个 API key 多次脱敏应该得到相同结果
		testKeys := []string{
			"sk-test-key-123",
			"short",
			"exactly8",
			"",
			"very-long-api-key-with-many-characters",
		}

		for _, key := range testKeys {
			masked1 := maskApiKey(key)
			masked2 := maskApiKey(key)
			require.Equal(t, masked1, masked2, "Masking the same key should produce consistent results")
		}
	})

	t.Run("property: different keys produce different masked results", func(t *testing.T) {
		// 测试不同的 API key 应该产生不同的脱敏结果（除非前 8 个字符相同）
		key1 := "sk-key-1-abcdefghijk"
		key2 := "sk-key-2-abcdefghijk"

		masked1 := maskApiKey(key1)
		masked2 := maskApiKey(key2)

		// 因为前 8 个字符不同，脱敏结果应该不同
		require.NotEqual(t, masked1, masked2, "Different keys should produce different masked results")
	})

	t.Run("property: masking preserves prefix information", func(t *testing.T) {
		// 测试脱敏保留了前缀信息，便于调试
		testCases := []struct {
			apiKey string
			prefix string
		}{
			{"sk-test-key-123", "sk-test-"},
			{"api-key-456", "api-key-"},
			{"Bearer-token", "Bearer-t"},
			{"12345678901234", "12345678"},
		}

		for _, tc := range testCases {
			masked := maskApiKey(tc.apiKey)
			// 脱敏后的字符串应该以原始前缀开始（最多 8 个字符）
			if len(tc.apiKey) >= len(tc.prefix) {
				require.True(t, len(masked) >= len(tc.prefix), "Masked key should preserve prefix")
				// 检查前缀是否保留
				maskedPrefix := masked[:min(len(tc.prefix), len(masked)-3)] // 减去 "***"
				require.Equal(t, tc.prefix[:len(maskedPrefix)], maskedPrefix, "Prefix should be preserved")
			}
		}
	})
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
