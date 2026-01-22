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
	"mime"
	"net/textproto"
	"strings"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/log"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/pkg/errors"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

func main() {}

const (
	ModeBypass    = 0
	ModeJSON      = 1
	ModeMultipart = 2
)

// FixedSourceHeader represents a header mapping from a fixed source
type FixedSourceHeader struct {
	Source string `yaml:"source"` // Fixed source: authority, route_name, cluster_name, consumer_name
	Target string `yaml:"target"` // Target header name
}

// StaticHeader represents a header with a static value
type StaticHeader struct {
	Key   string `yaml:"key"`   // Header name
	Value string `yaml:"value"` // Header value
}

// PrioritySourceHeader represents a header with multiple source candidates
type PrioritySourceHeader struct {
	Target      string   `yaml:"target"`                // Target header name
	Sources     []string `yaml:"sources"`               // Source header list (by priority)
	StripPrefix string   `yaml:"stripPrefix,omitempty"` // Optional: prefix to strip (e.g., "Bearer ")
}

type AiHeaderModifierConfig struct {
	// AI functionality configuration
	ModelKey           string   `yaml:"modelKey"`
	AddProviderHeader  string   `yaml:"addProviderHeader"`
	ModelToHeader      string   `yaml:"modelToHeader"`
	EnableOnPathSuffix []string `yaml:"enableOnPathSuffix"`

	// Unified header configuration
	FixedSourceHeaders    []FixedSourceHeader    `yaml:"fixedSourceHeaders"`
	StaticHeaders         []StaticHeader         `yaml:"staticHeaders"`
	PrioritySourceHeaders []PrioritySourceHeader `yaml:"prioritySourceHeaders"`

	// Internal state
	mode     int
	boundary string
}

func init() {
	wrapper.SetCtx(
		"ai-header-modifier",
		wrapper.ParseConfigBy(parseConfig),
		wrapper.ProcessRequestHeadersBy(onHttpRequestHeaders),
		wrapper.ProcessRequestBodyBy(onHttpRequestBody),
		wrapper.WithRebuildAfterRequests[AiHeaderModifierConfig](1000),
	)
}

// @Name ai-header-modifier
// @Category ai
// @Phase UNSPECIFIED_PHASE
// @Priority 100
// @Title zh-CN AI 请求头修改器
// @Title en-US AI Header Modifier
// @Description zh-CN ai-header-modifier 插件从 LLM 请求体中提取模型信息并添加到请求头中,用于路由和流量管理。支持从 JSON 和 multipart/form-data 格式的请求体中提取模型名称,并可选地提取提供商信息。
// @Description en-US The ai-header-modifier plugin extracts model information from LLM request bodies and adds it to request headers for routing and traffic management. It supports extracting model names from both JSON and multipart/form-data request bodies, with optional provider extraction.
// @IconUrl https://img.alicdn.com/imgextra/i1/O1CN01iVx287RltL_!!6000000004419-2-tps-42-42.png
// @Version 1.0.0
//
// @Contact.name Higress Team
// @Contact.url http://higress.io/
// @Contact.email admin@higress.io
//
// @Example
// modelKey: model
// modelToHeader: x-higress-llm-model
// addProviderHeader: x-higress-llm-provider
// enableOnPathSuffix:
//   - /v1/chat/completions
//   - /v1/embeddings

func parseConfig(json gjson.Result, config *AiHeaderModifierConfig, log log.Log) error {
	// Set default values
	config.ModelKey = "model"
	config.EnableOnPathSuffix = []string{
		"/completions",
		"/embeddings",
		"/images/generations",
		"/audio/speech",
		"/fine_tuning/jobs",
		"/moderations",
		"/image-synthesis",
		"/video-synthesis",
		"/rerank",
		"/messages",
	}

	// Parse modelKey
	if modelKey := json.Get("modelKey"); modelKey.Exists() {
		if modelKey.Type != gjson.String {
			return errors.New("modelKey must be a string")
		}
		config.ModelKey = modelKey.String()
	}

	// Parse addProviderHeader
	if addProviderHeader := json.Get("addProviderHeader"); addProviderHeader.Exists() {
		if addProviderHeader.Type != gjson.String {
			return errors.New("addProviderHeader must be a string")
		}
		config.AddProviderHeader = addProviderHeader.String()
	}

	// Parse modelToHeader
	if modelToHeader := json.Get("modelToHeader"); modelToHeader.Exists() {
		if modelToHeader.Type != gjson.String {
			return errors.New("modelToHeader must be a string")
		}
		config.ModelToHeader = modelToHeader.String()
	}

	// Parse enableOnPathSuffix
	if enableOnPathSuffix := json.Get("enableOnPathSuffix"); enableOnPathSuffix.Exists() {
		if enableOnPathSuffix.Type != gjson.JSON {
			return errors.New("enableOnPathSuffix must be an array")
		}
		suffixes := make([]string, 0)
		enableOnPathSuffix.ForEach(func(_, value gjson.Result) bool {
			if value.Type != gjson.String {
				return false
			}
			suffixes = append(suffixes, value.String())
			return true
		})
		if len(suffixes) > 0 {
			config.EnableOnPathSuffix = suffixes
		}
	}

	// Parse fixedSourceHeaders
	if fixedSourceHeaders := json.Get("fixedSourceHeaders"); fixedSourceHeaders.Exists() {
		if fixedSourceHeaders.Type != gjson.JSON {
			return errors.New("fixedSourceHeaders must be an array")
		}
		fixedSourceHeaders.ForEach(func(_, value gjson.Result) bool {
			source := value.Get("source").String()
			target := value.Get("target").String()
			if source != "" && target != "" {
				config.FixedSourceHeaders = append(config.FixedSourceHeaders, FixedSourceHeader{
					Source: source,
					Target: target,
				})
			}
			return true
		})
	}

	// Parse staticHeaders
	if staticHeaders := json.Get("staticHeaders"); staticHeaders.Exists() {
		if staticHeaders.Type != gjson.JSON {
			return errors.New("staticHeaders must be an array")
		}
		staticHeaders.ForEach(func(_, value gjson.Result) bool {
			key := value.Get("key").String()
			val := value.Get("value").String()
			if key != "" && val != "" {
				config.StaticHeaders = append(config.StaticHeaders, StaticHeader{
					Key:   key,
					Value: val,
				})
			}
			return true
		})
	}

	// Parse prioritySourceHeaders
	if prioritySourceHeaders := json.Get("prioritySourceHeaders"); prioritySourceHeaders.Exists() {
		if prioritySourceHeaders.Type != gjson.JSON {
			return errors.New("prioritySourceHeaders must be an array")
		}
		prioritySourceHeaders.ForEach(func(_, value gjson.Result) bool {
			target := value.Get("target").String()
			stripPrefix := value.Get("stripPrefix").String()

			var sources []string
			if sourcesArray := value.Get("sources"); sourcesArray.Exists() && sourcesArray.Type == gjson.JSON {
				sourcesArray.ForEach(func(_, src gjson.Result) bool {
					if src.Type == gjson.String {
						sources = append(sources, src.String())
					}
					return true
				})
			}

			if target != "" && len(sources) > 0 {
				config.PrioritySourceHeaders = append(config.PrioritySourceHeaders, PrioritySourceHeader{
					Target:      target,
					Sources:     sources,
					StripPrefix: stripPrefix,
				})
			}
			return true
		})
	}

	// Validate that at least one configuration is provided
	hasAIConfig := config.AddProviderHeader != "" || config.ModelToHeader != ""
	hasHeaderConfig := len(config.FixedSourceHeaders) > 0 ||
		len(config.StaticHeaders) > 0 ||
		len(config.PrioritySourceHeaders) > 0

	if !hasAIConfig && !hasHeaderConfig {
		return errors.New("at least one configuration must be provided (AI headers or custom headers)")
	}

	// Validate fixedSourceHeaders
	validSources := map[string]bool{
		"authority":     true,
		"route_name":    true,
		"cluster_name":  true,
		"consumer_name": true,
	}
	for _, mapping := range config.FixedSourceHeaders {
		if !validSources[mapping.Source] {
			return errors.Errorf("invalid fixed source '%s', must be one of: authority, route_name, cluster_name, consumer_name", mapping.Source)
		}
	}

	log.Infof("ai-header-modifier config: modelKey=%s, modelToHeader=%s, addProviderHeader=%s, enableOnPathSuffix=%v",
		config.ModelKey, config.ModelToHeader, config.AddProviderHeader, config.EnableOnPathSuffix)
	log.Infof("header config: fixedSourceHeaders=%d, staticHeaders=%d, prioritySourceHeaders=%d",
		len(config.FixedSourceHeaders), len(config.StaticHeaders), len(config.PrioritySourceHeaders))

	return nil
}

// processStaticHeaders processes static header configurations
func processStaticHeaders(config AiHeaderModifierConfig, log log.Log) {
	for _, header := range config.StaticHeaders {
		err := proxywasm.ReplaceHttpRequestHeader(header.Key, header.Value)
		if err != nil {
			log.Warnf("Failed to set static header %s: %v", header.Key, err)
		} else {
			log.Debugf("Set static header %s=%s", header.Key, header.Value)
		}
	}
}

// processFixedSourceHeaders processes fixed source header mappings
func processFixedSourceHeaders(config AiHeaderModifierConfig, log log.Log) {
	for _, mapping := range config.FixedSourceHeaders {
		var value string
		var err error

		switch mapping.Source {
		case "authority":
			value, err = proxywasm.GetHttpRequestHeader(":authority")
			if err != nil {
				log.Debugf("Failed to get :authority header: %v", err)
				continue
			}
			// Strip port from authority (e.g., "example.com:8080" -> "example.com")
			if idx := strings.Index(value, ":"); idx != -1 {
				value = value[:idx]
				log.Debugf("Stripped port from authority, new value: %s", value)
			}
		case "route_name":
			valueBytes, propErr := proxywasm.GetProperty([]string{"route_name"})
			if propErr != nil {
				log.Debugf("Failed to get route_name property: %v", propErr)
				continue
			}
			value = string(valueBytes)
		case "cluster_name":
			valueBytes, propErr := proxywasm.GetProperty([]string{"cluster_name"})
			if propErr != nil {
				log.Debugf("Failed to get cluster_name property: %v", propErr)
				continue
			}
			value = string(valueBytes)
		case "consumer_name":
			valueBytes, propErr := proxywasm.GetProperty([]string{"consumer_name"})
			if propErr != nil {
				log.Debugf("Failed to get consumer_name property: %v", propErr)
				continue
			}
			value = string(valueBytes)
		default:
			log.Warnf("Unknown fixed source: %s", mapping.Source)
			continue
		}

		if value == "" {
			log.Debugf("Empty value from source %s, skipping", mapping.Source)
			continue
		}

		err = proxywasm.ReplaceHttpRequestHeader(mapping.Target, value)
		if err != nil {
			log.Warnf("Failed to set header %s: %v", mapping.Target, err)
		} else {
			log.Debugf("Set header %s=%s (from %s)", mapping.Target, value, mapping.Source)
		}
	}
}

// processPrioritySourceHeaders processes priority source header configurations
func processPrioritySourceHeaders(config AiHeaderModifierConfig, log log.Log) {
	for _, mapping := range config.PrioritySourceHeaders {
		var value string
		var foundSource string

		// Try each source in priority order
		for _, source := range mapping.Sources {
			sourceValue, err := proxywasm.GetHttpRequestHeader(source)
			if err != nil || sourceValue == "" {
				continue
			}

			value = sourceValue
			foundSource = source
			break
		}

		if value == "" {
			log.Debugf("No value found for target %s from sources %v", mapping.Target, mapping.Sources)
			continue
		}

		log.Debugf("Found value for %s from source %s", mapping.Target, foundSource)

		// Strip prefix if configured
		if mapping.StripPrefix != "" {
			if strings.HasPrefix(value, mapping.StripPrefix) {
				value = strings.TrimSpace(value[len(mapping.StripPrefix):])
				log.Debugf("Stripped prefix '%s', new value: %s", mapping.StripPrefix, value)
			}
		}

		if value == "" {
			log.Debugf("Value is empty after stripping prefix, skipping")
			continue
		}

		err := proxywasm.ReplaceHttpRequestHeader(mapping.Target, value)
		if err != nil {
			log.Warnf("Failed to set header %s: %v", mapping.Target, err)
		} else {
			log.Debugf("Set header %s=%s (from %s)", mapping.Target, value, foundSource)
		}
	}
}

func onHttpRequestHeaders(ctx wrapper.HttpContext, config AiHeaderModifierConfig, log log.Log) types.Action {
	// Process custom headers first (always execute, not affected by path filtering)
	processStaticHeaders(config, log)
	processFixedSourceHeaders(config, log)
	processPrioritySourceHeaders(config, log)

	// Check if request has body using the reliable method
	// This method checks endOfStream flag and is not affected by content-length header removal
	if !ctx.HasRequestBody() {
		log.Debug("No request body, skipping AI processing")
		return types.ActionContinue
	}

	// Get request path and strip query parameters
	path, err := proxywasm.GetHttpRequestHeader(":path")
	if err != nil {
		log.Warnf("Failed to get request path: %v", err)
		return types.ActionContinue
	}

	// Strip query parameters
	uri := path
	if idx := strings.Index(path, "?"); idx != -1 {
		uri = path[:idx]
	}

	// Check if path matches any enabled suffix
	enabled := false
	for _, suffix := range config.EnableOnPathSuffix {
		if suffix == "*" || strings.HasSuffix(uri, suffix) {
			enabled = true
			break
		}
	}

	if !enabled {
		log.Debugf("Path %s does not match any enabled suffix, skipping processing", uri)
		return types.ActionContinue
	}

	// Get content-type
	contentType, err := proxywasm.GetHttpRequestHeader("content-type")
	if err != nil {
		log.Warnf("Failed to get content-type: %v", err)
		return types.ActionContinue
	}

	// Determine processing mode based on content-type
	if strings.Contains(contentType, "application/json") {
		config.mode = ModeJSON
		log.Debug("Enabled JSON mode")
	} else if strings.Contains(contentType, "multipart/form-data") {
		// Extract boundary from content-type
		boundary := extractBoundary(contentType)
		if boundary == "" {
			log.Warnf("No boundary found in multipart/form-data content-type: %s", contentType)
			return types.ActionContinue
		}
		config.mode = ModeMultipart
		config.boundary = boundary
		log.Debugf("Enabled multipart/form-data mode, boundary=%s", boundary)
	} else {
		log.Debugf("Unsupported content-type: %s, skipping processing", contentType)
		return types.ActionContinue
	}

	// Store config in context for use in body processing
	ctx.SetContext("config", config)

	// Remove content-length header to allow body buffering
	proxywasm.RemoveHttpRequestHeader("content-length")

	// Stop iteration to buffer the body
	return types.HeaderStopIteration
}

func onHttpRequestBody(ctx wrapper.HttpContext, config AiHeaderModifierConfig, body []byte, log log.Log) types.Action {
	// Retrieve config from context
	storedConfig, ok := ctx.GetContext("config").(AiHeaderModifierConfig)
	if !ok {
		log.Warn("Failed to retrieve config from context")
		return types.ActionContinue
	}

	// Process based on mode
	switch storedConfig.mode {
	case ModeJSON:
		processJSONBody(storedConfig, body, log)
	case ModeMultipart:
		processMultipartBody(storedConfig, body, log)
	default:
		log.Debug("Bypass mode, no processing needed")
	}

	return types.ActionContinue
}

func processJSONBody(config AiHeaderModifierConfig, body []byte, log log.Log) {
	// Validate JSON
	if !gjson.ValidBytes(body) {
		log.Warn("Invalid JSON body, skipping processing")
		return
	}

	// Extract model value
	modelValue := gjson.GetBytes(body, config.ModelKey)
	if !modelValue.Exists() {
		log.Debugf("Model key '%s' not found in JSON body", config.ModelKey)
		return
	}

	if modelValue.Type != gjson.String {
		log.Warnf("Model value is not a string: %v", modelValue.Type)
		return
	}

	modelStr := modelValue.String()
	log.Debugf("Extracted model value: %s", modelStr)

	// Add modelToHeader if configured
	if config.ModelToHeader != "" {
		err := proxywasm.ReplaceHttpRequestHeader(config.ModelToHeader, modelStr)
		if err != nil {
			log.Warnf("Failed to add model header: %v", err)
		} else {
			log.Debugf("Added header %s: %s", config.ModelToHeader, modelStr)
		}
	}

	// Extract provider if configured and model contains "/"
	if config.AddProviderHeader != "" {
		if idx := strings.Index(modelStr, "/"); idx != -1 {
			provider := modelStr[:idx]
			modelName := modelStr[idx+1:]

			// Add provider header
			err := proxywasm.ReplaceHttpRequestHeader(config.AddProviderHeader, provider)
			if err != nil {
				log.Warnf("Failed to add provider header: %v", err)
			} else {
				log.Debugf("Added header %s: %s", config.AddProviderHeader, provider)
			}

			// Rewrite body to replace model value with just the model name
			newBody, err := sjson.SetBytes(body, config.ModelKey, modelName)
			if err != nil {
				log.Warnf("Failed to rewrite JSON body: %v", err)
				return
			}

			err = proxywasm.ReplaceHttpRequestBody(newBody)
			if err != nil {
				log.Warnf("Failed to replace request body: %v", err)
			} else {
				log.Debugf("Rewrote body: model changed from '%s' to '%s'", modelStr, modelName)
			}
		} else {
			log.Debugf("Model value '%s' does not contain '/', skipping provider extraction", modelStr)
		}
	}
}

func processMultipartBody(config AiHeaderModifierConfig, body []byte, log log.Log) {
	// Convert body to string for easier manipulation
	bodyStr := string(body)

	// Construct the expected Content-Disposition header for the model field
	modelParamHeader := "Content-Disposition: form-data; name=\"" + config.ModelKey + "\""

	// Find the model parameter section
	headerPos := strings.Index(bodyStr, modelParamHeader)
	if headerPos == -1 {
		log.Debugf("Model key '%s' not found in multipart body", config.ModelKey)
		return
	}

	log.Debugf("Found model field at position %d", headerPos)

	// Find the value start (after \r\n\r\n)
	valueStartMarker := "\r\n\r\n"
	valueStartPos := strings.Index(bodyStr[headerPos:], valueStartMarker)
	if valueStartPos == -1 {
		log.Warn("Could not find value start marker (\\r\\n\\r\\n) in multipart body")
		return
	}
	valueStartPos = headerPos + valueStartPos + len(valueStartMarker)

	// Find the value end (next \r\n)
	valueEndPos := strings.Index(bodyStr[valueStartPos:], "\r\n")
	if valueEndPos == -1 {
		log.Warn("Could not find value end marker (\\r\\n) in multipart body")
		return
	}
	valueEndPos = valueStartPos + valueEndPos

	// Extract the model value
	modelValue := bodyStr[valueStartPos:valueEndPos]
	log.Debugf("Extracted model value: %s", modelValue)

	// Add modelToHeader if configured
	if config.ModelToHeader != "" {
		err := proxywasm.ReplaceHttpRequestHeader(config.ModelToHeader, modelValue)
		if err != nil {
			log.Warnf("Failed to add model header: %v", err)
		} else {
			log.Debugf("Added header %s: %s", config.ModelToHeader, modelValue)
		}
	}

	// Extract provider if configured and model contains "/"
	if config.AddProviderHeader != "" {
		if idx := strings.Index(modelValue, "/"); idx != -1 {
			provider := modelValue[:idx]
			modelName := modelValue[idx+1:]

			// Add provider header
			err := proxywasm.ReplaceHttpRequestHeader(config.AddProviderHeader, provider)
			if err != nil {
				log.Warnf("Failed to add provider header: %v", err)
			} else {
				log.Debugf("Added header %s: %s", config.AddProviderHeader, provider)
			}

			// Rewrite body to replace model value
			newBody := rewriteMultipartBody(body, config.ModelKey, modelValue, modelName, log)
			if newBody != nil {
				err = proxywasm.ReplaceHttpRequestBody(newBody)
				if err != nil {
					log.Warnf("Failed to replace request body: %v", err)
				} else {
					log.Debugf("Rewrote multipart body: model changed from '%s' to '%s'", modelValue, modelName)
				}
			}
		} else {
			log.Debugf("Model value '%s' does not contain '/', skipping provider extraction", modelValue)
		}
	}
}

func rewriteMultipartBody(body []byte, modelKey, oldValue, newValue string, log log.Log) []byte {
	// Use string manipulation approach for simplicity and performance
	bodyStr := string(body)

	// Search for the model field in the multipart body
	modelParamHeader := "Content-Disposition: form-data; name=\"" + modelKey + "\""

	// Find the model parameter section
	headerPos := strings.Index(bodyStr, modelParamHeader)
	if headerPos == -1 {
		log.Warn("Could not find model parameter in multipart body for rewriting")
		return nil
	}

	// Find the value start (after \r\n\r\n)
	valueStartMarker := "\r\n\r\n"
	valueStartPos := strings.Index(bodyStr[headerPos:], valueStartMarker)
	if valueStartPos == -1 {
		log.Warn("Could not find value start marker in multipart body")
		return nil
	}
	valueStartPos = headerPos + valueStartPos + len(valueStartMarker)

	// Find the value end (next \r\n)
	valueEndPos := strings.Index(bodyStr[valueStartPos:], "\r\n")
	if valueEndPos == -1 {
		log.Warn("Could not find value end marker in multipart body")
		return nil
	}
	valueEndPos = valueStartPos + valueEndPos

	// Extract the current value to verify it matches
	currentValue := bodyStr[valueStartPos:valueEndPos]
	if currentValue != oldValue {
		log.Warnf("Current value '%s' does not match expected value '%s'", currentValue, oldValue)
		return nil
	}

	// Reconstruct the body with the new value
	newBody := bodyStr[:valueStartPos] + newValue + bodyStr[valueEndPos:]

	return []byte(newBody)
}

func extractBoundary(contentType string) string {
	// Parse content-type to extract boundary
	_, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return ""
	}
	return params["boundary"]
}

// Helper function to create multipart part header
func createPartHeader(formName string) textproto.MIMEHeader {
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", "form-data; name=\""+formName+"\"")
	return h
}
