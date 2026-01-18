package main

import (
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
	CtxKeyApiKey        = "ai-billing-api-key"
	CtxKeyBillingInfo   = "ai-billing-info"
	CtxKeyIsStreaming   = "ai-billing-is-streaming"
	CtxKeyRequestDenied = "ai-billing-request-denied"
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
	BillingService             BillingServiceConfig `yaml:"billingService"`
	FailBalanceMessage         string               `yaml:"failBalanceMessage"`
	InsufficientBalanceMessage string               `yaml:"insufficientBalanceMessage"`
	FailCostMessage            string               `yaml:"failCostMessage"`
	billingClient              wrapper.HttpClient
}

// BillingServiceConfig holds the billing service connection details
type BillingServiceConfig struct {
	ServiceAddress string `yaml:"serviceAddress"`
	Namespace      string `yaml:"namespace"`
	Protocol       string `yaml:"protocol"`
	Port           int    `yaml:"port"`
}

// BalanceRequest represents the request to check user balance
type BalanceRequest struct {
	ApiKey string `json:"apikey"`
}

// BalanceResponse represents the response from balance check
type BalanceResponse struct {
	Balance   string `json:"balance"`
	UID       int64  `json:"uid"`
	UpdatedAt int64  `json:"updated_at"`
}

// CostRequest represents the request to deduct cost
type CostRequest struct {
	ApiKey       string `json:"apikey"`
	InputTokens  int64  `json:"input_tokens"`
	OutputTokens int64  `json:"output_tokens"`
	ModelName    string `json:"model_name"`
	Provider     string `json:"provider"`
	RequestID    string `json:"request_id"`
}

// CostResponse represents the response from cost deduction
type CostResponse struct {
	BillingEventID   int64  `json:"billing_event_id"`
	Cost             string `json:"cost"`
	CostActual       string `json:"cost_actual"`
	DiscountRatio    string `json:"discount_ratio"`
	RemainingBalance string `json:"remaining_balance"`
	Success          bool   `json:"success"`
}

// BillingInfo holds the billing information extracted from LLM response
type BillingInfo struct {
	InputTokens  int64
	OutputTokens int64
	Model        string
	Provider     string
	RequestID    string
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

	// Parse namespace with default
	namespace := billingService.Get("namespace").String()
	if namespace == "" {
		namespace = "higress-system"
	}
	config.BillingService.Namespace = namespace

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

	// Initialize HTTP client for billing service
	// Use FQDNCluster with service name directly (not full FQDN)
	// Envoy/Istio will handle the service discovery
	config.billingClient = wrapper.NewClusterClient(wrapper.FQDNCluster{
		FQDN: config.BillingService.ServiceAddress,
		Port: int64(port),
	})

	clusterName := config.billingClient.ClusterName()
	log.Infof("[%s] configuration parsed successfully: version=1.0.12-alpha service=%s://%s:%d cluster=%s",
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

	// Extract API key
	apiKey, err := extractApiKey(ctx)
	if err != nil {
		log.Warnf("[%s] failed to extract API key: %v", pluginName, err)
		sendErrorResponseAndMarkDenied(ctx, http.StatusUnauthorized, "Missing API Key")
		return types.ActionContinue
	}

	// Store API key in context
	ctx.SetContext(CtxKeyApiKey, apiKey)
	log.Debugf("[%s] API key extracted: %s", pluginName, maskApiKey(apiKey))

	// Check balance
	return checkBalance(ctx, config, apiKey)
}

// checkBalance checks the user's balance with the billing service
func checkBalance(ctx wrapper.HttpContext, config BillingConfig, apiKey string) types.Action {
	log.Debugf("[%s] checking balance for apikey=%s", pluginName, maskApiKey(apiKey))

	// Build request body
	requestBody := BalanceRequest{
		ApiKey: apiKey,
	}
	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		log.Errorf("[%s] failed to marshal balance request: %v", pluginName, err)
		sendErrorResponseAndMarkDenied(ctx, http.StatusInternalServerError, config.FailBalanceMessage)
		return types.ActionContinue
	}

	// Make async HTTP call to billing service
	// Note: Post() expects only the path, not the full URL
	path := "/v1/amount"

	clusterName := config.billingClient.ClusterName()
	log.Infof("[%s] sending balance check request: cluster=%s path=%s body=%s", pluginName, clusterName, path, string(bodyBytes))

	err = config.billingClient.Post(path, [][2]string{
		{"content-type", "application/json"},
	}, bodyBytes, func(statusCode int, responseHeaders http.Header, responseBody []byte) {
		log.Infof("[%s] balance check response: status=%d body=%s", pluginName, statusCode, string(responseBody))

		// Handle response in callback
		if statusCode != http.StatusOK {
			log.Errorf("[%s] balance check failed: apikey=%s status=%d body=%s",
				pluginName, maskApiKey(apiKey), statusCode, string(responseBody))
			sendErrorResponseAndMarkDenied(ctx, http.StatusServiceUnavailable, config.FailBalanceMessage)
			return
		}

		// Parse response
		var balanceResp BalanceResponse
		if err := json.Unmarshal(responseBody, &balanceResp); err != nil {
			log.Errorf("[%s] failed to parse balance response: apikey=%s error=%v body=%s",
				pluginName, maskApiKey(apiKey), err, string(responseBody))
			sendErrorResponseAndMarkDenied(ctx, http.StatusServiceUnavailable, config.FailBalanceMessage)
			return
		}

		// Parse balance as float
		balance, err := strconv.ParseFloat(balanceResp.Balance, 64)
		if err != nil {
			log.Errorf("[%s] failed to parse balance value: apikey=%s balance=%s error=%v",
				pluginName, maskApiKey(apiKey), balanceResp.Balance, err)
			sendErrorResponseAndMarkDenied(ctx, http.StatusServiceUnavailable, config.FailBalanceMessage)
			return
		}

		log.Infof("[%s] balance check: apikey=%s balance=%s", pluginName, maskApiKey(apiKey), balanceResp.Balance)

		// Check if balance is sufficient
		// Use a small epsilon to handle floating point precision issues
		const epsilon = 0.0001
		if balance < epsilon {
			log.Warnf("[%s] insufficient balance: apikey=%s balance=%s",
				pluginName, maskApiKey(apiKey), balanceResp.Balance)
			sendErrorResponseAndMarkDenied(ctx, http.StatusPaymentRequired, config.InsufficientBalanceMessage)
			return
		}

		// Balance is sufficient, resume request
		log.Debugf("[%s] balance sufficient, resuming request", pluginName)
		proxywasm.ResumeHttpRequest()
	}, 5000) // 5 second timeout

	if err != nil {
		log.Errorf("[%s] failed to send balance check request: apikey=%s path=%s error=%v",
			pluginName, maskApiKey(apiKey), path, err)
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
	statusCode, err := proxywasm.GetHttpResponseHeader(":status")
	if err != nil || statusCode != "200" {
		log.Debugf("[%s] skipping billing for non-200 response: status=%s", pluginName, statusCode)
		return types.ActionContinue
	}

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

	// Get API key from context
	apiKey, ok := ctx.GetContext(CtxKeyApiKey).(string)
	if !ok {
		log.Errorf("[%s] failed to get API key from context", pluginName)
		// FAIL_CLOSE: Send error response for internal error
		sendErrorResponse(http.StatusInternalServerError, "Internal error")
		return types.ActionContinue
	}

	log.Infof("[%s] extracted billing info: apikey=%s requestId=%s inputTokens=%d outputTokens=%d model=%s provider=%s",
		pluginName, maskApiKey(apiKey), requestID, usage.InputToken, usage.OutputToken, model, provider)

	// Deduct cost
	billingInfo := &BillingInfo{
		InputTokens:  usage.InputToken,
		OutputTokens: usage.OutputToken,
		Model:        model,
		Provider:     provider,
		RequestID:    requestID,
	}

	return deductCost(ctx, config, billingInfo, apiKey)
}

// onHttpStreamingResponseBody handles the streaming response body
func onHttpStreamingResponseBody(ctx wrapper.HttpContext, config BillingConfig, data []byte, endOfStream bool) []byte {
	// Check if request was denied in request phase
	if denied, ok := ctx.GetContext(CtxKeyRequestDenied).(bool); ok && denied {
		log.Debugf("[%s] request was denied, skipping streaming response body processing", pluginName)
		return data
	}

	// Call GetTokenUsage on each chunk
	usage := tokenusage.GetTokenUsage(ctx, data)
	if usage.TotalToken > 0 {
		log.Debugf("[%s] extracted token usage from stream: inputTokens=%d outputTokens=%d",
			pluginName, usage.InputToken, usage.OutputToken)

		// Extract model from request header
		model := extractModel(ctx)
		if model == "" {
			log.Warnf("[%s] model not found in x-higress-llm-model header, using model from response: %s", pluginName, usage.Model)
			model = usage.Model
		}

		// Store billing info in context
		billingInfo := &BillingInfo{
			InputTokens:  usage.InputToken,
			OutputTokens: usage.OutputToken,
			Model:        model,
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
		log.Errorf("[%s] failed to extract billing info from stream", pluginName)
		// FAIL_CLOSE: Block response if we cannot extract billing information
		// This is a critical error that indicates the response format is invalid
		sendErrorResponse(http.StatusInternalServerError, "Failed to extract billing information from stream")
		return nil
	}

	// Extract request ID and provider
	billingInfo.RequestID = extractRequestID(ctx, data)
	billingInfo.Provider = extractProvider(ctx)

	// Get API key from context
	apiKey, ok := ctx.GetContext(CtxKeyApiKey).(string)
	if !ok {
		log.Errorf("[%s] failed to get API key from context", pluginName)
		// FAIL_CLOSE: Block response on internal error
		sendErrorResponse(http.StatusInternalServerError, "Internal error")
		return nil
	}

	log.Infof("[%s] extracted billing info from stream: apikey=%s requestId=%s inputTokens=%d outputTokens=%d model=%s provider=%s",
		pluginName, maskApiKey(apiKey), billingInfo.RequestID, billingInfo.InputTokens, billingInfo.OutputTokens, billingInfo.Model, billingInfo.Provider)

	// Deduct cost asynchronously - in streaming mode, we don't pause the response
	// because the stream has already been sent to the client
	// We just fire the billing request and let the stream continue
	deductCostAsync(ctx, config, billingInfo, apiKey)

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
func extractModel(ctx wrapper.HttpContext) string {
	// Try to get from x-higress-llm-model header
	if model, err := proxywasm.GetHttpRequestHeader("x-higress-llm-model"); err == nil && model != "" {
		return model
	}

	// Fallback to empty string if not found
	return ""
}

// extractProvider extracts the provider information from context or route
func extractProvider(ctx wrapper.HttpContext) string {
	// Priority 1: Try to get from x-ai-Provider header (set by ai-proxy plugin)
	// This header is user origin request add
	if provider, err := proxywasm.GetHttpRequestHeader("x-ai-provider"); err == nil && provider != "" {
		return provider
	}

	return "default"
}

// deductCost deducts the cost from the user's balance
// Used in non-streaming mode where we can pause and block the response
func deductCost(ctx wrapper.HttpContext, config BillingConfig, billingInfo *BillingInfo, apiKey string) types.Action {
	log.Debugf("[%s] deducting cost for apikey=%s", pluginName, maskApiKey(apiKey))

	// Build request body
	requestBody := CostRequest{
		ApiKey:       apiKey,
		InputTokens:  billingInfo.InputTokens,
		OutputTokens: billingInfo.OutputTokens,
		ModelName:    billingInfo.Model,
		Provider:     billingInfo.Provider,
		RequestID:    billingInfo.RequestID,
	}
	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		log.Errorf("[%s] failed to marshal cost request: %v", pluginName, err)
		sendErrorResponse(http.StatusInternalServerError, config.FailCostMessage)
		return types.ActionContinue
	}

	// Make async HTTP call to billing service
	// Note: Post() expects only the path, not the full URL
	path := "/v1/cost"

	log.Debugf("[%s] sending cost deduction request: path=%s body=%s", pluginName, path, string(bodyBytes))

	err = config.billingClient.Post(path, [][2]string{
		{"content-type", "application/json"},
	}, bodyBytes, func(statusCode int, responseHeaders http.Header, responseBody []byte) {
		log.Infof("[%s] cost deduction response: status=%d body=%s", pluginName, statusCode, string(responseBody))

		// Handle response in callback
		if statusCode != http.StatusOK {
			log.Errorf("[%s] cost deduction failed: apikey=%s requestId=%s status=%d body=%s",
				pluginName, maskApiKey(apiKey), billingInfo.RequestID, statusCode, string(responseBody))
			sendErrorResponse(http.StatusServiceUnavailable, config.FailCostMessage)
			return
		}

		// Parse response
		var costResp CostResponse
		if err := json.Unmarshal(responseBody, &costResp); err != nil {
			log.Errorf("[%s] failed to parse cost response: apikey=%s requestId=%s error=%v body=%s",
				pluginName, maskApiKey(apiKey), billingInfo.RequestID, err, string(responseBody))
			sendErrorResponse(http.StatusServiceUnavailable, config.FailCostMessage)
			return
		}

		log.Infof("[%s] cost deduction: apikey=%s requestId=%s inputTokens=%d outputTokens=%d cost=%s success=%t",
			pluginName, maskApiKey(apiKey), billingInfo.RequestID, billingInfo.InputTokens, billingInfo.OutputTokens, costResp.Cost, costResp.Success)

		// Check if cost deduction was successful
		if !costResp.Success {
			log.Warnf("[%s] cost deduction failed: apikey=%s requestId=%s",
				pluginName, maskApiKey(apiKey), billingInfo.RequestID)
			sendErrorResponse(http.StatusPaymentRequired, config.InsufficientBalanceMessage)
			return
		}

		// Cost deduction successful, resume response
		log.Debugf("[%s] cost deduction successful, resuming response", pluginName)
		proxywasm.ResumeHttpResponse()
	}, 5000) // 5 second timeout

	if err != nil {
		log.Errorf("[%s] failed to send cost deduction request: apikey=%s requestId=%s path=%s error=%v",
			pluginName, maskApiKey(apiKey), billingInfo.RequestID, path, err)
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
func deductCostAsync(ctx wrapper.HttpContext, config BillingConfig, billingInfo *BillingInfo, apiKey string) {
	log.Debugf("[%s] deducting cost asynchronously for apikey=%s", pluginName, maskApiKey(apiKey))

	// Build request body
	requestBody := CostRequest{
		ApiKey:       apiKey,
		InputTokens:  billingInfo.InputTokens,
		OutputTokens: billingInfo.OutputTokens,
		ModelName:    billingInfo.Model,
		Provider:     billingInfo.Provider,
		RequestID:    billingInfo.RequestID,
	}
	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		log.Errorf("[%s] failed to marshal cost request: %v", pluginName, err)
		return
	}

	// Make async HTTP call to billing service
	path := "/v1/cost"

	log.Debugf("[%s] sending async cost deduction request: path=%s body=%s", pluginName, path, string(bodyBytes))

	err = config.billingClient.Post(path, [][2]string{
		{"content-type", "application/json"},
	}, bodyBytes, func(statusCode int, responseHeaders http.Header, responseBody []byte) {
		log.Infof("[%s] async cost deduction response: status=%d body=%s", pluginName, statusCode, string(responseBody))

		// Handle response in callback
		if statusCode != http.StatusOK {
			log.Errorf("[%s] async cost deduction failed: apikey=%s requestId=%s status=%d body=%s",
				pluginName, maskApiKey(apiKey), billingInfo.RequestID, statusCode, string(responseBody))
			return
		}

		// Parse response
		var costResp CostResponse
		if err := json.Unmarshal(responseBody, &costResp); err != nil {
			log.Errorf("[%s] failed to parse async cost response: apikey=%s requestId=%s error=%v body=%s",
				pluginName, maskApiKey(apiKey), billingInfo.RequestID, err, string(responseBody))
			return
		}

		log.Infof("[%s] async cost deduction: apikey=%s requestId=%s inputTokens=%d outputTokens=%d cost=%s success=%t",
			pluginName, maskApiKey(apiKey), billingInfo.RequestID, billingInfo.InputTokens, billingInfo.OutputTokens, costResp.Cost, costResp.Success)

		// Check if cost deduction was successful
		if !costResp.Success {
			log.Warnf("[%s] async cost deduction failed: apikey=%s requestId=%s",
				pluginName, maskApiKey(apiKey), billingInfo.RequestID)
			return
		}

		log.Debugf("[%s] async cost deduction successful", pluginName)
	}, 5000) // 5 second timeout

	if err != nil {
		log.Errorf("[%s] failed to send async cost deduction request: apikey=%s requestId=%s path=%s error=%v",
			pluginName, maskApiKey(apiKey), billingInfo.RequestID, path, err)
	}
}
