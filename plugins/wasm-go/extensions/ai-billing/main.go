package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/log"
	"github.com/higress-group/wasm-go/pkg/tokenusage"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/tidwall/gjson"
)

const (
	pluginName = "ai-billing"
)

// Context keys for storing data across request lifecycle
const (
	CtxKeyTenantInfo        = "ai-billing-tenant-info"
	CtxKeyApiKey            = "ai-billing-api-key" // Optional, for debug logging only
	CtxKeyConsumerApiKey    = "ai-billing-consumer-apikey"
	CtxKeyApikeyID          = "ai-billing-apikey-id"
	CtxKeyBillingInfo       = "ai-billing-info"
	CtxKeyIsStreaming       = "ai-billing-is-streaming"
	CtxKeyRequestDenied     = "ai-billing-request-denied"
	CtxKeyStatusCode        = "ai-billing-status-code"
	CtxKeyStreamDiagnostics = "ai-billing-stream-diagnostics"
	CtxKeyFailedStreamBody  = "ai-billing-failed-stream-body"
)

func main() {}

func init() {
	wrapper.SetCtx(
		pluginName,
		wrapper.ParseConfig(parseConfig),
		wrapper.ProcessRequestHeaders(onHttpRequestHeaders),
		wrapper.ProcessResponseHeaders(onHttpResponseHeaders),
		wrapper.ProcessResponseBody(onHttpResponseBody),
		wrapper.ProcessStreamingResponseBody(onHttpStreamingResponseBody),
	)
}

// BillingConfig holds the plugin configuration
type BillingConfig struct {
	BillingService                       BillingServiceConfig `yaml:"billingService"`
	FailPricingMessage                   string               `yaml:"failPricingMessage"`
	FailBalanceMessage                   string               `yaml:"failBalanceMessage"`
	InsufficientBalanceMessage           string               `yaml:"insufficientBalanceMessage"`
	FailCostMessage                      string               `yaml:"failCostMessage"`
	DebugLogFailedStreamResponse         bool                 `yaml:"debugLogFailedStreamResponse"`
	DebugLogFailedStreamResponseMaxBytes int                  `yaml:"debugLogFailedStreamResponseMaxBytes"`
	billingClient                        wrapper.HttpClient
	pricingCache                         map[string]bool // key: provider:model
}

// BillingServiceConfig holds the billing service connection details
type BillingServiceConfig struct {
	ServiceAddress string `yaml:"serviceAddress"`
	Protocol       string `yaml:"protocol"`
	Port           int    `yaml:"port"`
}

// TenantInfo holds tenant information and HMAC authentication headers
type TenantInfo struct {
	// HMAC Authentication Headers
	SignVersion string // x-internal-auth-sign-version
	Timestamp   string // x-internal-auth-ts
	Nonce       string // x-internal-auth-nonce
	Signature   string // x-internal-auth-sign

	// Tenant/Consumer Information
	ConsumerID       string // x-consumer-id
	ConsumerName     string // x-mse-consumer-name
	TenantID         string // x-mse-tenant-id
	DomainResourceID string // x-domain-resource-id
	RouterResourceID string // x-router-resource-id
}

// BalanceRequest represents the request to check user balance
type BalanceRequest struct {
	ApiKey string `json:"apikey"`
}

// BalanceResponse represents the response from balance check
type BalanceResponse struct {
	Success   bool   `json:"success"`
	Message   string `json:"message"`
	Balance   string `json:"balance"`
	UID       int64  `json:"uid"`
	UpdatedAt int64  `json:"updated_at"`
}

// CostRequest represents the request to deduct cost
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
	ApiKey           string  `json:"apikey,omitempty"`
	ApikeyID         *int64  `json:"apikey_id,omitempty"`
	// Note: consumer_id, consumer_name, tenant_id are in headers, not body
}

// CostResponse represents the response from cost deduction
type CostResponse struct {
	Success          bool   `json:"success"`
	Message          string `json:"message"`
	BillingEventID   int64  `json:"billing_event_id"`
	Cost             string `json:"cost"`
	CostActual       string `json:"cost_actual"`
	DiscountRatio    string `json:"discount_ratio"`
	RemainingBalance string `json:"remaining_balance"`
}

// PricingResponse represents the response from pricing query
type PricingResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    struct {
		Provider  string `json:"provider"`
		ModelName string `json:"model_name"`
		// Additional pricing fields can be added here if needed
	} `json:"data"`
}

// BillingInfo holds the billing information extracted from LLM response
type BillingInfo struct {
	InputTokens      int64
	OutputTokens     int64
	CacheReadTokens  int64
	CacheWriteTokens int64
	Model            string
	Provider         string
	RequestID        string
}

// BillingTokenUsage is the provider-neutral usage view consumed by ai-billing.
type BillingTokenUsage struct {
	InputTokens           int64
	OutputTokens          int64
	CacheReadTokens       int64
	CacheWriteTokens      int64
	RawInputTokenDetails  map[string]int64
	RawOutputTokenDetails map[string]int64
}

// streamDiagnostics stores only structural SSE metadata. Response text is
// deliberately not retained or logged because it can contain sensitive data.
type streamDiagnostics struct {
	callbackCount             int
	totalBytes                int
	usageCandidateCallbacks   int
	completedWithUsage        int
	completedWithoutUsage     int
	sawUsage                  bool
	sawUsageMetadata          bool
	sawResponseCompleted      bool
	sawDone                   bool
	sawInputTokens            bool
	sawOutputTokens           bool
	sawTotalTokens            bool
	sawPromptTokens           bool
	sawCompletionTokens       bool
	lastCallbackBytes         int
	lastCallbackHasUsage      bool
	lastCallbackHasCompletion bool
}

type failedStreamBody struct {
	data      []byte
	truncated bool
}

func captureFailedStreamBody(ctx wrapper.HttpContext, config BillingConfig, data []byte) {
	if !config.DebugLogFailedStreamResponse || len(data) == 0 {
		return
	}

	body, _ := ctx.GetContext(CtxKeyFailedStreamBody).(*failedStreamBody)
	if body == nil {
		body = &failedStreamBody{}
		ctx.SetContext(CtxKeyFailedStreamBody, body)
	}
	remaining := config.DebugLogFailedStreamResponseMaxBytes - len(body.data)
	if remaining <= 0 {
		body.truncated = true
		return
	}
	if len(data) > remaining {
		body.data = append(body.data, data[:remaining]...)
		body.truncated = true
		return
	}
	body.data = append(body.data, data...)
}

func getFailedStreamBody(ctx wrapper.HttpContext) failedStreamBody {
	if body, ok := ctx.GetContext(CtxKeyFailedStreamBody).(*failedStreamBody); ok && body != nil {
		return *body
	}
	return failedStreamBody{}
}

func observeStreamingChunk(diagnostics *streamDiagnostics, data []byte) {
	diagnostics.callbackCount++
	diagnostics.totalBytes += len(data)
	diagnostics.lastCallbackBytes = len(data)

	normalized := wrapper.UnifySSEChunk(data)
	hasUsage := bytes.Contains(normalized, []byte(`"usage"`))
	hasUsageMetadata := bytes.Contains(normalized, []byte(`"usageMetadata"`))
	hasCompletion := bytes.Contains(normalized, []byte(`"response.completed"`))
	hasDone := bytes.Contains(normalized, []byte("[DONE]"))
	hasInputTokens := bytes.Contains(normalized, []byte("\"input_tokens\""))
	hasOutputTokens := bytes.Contains(normalized, []byte("\"output_tokens\""))
	hasTotalTokens := bytes.Contains(normalized, []byte("\"total_tokens\""))
	hasPromptTokens := bytes.Contains(normalized, []byte("\"prompt_tokens\""))
	hasCompletionTokens := bytes.Contains(normalized, []byte("\"completion_tokens\""))

	// Use regular string literals for JSON field/event markers. They must not
	// include a literal backslash before the quote.
	hasUsage = bytes.Contains(normalized, []byte("\"usage\""))
	hasUsageMetadata = bytes.Contains(normalized, []byte("\"usageMetadata\""))
	hasCompletion = bytes.Contains(normalized, []byte("\"response.completed\""))

	diagnostics.lastCallbackHasUsage = hasUsage || hasUsageMetadata
	diagnostics.lastCallbackHasCompletion = hasCompletion
	if diagnostics.lastCallbackHasUsage {
		diagnostics.usageCandidateCallbacks++
	}
	if hasCompletion {
		if diagnostics.lastCallbackHasUsage {
			diagnostics.completedWithUsage++
		} else {
			diagnostics.completedWithoutUsage++
		}
	}
	diagnostics.sawUsage = diagnostics.sawUsage || hasUsage
	diagnostics.sawUsageMetadata = diagnostics.sawUsageMetadata || hasUsageMetadata
	diagnostics.sawResponseCompleted = diagnostics.sawResponseCompleted || hasCompletion
	diagnostics.sawDone = diagnostics.sawDone || hasDone
	diagnostics.sawInputTokens = diagnostics.sawInputTokens || hasInputTokens
	diagnostics.sawOutputTokens = diagnostics.sawOutputTokens || hasOutputTokens
	diagnostics.sawTotalTokens = diagnostics.sawTotalTokens || hasTotalTokens
	diagnostics.sawPromptTokens = diagnostics.sawPromptTokens || hasPromptTokens
	diagnostics.sawCompletionTokens = diagnostics.sawCompletionTokens || hasCompletionTokens
}

func recordStreamingDiagnostics(ctx wrapper.HttpContext, data []byte) *streamDiagnostics {
	diagnostics, _ := ctx.GetContext(CtxKeyStreamDiagnostics).(*streamDiagnostics)
	if diagnostics == nil {
		diagnostics = &streamDiagnostics{}
		ctx.SetContext(CtxKeyStreamDiagnostics, diagnostics)
	}
	observeStreamingChunk(diagnostics, data)
	return diagnostics
}

func getStreamingDiagnostics(ctx wrapper.HttpContext) streamDiagnostics {
	if diagnostics, ok := ctx.GetContext(CtxKeyStreamDiagnostics).(*streamDiagnostics); ok && diagnostics != nil {
		return *diagnostics
	}
	return streamDiagnostics{}
}

func buildBillingTokenUsage(usage tokenusage.TokenUsage) BillingTokenUsage {
	billingUsage := BillingTokenUsage{
		InputTokens:           usage.InputToken,
		OutputTokens:          usage.OutputToken,
		CacheReadTokens:       usage.AnthropicCacheReadInputToken,
		CacheWriteTokens:      usage.AnthropicCacheCreationInputToken,
		RawInputTokenDetails:  usage.InputTokenDetails,
		RawOutputTokenDetails: usage.OutputTokenDetails,
	}

	// OpenAI chat/completions reports cached_tokens inside prompt_tokens.
	// To avoid double charging in billing-service's additive formula, move
	// cached_tokens into CacheReadTokens and subtract it from InputTokens here.
	if cachedTokens, ok := usage.InputTokenDetails["cached_tokens"]; ok && cachedTokens > 0 {
		originalInputTokens := billingUsage.InputTokens
		billingUsage.CacheReadTokens = cachedTokens
		if cachedTokens <= billingUsage.InputTokens {
			billingUsage.InputTokens -= cachedTokens
		} else {
			billingUsage.InputTokens = 0
		}
		log.Debugf("[%s] normalized cached tokens for billing: model=%s originalInputTokens=%d cachedTokens=%d normalizedInputTokens=%d",
			pluginName, usage.Model, originalInputTokens, cachedTokens, billingUsage.InputTokens)
	}

	log.Debugf("[%s] build billing token usage: model=%s inputTokens=%d outputTokens=%d cacheReadTokens=%d cacheWriteTokens=%d rawInputTokenDetails=%v rawOutputTokenDetails=%v",
		pluginName, usage.Model, billingUsage.InputTokens, billingUsage.OutputTokens, billingUsage.CacheReadTokens, billingUsage.CacheWriteTokens, billingUsage.RawInputTokenDetails, billingUsage.RawOutputTokenDetails)

	return billingUsage
}

// parseConfig parses the plugin configuration
func parseConfig(json gjson.Result, config *BillingConfig) error {
	log.Infof("[%s] parsing configuration", pluginName)

	// Parse billing service configuration
	billingService := json.Get("billingService")
	if !billingService.Exists() {
		return errors.New("missing billingService in config")
	}

	serviceAddress := billingService.Get("serviceAddress").String()
	if serviceAddress == "" {
		return errors.New("billingService.serviceAddress must not be empty")
	}
	config.BillingService.ServiceAddress = serviceAddress

	// Parse protocol with default
	protocol := billingService.Get("protocol").String()
	if protocol == "" {
		protocol = "http"
	}
	config.BillingService.Protocol = protocol

	// Parse port with default
	port := billingService.Get("port").Int()
	if port == 0 {
		port = 8888
	}
	config.BillingService.Port = int(port)

	// Parse error messages with defaults
	config.FailPricingMessage = json.Get("failPricingMessage").String()
	if config.FailPricingMessage == "" {
		config.FailPricingMessage = "503 Pricing Information Unavailable"
	}

	config.FailBalanceMessage = json.Get("failBalanceMessage").String()
	if config.FailBalanceMessage == "" {
		config.FailBalanceMessage = "503 Billing Service Balance Unavailable"
	}

	config.InsufficientBalanceMessage = json.Get("insufficientBalanceMessage").String()
	if config.InsufficientBalanceMessage == "" {
		config.InsufficientBalanceMessage = "余额不足"
	}

	config.FailCostMessage = json.Get("failCostMessage").String()
	if config.FailCostMessage == "" {
		config.FailCostMessage = "503 Billing Service Cost Unavailable"
	}

	config.DebugLogFailedStreamResponse = json.Get("debugLogFailedStreamResponse").Bool()
	if config.DebugLogFailedStreamResponse {
		maxBytes := json.Get("debugLogFailedStreamResponseMaxBytes").Int()
		if maxBytes <= 0 {
			maxBytes = 1024 * 1024
		}
		config.DebugLogFailedStreamResponseMaxBytes = int(maxBytes)
	}

	// Initialize pricing cache
	config.pricingCache = make(map[string]bool)

	// Initialize HTTP client for billing service
	// Use FQDNCluster with service name directly (not full FQDN)
	// Envoy/Istio will handle the service discovery
	config.billingClient = wrapper.NewClusterClient(wrapper.FQDNCluster{
		FQDN: config.BillingService.ServiceAddress,
		Port: int64(port),
	})

	clusterName := config.billingClient.ClusterName()
	log.Infof("[%s] configuration parsed successfully: version=2.0.0-tenant service=%s://%s:%d cluster=%s",
		pluginName, protocol, config.BillingService.ServiceAddress, port, clusterName)

	return nil
}

// extractApiKey extracts the API key from request headers
func extractApiKey(ctx wrapper.HttpContext) (string, error) {
	// Try x-hi-original-auth first
	apiKey, err := proxywasm.GetHttpRequestHeader("x-hi-original-auth")
	if err == nil && apiKey != "" {
		// Remove "Bearer " prefix if present
		apiKey = strings.TrimSpace(strings.TrimPrefix(apiKey, "Bearer"))
		if apiKey != "" && apiKey != "Bearer" {
			return apiKey, nil
		}
	}

	// Fall back to Authorization header (try both cases since HTTP headers are case-insensitive)
	apiKey, err = proxywasm.GetHttpRequestHeader("Authorization")
	if err == nil && apiKey != "" {
		// Remove "Bearer " prefix if present
		apiKey = strings.TrimSpace(strings.TrimPrefix(apiKey, "Bearer"))
		if apiKey != "" && apiKey != "Bearer" {
			return apiKey, nil
		}
	}

	// Try lowercase authorization as fallback
	apiKey, err = proxywasm.GetHttpRequestHeader("authorization")
	if err == nil && apiKey != "" {
		// Remove "Bearer " prefix if present
		apiKey = strings.TrimSpace(strings.TrimPrefix(apiKey, "Bearer"))
		if apiKey != "" && apiKey != "Bearer" {
			return apiKey, nil
		}
	}

	return "", errors.New("API key not found in request headers")
}

// maskApiKey masks the API key for logging (show only first 8 characters)
func maskApiKey(apiKey string) string {
	if len(apiKey) <= 8 {
		return apiKey + "***"
	}
	return apiKey[:8] + "***"
}

// extractConsumerApiKey extracts the consumer API key from x-mse-consumer-apikey header
// Returns empty string if header is not present
func extractConsumerApiKey() string {
	apiKey, err := proxywasm.GetHttpRequestHeader("x-mse-consumer-apikey")
	if err != nil || apiKey == "" {
		return ""
	}
	return apiKey
}

// extractApikeyID extracts the apikey ID from x-mse-apikey-id header
// Returns 0 if header is not present or cannot be parsed as integer
func extractApikeyID() int64 {
	apikeyIDStr, err := proxywasm.GetHttpRequestHeader("x-mse-apikey-id")
	if err != nil || apikeyIDStr == "" {
		return 0
	}

	// Parse string to int64
	apikeyID, err := strconv.ParseInt(apikeyIDStr, 10, 64)
	if err != nil {
		log.Errorf("[%s] failed to parse apikey ID '%s' as integer: %v", pluginName, apikeyIDStr, err)
		return 0
	}

	return apikeyID
}

// extractTenantInfo extracts tenant information and HMAC authentication headers from the request
func extractTenantInfo(ctx wrapper.HttpContext) (*TenantInfo, error) {
	tenantInfo := &TenantInfo{}
	var missingFields []string

	// Extract HMAC Authentication Headers
	if signVersion, err := proxywasm.GetHttpRequestHeader("x-internal-auth-sign-version"); err != nil || signVersion == "" {
		missingFields = append(missingFields, "x-internal-auth-sign-version")
	} else {
		tenantInfo.SignVersion = signVersion
	}

	if timestamp, err := proxywasm.GetHttpRequestHeader("x-internal-auth-ts"); err != nil || timestamp == "" {
		missingFields = append(missingFields, "x-internal-auth-ts")
	} else {
		tenantInfo.Timestamp = timestamp
	}

	if nonce, err := proxywasm.GetHttpRequestHeader("x-internal-auth-nonce"); err != nil || nonce == "" {
		missingFields = append(missingFields, "x-internal-auth-nonce")
	} else {
		tenantInfo.Nonce = nonce
	}

	if signature, err := proxywasm.GetHttpRequestHeader("x-internal-auth-sign"); err != nil || signature == "" {
		missingFields = append(missingFields, "x-internal-auth-sign")
	} else {
		tenantInfo.Signature = signature
	}

	// Extract Tenant/Consumer Information
	if consumerID, err := proxywasm.GetHttpRequestHeader("x-consumer-id"); err != nil || consumerID == "" {
		missingFields = append(missingFields, "x-consumer-id")
	} else {
		tenantInfo.ConsumerID = consumerID
	}

	if consumerName, err := proxywasm.GetHttpRequestHeader("x-mse-consumer-name"); err != nil || consumerName == "" {
		missingFields = append(missingFields, "x-mse-consumer-name")
	} else {
		tenantInfo.ConsumerName = consumerName
	}

	if tenantID, err := proxywasm.GetHttpRequestHeader("x-mse-tenant-id"); err != nil || tenantID == "" {
		missingFields = append(missingFields, "x-mse-tenant-id")
	} else {
		tenantInfo.TenantID = tenantID
	}

	if domainResourceID, err := proxywasm.GetHttpRequestHeader("x-domain-resource-id"); err != nil || domainResourceID == "" {
		missingFields = append(missingFields, "x-domain-resource-id")
	} else {
		tenantInfo.DomainResourceID = domainResourceID
	}

	if routerResourceID, err := proxywasm.GetHttpRequestHeader("x-router-resource-id"); err != nil || routerResourceID == "" {
		missingFields = append(missingFields, "x-router-resource-id")
	} else {
		tenantInfo.RouterResourceID = routerResourceID
	}

	// Check if any required fields are missing
	if len(missingFields) > 0 {
		return nil, fmt.Errorf("missing required tenant headers: %s", strings.Join(missingFields, ", "))
	}

	return tenantInfo, nil
}

// sendErrorResponse sends an error response to the client
// Can be used in both request and response phases
// When response is paused, this will send the error and the response won't be resumed
func sendErrorResponse(statusCode int, message string) {
	errorBody := fmt.Sprintf(`{"error":{"message":"%s","type":"billing_error"}}`, message)
	_ = proxywasm.SendHttpResponseWithDetail(uint32(statusCode), "ai-billing.error", [][2]string{
		{"content-type", "application/json"},
	}, []byte(errorBody), -1)
}

// sendErrorResponseAndMarkDenied sends an error response and marks the request as denied
// This is used in request phase to ensure subsequent phases don't process the request
func sendErrorResponseAndMarkDenied(ctx wrapper.HttpContext, statusCode int, message string) {
	ctx.SetContext(CtxKeyRequestDenied, true)
	sendErrorResponse(statusCode, message)
}

// onHttpRequestHeaders handles the request headers phase
func onHttpRequestHeaders(ctx wrapper.HttpContext, config BillingConfig) types.Action {
	log.Debugf("[%s] processing request headers", pluginName)

	// Extract tenant information (required)
	tenantInfo, err := extractTenantInfo(ctx)
	if err != nil {
		log.Errorf("[%s] failed to extract tenant info: %v", pluginName, err)
		sendErrorResponseAndMarkDenied(ctx, http.StatusUnauthorized, "Missing or invalid tenant authentication headers")
		return types.ActionContinue
	}

	// Store tenant info in context
	ctx.SetContext(CtxKeyTenantInfo, tenantInfo)
	log.Infof("[%s] tenant info extracted: tenantId=%s consumerId=%s consumerName=%s",
		pluginName, tenantInfo.TenantID, tenantInfo.ConsumerID, tenantInfo.ConsumerName)

	// Extract consumer API key from x-mse-consumer-apikey header
	consumerApiKey := extractConsumerApiKey()
	ctx.SetContext(CtxKeyConsumerApiKey, consumerApiKey)
	log.Debugf("[%s] consumer apikey extracted: %s", pluginName, maskApiKey(consumerApiKey))

	// Extract apikey ID from x-mse-apikey-id header
	apikeyID := extractApikeyID()
	ctx.SetContext(CtxKeyApikeyID, apikeyID)
	if apikeyID == 0 {
		log.Errorf("[%s] apikey ID not found in x-mse-apikey-id header, will use consumer-level quota tracking", pluginName)
	} else {
		log.Debugf("[%s] apikey ID extracted: %d", pluginName, apikeyID)
	}

	// Optional: Extract API key for debug logging
	apiKey, err := extractApiKey(ctx)
	if err == nil && apiKey != "" {
		ctx.SetContext(CtxKeyApiKey, apiKey)
		log.Debugf("[%s] API key extracted: %s", pluginName, maskApiKey(apiKey))
	} else {
		log.Debugf("[%s] API key not found (optional for debug logging)", pluginName)
	}

	// Extract provider and model for pricing check
	provider := extractProvider(ctx)
	model := extractModel(ctx)

	if provider == "" || model == "" {
		log.Warnf("[%s] provider or model not found, skipping pricing check: provider=%s model=%s",
			pluginName, provider, model)
		// If we can't determine provider/model, skip pricing check and go directly to balance check
		return checkBalance(ctx, config, tenantInfo)
	}

	// Check pricing (with cache)
	action := checkPricing(ctx, &config, tenantInfo, provider, model)
	if action == types.ActionPause {
		// Pricing query is async, will call checkBalance in callback
		return action
	}

	// Pricing was cached or error occurred, continue with balance check
	return checkBalance(ctx, config, tenantInfo)
}

// checkPricingCache checks if pricing information is cached for the given provider and model
func checkPricingCache(config *BillingConfig, provider, model string) bool {
	cacheKey := fmt.Sprintf("%s:%s", provider, model)
	return config.pricingCache[cacheKey]
}

// storePricingCache stores pricing information in the cache
func storePricingCache(config *BillingConfig, provider, model string) {
	cacheKey := fmt.Sprintf("%s:%s", provider, model)
	config.pricingCache[cacheKey] = true
	log.Debugf("[%s] pricing cached: key=%s", pluginName, cacheKey)
}

// checkPricing checks if pricing information exists for the given provider and model
// Returns types.ActionPause if query is in progress, types.ActionContinue if cached or error
func checkPricing(ctx wrapper.HttpContext, config *BillingConfig, tenantInfo *TenantInfo, provider, model string) types.Action {
	// Check cache first
	if checkPricingCache(config, provider, model) {
		log.Infof("[%s] pricing cache hit: tenantId=%s provider=%s model=%s",
			pluginName, tenantInfo.TenantID, provider, model)
		return types.ActionContinue
	}

	log.Infof("[%s] pricing cache miss, querying: tenantId=%s consumerId=%s provider=%s model=%s",
		pluginName, tenantInfo.TenantID, tenantInfo.ConsumerID, provider, model)

	// Build request headers with tenant info and HMAC authentication
	headers := [][2]string{
		{"x-internal-auth-sign-version", tenantInfo.SignVersion},
		{"x-internal-auth-ts", tenantInfo.Timestamp},
		{"x-internal-auth-nonce", tenantInfo.Nonce},
		{"x-internal-auth-sign", tenantInfo.Signature},
		{"x-consumer-id", tenantInfo.ConsumerID},
		{"x-mse-consumer-name", tenantInfo.ConsumerName},
		{"x-mse-tenant-id", tenantInfo.TenantID},
		{"x-domain-resource-id", tenantInfo.DomainResourceID},
		{"x-router-resource-id", tenantInfo.RouterResourceID},
	}

	// Build URL with query parameters
	path := fmt.Sprintf("/v1/pricing/global?provider=%s&model_name=%s", provider, model)

	log.Debugf("[%s] sending pricing query request: path=%s", pluginName, path)

	err := config.billingClient.Get(path, headers, func(statusCode int, responseHeaders http.Header, responseBody []byte) {
		log.Infof("[%s] pricing query response: status=%d body=%s", pluginName, statusCode, string(responseBody))

		// Handle response in callback
		if statusCode != http.StatusOK {
			log.Errorf("[%s] pricing query failed: tenantId=%s consumerId=%s provider=%s model=%s status=%d body=%s",
				pluginName, tenantInfo.TenantID, tenantInfo.ConsumerID, provider, model, statusCode, string(responseBody))
			sendErrorResponseAndMarkDenied(ctx, http.StatusServiceUnavailable, config.FailPricingMessage)
			return
		}

		// Parse response
		var pricingResp PricingResponse
		if err := json.Unmarshal(responseBody, &pricingResp); err != nil {
			log.Errorf("[%s] failed to parse pricing response: tenantId=%s consumerId=%s provider=%s model=%s error=%v body=%s",
				pluginName, tenantInfo.TenantID, tenantInfo.ConsumerID, provider, model, err, string(responseBody))
			sendErrorResponseAndMarkDenied(ctx, http.StatusServiceUnavailable, config.FailPricingMessage)
			return
		}

		// Check success field
		if !pricingResp.Success {
			// Use message from billing service if available, otherwise use config default
			errorMessage := config.FailPricingMessage
			if pricingResp.Message != "" {
				errorMessage = pricingResp.Message
			}
			log.Errorf("[%s] pricing query failed: tenantId=%s consumerId=%s provider=%s model=%s message=%s",
				pluginName, tenantInfo.TenantID, tenantInfo.ConsumerID, provider, model, errorMessage)
			sendErrorResponseAndMarkDenied(ctx, http.StatusServiceUnavailable, errorMessage)
			return
		}

		// Store pricing in cache
		storePricingCache(config, provider, model)
		log.Infof("[%s] pricing query successful: tenantId=%s consumerId=%s provider=%s model=%s",
			pluginName, tenantInfo.TenantID, tenantInfo.ConsumerID, provider, model)

		// Continue with balance check
		action := checkBalance(ctx, *config, tenantInfo)
		if action == types.ActionPause {
			// Balance check is async, don't resume yet
			return
		}
		// Balance check completed synchronously (error case), resume request
		proxywasm.ResumeHttpRequest()
	}, 5000) // 5 second timeout

	if err != nil {
		log.Errorf("[%s] failed to send pricing query request: tenantId=%s consumerId=%s provider=%s model=%s path=%s error=%v",
			pluginName, tenantInfo.TenantID, tenantInfo.ConsumerID, provider, model, path, err)
		sendErrorResponseAndMarkDenied(ctx, http.StatusServiceUnavailable, config.FailPricingMessage)
		return types.ActionContinue
	}

	log.Debugf("[%s] pricing query request sent, pausing request", pluginName)
	// Pause processing until callback completes
	return types.ActionPause
}

// checkBalance checks the user's balance with the billing service
func checkBalance(ctx wrapper.HttpContext, config BillingConfig, tenantInfo *TenantInfo) types.Action {
	log.Infof("[%s] checking balance: tenantId=%s consumerId=%s consumerName=%s",
		pluginName, tenantInfo.TenantID, tenantInfo.ConsumerID, tenantInfo.ConsumerName)

	// Build request headers with tenant info and HMAC authentication
	headers := [][2]string{
		{"x-internal-auth-sign-version", tenantInfo.SignVersion},
		{"x-internal-auth-ts", tenantInfo.Timestamp},
		{"x-internal-auth-nonce", tenantInfo.Nonce},
		{"x-internal-auth-sign", tenantInfo.Signature},
		{"x-consumer-id", tenantInfo.ConsumerID},
		{"x-mse-consumer-name", tenantInfo.ConsumerName},
		{"x-mse-tenant-id", tenantInfo.TenantID},
		{"x-domain-resource-id", tenantInfo.DomainResourceID},
		{"x-router-resource-id", tenantInfo.RouterResourceID},
	}

	// Make async HTTP GET call to billing service (no request body)
	path := "/v1/amount"

	clusterName := config.billingClient.ClusterName()
	log.Debugf("[%s] sending balance check request: cluster=%s path=%s", pluginName, clusterName, path)

	err := config.billingClient.Get(path, headers, func(statusCode int, responseHeaders http.Header, responseBody []byte) {
		log.Infof("[%s] balance check response: status=%d body=%s", pluginName, statusCode, string(responseBody))

		// Handle response in callback
		if statusCode != http.StatusOK {
			log.Errorf("[%s] balance check failed: tenantId=%s consumerId=%s status=%d body=%s",
				pluginName, tenantInfo.TenantID, tenantInfo.ConsumerID, statusCode, string(responseBody))
			sendErrorResponseAndMarkDenied(ctx, http.StatusServiceUnavailable, config.FailBalanceMessage)
			return
		}

		// Parse response
		var balanceResp BalanceResponse
		if err := json.Unmarshal(responseBody, &balanceResp); err != nil {
			log.Errorf("[%s] failed to parse balance response: tenantId=%s consumerId=%s error=%v body=%s",
				pluginName, tenantInfo.TenantID, tenantInfo.ConsumerID, err, string(responseBody))
			sendErrorResponseAndMarkDenied(ctx, http.StatusServiceUnavailable, config.FailBalanceMessage)
			return
		}

		// Check success field
		if !balanceResp.Success {
			// Use message from billing service if available, otherwise use config default
			errorMessage := config.FailBalanceMessage
			if balanceResp.Message != "" {
				errorMessage = balanceResp.Message
			}
			log.Errorf("[%s] balance check failed: tenantId=%s consumerId=%s message=%s",
				pluginName, tenantInfo.TenantID, tenantInfo.ConsumerID, errorMessage)
			sendErrorResponseAndMarkDenied(ctx, http.StatusServiceUnavailable, errorMessage)
			return
		}

		// Parse balance as float
		balance, err := strconv.ParseFloat(balanceResp.Balance, 64)
		if err != nil {
			log.Errorf("[%s] failed to parse balance value: tenantId=%s consumerId=%s balance=%s error=%v",
				pluginName, tenantInfo.TenantID, tenantInfo.ConsumerID, balanceResp.Balance, err)
			sendErrorResponseAndMarkDenied(ctx, http.StatusServiceUnavailable, config.FailBalanceMessage)
			return
		}

		log.Infof("[%s] balance check: tenantId=%s consumerId=%s consumerName=%s balance=%s",
			pluginName, tenantInfo.TenantID, tenantInfo.ConsumerID, tenantInfo.ConsumerName, balanceResp.Balance)

		// Check if balance is sufficient
		// Use a small epsilon to handle floating point precision issues
		const epsilon = 0.0001
		if balance < epsilon {
			log.Warnf("[%s] insufficient balance: tenantId=%s consumerId=%s consumerName=%s balance=%s",
				pluginName, tenantInfo.TenantID, tenantInfo.ConsumerID, tenantInfo.ConsumerName, balanceResp.Balance)
			sendErrorResponseAndMarkDenied(ctx, http.StatusPaymentRequired, config.InsufficientBalanceMessage)
			return
		}

		// Balance is sufficient, resume request
		log.Debugf("[%s] balance sufficient, resuming request", pluginName)
		proxywasm.ResumeHttpRequest()
	}, 5000) // 5 second timeout

	if err != nil {
		log.Errorf("[%s] failed to send balance check request: tenantId=%s consumerId=%s path=%s error=%v",
			pluginName, tenantInfo.TenantID, tenantInfo.ConsumerID, path, err)
		sendErrorResponseAndMarkDenied(ctx, http.StatusServiceUnavailable, config.FailBalanceMessage)
		return types.ActionContinue
	}

	log.Debugf("[%s] balance check request sent, pausing request", pluginName)
	// Pause processing until callback completes
	return types.ActionPause
}

// onHttpResponseHeaders handles the response headers phase
func onHttpResponseHeaders(ctx wrapper.HttpContext, config BillingConfig) types.Action {
	// Check if request was denied in request phase
	if denied, ok := ctx.GetContext(CtxKeyRequestDenied).(bool); ok && denied {
		log.Debugf("[%s] request was denied, skipping response processing", pluginName)
		return types.ActionContinue
	}

	log.Debugf("[%s] processing response headers", pluginName)

	// Check HTTP status code
	statusCodeStr, err := proxywasm.GetHttpResponseHeader(":status")
	statusCodeInt := 200 // default to 200
	if err == nil {
		// Parse status code string to int
		if code, parseErr := strconv.Atoi(statusCodeStr); parseErr == nil {
			statusCodeInt = code
		}
	}

	// Store status code in context for use in response body phase
	ctx.SetContext(CtxKeyStatusCode, statusCodeInt)

	if err != nil || statusCodeStr != "200" {
		// Non-200 response: skip billing and log with appropriate level and context

		// Extract context information for logging (consumer, provider, model)
		consumer := "unknown"
		if tenantInfo, ok := ctx.GetContext(CtxKeyTenantInfo).(*TenantInfo); ok && tenantInfo != nil {
			// Use ConsumerName if available, otherwise use ConsumerID
			if tenantInfo.ConsumerName != "" {
				consumer = tenantInfo.ConsumerName
			} else if tenantInfo.ConsumerID != "" {
				consumer = tenantInfo.ConsumerID
			}
		}
		provider := extractProvider(ctx)
		model := extractModel(ctx)

		// Log with appropriate level based on status code type
		statusInt, parseErr := strconv.Atoi(statusCodeStr)
		if parseErr == nil {
			if statusInt >= 400 && statusInt < 500 {
				// 4xx: Client errors - Info level
				log.Infof("[%s] skipping billing for 4xx response: consumer=%s, provider=%s, model=%s, status=%s",
					pluginName, consumer, provider, model, statusCodeStr)
			} else if statusInt >= 500 && statusInt < 600 {
				// 5xx: Server errors - Warn level
				log.Errorf("[%s] skipping billing for 5xx response: consumer=%s, provider=%s, model=%s, status=%s",
					pluginName, consumer, provider, model, statusCodeStr)
			} else {
				// Other non-200 (e.g., 3xx) - Debug level
				log.Errorf("[%s] skipping billing for non-200 response: consumer=%s, provider=%s, model=%s, status=%s",
					pluginName, consumer, provider, model, statusCodeStr)
			}
		} else {
			// Failed to parse status code - Debug level
			log.Errorf("[%s] skipping billing for non-200 response: status=%s", pluginName, statusCodeStr)
		}

		// Return ActionContinue to passthrough the response
		return types.ActionContinue
	}

	// 200 response: Continue with normal billing flow
	// Detect streaming vs non-streaming response
	contentType, _ := proxywasm.GetHttpResponseHeader("content-type")
	isStreaming := strings.Contains(contentType, "text/event-stream")
	ctx.SetContext(CtxKeyIsStreaming, isStreaming)

	if isStreaming {
		log.Debugf("[%s] detected streaming response", pluginName)
	} else {
		log.Debugf("[%s] detected non-streaming response, buffering body", pluginName)
		ctx.BufferResponseBody()
	}

	return types.ActionContinue
}

// onHttpResponseBody handles the non-streaming response body
func onHttpResponseBody(ctx wrapper.HttpContext, config BillingConfig, body []byte) types.Action {
	// Check if request was denied in request phase
	if denied, ok := ctx.GetContext(CtxKeyRequestDenied).(bool); ok && denied {
		log.Debugf("[%s] request was denied, skipping response body processing", pluginName)
		return types.ActionContinue
	}

	// Check HTTP status code from context (set in onHttpResponseHeaders)
	// We cannot reliably get :status header in response body phase
	if statusCode, ok := ctx.GetContext(CtxKeyStatusCode).(int); ok && statusCode != 200 {
		log.Debugf("[%s] skipping response body processing for non-200 response: status=%d", pluginName, statusCode)
		return types.ActionContinue
	}

	log.Debugf("[%s] processing response body", pluginName)

	// Extract token usage
	usage := tokenusage.GetTokenUsage(ctx, body)
	if usage.TotalToken == 0 {
		log.Errorf("[%s] failed to extract token usage from response", pluginName)
		// FAIL_CLOSE: For non-streaming responses, we can send error response
		// because we buffered the entire response
		sendErrorResponse(http.StatusInternalServerError, "Failed to extract billing information")
		return types.ActionContinue
	}

	// Extract request ID (from request header or response body)
	requestID := extractRequestID(ctx, body)

	// Extract provider
	provider := extractProvider(ctx)

	// Extract model from request header
	model := extractModel(ctx)
	if model == "" {
		log.Warnf("[%s] model not found in x-higress-llm-model header, using model from response: %s", pluginName, usage.Model)
		model = usage.Model
	}

	// Get tenant info from context
	tenantInfo, ok := ctx.GetContext(CtxKeyTenantInfo).(*TenantInfo)
	if !ok {
		log.Errorf("[%s] failed to get tenant info from context", pluginName)
		// FAIL_CLOSE: Send error response for internal error
		sendErrorResponse(http.StatusInternalServerError, "Internal error")
		return types.ActionContinue
	}

	billingUsage := buildBillingTokenUsage(usage)

	log.Infof("[%s] extracted billing info: tenantId=%s consumerId=%s consumerName=%s requestId=%s inputTokens=%d outputTokens=%d cacheReadTokens=%d cacheWriteTokens=%d model=%s provider=%s",
		pluginName, tenantInfo.TenantID, tenantInfo.ConsumerID, tenantInfo.ConsumerName, requestID, billingUsage.InputTokens, billingUsage.OutputTokens, billingUsage.CacheReadTokens, billingUsage.CacheWriteTokens, model, provider)

	// Deduct cost
	billingInfo := &BillingInfo{
		InputTokens:      billingUsage.InputTokens,
		OutputTokens:     billingUsage.OutputTokens,
		CacheReadTokens:  billingUsage.CacheReadTokens,
		CacheWriteTokens: billingUsage.CacheWriteTokens,
		Model:            model,
		Provider:         provider,
		RequestID:        requestID,
	}

	return deductCost(ctx, config, tenantInfo, billingInfo)
}

// onHttpStreamingResponseBody handles the streaming response body
func onHttpStreamingResponseBody(ctx wrapper.HttpContext, config BillingConfig, data []byte, endOfStream bool) []byte {
	// Check if request was denied in request phase
	if denied, ok := ctx.GetContext(CtxKeyRequestDenied).(bool); ok && denied {
		log.Debugf("[%s] request was denied, skipping streaming response body processing", pluginName)
		return data
	}

	// Check HTTP status code from context (set in onHttpResponseHeaders)
	// We cannot reliably get :status header in response body phase
	if statusCode, ok := ctx.GetContext(CtxKeyStatusCode).(int); ok && statusCode != 200 {
		log.Debugf("[%s] skipping streaming response body processing for non-200 response: status=%d", pluginName, statusCode)
		return data
	}

	// Track only safe structural information so failed usage extraction can be
	// distinguished from missing usage and callback-level SSE fragmentation.
	recordStreamingDiagnostics(ctx, data)
	captureFailedStreamBody(ctx, config, data)

	// Call GetTokenUsage on each chunk
	usage := tokenusage.GetTokenUsage(ctx, data)
	if usage.TotalToken > 0 {
		billingUsage := buildBillingTokenUsage(usage)
		log.Debugf("[%s] extracted token usage from stream: inputTokens=%d outputTokens=%d cacheReadTokens=%d cacheWriteTokens=%d",
			pluginName, billingUsage.InputTokens, billingUsage.OutputTokens, billingUsage.CacheReadTokens, billingUsage.CacheWriteTokens)

		// Extract model from request header
		model := extractModel(ctx)
		if model == "" {
			log.Warnf("[%s] model not found in x-higress-llm-model header, using model from response: %s", pluginName, usage.Model)
			model = usage.Model
		}

		// Store billing info in context
		billingInfo := &BillingInfo{
			InputTokens:      billingUsage.InputTokens,
			OutputTokens:     billingUsage.OutputTokens,
			CacheReadTokens:  billingUsage.CacheReadTokens,
			CacheWriteTokens: billingUsage.CacheWriteTokens,
			Model:            model,
		}
		ctx.SetContext(CtxKeyBillingInfo, billingInfo)
	}

	// If not end of stream, continue
	if !endOfStream {
		return data
	}

	// At end of stream, deduct cost if we have billing info
	billingInfo, ok := ctx.GetContext(CtxKeyBillingInfo).(*BillingInfo)
	if !ok || billingInfo == nil {
		diagnostics := getStreamingDiagnostics(ctx)
		requestID := extractRequestID(ctx, data)
		log.Errorf("[%s] failed to extract billing info from stream: requestId=%s provider=%s model=%s callbacks=%d totalBytes=%d usageCandidateCallbacks=%d completedWithUsage=%d completedWithoutUsage=%d sawUsage=%t sawUsageMetadata=%t sawResponseCompleted=%t sawDone=%t sawInputTokens=%t sawOutputTokens=%t sawTotalTokens=%t sawPromptTokens=%t sawCompletionTokens=%t lastCallbackBytes=%d lastCallbackHasUsage=%t lastCallbackHasCompletion=%t",
			pluginName, requestID, extractProvider(ctx), extractModel(ctx), diagnostics.callbackCount, diagnostics.totalBytes,
			diagnostics.usageCandidateCallbacks, diagnostics.completedWithUsage, diagnostics.completedWithoutUsage, diagnostics.sawUsage, diagnostics.sawUsageMetadata,
			diagnostics.sawResponseCompleted, diagnostics.sawDone, diagnostics.sawInputTokens, diagnostics.sawOutputTokens,
			diagnostics.sawTotalTokens, diagnostics.sawPromptTokens, diagnostics.sawCompletionTokens, diagnostics.lastCallbackBytes,
			diagnostics.lastCallbackHasUsage, diagnostics.lastCallbackHasCompletion)
		if config.DebugLogFailedStreamResponse {
			body := getFailedStreamBody(ctx)
			log.Errorf("[%s] failed stream response body: requestId=%s capturedBytes=%d truncated=%t body=%s", pluginName, requestID, len(body.data), body.truncated, string(body.data))
		}
		// FAIL_CLOSE: Block response if we cannot extract billing information
		// This is a critical error that indicates the response format is invalid
		sendErrorResponse(http.StatusInternalServerError, "Failed to extract billing information from stream")
		return nil
	}

	// Extract request ID and provider
	billingInfo.RequestID = extractRequestID(ctx, data)
	billingInfo.Provider = extractProvider(ctx)

	// Get tenant info from context
	tenantInfo, ok := ctx.GetContext(CtxKeyTenantInfo).(*TenantInfo)
	if !ok {
		log.Errorf("[%s] failed to get tenant info from context", pluginName)
		// FAIL_CLOSE: Block response on internal error
		sendErrorResponse(http.StatusInternalServerError, "Internal error")
		return nil
	}

	log.Infof("[%s] extracted billing info from stream: tenantId=%s consumerId=%s consumerName=%s requestId=%s inputTokens=%d outputTokens=%d cacheReadTokens=%d cacheWriteTokens=%d model=%s provider=%s",
		pluginName, tenantInfo.TenantID, tenantInfo.ConsumerID, tenantInfo.ConsumerName, billingInfo.RequestID, billingInfo.InputTokens, billingInfo.OutputTokens, billingInfo.CacheReadTokens, billingInfo.CacheWriteTokens, billingInfo.Model, billingInfo.Provider)

	// Deduct cost asynchronously - in streaming mode, we don't pause the response
	// because the stream has already been sent to the client
	// We just fire the billing request and let the stream continue
	deductCostAsync(ctx, config, tenantInfo, billingInfo)

	// Return data to continue the stream
	return data
}

// extractRequestID extracts the request ID from request headers or response body
func extractRequestID(ctx wrapper.HttpContext, data []byte) string {
	// Priority 1: Try to get from request header x-request-id
	if requestID, err := proxywasm.GetHttpRequestHeader("x-request-id"); err == nil && requestID != "" {
		return requestID
	}

	// Priority 2: Try to extract from response body
	if requestID := wrapper.GetValueFromBody(data, []string{
		"id",
		"response.id",
		"responseId",
		"message.id",
	}); requestID != nil {
		return requestID.String()
	}

	// Priority 3: Return empty string if not found
	// The billing service should handle missing request IDs
	return ""
}

// extractModel extracts the model name from the x-higress-llm-model request header
// If the model contains "/", it returns only the part after "/" (the actual model name)
func extractModel(ctx wrapper.HttpContext) string {
	// Try to get from x-higress-llm-model header
	if model, err := proxywasm.GetHttpRequestHeader("x-higress-llm-model"); err == nil && model != "" {
		// If model contains "/", extract only the part after "/"
		if strings.Contains(model, "/") {
			idx := strings.Index(model, "/")
			return model[idx+1:]
		}
		return model
	}

	// Fallback to empty string if not found
	return ""
}

// extractProvider extracts the provider information from context or route
func extractProvider(ctx wrapper.HttpContext) string {
	// Priority 1: Try to get from x-request-llm-provider header (set by ai-proxy plugin)
	// This header is user origin request add
	if provider, err := proxywasm.GetHttpRequestHeader("x-request-llm-provider"); err == nil && provider != "" {
		return provider
	}

	return "default"
}

// deductCost deducts the cost from the user's balance
// Used in non-streaming mode where we can pause and block the response
func deductCost(ctx wrapper.HttpContext, config BillingConfig, tenantInfo *TenantInfo, billingInfo *BillingInfo) types.Action {
	log.Infof("[%s] deducting cost: tenantId=%s consumerId=%s consumerName=%s",
		pluginName, tenantInfo.TenantID, tenantInfo.ConsumerID, tenantInfo.ConsumerName)

	// Get consumer API key from context
	consumerApiKey := ""
	if key, ok := ctx.GetContext(CtxKeyConsumerApiKey).(string); ok {
		consumerApiKey = key
	}

	// Get apikey ID from context
	var apikeyID *int64
	if id, ok := ctx.GetContext(CtxKeyApikeyID).(int64); ok && id > 0 {
		apikeyID = &id
	}

	// Build request body (without consumer_id, consumer_name, tenant_id - those are in headers)
	requestBody := CostRequest{
		Provider:         billingInfo.Provider,
		ModelName:        billingInfo.Model,
		RequestID:        billingInfo.RequestID,
		InputTokens:      billingInfo.InputTokens,
		OutputTokens:     billingInfo.OutputTokens,
		CacheReadTokens:  billingInfo.CacheReadTokens,
		CacheWriteTokens: billingInfo.CacheWriteTokens,
		ApiKey:           consumerApiKey,
		ApikeyID:         apikeyID,
	}
	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		log.Errorf("[%s] failed to marshal cost request: %v", pluginName, err)
		sendErrorResponse(http.StatusInternalServerError, config.FailCostMessage)
		return types.ActionContinue
	}

	// Build request headers with tenant info and HMAC authentication
	headers := [][2]string{
		{"content-type", "application/json"},
		{"x-internal-auth-sign-version", tenantInfo.SignVersion},
		{"x-internal-auth-ts", tenantInfo.Timestamp},
		{"x-internal-auth-nonce", tenantInfo.Nonce},
		{"x-internal-auth-sign", tenantInfo.Signature},
		{"x-consumer-id", tenantInfo.ConsumerID},
		{"x-mse-consumer-name", tenantInfo.ConsumerName},
		{"x-mse-tenant-id", tenantInfo.TenantID},
		{"x-domain-resource-id", tenantInfo.DomainResourceID},
		{"x-router-resource-id", tenantInfo.RouterResourceID},
	}

	// Make async HTTP call to billing service
	path := "/v1/cost"

	log.Debugf("[%s] sending cost deduction request: path=%s body=%s", pluginName, path, string(bodyBytes))

	err = config.billingClient.Post(path, headers, bodyBytes, func(statusCode int, responseHeaders http.Header, responseBody []byte) {
		log.Infof("[%s] cost deduction response: status=%d body=%s", pluginName, statusCode, string(responseBody))

		// Handle response in callback
		if statusCode != http.StatusOK {
			log.Errorf("[%s] cost deduction failed: tenantId=%s consumerId=%s requestId=%s status=%d body=%s",
				pluginName, tenantInfo.TenantID, tenantInfo.ConsumerID, billingInfo.RequestID, statusCode, string(responseBody))
			sendErrorResponse(http.StatusServiceUnavailable, config.FailCostMessage)
			return
		}

		// Parse response
		var costResp CostResponse
		if err := json.Unmarshal(responseBody, &costResp); err != nil {
			log.Errorf("[%s] failed to parse cost response: tenantId=%s consumerId=%s requestId=%s error=%v body=%s",
				pluginName, tenantInfo.TenantID, tenantInfo.ConsumerID, billingInfo.RequestID, err, string(responseBody))
			sendErrorResponse(http.StatusServiceUnavailable, config.FailCostMessage)
			return
		}

		// Check if cost deduction was successful
		if !costResp.Success {
			// Use message from billing service if available, otherwise use config default
			errorMessage := config.InsufficientBalanceMessage
			if costResp.Message != "" {
				errorMessage = costResp.Message
			}
			log.Warnf("[%s] cost deduction failed: tenantId=%s consumerId=%s requestId=%s message=%s",
				pluginName, tenantInfo.TenantID, tenantInfo.ConsumerID, billingInfo.RequestID, errorMessage)
			sendErrorResponse(http.StatusPaymentRequired, errorMessage)
			return
		}

		log.Infof("[%s] cost deduction: tenantId=%s consumerId=%s consumerName=%s requestId=%s inputTokens=%d outputTokens=%d cacheReadTokens=%d cacheWriteTokens=%d cost=%s success=%t",
			pluginName, tenantInfo.TenantID, tenantInfo.ConsumerID, tenantInfo.ConsumerName, billingInfo.RequestID, billingInfo.InputTokens, billingInfo.OutputTokens, billingInfo.CacheReadTokens, billingInfo.CacheWriteTokens, costResp.Cost, costResp.Success)

		// Cost deduction successful, resume response
		log.Debugf("[%s] cost deduction successful, resuming response", pluginName)
		proxywasm.ResumeHttpResponse()
	}, 5000) // 5 second timeout

	if err != nil {
		log.Errorf("[%s] failed to send cost deduction request: tenantId=%s consumerId=%s requestId=%s path=%s error=%v",
			pluginName, tenantInfo.TenantID, tenantInfo.ConsumerID, billingInfo.RequestID, path, err)
		sendErrorResponse(http.StatusServiceUnavailable, config.FailCostMessage)
		return types.ActionContinue
	}

	log.Debugf("[%s] cost deduction request sent, pausing response", pluginName)
	// Pause processing until callback completes
	return types.ActionPause
}

// deductCostAsync deducts the cost from the user's balance asynchronously
// Used in streaming mode where we cannot pause the response
// Errors are logged but do not block the response
func deductCostAsync(ctx wrapper.HttpContext, config BillingConfig, tenantInfo *TenantInfo, billingInfo *BillingInfo) {
	log.Infof("[%s] deducting cost asynchronously: tenantId=%s consumerId=%s consumerName=%s",
		pluginName, tenantInfo.TenantID, tenantInfo.ConsumerID, tenantInfo.ConsumerName)

	// Get consumer API key from context
	consumerApiKey := ""
	if key, ok := ctx.GetContext(CtxKeyConsumerApiKey).(string); ok {
		consumerApiKey = key
	}

	// Get apikey ID from context
	var apikeyID *int64
	if id, ok := ctx.GetContext(CtxKeyApikeyID).(int64); ok && id > 0 {
		apikeyID = &id
	}

	// Build request body (without consumer_id, consumer_name, tenant_id - those are in headers)
	requestBody := CostRequest{
		Provider:         billingInfo.Provider,
		ModelName:        billingInfo.Model,
		RequestID:        billingInfo.RequestID,
		InputTokens:      billingInfo.InputTokens,
		OutputTokens:     billingInfo.OutputTokens,
		CacheReadTokens:  billingInfo.CacheReadTokens,
		CacheWriteTokens: billingInfo.CacheWriteTokens,
		ApiKey:           consumerApiKey,
		ApikeyID:         apikeyID,
	}
	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		log.Errorf("[%s] failed to marshal cost request: tenantId=%s consumerId=%s error=%v",
			pluginName, tenantInfo.TenantID, tenantInfo.ConsumerID, err)
		return
	}

	// Build request headers with tenant info and HMAC authentication
	headers := [][2]string{
		{"content-type", "application/json"},
		{"x-internal-auth-sign-version", tenantInfo.SignVersion},
		{"x-internal-auth-ts", tenantInfo.Timestamp},
		{"x-internal-auth-nonce", tenantInfo.Nonce},
		{"x-internal-auth-sign", tenantInfo.Signature},
		{"x-consumer-id", tenantInfo.ConsumerID},
		{"x-mse-consumer-name", tenantInfo.ConsumerName},
		{"x-mse-tenant-id", tenantInfo.TenantID},
		{"x-domain-resource-id", tenantInfo.DomainResourceID},
		{"x-router-resource-id", tenantInfo.RouterResourceID},
	}

	// Make async HTTP call to billing service
	path := "/v1/cost"

	log.Debugf("[%s] sending async cost deduction request: path=%s body=%s", pluginName, path, string(bodyBytes))

	err = config.billingClient.Post(path, headers, bodyBytes, func(statusCode int, responseHeaders http.Header, responseBody []byte) {
		log.Infof("[%s] async cost deduction response: status=%d body=%s", pluginName, statusCode, string(responseBody))

		// Handle response in callback
		if statusCode != http.StatusOK {
			log.Errorf("[%s] async cost deduction failed: tenantId=%s consumerId=%s consumerName=%s requestId=%s status=%d body=%s reason=http_error",
				pluginName, tenantInfo.TenantID, tenantInfo.ConsumerID, tenantInfo.ConsumerName, billingInfo.RequestID, statusCode, string(responseBody))
			return
		}

		// Parse response
		var costResp CostResponse
		if err := json.Unmarshal(responseBody, &costResp); err != nil {
			log.Errorf("[%s] failed to parse async cost response: tenantId=%s consumerId=%s consumerName=%s requestId=%s error=%v body=%s reason=parse_error",
				pluginName, tenantInfo.TenantID, tenantInfo.ConsumerID, tenantInfo.ConsumerName, billingInfo.RequestID, err, string(responseBody))
			return
		}

		// Check if cost deduction was successful
		if !costResp.Success {
			errorMessage := "billing_service_returned_failure"
			if costResp.Message != "" {
				errorMessage = costResp.Message
			}
			log.Errorf("[%s] async cost deduction failed: tenantId=%s consumerId=%s consumerName=%s requestId=%s reason=%s",
				pluginName, tenantInfo.TenantID, tenantInfo.ConsumerID, tenantInfo.ConsumerName, billingInfo.RequestID, errorMessage)
			return
		}

		log.Infof("[%s] async cost deduction: tenantId=%s consumerId=%s consumerName=%s requestId=%s inputTokens=%d outputTokens=%d cacheReadTokens=%d cacheWriteTokens=%d cost=%s success=%t",
			pluginName, tenantInfo.TenantID, tenantInfo.ConsumerID, tenantInfo.ConsumerName, billingInfo.RequestID, billingInfo.InputTokens, billingInfo.OutputTokens, billingInfo.CacheReadTokens, billingInfo.CacheWriteTokens, costResp.Cost, costResp.Success)

		log.Debugf("[%s] async cost deduction successful", pluginName)
	}, 5000) // 5 second timeout

	if err != nil {
		log.Errorf("[%s] failed to send async cost deduction request: tenantId=%s consumerId=%s consumerName=%s requestId=%s path=%s error=%v reason=send_error",
			pluginName, tenantInfo.TenantID, tenantInfo.ConsumerID, tenantInfo.ConsumerName, billingInfo.RequestID, path, err)
	}
}
