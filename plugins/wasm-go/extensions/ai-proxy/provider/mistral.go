package provider

import (
	"errors"
	"net/http"

	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-proxy/util"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/wrapper"
)

const (
	mistralDomain = "api.mistral.ai"
)

type mistralProviderInitializer struct{}

func (m *mistralProviderInitializer) ValidateConfig(config *ProviderConfig) error {
	if config.apiTokens == nil || len(config.apiTokens) == 0 {
		return errors.New("no apiToken found in provider config")
	}
	return nil
}

func (m *mistralProviderInitializer) DefaultCapabilities() map[string]string {
	return map[string]string{
		// The chat interface of mistral is the same as that of OpenAI. docs: https://docs.mistral.ai/api/
		string(ApiNameChatCompletion): PathOpenAIChatCompletions,
		string(ApiNameEmbeddings):     PathOpenAIEmbeddings,
	}
}

func (m *mistralProviderInitializer) CreateProvider(config ProviderConfig) (Provider, error) {
	config.setDefaultCapabilities(m.DefaultCapabilities())
	return &mistralProvider{
		config:       config,
		contextCache: createContextCache(&config),
	}, nil
}

type mistralProvider struct {
	config       ProviderConfig
	contextCache *contextCache
}

func (m *mistralProvider) GetProviderType() string {
	return providerTypeMistral
}

func (m *mistralProvider) OnRequestHeaders(ctx wrapper.HttpContext, apiName ApiName) error {
	// 调用原有的处理逻辑（包括 TransformRequestHeaders）
	m.config.handleRequestHeaders(m, ctx, apiName)

	// 在 handleRequestHeaders 之后删除，确保不会被 saveContextsToHeaders 覆盖
	_ = proxywasm.RemoveHttpRequestHeader("x-hi-original-auth")
	_ = proxywasm.RemoveHttpRequestHeader("x-mse-consumer-apikey")
	_ = proxywasm.RemoveHttpRequestHeader("x-mse-tenant-id")

	return nil
}

func (m *mistralProvider) OnRequestBody(ctx wrapper.HttpContext, apiName ApiName, body []byte) (types.Action, error) {
	if !m.config.isSupportedAPI(apiName) {
		return types.ActionContinue, errUnsupportedApiName
	}
	return m.config.handleRequestBody(m, m.contextCache, ctx, apiName, body)
}

func (m *mistralProvider) TransformRequestHeaders(ctx wrapper.HttpContext, apiName ApiName, headers http.Header) {
	util.OverwriteRequestHostHeader(headers, mistralDomain)
	util.OverwriteRequestAuthorizationHeader(headers, "Bearer "+m.config.GetApiTokenInUse(ctx))
	headers.Del("Content-Length")
}
