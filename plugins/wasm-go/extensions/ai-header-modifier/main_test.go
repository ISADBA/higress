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
	"encoding/json"
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/test"
	"github.com/stretchr/testify/require"
)

// getHeader is a helper function to get a header value from the headers array
func getHeader(headers [][2]string, key string) string {
	for _, h := range headers {
		if h[0] == key {
			return h[1]
		}
	}
	return ""
}

func TestParseConfig(t *testing.T) {
	tests := []struct {
		name        string
		config      map[string]interface{}
		expectError bool
		validate    func(*testing.T, *AiHeaderModifierConfig)
	}{
		{
			name: "default config",
			config: map[string]interface{}{
				"modelToHeader": "x-model",
			},
			expectError: false,
			validate: func(t *testing.T, config *AiHeaderModifierConfig) {
				require.Equal(t, "model", config.ModelKey)
				require.Equal(t, "x-model", config.ModelToHeader)
				require.Equal(t, "", config.AddProviderHeader)
				require.Len(t, config.EnableOnPathSuffix, 10)
			},
		},
		{
			name: "custom config",
			config: map[string]interface{}{
				"modelKey":           "llm_model",
				"modelToHeader":      "x-llm-model",
				"addProviderHeader":  "x-llm-provider",
				"enableOnPathSuffix": []string{"/v1/chat/completions", "/v1/embeddings"},
			},
			expectError: false,
			validate: func(t *testing.T, config *AiHeaderModifierConfig) {
				require.Equal(t, "llm_model", config.ModelKey)
				require.Equal(t, "x-llm-model", config.ModelToHeader)
				require.Equal(t, "x-llm-provider", config.AddProviderHeader)
				require.Equal(t, []string{"/v1/chat/completions", "/v1/embeddings"}, config.EnableOnPathSuffix)
			},
		},
		{
			name: "wildcard path",
			config: map[string]interface{}{
				"modelToHeader":      "x-model",
				"enableOnPathSuffix": []string{"*"},
			},
			expectError: false,
			validate: func(t *testing.T, config *AiHeaderModifierConfig) {
				require.Equal(t, []string{"*"}, config.EnableOnPathSuffix)
			},
		},
		{
			name: "no headers configured",
			config: map[string]interface{}{
				"modelKey": "model",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			test.RunGoTest(t, func(t *testing.T) {
				configData, _ := json.Marshal(tt.config)
				host, status := test.NewTestHost(configData)
				defer host.Reset()

				if tt.expectError {
					require.NotEqual(t, types.OnPluginStartStatusOK, status)
				} else {
					require.Equal(t, types.OnPluginStartStatusOK, status)
					config, err := host.GetMatchConfig()
					require.NoError(t, err)
					require.NotNil(t, config)

					if tt.validate != nil {
						aiConfig := config.(*AiHeaderModifierConfig)
						tt.validate(t, aiConfig)
					}
				}
			})
		})
	}
}

func TestExtractBoundary(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		expected    string
	}{
		{
			name:        "simple boundary",
			contentType: "multipart/form-data; boundary=----WebKitFormBoundary7MA4YWxkTrZu0gW",
			expected:    "----WebKitFormBoundary7MA4YWxkTrZu0gW",
		},
		{
			name:        "boundary with quotes",
			contentType: `multipart/form-data; boundary="----WebKitFormBoundary7MA4YWxkTrZu0gW"`,
			expected:    "----WebKitFormBoundary7MA4YWxkTrZu0gW",
		},
		{
			name:        "no boundary",
			contentType: "multipart/form-data",
			expected:    "",
		},
		{
			name:        "invalid content-type",
			contentType: "application/json",
			expected:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractBoundary(tt.contentType)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestJSONBodyProcessing(t *testing.T) {
	configData, _ := json.Marshal(map[string]interface{}{
		"modelKey":           "model",
		"modelToHeader":      "x-model",
		"addProviderHeader":  "x-provider",
		"enableOnPathSuffix": []string{"/v1/chat/completions"},
	})

	test.RunTest(t, func(t *testing.T) {
		host, status := test.NewTestHost(configData)
		defer host.Reset()
		require.Equal(t, types.OnPluginStartStatusOK, status)

		// Test JSON request with provider
		requestBody := `{"model": "openai/gpt-4", "messages": [{"role": "user", "content": "Hello"}]}`
		action := host.CallOnHttpRequestHeaders([][2]string{
			{":authority", "test.com"},
			{":path", "/v1/chat/completions"},
			{"content-type", "application/json"},
			{"content-length", "100"},
		})
		require.Equal(t, types.HeaderStopIteration, action)

		action = host.CallOnHttpRequestBody([]byte(requestBody))
		require.Equal(t, types.ActionContinue, action)

		// Verify headers were added
		headers := host.GetRequestHeaders()
		modelHeader := getHeader(headers, "x-model")
		require.Equal(t, "openai/gpt-4", modelHeader)

		providerHeader := getHeader(headers, "x-provider")
		require.Equal(t, "openai", providerHeader)

		// Verify body was rewritten
		modifiedBody := host.GetRequestBody()
		require.Contains(t, string(modifiedBody), `"model": "gpt-4"`)
		require.NotContains(t, string(modifiedBody), "openai/gpt-4")
	})
}

func TestJSONBodyWithoutProvider(t *testing.T) {
	configData, _ := json.Marshal(map[string]interface{}{
		"modelKey":           "model",
		"modelToHeader":      "x-model",
		"enableOnPathSuffix": []string{"/v1/chat/completions"},
	})

	test.RunTest(t, func(t *testing.T) {
		host, status := test.NewTestHost(configData)
		defer host.Reset()
		require.Equal(t, types.OnPluginStartStatusOK, status)

		// Test JSON request without provider extraction
		requestBody := `{"model": "gpt-4", "messages": [{"role": "user", "content": "Hello"}]}`
		action := host.CallOnHttpRequestHeaders([][2]string{
			{":authority", "test.com"},
			{":path", "/v1/chat/completions"},
			{"content-type", "application/json"},
			{"content-length", "100"},
		})
		require.Equal(t, types.HeaderStopIteration, action)

		action = host.CallOnHttpRequestBody([]byte(requestBody))
		require.Equal(t, types.ActionContinue, action)

		// Verify model header was added
		headers := host.GetRequestHeaders()
		modelHeader := getHeader(headers, "x-model")
		require.Equal(t, "gpt-4", modelHeader)

		// Verify body was not modified
		modifiedBody := host.GetRequestBody()
		require.Contains(t, string(modifiedBody), `"model": "gpt-4"`)
	})
}

func TestPathFiltering(t *testing.T) {
	configData, _ := json.Marshal(map[string]interface{}{
		"modelKey":           "model",
		"modelToHeader":      "x-model",
		"enableOnPathSuffix": []string{"/v1/chat/completions"},
	})

	test.RunTest(t, func(t *testing.T) {
		host, status := test.NewTestHost(configData)
		defer host.Reset()
		require.Equal(t, types.OnPluginStartStatusOK, status)

		// Test path that doesn't match
		action := host.CallOnHttpRequestHeaders([][2]string{
			{":authority", "test.com"},
			{":path", "/v1/models"},
			{"content-type", "application/json"},
			{"content-length", "100"},
		})
		require.Equal(t, types.ActionContinue, action)
	})
}

func TestWildcardPath(t *testing.T) {
	configData, _ := json.Marshal(map[string]interface{}{
		"modelKey":           "model",
		"modelToHeader":      "x-model",
		"enableOnPathSuffix": []string{"*"},
	})

	test.RunTest(t, func(t *testing.T) {
		host, status := test.NewTestHost(configData)
		defer host.Reset()
		require.Equal(t, types.OnPluginStartStatusOK, status)

		// Test any path should match
		requestBody := `{"model": "gpt-4", "messages": []}`
		action := host.CallOnHttpRequestHeaders([][2]string{
			{":authority", "test.com"},
			{":path", "/any/random/path"},
			{"content-type", "application/json"},
			{"content-length", "100"},
		})
		require.Equal(t, types.HeaderStopIteration, action)

		action = host.CallOnHttpRequestBody([]byte(requestBody))
		require.Equal(t, types.ActionContinue, action)

		// Verify header was added
		modelHeader, _ := test.GetHeaderValue(host.GetRequestHeaders(), "x-model")
		require.Equal(t, "gpt-4", modelHeader)
	})
}

func TestNoRequestBody(t *testing.T) {
	configData, _ := json.Marshal(map[string]interface{}{
		"modelKey":           "model",
		"modelToHeader":      "x-model",
		"enableOnPathSuffix": []string{"/v1/chat/completions"},
	})

	test.RunTest(t, func(t *testing.T) {
		host, status := test.NewTestHost(configData)
		defer host.Reset()
		require.Equal(t, types.OnPluginStartStatusOK, status)

		// Test request without body
		action := host.CallOnHttpRequestHeaders([][2]string{
			{":authority", "test.com"},
			{":path", "/v1/chat/completions"},
			{"content-type", "application/json"},
		})
		require.Equal(t, types.ActionContinue, action)
	})
}

func TestInvalidJSON(t *testing.T) {
	configData, _ := json.Marshal(map[string]interface{}{
		"modelKey":           "model",
		"modelToHeader":      "x-model",
		"enableOnPathSuffix": []string{"/v1/chat/completions"},
	})

	test.RunTest(t, func(t *testing.T) {
		host, status := test.NewTestHost(configData)
		defer host.Reset()
		require.Equal(t, types.OnPluginStartStatusOK, status)

		// Test invalid JSON
		requestBody := `{invalid json}`
		action := host.CallOnHttpRequestHeaders([][2]string{
			{":authority", "test.com"},
			{":path", "/v1/chat/completions"},
			{"content-type", "application/json"},
			{"content-length", "100"},
		})
		require.Equal(t, types.HeaderStopIteration, action)

		action = host.CallOnHttpRequestBody([]byte(requestBody))
		require.Equal(t, types.ActionContinue, action)

		// Should not add headers for invalid JSON
		modelHeader, _ := test.GetHeaderValue(host.GetRequestHeaders(), "x-model")
		require.Equal(t, "", modelHeader)
	})
}

// buildMultipartBody constructs a properly formatted multipart/form-data body
func buildMultipartBody(boundary string, parts map[string]string) string {
	var body string
	for name, value := range parts {
		body += "--" + boundary + "\r\n"
		body += "Content-Disposition: form-data; name=\"" + name + "\"\r\n\r\n"
		body += value + "\r\n"
	}
	body += "--" + boundary + "--\r\n"
	return body
}

func TestMultipartBodyWithoutProvider(t *testing.T) {
	configData, _ := json.Marshal(map[string]interface{}{
		"modelKey":           "model",
		"modelToHeader":      "x-model",
		"enableOnPathSuffix": []string{"/v1/chat/completions"},
	})

	test.RunTest(t, func(t *testing.T) {
		host, status := test.NewTestHost(configData)
		defer host.Reset()
		require.Equal(t, types.OnPluginStartStatusOK, status)

		boundary := "boundary123456789"
		requestBody := buildMultipartBody(boundary, map[string]string{
			"model":    "gpt-4",
			"messages": "[{\"role\":\"user\",\"content\":\"Hello\"}]",
		})

		action := host.CallOnHttpRequestHeaders([][2]string{
			{":authority", "test.com"},
			{":path", "/v1/chat/completions"},
			{"content-type", "multipart/form-data; boundary=" + boundary},
			{"content-length", "100"},
		})
		require.Equal(t, types.HeaderStopIteration, action)

		action = host.CallOnHttpRequestBody([]byte(requestBody))
		require.Equal(t, types.ActionContinue, action)

		// Verify header was added
		modelHeader, _ := test.GetHeaderValue(host.GetRequestHeaders(), "x-model")
		require.Equal(t, "gpt-4", modelHeader)

		// Verify body was not modified (no provider extraction)
		modifiedBody := string(host.GetRequestBody())
		require.Contains(t, modifiedBody, "gpt-4")
	})
}

func TestMultipartBodyWithProvider(t *testing.T) {
	configData, _ := json.Marshal(map[string]interface{}{
		"modelKey":           "model",
		"modelToHeader":      "x-model",
		"addProviderHeader":  "x-provider",
		"enableOnPathSuffix": []string{"/v1/chat/completions"},
	})

	test.RunTest(t, func(t *testing.T) {
		host, status := test.NewTestHost(configData)
		defer host.Reset()
		require.Equal(t, types.OnPluginStartStatusOK, status)

		boundary := "boundary123456789"
		requestBody := buildMultipartBody(boundary, map[string]string{
			"model":    "openai/gpt-4",
			"messages": "[{\"role\":\"user\",\"content\":\"Hello\"}]",
		})

		action := host.CallOnHttpRequestHeaders([][2]string{
			{":authority", "test.com"},
			{":path", "/v1/chat/completions"},
			{"content-type", "multipart/form-data; boundary=" + boundary},
			{"content-length", "100"},
		})
		require.Equal(t, types.HeaderStopIteration, action)

		action = host.CallOnHttpRequestBody([]byte(requestBody))
		require.Equal(t, types.ActionContinue, action)

		// Verify headers were added
		modelHeader, _ := test.GetHeaderValue(host.GetRequestHeaders(), "x-model")
		require.Equal(t, "openai/gpt-4", modelHeader)

		providerHeader, _ := test.GetHeaderValue(host.GetRequestHeaders(), "x-provider")
		require.Equal(t, "openai", providerHeader)

		// Verify body was rewritten
		modifiedBody := string(host.GetRequestBody())
		require.Contains(t, modifiedBody, "name=\"model\"\r\n\r\ngpt-4\r\n")
		require.NotContains(t, modifiedBody, "openai/gpt-4")
	})
}

func TestMultipartBodyWithComplexProvider(t *testing.T) {
	configData, _ := json.Marshal(map[string]interface{}{
		"modelKey":           "model",
		"modelToHeader":      "x-model",
		"addProviderHeader":  "x-provider",
		"enableOnPathSuffix": []string{"/v1/chat/completions"},
	})

	test.RunTest(t, func(t *testing.T) {
		host, status := test.NewTestHost(configData)
		defer host.Reset()
		require.Equal(t, types.OnPluginStartStatusOK, status)

		boundary := "boundary123456789"
		requestBody := buildMultipartBody(boundary, map[string]string{
			"model":    "azure/openai/gpt-4-turbo",
			"messages": "[{\"role\":\"user\",\"content\":\"Hello\"}]",
		})

		action := host.CallOnHttpRequestHeaders([][2]string{
			{":authority", "test.com"},
			{":path", "/v1/chat/completions"},
			{"content-type", "multipart/form-data; boundary=" + boundary},
			{"content-length", "100"},
		})
		require.Equal(t, types.HeaderStopIteration, action)

		action = host.CallOnHttpRequestBody([]byte(requestBody))
		require.Equal(t, types.ActionContinue, action)

		// Verify headers - only first slash splits provider
		modelHeader, _ := test.GetHeaderValue(host.GetRequestHeaders(), "x-model")
		require.Equal(t, "azure/openai/gpt-4-turbo", modelHeader)

		providerHeader, _ := test.GetHeaderValue(host.GetRequestHeaders(), "x-provider")
		require.Equal(t, "azure", providerHeader)

		// Verify body was rewritten to remove only the provider prefix
		modifiedBody := string(host.GetRequestBody())
		require.Contains(t, modifiedBody, "name=\"model\"\r\n\r\nopenai/gpt-4-turbo\r\n")
		require.NotContains(t, modifiedBody, "azure/openai/gpt-4-turbo")
	})
}

func TestMultipartBodyMissingModel(t *testing.T) {
	configData, _ := json.Marshal(map[string]interface{}{
		"modelKey":           "model",
		"modelToHeader":      "x-model",
		"addProviderHeader":  "x-provider",
		"enableOnPathSuffix": []string{"/v1/chat/completions"},
	})

	test.RunTest(t, func(t *testing.T) {
		host, status := test.NewTestHost(configData)
		defer host.Reset()
		require.Equal(t, types.OnPluginStartStatusOK, status)

		boundary := "boundary123456789"
		requestBody := buildMultipartBody(boundary, map[string]string{
			"messages": "[{\"role\":\"user\",\"content\":\"Hello\"}]",
		})

		action := host.CallOnHttpRequestHeaders([][2]string{
			{":authority", "test.com"},
			{":path", "/v1/chat/completions"},
			{"content-type", "multipart/form-data; boundary=" + boundary},
			{"content-length", "100"},
		})
		require.Equal(t, types.HeaderStopIteration, action)

		action = host.CallOnHttpRequestBody([]byte(requestBody))
		require.Equal(t, types.ActionContinue, action)

		// Verify no headers were added
		modelHeader, _ := test.GetHeaderValue(host.GetRequestHeaders(), "x-model")
		require.Equal(t, "", modelHeader)

		providerHeader, _ := test.GetHeaderValue(host.GetRequestHeaders(), "x-provider")
		require.Equal(t, "", providerHeader)
	})
}

func TestMultipartBodyOnlyProviderHeader(t *testing.T) {
	configData, _ := json.Marshal(map[string]interface{}{
		"modelKey":           "model",
		"addProviderHeader":  "x-provider",
		"enableOnPathSuffix": []string{"/v1/chat/completions"},
	})

	test.RunTest(t, func(t *testing.T) {
		host, status := test.NewTestHost(configData)
		defer host.Reset()
		require.Equal(t, types.OnPluginStartStatusOK, status)

		boundary := "boundary123456789"
		requestBody := buildMultipartBody(boundary, map[string]string{
			"model":    "anthropic/claude-3-opus",
			"messages": "[{\"role\":\"user\",\"content\":\"Hello\"}]",
		})

		action := host.CallOnHttpRequestHeaders([][2]string{
			{":authority", "test.com"},
			{":path", "/v1/chat/completions"},
			{"content-type", "multipart/form-data; boundary=" + boundary},
			{"content-length", "100"},
		})
		require.Equal(t, types.HeaderStopIteration, action)

		action = host.CallOnHttpRequestBody([]byte(requestBody))
		require.Equal(t, types.ActionContinue, action)

		// Verify only provider header was added
		modelHeader, _ := test.GetHeaderValue(host.GetRequestHeaders(), "x-model")
		require.Equal(t, "", modelHeader)

		providerHeader, _ := test.GetHeaderValue(host.GetRequestHeaders(), "x-provider")
		require.Equal(t, "anthropic", providerHeader)

		// Verify body was rewritten
		modifiedBody := string(host.GetRequestBody())
		require.Contains(t, modifiedBody, "name=\"model\"\r\n\r\nclaude-3-opus\r\n")
		require.NotContains(t, modifiedBody, "anthropic/claude-3-opus")
	})
}

func TestMultipartBodyMalformedBoundary(t *testing.T) {
	configData, _ := json.Marshal(map[string]interface{}{
		"modelKey":           "model",
		"modelToHeader":      "x-model",
		"addProviderHeader":  "x-provider",
		"enableOnPathSuffix": []string{"/v1/chat/completions"},
	})

	test.RunTest(t, func(t *testing.T) {
		host, status := test.NewTestHost(configData)
		defer host.Reset()
		require.Equal(t, types.OnPluginStartStatusOK, status)

		// Plain text body (not multipart format)
		requestBody := "model=openai/gpt-4&messages=hello"

		action := host.CallOnHttpRequestHeaders([][2]string{
			{":authority", "test.com"},
			{":path", "/v1/chat/completions"},
			{"content-type", "multipart/form-data; boundary=boundary123"},
			{"content-length", "100"},
		})
		require.Equal(t, types.HeaderStopIteration, action)

		action = host.CallOnHttpRequestBody([]byte(requestBody))
		require.Equal(t, types.ActionContinue, action)

		// Should not crash, no headers added
		modelHeader, _ := test.GetHeaderValue(host.GetRequestHeaders(), "x-model")
		require.Equal(t, "", modelHeader)
	})
}

func TestMultipartBodyMultipleParts(t *testing.T) {
	configData, _ := json.Marshal(map[string]interface{}{
		"modelKey":           "model",
		"modelToHeader":      "x-model",
		"addProviderHeader":  "x-provider",
		"enableOnPathSuffix": []string{"/v1/chat/completions"},
	})

	test.RunTest(t, func(t *testing.T) {
		host, status := test.NewTestHost(configData)
		defer host.Reset()
		require.Equal(t, types.OnPluginStartStatusOK, status)

		boundary := "boundary123456789"
		requestBody := buildMultipartBody(boundary, map[string]string{
			"temperature": "0.7",
			"model":       "google/gemini-pro",
			"messages":    "[{\"role\":\"user\",\"content\":\"Hello\"}]",
			"max_tokens":  "1000",
		})

		action := host.CallOnHttpRequestHeaders([][2]string{
			{":authority", "test.com"},
			{":path", "/v1/chat/completions"},
			{"content-type", "multipart/form-data; boundary=" + boundary},
			{"content-length", "100"},
		})
		require.Equal(t, types.HeaderStopIteration, action)

		action = host.CallOnHttpRequestBody([]byte(requestBody))
		require.Equal(t, types.ActionContinue, action)

		// Verify headers
		modelHeader, _ := test.GetHeaderValue(host.GetRequestHeaders(), "x-model")
		require.Equal(t, "google/gemini-pro", modelHeader)

		providerHeader, _ := test.GetHeaderValue(host.GetRequestHeaders(), "x-provider")
		require.Equal(t, "google", providerHeader)

		// Verify only model field was rewritten, others unchanged
		modifiedBody := string(host.GetRequestBody())
		require.Contains(t, modifiedBody, "name=\"model\"\r\n\r\ngemini-pro\r\n")
		require.Contains(t, modifiedBody, "name=\"temperature\"\r\n\r\n0.7\r\n")
		require.Contains(t, modifiedBody, "name=\"max_tokens\"\r\n\r\n1000\r\n")
		require.NotContains(t, modifiedBody, "google/gemini-pro")
	})
}

func TestMultipartBodyCustomModelKey(t *testing.T) {
	configData, _ := json.Marshal(map[string]interface{}{
		"modelKey":           "llm_model",
		"modelToHeader":      "x-model",
		"addProviderHeader":  "x-provider",
		"enableOnPathSuffix": []string{"/v1/chat/completions"},
	})

	test.RunTest(t, func(t *testing.T) {
		host, status := test.NewTestHost(configData)
		defer host.Reset()
		require.Equal(t, types.OnPluginStartStatusOK, status)

		boundary := "boundary123456789"
		requestBody := buildMultipartBody(boundary, map[string]string{
			"llm_model": "claude-3-sonnet",
			"messages":  "[{\"role\":\"user\",\"content\":\"Hello\"}]",
		})

		action := host.CallOnHttpRequestHeaders([][2]string{
			{":authority", "test.com"},
			{":path", "/v1/chat/completions"},
			{"content-type", "multipart/form-data; boundary=" + boundary},
			{"content-length", "100"},
		})
		require.Equal(t, types.HeaderStopIteration, action)

		action = host.CallOnHttpRequestBody([]byte(requestBody))
		require.Equal(t, types.ActionContinue, action)

		// Verify header was added with custom key
		modelHeader, _ := test.GetHeaderValue(host.GetRequestHeaders(), "x-model")
		require.Equal(t, "claude-3-sonnet", modelHeader)
	})
}

func TestMultipartBodyNoBoundary(t *testing.T) {
	configData, _ := json.Marshal(map[string]interface{}{
		"modelKey":           "model",
		"modelToHeader":      "x-model",
		"enableOnPathSuffix": []string{"/v1/chat/completions"},
	})

	test.RunTest(t, func(t *testing.T) {
		host, status := test.NewTestHost(configData)
		defer host.Reset()
		require.Equal(t, types.OnPluginStartStatusOK, status)

		// Content-type without boundary parameter
		action := host.CallOnHttpRequestHeaders([][2]string{
			{":authority", "test.com"},
			{":path", "/v1/chat/completions"},
			{"content-type", "multipart/form-data"},
			{"content-length", "100"},
		})
		// Should continue without processing
		require.Equal(t, types.ActionContinue, action)
	})
}

func TestMultipartBodyEmptyModel(t *testing.T) {
	configData, _ := json.Marshal(map[string]interface{}{
		"modelKey":           "model",
		"modelToHeader":      "x-model",
		"enableOnPathSuffix": []string{"/v1/chat/completions"},
	})

	test.RunTest(t, func(t *testing.T) {
		host, status := test.NewTestHost(configData)
		defer host.Reset()
		require.Equal(t, types.OnPluginStartStatusOK, status)

		boundary := "boundary123456789"
		requestBody := buildMultipartBody(boundary, map[string]string{
			"model":    "",
			"messages": "[{\"role\":\"user\",\"content\":\"Hello\"}]",
		})

		action := host.CallOnHttpRequestHeaders([][2]string{
			{":authority", "test.com"},
			{":path", "/v1/chat/completions"},
			{"content-type", "multipart/form-data; boundary=" + boundary},
			{"content-length", "100"},
		})
		require.Equal(t, types.HeaderStopIteration, action)

		action = host.CallOnHttpRequestBody([]byte(requestBody))
		require.Equal(t, types.ActionContinue, action)

		// Verify header was set to empty string
		modelHeader, _ := test.GetHeaderValue(host.GetRequestHeaders(), "x-model")
		require.Equal(t, "", modelHeader)
	})
}

func TestJSONBodyModelNotString(t *testing.T) {
	configData, _ := json.Marshal(map[string]interface{}{
		"modelKey":           "model",
		"modelToHeader":      "x-model",
		"enableOnPathSuffix": []string{"/v1/chat/completions"},
	})

	test.RunTest(t, func(t *testing.T) {
		host, status := test.NewTestHost(configData)
		defer host.Reset()
		require.Equal(t, types.OnPluginStartStatusOK, status)

		// Model value is a number instead of string
		requestBody := `{"model": 123, "messages": [{"role": "user", "content": "Hello"}]}`
		action := host.CallOnHttpRequestHeaders([][2]string{
			{":authority", "test.com"},
			{":path", "/v1/chat/completions"},
			{"content-type", "application/json"},
			{"content-length", "100"},
		})
		require.Equal(t, types.HeaderStopIteration, action)

		action = host.CallOnHttpRequestBody([]byte(requestBody))
		require.Equal(t, types.ActionContinue, action)

		// Should not add headers for non-string model
		modelHeader, _ := test.GetHeaderValue(host.GetRequestHeaders(), "x-model")
		require.Equal(t, "", modelHeader)
	})
}

func TestJSONBodyNestedModel(t *testing.T) {
	configData, _ := json.Marshal(map[string]interface{}{
		"modelKey":           "model",
		"modelToHeader":      "x-model",
		"enableOnPathSuffix": []string{"/v1/chat/completions"},
	})

	test.RunTest(t, func(t *testing.T) {
		host, status := test.NewTestHost(configData)
		defer host.Reset()
		require.Equal(t, types.OnPluginStartStatusOK, status)

		// Nested model field - should only extract top-level
		requestBody := `{"model": "gpt-4", "config": {"model": "nested"}, "messages": []}`
		action := host.CallOnHttpRequestHeaders([][2]string{
			{":authority", "test.com"},
			{":path", "/v1/chat/completions"},
			{"content-type", "application/json"},
			{"content-length", "100"},
		})
		require.Equal(t, types.HeaderStopIteration, action)

		action = host.CallOnHttpRequestBody([]byte(requestBody))
		require.Equal(t, types.ActionContinue, action)

		// Should extract only top-level model
		modelHeader, _ := test.GetHeaderValue(host.GetRequestHeaders(), "x-model")
		require.Equal(t, "gpt-4", modelHeader)
	})
}

// ========== Tests for Custom Header Configuration ==========

func TestStaticHeaders(t *testing.T) {
	configData, _ := json.Marshal(map[string]interface{}{
		"staticHeaders": []map[string]interface{}{
			{"key": "x-mse-gateway-instance-id", "value": "gateway-001"},
			{"key": "x-environment", "value": "production"},
		},
	})

	test.RunTest(t, func(t *testing.T) {
		host, status := test.NewTestHost(configData)
		defer host.Reset()
		require.Equal(t, types.OnPluginStartStatusOK, status)

		action := host.CallOnHttpRequestHeaders([][2]string{
			{":authority", "test.com"},
			{":path", "/test"},
		})
		require.Equal(t, types.ActionContinue, action)

		// Verify static headers were added
		headers := host.GetRequestHeaders()
		gatewayId, _ := test.GetHeaderValue(headers, "x-mse-gateway-instance-id")
		require.Equal(t, "gateway-001", gatewayId)

		env, _ := test.GetHeaderValue(headers, "x-environment")
		require.Equal(t, "production", env)
	})
}

func TestFixedSourceHeaders(t *testing.T) {
	configData, _ := json.Marshal(map[string]interface{}{
		"fixedSourceHeaders": []map[string]interface{}{
			{"source": "authority", "target": "x-mse-domain-name"},
		},
	})

	test.RunTest(t, func(t *testing.T) {
		host, status := test.NewTestHost(configData)
		defer host.Reset()
		require.Equal(t, types.OnPluginStartStatusOK, status)

		action := host.CallOnHttpRequestHeaders([][2]string{
			{":authority", "example.com"},
			{":path", "/test"},
		})
		require.Equal(t, types.ActionContinue, action)

		// Verify authority was copied
		headers := host.GetRequestHeaders()
		domain, _ := test.GetHeaderValue(headers, "x-mse-domain-name")
		require.Equal(t, "example.com", domain)
	})
}

func TestPrioritySourceHeaders(t *testing.T) {
	configData, _ := json.Marshal(map[string]interface{}{
		"prioritySourceHeaders": []map[string]interface{}{
			{
				"target":      "x-mse-consumer-apikey",
				"sources":     []string{"authorization", "x-api-key", "api-key"},
				"stripPrefix": "Bearer ",
			},
		},
	})

	test.RunTest(t, func(t *testing.T) {
		host, status := test.NewTestHost(configData)
		defer host.Reset()
		require.Equal(t, types.OnPluginStartStatusOK, status)

		// Test with Authorization header
		action := host.CallOnHttpRequestHeaders([][2]string{
			{":authority", "test.com"},
			{":path", "/test"},
			{"authorization", "Bearer sk-1234567890"},
		})
		require.Equal(t, types.ActionContinue, action)

		headers := host.GetRequestHeaders()
		apikey, _ := test.GetHeaderValue(headers, "x-mse-consumer-apikey")
		require.Equal(t, "sk-1234567890", apikey)
	})
}

func TestPrioritySourceHeadersFallback(t *testing.T) {
	configData, _ := json.Marshal(map[string]interface{}{
		"prioritySourceHeaders": []map[string]interface{}{
			{
				"target":  "x-mse-consumer-apikey",
				"sources": []string{"authorization", "x-api-key", "api-key"},
			},
		},
	})

	test.RunTest(t, func(t *testing.T) {
		host, status := test.NewTestHost(configData)
		defer host.Reset()
		require.Equal(t, types.OnPluginStartStatusOK, status)

		// Test with x-api-key (fallback to priority 2)
		action := host.CallOnHttpRequestHeaders([][2]string{
			{":authority", "test.com"},
			{":path", "/test"},
			{"x-api-key", "sk-xyz789"},
		})
		require.Equal(t, types.ActionContinue, action)

		headers := host.GetRequestHeaders()
		apikey, _ := test.GetHeaderValue(headers, "x-mse-consumer-apikey")
		require.Equal(t, "sk-xyz789", apikey)
	})
}

func TestPrioritySourceHeadersWithMultiple(t *testing.T) {
	configData, _ := json.Marshal(map[string]interface{}{
		"prioritySourceHeaders": []map[string]interface{}{
			{
				"target":  "x-mse-consumer-apikey",
				"sources": []string{"authorization", "x-api-key"},
			},
		},
	})

	test.RunTest(t, func(t *testing.T) {
		host, status := test.NewTestHost(configData)
		defer host.Reset()
		require.Equal(t, types.OnPluginStartStatusOK, status)

		// Test with both headers - should use priority 1
		action := host.CallOnHttpRequestHeaders([][2]string{
			{":authority", "test.com"},
			{":path", "/test"},
			{"authorization", "sk-priority1"},
			{"x-api-key", "sk-priority2"},
		})
		require.Equal(t, types.ActionContinue, action)

		headers := host.GetRequestHeaders()
		apikey, _ := test.GetHeaderValue(headers, "x-mse-consumer-apikey")
		require.Equal(t, "sk-priority1", apikey)
	})
}

func TestMixedConfiguration(t *testing.T) {
	configData, _ := json.Marshal(map[string]interface{}{
		// AI configuration
		"modelKey":           "model",
		"modelToHeader":      "x-model",
		"enableOnPathSuffix": []string{"/v1/chat/completions"},

		// Static headers
		"staticHeaders": []map[string]interface{}{
			{"key": "x-mse-gateway-instance-id", "value": "gateway-001"},
		},

		// Fixed source headers
		"fixedSourceHeaders": []map[string]interface{}{
			{"source": "authority", "target": "x-mse-domain-name"},
		},

		// Priority source headers
		"prioritySourceHeaders": []map[string]interface{}{
			{
				"target":      "x-mse-consumer-apikey",
				"sources":     []string{"authorization"},
				"stripPrefix": "Bearer ",
			},
		},
	})

	test.RunTest(t, func(t *testing.T) {
		host, status := test.NewTestHost(configData)
		defer host.Reset()
		require.Equal(t, types.OnPluginStartStatusOK, status)

		requestBody := `{"model": "gpt-4", "messages": [{"role": "user", "content": "Hello"}]}`
		action := host.CallOnHttpRequestHeaders([][2]string{
			{":authority", "api.example.com"},
			{":path", "/v1/chat/completions"},
			{"content-type", "application/json"},
			{"content-length", "100"},
			{"authorization", "Bearer sk-test123"},
		})
		require.Equal(t, types.HeaderStopIteration, action)

		action = host.CallOnHttpRequestBody([]byte(requestBody))
		require.Equal(t, types.ActionContinue, action)

		headers := host.GetRequestHeaders()

		// Verify AI headers
		modelHeader, _ := test.GetHeaderValue(headers, "x-model")
		require.Equal(t, "gpt-4", modelHeader)

		// Verify static headers
		gatewayId, _ := test.GetHeaderValue(headers, "x-mse-gateway-instance-id")
		require.Equal(t, "gateway-001", gatewayId)

		// Verify fixed source headers
		domain, _ := test.GetHeaderValue(headers, "x-mse-domain-name")
		require.Equal(t, "api.example.com", domain)

		// Verify priority source headers
		apikey, _ := test.GetHeaderValue(headers, "x-mse-consumer-apikey")
		require.Equal(t, "sk-test123", apikey)
	})
}

func TestOnlyCustomHeaders(t *testing.T) {
	configData, _ := json.Marshal(map[string]interface{}{
		"staticHeaders": []map[string]interface{}{
			{"key": "x-custom", "value": "test"},
		},
	})

	test.RunTest(t, func(t *testing.T) {
		host, status := test.NewTestHost(configData)
		defer host.Reset()
		require.Equal(t, types.OnPluginStartStatusOK, status)

		action := host.CallOnHttpRequestHeaders([][2]string{
			{":authority", "test.com"},
			{":path", "/test"},
		})
		require.Equal(t, types.ActionContinue, action)

		headers := host.GetRequestHeaders()
		custom, _ := test.GetHeaderValue(headers, "x-custom")
		require.Equal(t, "test", custom)
	})
}

func TestFixedSourceHeadersAuthorityWithPort(t *testing.T) {
	configData, _ := json.Marshal(map[string]interface{}{
		"fixedSourceHeaders": []map[string]interface{}{
			{"source": "authority", "target": "x-mse-domain-name"},
		},
	})

	test.RunTest(t, func(t *testing.T) {
		host, status := test.NewTestHost(configData)
		defer host.Reset()
		require.Equal(t, types.OnPluginStartStatusOK, status)

		// Test with authority containing port
		action := host.CallOnHttpRequestHeaders([][2]string{
			{":authority", "isadba.com:8080"},
			{":path", "/test"},
		})
		require.Equal(t, types.ActionContinue, action)

		// Verify port was stripped
		headers := host.GetRequestHeaders()
		domain, _ := test.GetHeaderValue(headers, "x-mse-domain-name")
		require.Equal(t, "isadba.com", domain)
	})
}

func TestFixedSourceHeadersAuthorityWithoutPort(t *testing.T) {
	configData, _ := json.Marshal(map[string]interface{}{
		"fixedSourceHeaders": []map[string]interface{}{
			{"source": "authority", "target": "x-mse-domain-name"},
		},
	})

	test.RunTest(t, func(t *testing.T) {
		host, status := test.NewTestHost(configData)
		defer host.Reset()
		require.Equal(t, types.OnPluginStartStatusOK, status)

		// Test with authority without port
		action := host.CallOnHttpRequestHeaders([][2]string{
			{":authority", "example.com"},
			{":path", "/test"},
		})
		require.Equal(t, types.ActionContinue, action)

		// Verify domain remains unchanged
		headers := host.GetRequestHeaders()
		domain, _ := test.GetHeaderValue(headers, "x-mse-domain-name")
		require.Equal(t, "example.com", domain)
	})
}

func TestFixedSourceHeadersAuthorityWithIPv4AndPort(t *testing.T) {
	configData, _ := json.Marshal(map[string]interface{}{
		"fixedSourceHeaders": []map[string]interface{}{
			{"source": "authority", "target": "x-mse-domain-name"},
		},
	})

	test.RunTest(t, func(t *testing.T) {
		host, status := test.NewTestHost(configData)
		defer host.Reset()
		require.Equal(t, types.OnPluginStartStatusOK, status)

		// Test with IPv4 address and port
		action := host.CallOnHttpRequestHeaders([][2]string{
			{":authority", "192.168.1.1:8080"},
			{":path", "/test"},
		})
		require.Equal(t, types.ActionContinue, action)

		// Verify port was stripped
		headers := host.GetRequestHeaders()
		domain, _ := test.GetHeaderValue(headers, "x-mse-domain-name")
		require.Equal(t, "192.168.1.1", domain)
	})
}
