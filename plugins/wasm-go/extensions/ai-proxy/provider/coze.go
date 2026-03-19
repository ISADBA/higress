package provider

import (
	"errors"
	"net/http"

	"github.com/alibaba/higress/plugins/wasm-go/extensions/ai-proxy/util"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/wasm-go/pkg/wrapper"
)

const (
	cozeDomain = "api.coze.cn"
)

type cozeProviderInitializer struct{}

func (m *cozeProviderInitializer) ValidateConfig(config *ProviderConfig) error {
	if config.apiTokens == nil || len(config.apiTokens) == 0 {
		return errors.New("no apiToken found in provider config")
	}
	return nil
}

func (m *cozeProviderInitializer) DefaultCapabilities() map[string]string {
	return map[string]string{}
}

func (m *cozeProviderInitializer) CreateProvider(config ProviderConfig) (Provider, error) {
	config.setDefaultCapabilities(m.DefaultCapabilities())
	return &cozeProvider{
		config:       config,
		contextCache: createContextCache(&config),
	}, nil
}

type cozeProvider struct {
	config       ProviderConfig
	contextCache *contextCache
}

func (m *cozeProvider) GetProviderType() string {
	return providerTypeCoze
}

func (m *cozeProvider) OnRequestHeaders(ctx wrapper.HttpContext, apiName ApiName) error {
	// 调用原有的处理逻辑（包括 TransformRequestHeaders）
	m.config.handleRequestHeaders(m, ctx, apiName)

	// 在 handleRequestHeaders 之后删除，确保不会被 saveContextsToHeaders 覆盖
	_ = proxywasm.RemoveHttpRequestHeader("x-hi-original-auth")
	_ = proxywasm.RemoveHttpRequestHeader("x-mse-consumer-apikey")
	_ = proxywasm.RemoveHttpRequestHeader("x-mse-tenant-id")

	return nil
}

func (m *cozeProvider) TransformRequestHeaders(ctx wrapper.HttpContext, apiName ApiName, headers http.Header) {
	util.OverwriteRequestHostHeader(headers, cozeDomain)
	util.OverwriteRequestAuthorizationHeader(headers, "Bearer "+m.config.GetApiTokenInUse(ctx))
	headers.Del("Content-Length")
}
