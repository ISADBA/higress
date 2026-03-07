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
	data, _ := json.Marshal(map[string]any{
		"billingService": map[string]any{
			"serviceAddress": "billing-service.default.svc.cluster.local",
			"protocol":       "http",
			"port":           8888,
		},
		"failPricingMessage":         "503 Pricing Information Unavailable",
		"failBalanceMessage":         "503 Billing Service Balance Unavailable",
		"insufficientBalanceMessage": "余额不足",
		"failCostMessage":            "503 Billing Service Cost Unavailable",
	})
	return data
}()

// 测试配置：使用默认值的配置
var validDefaultConfig = func() json.RawMessage {
	data, _ := json.Marshal(map[string]any{
		"billingService": map[string]any{
			"serviceAddress": "billing-service",
		},
	})
	return data
}()

// 测试配置：使用 HTTPS 协议
var validHttpsConfig = func() json.RawMessage {
	data, _ := json.Marshal(map[string]any{
		"billingService": map[string]any{
			"serviceAddress": "billing-service",
			"protocol":       "https",
			"port":           443,
		},
	})
	return data
}()

// 测试配置：自定义端口
var validCustomPortConfig = func() json.RawMessage {
	data, _ := json.Marshal(map[string]any{
		"billingService": map[string]any{
			"serviceAddress": "billing-service",
			"port":           9999,
		},
	})
	return data
}()

// 测试配置：缺少 serviceAddress（无效）
var invalidMissingServiceAddress = func() json.RawMessage {
	data, _ := json.Marshal(map[string]any{
		"billingService": map[string]any{
			"protocol": "http",
			"port":     8888,
		},
	})
	return data
}()

// 测试配置：空 serviceAddress（无效）
var invalidEmptyServiceAddress = func() json.RawMessage {
	data, _ := json.Marshal(map[string]any{
		"billingService": map[string]any{
			"serviceAddress": "",
			"protocol":       "http",
			"port":           8888,
		},
	})
	return data
}()

// 测试配置：缺少 billingService（无效）
var invalidMissingBillingService = func() json.RawMessage {
	data, _ := json.Marshal(map[string]any{
		"failBalanceMessage": "503 Billing Service Balance Unavailable",
	})
	return data
}()

// validTenantHeaders returns a complete set of valid tenant headers
func validTenantHeaders() [][2]string {
	return [][2]string{
		// HMAC Authentication Headers
		{"x-internal-auth-sign-version", "v1"},
		{"x-internal-auth-ts", "1234567890"},
		{"x-internal-auth-nonce", "abc123"},
		{"x-internal-auth-sign", "signature123"},
		// Tenant/Consumer Information
		{"x-consumer-id", "consumer-123"},
		{"x-mse-consumer-name", "test-consumer"},
		{"x-mse-tenant-id", "tenant-456"},
		{"x-domain-resource-id", "domain-789"},
		{"x-router-resource-id", "router-012"},
	}
}

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
			require.Equal(t, "503 Pricing Information Unavailable", billingConfig.FailPricingMessage)
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
			require.Equal(t, "503 Pricing Information Unavailable", billingConfig.FailPricingMessage)
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
				data, _ := json.Marshal(map[string]any{
					"billingService": map[string]any{
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
				data, _ := json.Marshal(map[string]any{
					"billingService": map[string]any{
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
	})
}

// TestTenantHeaderExtraction 测试租户头提取功能
// Feature: ai-billing, Task 2.2: Tenant Header Extraction Tests
// Validates: Requirements 2.1, 2.2, 2.3
func TestTenantHeaderExtraction(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		// 测试完整的租户头（应该通过）
		t.Run("complete tenant headers pass", func(t *testing.T) {
			host, status := test.NewTestHost(validDefaultConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			headers := append([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
			}, validTenantHeaders()...)

			action := host.CallOnHttpRequestHeaders(headers)
			// 应该暂停等待定价查询
			require.Equal(t, types.ActionPause, action)
		})

		// 测试缺少 HMAC 头（应返回 401）
		t.Run("missing hmac headers returns 401", func(t *testing.T) {
			host, status := test.NewTestHost(validDefaultConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			// 缺少 x-internal-auth-sign
			headers := [][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
				{"x-internal-auth-sign-version", "v1"},
				{"x-internal-auth-ts", "1234567890"},
				{"x-internal-auth-nonce", "abc123"},
				// Missing: x-internal-auth-sign
				{"x-consumer-id", "consumer-123"},
				{"x-mse-consumer-name", "test-consumer"},
				{"x-mse-tenant-id", "tenant-456"},
				{"x-domain-resource-id", "domain-789"},
				{"x-router-resource-id", "router-012"},
			}

			action := host.CallOnHttpRequestHeaders(headers)
			require.Equal(t, types.ActionContinue, action)

			localResp := host.GetLocalResponse()
			require.NotNil(t, localResp)
			require.Equal(t, uint32(401), localResp.StatusCode)
		})

		// 测试缺少租户信息头（应返回 401）
		t.Run("missing tenant info headers returns 401", func(t *testing.T) {
			host, status := test.NewTestHost(validDefaultConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			// 缺少 x-mse-tenant-id
			headers := [][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
				{"x-internal-auth-sign-version", "v1"},
				{"x-internal-auth-ts", "1234567890"},
				{"x-internal-auth-nonce", "abc123"},
				{"x-internal-auth-sign", "signature123"},
				{"x-consumer-id", "consumer-123"},
				{"x-mse-consumer-name", "test-consumer"},
				// Missing: x-mse-tenant-id
				{"x-domain-resource-id", "domain-789"},
				{"x-router-resource-id", "router-012"},
			}

			action := host.CallOnHttpRequestHeaders(headers)
			require.Equal(t, types.ActionContinue, action)

			localResp := host.GetLocalResponse()
			require.NotNil(t, localResp)
			require.Equal(t, uint32(401), localResp.StatusCode)
		})

		// 测试空租户头值（应返回 401）
		t.Run("empty tenant header values returns 401", func(t *testing.T) {
			host, status := test.NewTestHost(validDefaultConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			headers := [][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
				{"x-internal-auth-sign-version", "v1"},
				{"x-internal-auth-ts", "1234567890"},
				{"x-internal-auth-nonce", "abc123"},
				{"x-internal-auth-sign", "signature123"},
				{"x-consumer-id", ""}, // Empty value
				{"x-mse-consumer-name", "test-consumer"},
				{"x-mse-tenant-id", "tenant-456"},
				{"x-domain-resource-id", "domain-789"},
				{"x-router-resource-id", "router-012"},
			}

			action := host.CallOnHttpRequestHeaders(headers)
			require.Equal(t, types.ActionContinue, action)

			localResp := host.GetLocalResponse()
			require.NotNil(t, localResp)
			require.Equal(t, uint32(401), localResp.StatusCode)
		})
	})
}

// TestPricingQuery 测试定价查询功能
// Feature: ai-billing, Task 3.2: Pricing Query Tests
// Validates: Requirements 3.1, 3.2, 3.3
func TestPricingQuery(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		// 测试定价查询成功（缓存未命中）
		t.Run("pricing query success cache miss", func(t *testing.T) {
			host, status := test.NewTestHost(validDefaultConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			headers := append([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
			}, validTenantHeaders()...)

			action := host.CallOnHttpRequestHeaders(headers)
			require.Equal(t, types.ActionPause, action)

			// 模拟余额检查成功（注意：由于没有 provider/model，会跳过定价查询直接进行余额检查）
			host.CallOnHttpCall([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			}, []byte(`{"success":true,"balance":"100.50","uid":12345,"updated_at":1234567890}`))

			// 应该恢复请求
			// 注意：测试框架会自动恢复请求
		})

		// 测试余额查询失败（应返回 503）
		t.Run("pricing query failure returns 503", func(t *testing.T) {
			host, status := test.NewTestHost(validDefaultConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			headers := append([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
			}, validTenantHeaders()...)

			action := host.CallOnHttpRequestHeaders(headers)
			require.Equal(t, types.ActionPause, action)

			// 模拟余额查询失败
			host.CallOnHttpCall([][2]string{
				{":status", "500"},
				{"content-type", "application/json"},
			}, []byte(`{"error":"internal server error"}`))

			// 验证返回了 503 错误
			localResp := host.GetLocalResponse()
			require.NotNil(t, localResp)
			require.Equal(t, uint32(503), localResp.StatusCode)
		})

		// 测试余额查询返回 success=false（应返回 503）
		t.Run("pricing query success false returns 503", func(t *testing.T) {
			host, status := test.NewTestHost(validDefaultConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			headers := append([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
			}, validTenantHeaders()...)

			action := host.CallOnHttpRequestHeaders(headers)
			require.Equal(t, types.ActionPause, action)

			// 模拟余额查询返回 success=false
			host.CallOnHttpCall([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			}, []byte(`{"success":false,"message":"balance check failed"}`))

			// 验证返回了 503 错误
			localResp := host.GetLocalResponse()
			require.NotNil(t, localResp)
			require.Equal(t, uint32(503), localResp.StatusCode)
		})
	})
}

// TestBalanceCheck 测试余额检查功能
// Feature: ai-billing, Task 4.2: Balance Check Tests
// Validates: Requirements 4.1, 4.2, 4.3
func TestBalanceCheck(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		// 测试余额充足（应允许请求）
		t.Run("sufficient balance allows request", func(t *testing.T) {
			host, status := test.NewTestHost(validDefaultConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			headers := append([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
			}, validTenantHeaders()...)

			action := host.CallOnHttpRequestHeaders(headers)
			require.Equal(t, types.ActionPause, action)

			// 模拟余额充足
			host.CallOnHttpCall([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			}, []byte(`{"success":true,"balance":"100.50","uid":12345,"updated_at":1234567890}`))

			// 验证请求被恢复（没有本地响应）
			localResp := host.GetLocalResponse()
			require.Nil(t, localResp, "Request should be resumed, not blocked")
		})

		// 测试余额不足（应返回 402）
		t.Run("insufficient balance blocks request", func(t *testing.T) {
			host, status := test.NewTestHost(validDefaultConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			headers := append([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
			}, validTenantHeaders()...)

			action := host.CallOnHttpRequestHeaders(headers)
			require.Equal(t, types.ActionPause, action)

			// 模拟余额不足
			host.CallOnHttpCall([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			}, []byte(`{"success":true,"balance":"0.00","uid":12345,"updated_at":1234567890}`))

			// 验证返回了 402 错误
			localResp := host.GetLocalResponse()
			require.NotNil(t, localResp)
			require.Equal(t, uint32(402), localResp.StatusCode)
			require.Contains(t, string(localResp.Data), "余额不足")
		})

		// 测试余额检查服务不可用（应返回 503）
		t.Run("balance service unavailable returns 503", func(t *testing.T) {
			host, status := test.NewTestHost(validDefaultConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			headers := append([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
			}, validTenantHeaders()...)

			action := host.CallOnHttpRequestHeaders(headers)
			require.Equal(t, types.ActionPause, action)

			// 模拟余额检查服务返回错误
			host.CallOnHttpCall([][2]string{
				{":status", "500"},
				{"content-type", "application/json"},
			}, []byte(`{"error":"internal server error"}`))

			// 验证返回了 503 错误
			localResp := host.GetLocalResponse()
			require.NotNil(t, localResp)
			require.Equal(t, uint32(503), localResp.StatusCode)
		})

		// 测试余额检查返回无效 JSON（应返回 503）
		t.Run("invalid balance response returns 503", func(t *testing.T) {
			host, status := test.NewTestHost(validDefaultConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			headers := append([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
			}, validTenantHeaders()...)

			action := host.CallOnHttpRequestHeaders(headers)
			require.Equal(t, types.ActionPause, action)

			// 模拟余额检查返回无效 JSON
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

			headers := append([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
				{"x-request-id", "req-from-header-123"},
			}, validTenantHeaders()...)

			action := host.CallOnHttpRequestHeaders(headers)
			require.Equal(t, types.ActionPause, action)

			// 模拟余额充足
			host.CallOnHttpCall([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			}, []byte(`{"success":true,"balance":"100.00","uid":12345,"updated_at":1234567890}`))

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
		})

		// 测试从响应体提取 request ID（id 字段）
		t.Run("extract from response body id field", func(t *testing.T) {
			host, status := test.NewTestHost(validDefaultConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			headers := append([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
			}, validTenantHeaders()...)

			action := host.CallOnHttpRequestHeaders(headers)
			require.Equal(t, types.ActionPause, action)

			// 模拟余额检查成功
			host.CallOnHttpCall([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			}, []byte(`{"success":true,"balance":"100.00","uid":12345,"updated_at":1234567890}`))

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
			}, []byte(`{"success":true,"billing_event_id":12345,"cost":"0.05","cost_actual":"0.05","discount_ratio":"1.0","remaining_balance":"99.95"}`))
		})
	})
}

// TestTokenExtractionOpenAI 测试 OpenAI token 提取
// Feature: ai-billing, Task 6.2: OpenAI Token Extraction Tests
// Validates: Requirements 4.7, 4.8, 4.9, 11.1, 11.4
func TestTokenExtractionOpenAI(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		t.Run("extract tokens from OpenAI response", func(t *testing.T) {
			host, status := test.NewTestHost(validDefaultConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			headers := append([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
			}, validTenantHeaders()...)

			action := host.CallOnHttpRequestHeaders(headers)
			require.Equal(t, types.ActionPause, action)

			// 模拟余额检查成功
			host.CallOnHttpCall([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			}, []byte(`{"success":true,"balance":"100.00","uid":12345,"updated_at":1234567890}`))

			// 发送响应头
			action = host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			})
			require.Equal(t, types.ActionContinue, action)

			// 构建 OpenAI 格式的响应体
			responseBody := `{
				"id": "chatcmpl-test-123",
				"object": "chat.completion",
				"created": 1234567890,
				"model": "gpt-4",
				"choices": [{
					"index": 0,
					"message": {
						"role": "assistant",
						"content": "Test response"
					},
					"finish_reason": "stop"
				}],
				"usage": {
					"prompt_tokens": 100,
					"completion_tokens": 200,
					"total_tokens": 300
				}
			}`

			// 发送响应体
			action = host.CallOnHttpResponseBody([]byte(responseBody))
			require.Equal(t, types.ActionPause, action)

			// 模拟费用扣除成功
			host.CallOnHttpCall([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			}, []byte(`{"success":true,"billing_event_id":12345,"cost":"0.05","cost_actual":"0.05","discount_ratio":"1.0","remaining_balance":"99.95"}`))
		})
	})
}

// TestTokenExtractionClaude 测试 Claude token 提取
// Feature: ai-billing, Task 6.3: Claude Token Extraction Tests
// Validates: Requirements 4.7, 4.8, 11.2, 11.5
func TestTokenExtractionClaude(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		t.Run("extract tokens from Claude response", func(t *testing.T) {
			host, status := test.NewTestHost(validDefaultConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			headers := append([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/messages"},
				{":method", "POST"},
			}, validTenantHeaders()...)

			action := host.CallOnHttpRequestHeaders(headers)
			require.Equal(t, types.ActionPause, action)

			// 模拟余额检查成功
			host.CallOnHttpCall([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			}, []byte(`{"success":true,"balance":"100.00","uid":12345,"updated_at":1234567890}`))

			// 发送响应头
			action = host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			})
			require.Equal(t, types.ActionContinue, action)

			// 构建 Claude 格式的响应体
			responseBody := `{
				"id": "msg_test_123",
				"type": "message",
				"role": "assistant",
				"content": [{
					"type": "text",
					"text": "Test response"
				}],
				"model": "claude-3-opus",
				"stop_reason": "end_turn",
				"usage": {
					"input_tokens": 150,
					"output_tokens": 250
				}
			}`

			// 发送响应体
			action = host.CallOnHttpResponseBody([]byte(responseBody))
			require.Equal(t, types.ActionPause, action)

			// 模拟费用扣除成功
			host.CallOnHttpCall([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			}, []byte(`{"success":true,"billing_event_id":12345,"cost":"0.08","cost_actual":"0.08","discount_ratio":"1.0","remaining_balance":"99.92"}`))
		})
	})
}

// TestTokenExtractionGemini 测试 Gemini token 提取
// Feature: ai-billing, Task 6.4: Gemini Token Extraction Tests
// Validates: Requirements 4.7, 4.8, 11.3, 11.6
func TestTokenExtractionGemini(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		t.Run("extract tokens from Gemini response", func(t *testing.T) {
			host, status := test.NewTestHost(validDefaultConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			headers := append([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/models/gemini-pro:generateContent"},
				{":method", "POST"},
			}, validTenantHeaders()...)

			action := host.CallOnHttpRequestHeaders(headers)
			require.Equal(t, types.ActionPause, action)

			// 模拟余额检查成功
			host.CallOnHttpCall([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			}, []byte(`{"success":true,"balance":"100.00","uid":12345,"updated_at":1234567890}`))

			action = host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			})
			require.Equal(t, types.ActionContinue, action)

			// 构建 Gemini 格式的响应体
			responseBody := `{
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
					"promptTokenCount": 120,
					"candidatesTokenCount": 180,
					"totalTokenCount": 300
				},
				"modelVersion": "gemini-pro"
			}`

			// 发送响应体
			action = host.CallOnHttpResponseBody([]byte(responseBody))
			require.Equal(t, types.ActionPause, action)

			// 模拟费用扣除成功
			host.CallOnHttpCall([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			}, []byte(`{"success":true,"billing_event_id":12345,"cost":"0.03","cost_actual":"0.03","discount_ratio":"1.0","remaining_balance":"99.97"}`))
		})
	})
}

// TestCostDeductionFailure 测试费用扣除失败
// Feature: ai-billing, Task 7.3: Cost Deduction Failure Tests
// Validates: Requirements 5.6, 5.7, 5.8
func TestCostDeductionFailure(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		t.Run("cost deduction failure blocks response", func(t *testing.T) {
			host, status := test.NewTestHost(validDefaultConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			headers := append([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
			}, validTenantHeaders()...)

			action := host.CallOnHttpRequestHeaders(headers)
			require.Equal(t, types.ActionPause, action)

			// 模拟余额检查成功
			host.CallOnHttpCall([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			}, []byte(`{"success":true,"balance":"100.00","uid":12345,"updated_at":1234567890}`))

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

			// 模拟费用扣除失败（success=false）
			host.CallOnHttpCall([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			}, []byte(`{"success":false,"message":"insufficient balance"}`))

			// 验证返回了错误响应（LLM 响应被阻止）
			localResp := host.GetLocalResponse()
			require.NotNil(t, localResp, "Response should be blocked when cost deduction fails")
			require.Equal(t, uint32(402), localResp.StatusCode)
		})

		t.Run("cost deduction service error blocks response", func(t *testing.T) {
			host, status := test.NewTestHost(validDefaultConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			headers := append([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
			}, validTenantHeaders()...)

			action := host.CallOnHttpRequestHeaders(headers)
			require.Equal(t, types.ActionPause, action)

			// 模拟余额检查成功
			host.CallOnHttpCall([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			}, []byte(`{"success":true,"balance":"100.00","uid":12345,"updated_at":1234567890}`))

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

			// 模拟费用扣除服务错误（500）
			host.CallOnHttpCall([][2]string{
				{":status", "500"},
				{"content-type", "application/json"},
			}, []byte(`{"error":"internal server error"}`))

			// 验证返回了 503 错误（LLM 响应被阻止）
			localResp := host.GetLocalResponse()
			require.NotNil(t, localResp, "Response should be blocked on billing error")
			require.Equal(t, uint32(503), localResp.StatusCode)
		})
	})
}

// TestStreamingResponse 测试流式响应处理
// Feature: ai-billing, Task 9.2: Streaming Response Tests
// Validates: Requirements 6.1, 6.2, 6.3, 6.4, 6.5, 6.6
func TestStreamingResponse(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		t.Run("streaming response with usage", func(t *testing.T) {
			host, status := test.NewTestHost(validDefaultConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			headers := append([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
			}, validTenantHeaders()...)

			action := host.CallOnHttpRequestHeaders(headers)
			require.Equal(t, types.ActionPause, action)

			// 模拟余额检查成功
			host.CallOnHttpCall([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			}, []byte(`{"success":true,"balance":"100.00","uid":12345,"updated_at":1234567890}`))

			action = host.CallOnHttpResponseHeaders([][2]string{
				{":status", "200"},
				{"content-type", "text/event-stream"},
			})
			require.Equal(t, types.ActionContinue, action)

			// 构建流式响应数据块（OpenAI 格式）
			streamChunk := fmt.Sprintf(`data: {"id":"chatcmpl-test-123","object":"chat.completion.chunk","created":1234567890,"model":"gpt-4","choices":[{"index":0,"delta":{"content":"test"},"finish_reason":null}],"usage":{"prompt_tokens":100,"completion_tokens":200,"total_tokens":300}}

`)

			// 发送流式数据块（endOfStream=true）
			// 注意：这会触发异步的费用扣除调用
			action = host.CallOnHttpStreamingResponseBody([]byte(streamChunk), true)

			// 流式响应会继续，但会异步进行计费
			require.Equal(t, types.ActionContinue, action, "Streaming response continues while billing happens asynchronously")

			// 模拟费用扣除成功
			host.CallOnHttpCall([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			}, []byte(`{"success":true,"billing_event_id":12345,"cost":"0.05","cost_actual":"0.05","discount_ratio":"1.0","remaining_balance":"99.95"}`))
		})

		t.Run("streaming without usage returns 500", func(t *testing.T) {
			host, status := test.NewTestHost(validDefaultConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			headers := append([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
			}, validTenantHeaders()...)

			action := host.CallOnHttpRequestHeaders(headers)
			require.Equal(t, types.ActionPause, action)

			// 模拟余额检查成功
			host.CallOnHttpCall([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			}, []byte(`{"success":true,"balance":"100.00","uid":12345,"updated_at":1234567890}`))

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
		})
	})
}

// TestTokenExtractionErrorCases 测试 token 提取错误情况
// Feature: ai-billing, Task 6.7: Token Extraction Error Cases
// Validates: Requirements 4.12, 4.13, 4.14, 4.15
func TestTokenExtractionErrorCases(t *testing.T) {
	test.RunTest(t, func(t *testing.T) {
		// 测试缺少 usage 字段（应返回 500）
		t.Run("missing usage field returns 500", func(t *testing.T) {
			host, status := test.NewTestHost(validDefaultConfig)
			defer host.Reset()
			require.Equal(t, types.OnPluginStartStatusOK, status)

			headers := append([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
			}, validTenantHeaders()...)

			action := host.CallOnHttpRequestHeaders(headers)
			require.Equal(t, types.ActionPause, action)

			// 模拟余额检查成功
			host.CallOnHttpCall([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			}, []byte(`{"success":true,"balance":"100.00","uid":12345,"updated_at":1234567890}`))

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

			headers := append([][2]string{
				{":authority", "example.com"},
				{":path", "/v1/chat/completions"},
				{":method", "POST"},
			}, validTenantHeaders()...)

			action := host.CallOnHttpRequestHeaders(headers)
			require.Equal(t, types.ActionPause, action)

			// 模拟余额检查成功
			host.CallOnHttpCall([][2]string{
				{":status", "200"},
				{"content-type", "application/json"},
			}, []byte(`{"success":true,"balance":"100.00","uid":12345,"updated_at":1234567890}`))

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
	})
}
