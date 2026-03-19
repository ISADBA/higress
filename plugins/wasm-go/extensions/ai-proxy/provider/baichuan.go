package provider

import (
	"errors"
	"net/http"

	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-proxy/util"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/wrapper"
)

// baichuanProvider is the provider for baichuan Ai service.

const (
	baichuanDomain = "api.baichuan-ai.com"
)

type baichuanProviderInitializer struct {
}

func (m *baichuanProviderInitializer) ValidateConfig(config *ProviderConfig) error {
	if config.apiTokens == nil || len(config.apiTokens) == 0 {
		return errors.New("no apiToken found in provider config")
	}
	return nil
}

func (m *baichuanProviderInitializer) DefaultCapabilities() map[string]string {
	return map[string]string{
		string(ApiNameChatCompletion): PathOpenAIChatCompletions,
		string(ApiNameEmbeddings):     PathOpenAIEmbeddings,
	}
}

func (m *baichuanProviderInitializer) CreateProvider(config ProviderConfig) (Provider, error) {
	config.setDefaultCapabilities(m.DefaultCapabilities())
	return &baichuanProvider{
		config:       config,
		contextCache: createContextCache(&config),
	}, nil
}

type baichuanProvider struct {
	config       ProviderConfig
	contextCache *contextCache
}

func (m *baichuanProvider) GetProviderType() string {
	return providerTypeBaichuan
}

func (m *baichuanProvider) OnRequestHeaders(ctx wrapper.HttpContext, apiName ApiName) error {
	// 调用原有的处理逻辑（包括 TransformRequestHeaders）
	m.config.handleRequestHeaders(m, ctx, apiName)

	// 在 handleRequestHeaders 之后删除，确保不会被 saveContextsToHeaders 覆盖
	_ = proxywasm.RemoveHttpRequestHeader("x-hi-original-auth")
	_ = proxywasm.RemoveHttpRequestHeader("x-mse-consumer-apikey")
	_ = proxywasm.RemoveHttpRequestHeader("x-mse-tenant-id")

	return nil
}

func (m *baichuanProvider) OnRequestBody(ctx wrapper.HttpContext, apiName ApiName, body []byte) (types.Action, error) {
	if !m.config.isSupportedAPI(apiName) {
		return types.ActionContinue, errUnsupportedApiName
	}
	return m.config.handleRequestBody(m, m.contextCache, ctx, apiName, body)
}

func (m *baichuanProvider) TransformRequestHeaders(ctx wrapper.HttpContext, apiName ApiName, headers http.Header) {
	util.OverwriteRequestPathHeaderByCapability(headers, string(apiName), m.config.capabilities)
	util.OverwriteRequestHostHeader(headers, baichuanDomain)
	util.OverwriteRequestAuthorizationHeader(headers, "Bearer "+m.config.GetApiTokenInUse(ctx))
	headers.Del("Content-Length")
}
