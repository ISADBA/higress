package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/log"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/tidwall/gjson"
)

const (
	pluginName = "ai-capability-handler"
)

// Context keys for storing data across request lifecycle
const (
	CtxKeyResponseCheck = "ai-capability-response-check"
	CtxKeyRequestDenied = "ai-capability-request-denied"
	CtxKeyRequestInfo   = "ai-capability-request-info"
)

func main() {}

func init() {
	wrapper.SetCtx(
		pluginName,
		wrapper.ParseConfig(parseConfig),
		wrapper.ProcessRequestHeaders(onHttpRequestHeaders),
		wrapper.ProcessRequestBody(onHttpRequestBody),
		wrapper.ProcessResponseHeaders(onHttpResponseHeaders),
		wrapper.ProcessResponseBody(onHttpResponseBody),
		wrapper.ProcessStreamingResponseBody(onHttpStreamingResponseBody),
	)
}

// CapabilityConfig holds the plugin configuration
type CapabilityConfig struct {
	CapabilityService  CapabilityServiceConfig `yaml:"capability_service"`
	TimeoutDenyMessage TimeoutDenyMessage      `yaml:"timeout_deny_message"`
	capabilityClient   wrapper.HttpClient
}

// CapabilityServiceConfig holds the capability service connection details
type CapabilityServiceConfig struct {
	URL           string `yaml:"url"`
	TimeoutMs     int    `yaml:"timeout_ms"`
	TimeoutAction string `yaml:"timeout_action"`
}

// TimeoutDenyMessage holds the timeout denial response configuration
type TimeoutDenyMessage struct {
	Code    int    `yaml:"code"`
	Message string `yaml:"message"`
}

// VerifyRequest represents the request to capability service
type VerifyRequest struct {
	RequestID string      `json:"request_id"`
	Request   RequestInfo `json:"request"`
}

// RequestInfo holds the request information
type RequestInfo struct {
	Method  string         `json:"method"`
	Path    string         `json:"path"`
	Headers []KeyValuePair `json:"headers"`
	Query   []KeyValuePair `json:"query"`
	Body    string         `json:"body"`
}

// KeyValuePair represents a key-value pair
type KeyValuePair struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// VerifyResponse represents the response from capability service
type VerifyResponse struct {
	Allow         bool           `json:"allow"`
	BlockInfo     *BlockInfo     `json:"block_info,omitempty"`
	Actions       []Action       `json:"actions"`
	ResponseCheck *ResponseCheck `json:"response_check,omitempty"`
}

// BlockInfo holds the blocking information
type BlockInfo struct {
	Code         int    `json:"code"`
	Message      string `json:"message"`
	CapabilityID int    `json:"capability_id"`
}

// Action represents an action to be performed on the request
type Action struct {
	Type         string `json:"type"`   // replace, unset
	Target       string `json:"target"` // header, body, query, path
	Key          string `json:"key"`
	Value        string `json:"value"`
	CapabilityID int    `json:"capability_id"`
}

// ResponseCheck holds response check configuration
type ResponseCheck struct {
	Enable       bool `json:"enable"`
	CapabilityID int  `json:"capability_id"`
}

// parseConfig parses the plugin configuration
func parseConfig(json gjson.Result, config *CapabilityConfig) error {
	// Skip logging in test environment to avoid runtime dependency
	// log.Infof("[%s] parsing configuration", pluginName)

	// Parse capability service configuration
	capabilityService := json.Get("capability_service")
	if !capabilityService.Exists() {
		return errors.New("missing capability_service in config")
	}

	serviceURL := capabilityService.Get("url").String()
	if serviceURL == "" {
		return errors.New("capability_service.url must not be empty")
	}
	config.CapabilityService.URL = serviceURL

	// Parse timeout with default
	timeoutMs := capabilityService.Get("timeout_ms").Int()
	if timeoutMs == 0 {
		timeoutMs = 1000
	}
	config.CapabilityService.TimeoutMs = int(timeoutMs)

	// Parse timeout action with default
	timeoutAction := capabilityService.Get("timeout_action").String()
	if timeoutAction == "" {
		timeoutAction = "continue"
	}
	if timeoutAction != "continue" && timeoutAction != "deny" {
		return errors.New("capability_service.timeout_action must be 'continue' or 'deny'")
	}
	config.CapabilityService.TimeoutAction = timeoutAction

	// Parse timeout deny message with defaults
	timeoutDenyMsg := json.Get("timeout_deny_message")
	config.TimeoutDenyMessage.Code = int(timeoutDenyMsg.Get("code").Int())
	if config.TimeoutDenyMessage.Code == 0 {
		config.TimeoutDenyMessage.Code = 501
	}

	config.TimeoutDenyMessage.Message = timeoutDenyMsg.Get("message").String()
	if config.TimeoutDenyMessage.Message == "" {
		config.TimeoutDenyMessage.Message = "能力处理超时，请联系管理员"
	}

	// Initialize HTTP client for capability service
	// Parse URL to extract host and port
	parsedURL, err := url.Parse(config.CapabilityService.URL)
	if err != nil {
		return fmt.Errorf("invalid capability service URL: %v", err)
	}

	host := parsedURL.Hostname()
	port := parsedURL.Port()
	if port == "" {
		if parsedURL.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}

	portInt, err := strconv.Atoi(port)
	if err != nil {
		return fmt.Errorf("invalid port in capability service URL: %v", err)
	}

	config.capabilityClient = wrapper.NewClusterClient(wrapper.FQDNCluster{
		FQDN: host,
		Port: int64(portInt),
	})

	// Skip logging in test environment to avoid runtime dependency
	// log.Infof("[%s] configuration parsed successfully: url=%s timeout=%dms action=%s",
	//	pluginName, config.CapabilityService.URL, config.CapabilityService.TimeoutMs, config.CapabilityService.TimeoutAction)

	return nil
}

// extractRequestInfo extracts request information for capability service
func extractRequestInfo(ctx wrapper.HttpContext) (*RequestInfo, error) {
	requestInfo := &RequestInfo{}

	// Extract method
	method, err := proxywasm.GetHttpRequestHeader(":method")
	if err != nil {
		return nil, fmt.Errorf("failed to get method: %v", err)
	}
	requestInfo.Method = method

	// Extract path (without query parameters)
	fullPath, err := proxywasm.GetHttpRequestHeader(":path")
	if err != nil {
		return nil, fmt.Errorf("failed to get path: %v", err)
	}

	// Parse URL to separate path and query
	parsedURL, err := url.Parse(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse path: %v", err)
	}
	requestInfo.Path = parsedURL.Path

	// Extract query parameters
	if parsedURL.RawQuery != "" {
		queryParams, err := url.ParseQuery(parsedURL.RawQuery)
		if err != nil {
			return nil, fmt.Errorf("failed to parse query: %v", err)
		}
		for key, values := range queryParams {
			for _, value := range values {
				requestInfo.Query = append(requestInfo.Query, KeyValuePair{
					Key:   key,
					Value: value,
				})
			}
		}
	}

	// Extract headers (skip pseudo headers that start with :)
	headers, err := proxywasm.GetHttpRequestHeaders()
	if err != nil {
		return nil, fmt.Errorf("failed to get headers: %v", err)
	}
	for _, header := range headers {
		if !strings.HasPrefix(header[0], ":") {
			requestInfo.Headers = append(requestInfo.Headers, KeyValuePair{
				Key:   header[0],
				Value: header[1],
			})
		}
	}

	return requestInfo, nil
}

// sendErrorResponse sends an error response to the client
func sendErrorResponse(statusCode int, message string) {
	errorBody := fmt.Sprintf(`{"error":{"message":"%s","type":"capability_error"}}`, message)
	_ = proxywasm.SendHttpResponseWithDetail(uint32(statusCode), "ai-capability-handler.error", [][2]string{
		{"content-type", "application/json"},
	}, []byte(errorBody), -1)
}

// sendErrorResponseAndMarkDenied sends an error response and marks the request as denied
func sendErrorResponseAndMarkDenied(ctx wrapper.HttpContext, statusCode int, message string) {
	ctx.SetContext(CtxKeyRequestDenied, true)
	sendErrorResponse(statusCode, message)
}

// onHttpRequestHeaders handles the request headers phase
func onHttpRequestHeaders(ctx wrapper.HttpContext, config CapabilityConfig) types.Action {
	log.Debugf("[%s] processing request headers", pluginName)

	// Extract request information
	requestInfo, err := extractRequestInfo(ctx)
	if err != nil {
		log.Errorf("[%s] failed to extract request info: %v", pluginName, err)
		sendErrorResponseAndMarkDenied(ctx, http.StatusInternalServerError, "Failed to extract request information")
		return types.ActionContinue
	}

	// Always buffer body and store context to handle all cases consistently
	// This ensures we process the request in onHttpRequestBody with complete information
	ctx.BufferRequestBody()
	ctx.SetContext(CtxKeyRequestInfo, requestInfo)

	log.Debugf("[%s] buffering request, waiting for body phase", pluginName)
	return types.ActionContinue
}

// onHttpRequestBody handles the request body phase
func onHttpRequestBody(ctx wrapper.HttpContext, config CapabilityConfig, body []byte) types.Action {
	log.Debugf("[%s] processing request body, size=%d bytes", pluginName, len(body))

	// Get request info from context
	requestInfo, ok := ctx.GetContext(CtxKeyRequestInfo).(*RequestInfo)
	if !ok {
		log.Errorf("[%s] failed to get request info from context", pluginName)
		sendErrorResponseAndMarkDenied(ctx, http.StatusInternalServerError, "Internal error")
		return types.ActionContinue
	}

	// Add body to request info (base64 encoded) if present
	if len(body) > 0 {
		requestInfo.Body = base64.StdEncoding.EncodeToString(body)
		log.Debugf("[%s] encoded request body: %d bytes", pluginName, len(requestInfo.Body))
	} else {
		log.Debugf("[%s] no request body to encode", pluginName)
	}

	return processCapabilityRequest(ctx, config, requestInfo)
}

// processCapabilityRequest processes the capability request
func processCapabilityRequest(ctx wrapper.HttpContext, config CapabilityConfig, requestInfo *RequestInfo) types.Action {
	log.Infof("[%s] processing capability request: method=%s path=%s", pluginName, requestInfo.Method, requestInfo.Path)

	// Generate request ID (use existing x-request-id if available)
	requestID, _ := proxywasm.GetHttpRequestHeader("x-request-id")
	if requestID == "" {
		requestID = fmt.Sprintf("cap-%d", 1000000) // Simple fallback
	}

	// Build verify request
	verifyReq := VerifyRequest{
		RequestID: requestID,
		Request:   *requestInfo,
	}

	// Marshal request body
	bodyBytes, err := json.Marshal(verifyReq)
	if err != nil {
		log.Errorf("[%s] failed to marshal verify request: %v", pluginName, err)
		return handleCapabilityError(ctx, config, "Failed to prepare capability request")
	}

	// Build request headers
	headers := [][2]string{
		{"content-type", "application/json"},
	}

	// Extract path from URL
	parsedURL, err := url.Parse(config.CapabilityService.URL)
	if err != nil {
		log.Errorf("[%s] failed to parse capability service URL: %v", pluginName, err)
		return handleCapabilityError(ctx, config, "Invalid capability service configuration")
	}

	path := parsedURL.Path
	if path == "" {
		path = "/v1/capability/verify-request"
	}

	log.Debugf("[%s] sending capability request: path=%s body=%s", pluginName, path, string(bodyBytes))

	// Make async HTTP call to capability service
	err = config.capabilityClient.Post(path, headers, bodyBytes, func(statusCode int, responseHeaders http.Header, responseBody []byte) {
		log.Infof("[%s] capability response: status=%d body=%s", pluginName, statusCode, string(responseBody))

		// Handle response in callback
		if statusCode != http.StatusOK {
			log.Errorf("[%s] capability request failed: status=%d body=%s", pluginName, statusCode, string(responseBody))
			handleCapabilityErrorInCallback(ctx, config, "Capability service error")
			return
		}

		// Parse response
		var verifyResp VerifyResponse
		if err := json.Unmarshal(responseBody, &verifyResp); err != nil {
			log.Errorf("[%s] failed to parse capability response: error=%v body=%s", pluginName, err, string(responseBody))
			handleCapabilityErrorInCallback(ctx, config, "Invalid capability service response")
			return
		}

		// Process response
		processCapabilityResponse(ctx, &verifyResp)
	}, uint32(config.CapabilityService.TimeoutMs))

	if err != nil {
		log.Errorf("[%s] failed to send capability request: error=%v", pluginName, err)
		return handleCapabilityError(ctx, config, "Failed to connect to capability service")
	}

	log.Debugf("[%s] capability request sent, pausing request", pluginName)
	// Pause processing until callback completes
	return types.ActionPause
}

// handleCapabilityError handles capability service errors
func handleCapabilityError(ctx wrapper.HttpContext, config CapabilityConfig, message string) types.Action {
	if config.CapabilityService.TimeoutAction == "deny" {
		log.Errorf("[%s] capability error, denying request: %s", pluginName, message)
		sendErrorResponseAndMarkDenied(ctx, config.TimeoutDenyMessage.Code, config.TimeoutDenyMessage.Message)
	} else {
		log.Warnf("[%s] capability error, continuing request: %s", pluginName, message)
	}
	return types.ActionContinue
}

// handleCapabilityErrorInCallback handles capability service errors in callback
func handleCapabilityErrorInCallback(ctx wrapper.HttpContext, config CapabilityConfig, message string) {
	if config.CapabilityService.TimeoutAction == "deny" {
		log.Errorf("[%s] capability error, denying request: %s", pluginName, message)
		sendErrorResponseAndMarkDenied(ctx, config.TimeoutDenyMessage.Code, config.TimeoutDenyMessage.Message)
	} else {
		log.Warnf("[%s] capability error, continuing request: %s", pluginName, message)
		proxywasm.ResumeHttpRequest()
	}
}

// processCapabilityResponse processes the capability service response
func processCapabilityResponse(ctx wrapper.HttpContext, response *VerifyResponse) {
	log.Infof("[%s] processing capability response: allow=%t actions=%d", pluginName, response.Allow, len(response.Actions))

	// Check if request should be blocked
	if !response.Allow {
		if response.BlockInfo != nil {
			log.Warnf("[%s] request blocked by capability: code=%d message=%s capability_id=%d",
				pluginName, response.BlockInfo.Code, response.BlockInfo.Message, response.BlockInfo.CapabilityID)
			sendErrorResponseAndMarkDenied(ctx, response.BlockInfo.Code, response.BlockInfo.Message)
		} else {
			log.Warnf("[%s] request blocked by capability: no block info provided", pluginName)
			sendErrorResponseAndMarkDenied(ctx, http.StatusForbidden, "Request blocked by capability")
		}
		return
	}

	// Process actions
	if len(response.Actions) > 0 {
		err := processActions(ctx, response.Actions)
		if err != nil {
			log.Errorf("[%s] failed to process actions: %v", pluginName, err)
			sendErrorResponseAndMarkDenied(ctx, http.StatusInternalServerError, "Failed to process capability actions")
			return
		}
	}

	// Store response check info for response phase
	if response.ResponseCheck != nil && response.ResponseCheck.Enable {
		ctx.SetContext(CtxKeyResponseCheck, response.ResponseCheck)
		log.Debugf("[%s] response check enabled: capability_id=%d", pluginName, response.ResponseCheck.CapabilityID)
	}

	// Resume request processing
	log.Debugf("[%s] capability processing completed, resuming request", pluginName)
	proxywasm.ResumeHttpRequest()
}

// processActions processes the actions returned by capability service
func processActions(ctx wrapper.HttpContext, actions []Action) error {
	log.Debugf("[%s] processing %d actions", pluginName, len(actions))

	var bodyModified bool
	var newBody []byte

	for i, action := range actions {
		log.Debugf("[%s] processing action %d: type=%s target=%s key=%s capability_id=%d",
			pluginName, i, action.Type, action.Target, action.Key, action.CapabilityID)

		switch action.Target {
		case "header":
			err := processHeaderAction(action)
			if err != nil {
				return fmt.Errorf("failed to process header action: %v", err)
			}

		case "query":
			err := processQueryAction(action)
			if err != nil {
				return fmt.Errorf("failed to process query action: %v", err)
			}

		case "path":
			err := processPathAction(action)
			if err != nil {
				return fmt.Errorf("failed to process path action: %v", err)
			}

		case "body":
			var err error
			newBody, err = processBodyAction(action, newBody)
			if err != nil {
				return fmt.Errorf("failed to process body action: %v", err)
			}
			bodyModified = true

		default:
			log.Warnf("[%s] unknown action target: %s", pluginName, action.Target)
		}
	}

	// If body was modified, update the request body and Content-Length
	if bodyModified {
		err := updateRequestBody(newBody)
		if err != nil {
			return fmt.Errorf("failed to update request body: %v", err)
		}
	}

	return nil
}

// processHeaderAction processes header actions
func processHeaderAction(action Action) error {
	switch action.Type {
	case "replace":
		err := proxywasm.ReplaceHttpRequestHeader(action.Key, action.Value)
		if err != nil {
			return fmt.Errorf("failed to replace header %s: %v", action.Key, err)
		}
		log.Debugf("[%s] header replaced: %s=%s", pluginName, action.Key, action.Value)

	case "unset":
		err := proxywasm.RemoveHttpRequestHeader(action.Key)
		if err != nil {
			// Log but don't fail - header might not exist
			log.Debugf("[%s] failed to remove header %s (might not exist): %v", pluginName, action.Key, err)
		} else {
			log.Debugf("[%s] header removed: %s", pluginName, action.Key)
		}

	default:
		return fmt.Errorf("unknown action type for header: %s", action.Type)
	}

	return nil
}

// processQueryAction processes query parameter actions
func processQueryAction(action Action) error {
	// Get current path
	currentPath, err := proxywasm.GetHttpRequestHeader(":path")
	if err != nil {
		return fmt.Errorf("failed to get current path: %v", err)
	}

	// Parse URL
	parsedURL, err := url.Parse(currentPath)
	if err != nil {
		return fmt.Errorf("failed to parse current path: %v", err)
	}

	// Parse query parameters
	queryParams := parsedURL.Query()

	switch action.Type {
	case "replace":
		queryParams.Set(action.Key, action.Value)
		log.Debugf("[%s] query parameter replaced: %s=%s", pluginName, action.Key, action.Value)

	case "unset":
		queryParams.Del(action.Key)
		log.Debugf("[%s] query parameter removed: %s", pluginName, action.Key)

	default:
		return fmt.Errorf("unknown action type for query: %s", action.Type)
	}

	// Rebuild URL with modified query
	parsedURL.RawQuery = queryParams.Encode()
	newPath := parsedURL.String()

	// Update path header
	err = proxywasm.ReplaceHttpRequestHeader(":path", newPath)
	if err != nil {
		return fmt.Errorf("failed to update path header: %v", err)
	}

	return nil
}

// processPathAction processes path actions
func processPathAction(action Action) error {
	switch action.Type {
	case "replace":
		// Get current path to preserve query parameters
		currentPath, err := proxywasm.GetHttpRequestHeader(":path")
		if err != nil {
			return fmt.Errorf("failed to get current path: %v", err)
		}

		// Parse URL to get query parameters
		parsedURL, err := url.Parse(currentPath)
		if err != nil {
			return fmt.Errorf("failed to parse current path: %v", err)
		}

		// Create new URL with new path but preserve query
		newURL := &url.URL{
			Path:     action.Value,
			RawQuery: parsedURL.RawQuery,
		}

		newPath := newURL.String()
		err = proxywasm.ReplaceHttpRequestHeader(":path", newPath)
		if err != nil {
			return fmt.Errorf("failed to replace path: %v", err)
		}
		log.Debugf("[%s] path replaced: %s", pluginName, newPath)

	case "unset":
		// Unset doesn't make sense for path, log warning
		log.Warnf("[%s] unset action not supported for path target", pluginName)

	default:
		return fmt.Errorf("unknown action type for path: %s", action.Type)
	}

	return nil
}

// processBodyAction processes body actions
func processBodyAction(action Action, currentBody []byte) ([]byte, error) {
	switch action.Type {
	case "replace":
		// Decode base64 value
		newBodyBytes, err := base64.StdEncoding.DecodeString(action.Value)
		if err != nil {
			return nil, fmt.Errorf("failed to decode base64 body: %v", err)
		}
		log.Debugf("[%s] body replaced with %d bytes", pluginName, len(newBodyBytes))
		return newBodyBytes, nil

	case "unset":
		// Unset body means empty body
		log.Debugf("[%s] body cleared", pluginName)
		return []byte{}, nil

	default:
		return nil, fmt.Errorf("unknown action type for body: %s", action.Type)
	}
}

// updateRequestBody updates the request body and Content-Length header
func updateRequestBody(newBody []byte) error {
	// Replace request body
	err := proxywasm.ReplaceHttpRequestBody(newBody)
	if err != nil {
		return fmt.Errorf("failed to replace request body: %v", err)
	}

	// Update Content-Length header
	contentLength := strconv.Itoa(len(newBody))
	err = proxywasm.ReplaceHttpRequestHeader("content-length", contentLength)
	if err != nil {
		return fmt.Errorf("failed to update content-length header: %v", err)
	}

	log.Debugf("[%s] request body updated: new_length=%d", pluginName, len(newBody))
	return nil
}

// onHttpResponseHeaders handles the response headers phase
func onHttpResponseHeaders(ctx wrapper.HttpContext, config CapabilityConfig) types.Action {
	// Check if request was denied in request phase
	if denied, ok := ctx.GetContext(CtxKeyRequestDenied).(bool); ok && denied {
		log.Debugf("[%s] request was denied, skipping response processing", pluginName)
		return types.ActionContinue
	}

	// Check if response check is enabled
	if responseCheck, ok := ctx.GetContext(CtxKeyResponseCheck).(*ResponseCheck); ok && responseCheck != nil {
		log.Infof("[%s] response check enabled: capability_id=%d", pluginName, responseCheck.CapabilityID)
		// Currently only log, no specific logic implemented
		// Future enhancement: implement actual response checking logic
	}

	return types.ActionContinue
}

// onHttpResponseBody handles the non-streaming response body
func onHttpResponseBody(ctx wrapper.HttpContext, config CapabilityConfig, body []byte) types.Action {
	// Check if request was denied in request phase
	if denied, ok := ctx.GetContext(CtxKeyRequestDenied).(bool); ok && denied {
		log.Debugf("[%s] request was denied, skipping response body processing", pluginName)
		return types.ActionContinue
	}

	// Check if response check is enabled
	if responseCheck, ok := ctx.GetContext(CtxKeyResponseCheck).(*ResponseCheck); ok && responseCheck != nil {
		log.Infof("[%s] response body received (non-streaming): capability_id=%d size=%d",
			pluginName, responseCheck.CapabilityID, len(body))
		// Currently only log, no specific logic implemented
		// Future enhancement: implement actual response checking logic
	}

	return types.ActionContinue
}

// onHttpStreamingResponseBody handles the streaming response body
func onHttpStreamingResponseBody(ctx wrapper.HttpContext, config CapabilityConfig, data []byte, endOfStream bool) []byte {
	// Check if request was denied in request phase
	if denied, ok := ctx.GetContext(CtxKeyRequestDenied).(bool); ok && denied {
		log.Debugf("[%s] request was denied, skipping streaming response body processing", pluginName)
		return data
	}

	// Check if response check is enabled
	if responseCheck, ok := ctx.GetContext(CtxKeyResponseCheck).(*ResponseCheck); ok && responseCheck != nil {
		if endOfStream {
			log.Infof("[%s] streaming response completed: capability_id=%d",
				pluginName, responseCheck.CapabilityID)
		} else {
			log.Debugf("[%s] streaming response chunk received: capability_id=%d size=%d",
				pluginName, responseCheck.CapabilityID, len(data))
		}
		// Currently only log, no specific logic implemented
		// Future enhancement: implement actual response checking logic
	}

	// Return data unchanged
	return data
}
