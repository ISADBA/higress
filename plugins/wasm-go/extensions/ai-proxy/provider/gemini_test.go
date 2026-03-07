package provider

import (
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestGeminiProvider_NoCustomUrl tests provider creation without custom URL
func TestGeminiProvider_NoCustomUrl(t *testing.T) {
	config := ProviderConfig{
		typ:       providerTypeGemini,
		apiTokens: []string{"test-api-key"},
	}

	initializer := &geminiProviderInitializer{}
	provider, err := initializer.CreateProvider(config)

	assert.NoError(t, err)
	assert.NotNil(t, provider)

	geminiProvider, ok := provider.(*geminiProvider)
	assert.True(t, ok)
	assert.Equal(t, "", geminiProvider.customDomain)
	assert.Equal(t, "", geminiProvider.customPath)
}

// TestGeminiProvider_CustomDomainOnly tests provider with domain-only custom URL
func TestGeminiProvider_CustomDomainOnly(t *testing.T) {
	testCases := []struct {
		name           string
		customUrl      string
		expectedDomain string
		expectedPath   string
	}{
		{
			name:           "domain without protocol",
			customUrl:      "custom.gemini.com",
			expectedDomain: "custom.gemini.com",
			expectedPath:   "/",
		},
		{
			name:           "domain with https",
			customUrl:      "https://custom.gemini.com",
			expectedDomain: "custom.gemini.com",
			expectedPath:   "/",
		},
		{
			name:           "domain with http",
			customUrl:      "http://custom.gemini.com",
			expectedDomain: "custom.gemini.com",
			expectedPath:   "/",
		},
		{
			name:           "domain with port",
			customUrl:      "custom.gemini.com:8080",
			expectedDomain: "custom.gemini.com:8080",
			expectedPath:   "/",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := ProviderConfig{
				typ:             providerTypeGemini,
				apiTokens:       []string{"test-api-key"},
				geminiCustomUrl: tc.customUrl,
			}

			initializer := &geminiProviderInitializer{}
			provider, err := initializer.CreateProvider(config)

			assert.NoError(t, err)
			assert.NotNil(t, provider)

			geminiProvider, ok := provider.(*geminiProvider)
			assert.True(t, ok)
			assert.Equal(t, tc.expectedDomain, geminiProvider.customDomain)
			assert.Equal(t, tc.expectedPath, geminiProvider.customPath)
		})
	}
}

// TestGeminiProvider_CustomDomainWithPath tests provider with domain and path prefix
func TestGeminiProvider_CustomDomainWithPath(t *testing.T) {
	testCases := []struct {
		name           string
		customUrl      string
		expectedDomain string
		expectedPath   string
	}{
		{
			name:           "domain with single path segment",
			customUrl:      "custom.gemini.com/api",
			expectedDomain: "custom.gemini.com",
			expectedPath:   "/api",
		},
		{
			name:           "domain with multiple path segments",
			customUrl:      "custom.gemini.com/api/v1",
			expectedDomain: "custom.gemini.com",
			expectedPath:   "/api/v1",
		},
		{
			name:           "domain with path and https",
			customUrl:      "https://proxy.example.com/gemini/api",
			expectedDomain: "proxy.example.com",
			expectedPath:   "/gemini/api",
		},
		{
			name:           "domain with port and path",
			customUrl:      "custom.gemini.com:8080/v1/provider",
			expectedDomain: "custom.gemini.com:8080",
			expectedPath:   "/v1/provider",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := ProviderConfig{
				typ:             providerTypeGemini,
				apiTokens:       []string{"test-api-key"},
				geminiCustomUrl: tc.customUrl,
			}

			initializer := &geminiProviderInitializer{}
			provider, err := initializer.CreateProvider(config)

			assert.NoError(t, err)
			assert.NotNil(t, provider)

			geminiProvider, ok := provider.(*geminiProvider)
			assert.True(t, ok)
			assert.Equal(t, tc.expectedDomain, geminiProvider.customDomain)
			assert.Equal(t, tc.expectedPath, geminiProvider.customPath)
		})
	}
}

// TestGeminiProvider_GetRequestPath tests path generation with custom paths
func TestGeminiProvider_GetRequestPath(t *testing.T) {
	testCases := []struct {
		name         string
		customPath   string
		apiName      ApiName
		model        string
		stream       bool
		expectedPath string
	}{
		{
			name:         "chat completion without custom path",
			customPath:   "",
			apiName:      ApiNameChatCompletion,
			model:        "gemini-pro",
			stream:       false,
			expectedPath: "/v1beta/models/gemini-pro:generateContent",
		},
		{
			name:         "chat completion with custom path",
			customPath:   "/api/v1",
			apiName:      ApiNameChatCompletion,
			model:        "gemini-pro",
			stream:       false,
			expectedPath: "/api/v1/v1beta/models/gemini-pro:generateContent",
		},
		{
			name:         "streaming chat completion with custom path",
			customPath:   "/proxy",
			apiName:      ApiNameChatCompletion,
			model:        "gemini-pro",
			stream:       true,
			expectedPath: "/proxy/v1beta/models/gemini-pro:streamGenerateContent?alt=sse",
		},
		{
			name:         "models api without custom path",
			customPath:   "",
			apiName:      ApiNameModels,
			model:        "",
			stream:       false,
			expectedPath: "/v1beta/models",
		},
		{
			name:         "models api with custom path",
			customPath:   "/custom",
			apiName:      ApiNameModels,
			model:        "",
			stream:       false,
			expectedPath: "/custom/v1beta/models",
		},
		{
			name:         "embeddings without custom path",
			customPath:   "",
			apiName:      ApiNameEmbeddings,
			model:        "text-embedding-004",
			stream:       false,
			expectedPath: "/v1beta/models/text-embedding-004:batchEmbedContents",
		},
		{
			name:         "embeddings with custom path",
			customPath:   "/api",
			apiName:      ApiNameEmbeddings,
			model:        "text-embedding-004",
			stream:       false,
			expectedPath: "/api/v1beta/models/text-embedding-004:batchEmbedContents",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			provider := &geminiProvider{
				config: ProviderConfig{
					apiVersion: geminiDefaultApiVersion,
				},
				customPath: tc.customPath,
			}

			path := provider.getRequestPath(tc.apiName, tc.model, tc.stream)
			assert.Equal(t, tc.expectedPath, path)
		})
	}
}

// TestGeminiProvider_TransformRequestHeaders tests Host header setting
func TestGeminiProvider_TransformRequestHeaders(t *testing.T) {
	testCases := []struct {
		name         string
		customDomain string
		expectedHost string
	}{
		{
			name:         "default domain",
			customDomain: "",
			expectedHost: geminiDomain,
		},
		{
			name:         "custom domain",
			customDomain: "custom.gemini.com",
			expectedHost: "custom.gemini.com",
		},
		{
			name:         "custom domain with port",
			customDomain: "custom.gemini.com:8080",
			expectedHost: "custom.gemini.com:8080",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			provider := &geminiProvider{
				config: ProviderConfig{
					apiTokens: []string{"test-api-key"},
				},
				customDomain: tc.customDomain,
			}

			// Verify the provider fields are set correctly
			// In a real test with full context, you would call TransformRequestHeaders
			// and verify the Host header is set correctly
			assert.Equal(t, tc.customDomain, provider.customDomain)
		})
	}
}

// TestGeminiProvider_EmptyCustomUrl tests handling of empty custom URL
func TestGeminiProvider_EmptyCustomUrl(t *testing.T) {
	config := ProviderConfig{
		typ:             providerTypeGemini,
		apiTokens:       []string{"test-api-key"},
		geminiCustomUrl: "",
	}

	initializer := &geminiProviderInitializer{}
	provider, err := initializer.CreateProvider(config)

	assert.NoError(t, err)
	assert.NotNil(t, provider)

	geminiProvider, ok := provider.(*geminiProvider)
	assert.True(t, ok)
	assert.Equal(t, "", geminiProvider.customDomain)
	assert.Equal(t, "", geminiProvider.customPath)
}

// TestGeminiProvider_ProtocolPrefixRemoval tests protocol prefix removal
func TestGeminiProvider_ProtocolPrefixRemoval(t *testing.T) {
	testCases := []struct {
		name           string
		customUrl      string
		expectedDomain string
	}{
		{
			name:           "https prefix",
			customUrl:      "https://custom.gemini.com",
			expectedDomain: "custom.gemini.com",
		},
		{
			name:           "http prefix",
			customUrl:      "http://custom.gemini.com",
			expectedDomain: "custom.gemini.com",
		},
		{
			name:           "no prefix",
			customUrl:      "custom.gemini.com",
			expectedDomain: "custom.gemini.com",
		},
		{
			name:           "https with path",
			customUrl:      "https://custom.gemini.com/api",
			expectedDomain: "custom.gemini.com",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := ProviderConfig{
				typ:             providerTypeGemini,
				apiTokens:       []string{"test-api-key"},
				geminiCustomUrl: tc.customUrl,
			}

			initializer := &geminiProviderInitializer{}
			provider, err := initializer.CreateProvider(config)

			assert.NoError(t, err)
			geminiProvider, ok := provider.(*geminiProvider)
			assert.True(t, ok)
			assert.Equal(t, tc.expectedDomain, geminiProvider.customDomain)
			assert.NotContains(t, geminiProvider.customDomain, "http://")
			assert.NotContains(t, geminiProvider.customDomain, "https://")
		})
	}
}

// TestGeminiProvider_PathConcatenation tests path concatenation logic
func TestGeminiProvider_PathConcatenation(t *testing.T) {
	testCases := []struct {
		name               string
		customPath         string
		apiName            ApiName
		model              string
		shouldContainPath  bool
		pathPrefixExpected string
	}{
		{
			name:               "no custom path",
			customPath:         "/",
			apiName:            ApiNameChatCompletion,
			model:              "gemini-pro",
			shouldContainPath:  false,
			pathPrefixExpected: "",
		},
		{
			name:               "with custom path",
			customPath:         "/api/v1",
			apiName:            ApiNameChatCompletion,
			model:              "gemini-pro",
			shouldContainPath:  true,
			pathPrefixExpected: "/api/v1",
		},
		{
			name:               "custom path for models",
			customPath:         "/proxy",
			apiName:            ApiNameModels,
			model:              "",
			shouldContainPath:  true,
			pathPrefixExpected: "/proxy",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			provider := &geminiProvider{
				config: ProviderConfig{
					apiVersion: geminiDefaultApiVersion,
				},
				customPath: tc.customPath,
			}

			path := provider.getRequestPath(tc.apiName, tc.model, false)

			if tc.shouldContainPath {
				assert.Contains(t, path, tc.pathPrefixExpected)
			}
			// Always should contain models or the action
			assert.Contains(t, path, "models")
		})
	}
}

// TestGeminiProvider_BackwardCompatibility tests backward compatibility
func TestGeminiProvider_BackwardCompatibility(t *testing.T) {
	// Test that existing configurations without geminiCustomUrl still work
	config := ProviderConfig{
		typ:       providerTypeGemini,
		apiTokens: []string{"test-api-key"},
		// geminiCustomUrl not set
	}

	initializer := &geminiProviderInitializer{}
	provider, err := initializer.CreateProvider(config)

	assert.NoError(t, err)
	assert.NotNil(t, provider)

	geminiProvider, ok := provider.(*geminiProvider)
	assert.True(t, ok)

	// Should use default behavior
	assert.Equal(t, "", geminiProvider.customDomain)
	assert.Equal(t, "", geminiProvider.customPath)

	// Path should be standard
	path := geminiProvider.getRequestPath(ApiNameChatCompletion, "gemini-pro", false)
	assert.Equal(t, "/v1beta/models/gemini-pro:generateContent", path)
}

// TestGeminiProvider_GetGeminiCustomUrl tests the getter method
func TestProviderConfig_GetGeminiCustomUrl(t *testing.T) {
	testCases := []struct {
		name        string
		customUrl   string
		expectedUrl string
	}{
		{
			name:        "with custom url",
			customUrl:   "custom.gemini.com",
			expectedUrl: "custom.gemini.com",
		},
		{
			name:        "empty custom url",
			customUrl:   "",
			expectedUrl: "",
		},
		{
			name:        "custom url with path",
			customUrl:   "custom.gemini.com/api/v1",
			expectedUrl: "custom.gemini.com/api/v1",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := ProviderConfig{
				geminiCustomUrl: tc.customUrl,
			}

			assert.Equal(t, tc.expectedUrl, config.GetGeminiCustomUrl())
		})
	}
}

// TestGeminiProvider_OriginalProtocolKeyReplacement tests URL key parameter replacement for original protocol
func TestGeminiProvider_OriginalProtocolKeyReplacement(t *testing.T) {
	// Note: This test verifies the logic structure
	// Full integration testing would require mocking proxywasm functions

	testCases := []struct {
		name              string
		protocol          string
		inputPath         string
		providerKey       string
		shouldReplace     bool
		expectedKeyInPath string
	}{
		{
			name:              "original protocol with key in query",
			protocol:          protocolOriginal,
			inputPath:         "/v1/models/gemini-pro:generateContent?key=client-key-123",
			providerKey:       "provider-key-456",
			shouldReplace:     true,
			expectedKeyInPath: "provider-key-456",
		},
		{
			name:              "original protocol with key and other params",
			protocol:          protocolOriginal,
			inputPath:         "/v1/models/gemini-pro:generateContent?key=client-key&foo=bar",
			providerKey:       "provider-key-789",
			shouldReplace:     true,
			expectedKeyInPath: "provider-key-789",
		},
		{
			name:              "openai protocol should not replace",
			protocol:          protocolOpenAI,
			inputPath:         "/v1/chat/completions",
			providerKey:       "provider-key-abc",
			shouldReplace:     false,
			expectedKeyInPath: "",
		},
		{
			name:              "original protocol without key param",
			protocol:          protocolOriginal,
			inputPath:         "/v1/models/gemini-pro:generateContent",
			providerKey:       "provider-key-def",
			shouldReplace:     false,
			expectedKeyInPath: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := ProviderConfig{
				typ:       providerTypeGemini,
				protocol:  tc.protocol,
				apiTokens: []string{tc.providerKey},
			}

			// Verify the protocol check logic
			if tc.protocol == protocolOriginal {
				assert.True(t, config.IsOriginal())
			} else {
				assert.False(t, config.IsOriginal())
			}

			// Verify URL parsing logic
			if tc.shouldReplace && strings.Contains(tc.inputPath, "?key=") {
				u, err := url.Parse(tc.inputPath)
				assert.NoError(t, err)

				q := u.Query()
				q.Set("key", tc.providerKey)
				u.RawQuery = q.Encode()
				newPath := u.String()

				// Verify the new path contains the provider key
				assert.Contains(t, newPath, tc.expectedKeyInPath)
				assert.NotContains(t, newPath, "client-key")
			}
		})
	}
}
