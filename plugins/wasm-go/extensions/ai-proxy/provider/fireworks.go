package provider

import (
	"errors"
	"net/http"
	"strings"

	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-proxy/util"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/wrapper"
)

// fireworksProvider is the provider for Fireworks AI service.

const (
	fireworksDomain = "api.fireworks.ai"
)

type fireworksProviderInitializer struct{}

func (f *fireworksProviderInitializer) ValidateConfig(config *ProviderConfig) error {
	if config.apiTokens == nil || len(config.apiTokens) == 0 {
		return errors.New("no apiToken found in provider config")
	}
	return nil
}

func (f *fireworksProviderInitializer) DefaultCapabilities() map[string]string {
	return map[string]string{
		string(ApiNameChatCompletion): PathOpenAIChatCompletions,
		string(ApiNameCompletion):     PathOpenAICompletions,
		string(ApiNameModels):         PathOpenAIModels,
	}
}

func (f *fireworksProviderInitializer) CreateProvider(config ProviderConfig) (Provider, error) {
	config.setDefaultCapabilities(f.DefaultCapabilities())
	return &fireworksProvider{
		config:       config,
		contextCache: createContextCache(&config),
	}, nil
}

type fireworksProvider struct {
	config       ProviderConfig
	contextCache *contextCache
}

func (f *fireworksProvider) GetProviderType() string {
	return providerTypeFireworks
}

func (f *fireworksProvider) OnRequestHeaders(ctx wrapper.HttpContext, apiName ApiName) error {
	// 调用原有的处理逻辑（包括 TransformRequestHeaders）
	f.config.handleRequestHeaders(f, ctx, apiName)

	// 在 handleRequestHeaders 之后删除，确保不会被 saveContextsToHeaders 覆盖
	_ = proxywasm.RemoveHttpRequestHeader("x-hi-original-auth")
	_ = proxywasm.RemoveHttpRequestHeader("x-mse-consumer-apikey")
	_ = proxywasm.RemoveHttpRequestHeader("x-mse-tenant-id")

	return nil
}

func (f *fireworksProvider) OnRequestBody(ctx wrapper.HttpContext, apiName ApiName, body []byte) (types.Action, error) {
	if !f.config.isSupportedAPI(apiName) {
		return types.ActionContinue, errUnsupportedApiName
	}
	return f.config.handleRequestBody(f, f.contextCache, ctx, apiName, body)
}

func (f *fireworksProvider) TransformRequestHeaders(ctx wrapper.HttpContext, apiName ApiName, headers http.Header) {
	util.OverwriteRequestPathHeaderByCapability(headers, string(apiName), f.config.capabilities)
	util.OverwriteRequestHostHeader(headers, fireworksDomain)
	util.OverwriteRequestAuthorizationHeader(headers, "Bearer "+f.config.GetApiTokenInUse(ctx))
	headers.Del("Content-Length")
}

func (f *fireworksProvider) GetApiName(path string) ApiName {
	if strings.Contains(path, PathOpenAIChatCompletions) {
		return ApiNameChatCompletion
	}
	if strings.Contains(path, PathOpenAICompletions) {
		return ApiNameCompletion
	}
	if strings.Contains(path, PathOpenAIModels) {
		return ApiNameModels
	}
	return ""
}
