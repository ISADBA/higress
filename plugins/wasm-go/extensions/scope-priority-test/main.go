// Copyright (c) 2022 Alibaba Group Holding Ltd.
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
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/tidwall/gjson"
)

func main() {}

// ScopePriorityTestConfig 配置结构
type ScopePriorityTestConfig struct {
	ScopeType             string `json:"scopeType"`
	Message               string `json:"message"`
	Priority              int    `json:"priority"`
	EnableRequestHeaders  bool   `json:"enableRequestHeaders"`
	EnableResponseHeaders bool   `json:"enableResponseHeaders"`
	EnableRequestBody     bool   `json:"enableRequestBody"`
	EnableResponseBody    bool   `json:"enableResponseBody"`
}

func init() {
	wrapper.SetCtx(
		"scope-priority-test",
		wrapper.ParseConfig(parseConfig),
		wrapper.ProcessRequestHeaders(onHttpRequestHeaders),
		wrapper.ProcessResponseHeaders(onHttpResponseHeaders),
		wrapper.ProcessRequestBody(onHttpRequestBody),
		wrapper.ProcessResponseBody(onHttpResponseBody),
	)
}

// parseConfig 解析配置
func parseConfig(json gjson.Result, config *ScopePriorityTestConfig) error {
	// 解析 scopeType，默认值为 "UNKNOWN"
	config.ScopeType = json.Get("scopeType").String()
	if config.ScopeType == "" {
		config.ScopeType = "UNKNOWN"
	}

	// 解析 message
	config.Message = json.Get("message").String()

	// 解析 priority
	config.Priority = int(json.Get("priority").Int())

	// 解析控制开关，默认值为 true
	if json.Get("enableRequestHeaders").Exists() {
		config.EnableRequestHeaders = json.Get("enableRequestHeaders").Bool()
	} else {
		config.EnableRequestHeaders = true
	}

	if json.Get("enableResponseHeaders").Exists() {
		config.EnableResponseHeaders = json.Get("enableResponseHeaders").Bool()
	} else {
		config.EnableResponseHeaders = true
	}

	if json.Get("enableRequestBody").Exists() {
		config.EnableRequestBody = json.Get("enableRequestBody").Bool()
	} else {
		config.EnableRequestBody = true
	}

	if json.Get("enableResponseBody").Exists() {
		config.EnableResponseBody = json.Get("enableResponseBody").Bool()
	} else {
		config.EnableResponseBody = true
	}

	return nil
}

// printContextInfo 打印上下文信息
func printContextInfo() {
	// 打印 consumer 信息
	consumerName, err := proxywasm.GetProperty([]string{"consumer_name"})
	if err == nil && len(consumerName) > 0 {
		proxywasm.LogInfof("Consumer Name: %s", string(consumerName))
	}

	// 打印路由信息
	routeName, err := proxywasm.GetProperty([]string{"route_name"})
	if err == nil && len(routeName) > 0 {
		proxywasm.LogInfof("Route Name: %s", string(routeName))
	}

	// 打印服务/集群信息
	clusterName, err := proxywasm.GetProperty([]string{"cluster_name"})
	if err == nil && len(clusterName) > 0 {
		proxywasm.LogInfof("Cluster Name: %s", string(clusterName))
	}

	// 打印请求 ID
	requestID, _ := proxywasm.GetHttpRequestHeader("x-request-id")
	if requestID != "" {
		proxywasm.LogInfof("Request ID: %s", requestID)
	}
}

// onHttpRequestHeaders 处理请求头
func onHttpRequestHeaders(ctx wrapper.HttpContext, config ScopePriorityTestConfig) types.Action {
	proxywasm.LogInfof("=== Scope Priority Test - Request Headers ===")
	proxywasm.LogInfof("Matched Scope Type: %s", config.ScopeType)
	proxywasm.LogInfof("Config Message: %s", config.Message)
	proxywasm.LogInfof("Config Priority: %d", config.Priority)

	if config.EnableRequestHeaders {
		// 打印伪头部
		method := ctx.Method()
		path := ctx.Path()
		host := ctx.Host()
		scheme := ctx.Scheme()

		proxywasm.LogInfof("Request Method: %s", method)
		proxywasm.LogInfof("Request Path: %s", path)
		proxywasm.LogInfof("Request Host: %s", host)
		proxywasm.LogInfof("Request Scheme: %s", scheme)

		// 打印所有请求头
		headers, _ := proxywasm.GetHttpRequestHeaders()
		proxywasm.LogInfof("Request Headers Count: %d", len(headers))
		for _, header := range headers {
			proxywasm.LogInfof("  %s: %s", header[0], header[1])
		}

		// 打印上下文信息
		printContextInfo()
	}

	proxywasm.LogInfof("===========================================")

	return types.ActionContinue
}

// onHttpResponseHeaders 处理响应头
func onHttpResponseHeaders(ctx wrapper.HttpContext, config ScopePriorityTestConfig) types.Action {
	if !config.EnableResponseHeaders {
		return types.ActionContinue
	}

	proxywasm.LogInfof("=== Scope Priority Test - Response Headers ===")
	proxywasm.LogInfof("Matched Scope Type: %s", config.ScopeType)

	// 打印响应状态码
	status, _ := proxywasm.GetHttpResponseHeader(":status")
	proxywasm.LogInfof("Response Status: %s", status)

	// 打印所有响应头
	headers, _ := proxywasm.GetHttpResponseHeaders()
	proxywasm.LogInfof("Response Headers Count: %d", len(headers))
	for _, header := range headers {
		proxywasm.LogInfof("  %s: %s", header[0], header[1])
	}

	proxywasm.LogInfof("===========================================")

	return types.ActionContinue
}

// onHttpRequestBody 处理请求体
func onHttpRequestBody(ctx wrapper.HttpContext, config ScopePriorityTestConfig, body []byte) types.Action {
	if !config.EnableRequestBody {
		return types.ActionContinue
	}

	proxywasm.LogInfof("=== Scope Priority Test - Request Body ===")
	proxywasm.LogInfof("Matched Scope Type: %s", config.ScopeType)
	proxywasm.LogInfof("Request Body Size: %d bytes", len(body))

	// 打印前 500 字节（如果有内容）
	if len(body) > 0 {
		maxLen := 500
		if len(body) < maxLen {
			maxLen = len(body)
		}
		proxywasm.LogInfof("Request Body Preview: %s", string(body[:maxLen]))
	}

	proxywasm.LogInfof("==========================================")

	return types.ActionContinue
}

// onHttpResponseBody 处理响应体
func onHttpResponseBody(ctx wrapper.HttpContext, config ScopePriorityTestConfig, body []byte) types.Action {
	if !config.EnableResponseBody {
		return types.ActionContinue
	}

	proxywasm.LogInfof("=== Scope Priority Test - Response Body ===")
	proxywasm.LogInfof("Matched Scope Type: %s", config.ScopeType)
	proxywasm.LogInfof("Response Body Size: %d bytes", len(body))

	// 打印前 500 字节（如果有内容）
	if len(body) > 0 {
		maxLen := 500
		if len(body) < maxLen {
			maxLen = len(body)
		}
		proxywasm.LogInfof("Response Body Preview: %s", string(body[:maxLen]))
	}

	proxywasm.LogInfof("===========================================")

	return types.ActionContinue
}
