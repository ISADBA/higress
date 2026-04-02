package main

import (
	"bytes"
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"regexp"
	"strings"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/log"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const (
	DefaultMaxBodyBytes = 100 * 1024 * 1024 // 100MB
	AutoModelPrefix     = "higress/auto"
)

func main() {}

func init() {
	wrapper.SetCtx(
		"model-router",
		wrapper.ParseConfig(parseConfig),
		wrapper.ProcessRequestHeaders(onHttpRequestHeaders),
		wrapper.ProcessRequestBody(onHttpRequestBody),
		wrapper.WithRebuildAfterRequests[ModelRouterConfig](1000),
		wrapper.WithRebuildMaxMemBytes[ModelRouterConfig](200*1024*1024),
	)
}

// AutoRoutingRule defines a regex-based routing rule for auto model selection
type AutoRoutingRule struct {
	Pattern *regexp.Regexp
	Model   string
}

type ModelRouterConfig struct {
	modelKey           string
	addProviderHeader  string
	modelToHeader      string
	enableOnPathSuffix []string
	// Auto routing configuration
	enableAutoRouting bool
	autoRoutingRules  []AutoRoutingRule
	defaultModel      string
	// URI provider extraction
	providers []string
}

func parseConfig(json gjson.Result, config *ModelRouterConfig) error {
	config.modelKey = json.Get("modelKey").String()
	if config.modelKey == "" {
		config.modelKey = "model"
	}
	config.addProviderHeader = json.Get("addProviderHeader").String()
	config.modelToHeader = json.Get("modelToHeader").String()

	enableOnPathSuffix := json.Get("enableOnPathSuffix")
	if enableOnPathSuffix.Exists() && enableOnPathSuffix.IsArray() {
		for _, item := range enableOnPathSuffix.Array() {
			config.enableOnPathSuffix = append(config.enableOnPathSuffix, item.String())
		}
	} else {
		// Default suffixes if not provided
		config.enableOnPathSuffix = []string{
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
	}

	// Parse auto routing configuration
	autoRouting := json.Get("autoRouting")
	if autoRouting.Exists() {
		config.enableAutoRouting = autoRouting.Get("enable").Bool()
		config.defaultModel = autoRouting.Get("defaultModel").String()

		rules := autoRouting.Get("rules")
		if rules.Exists() && rules.IsArray() {
			for _, rule := range rules.Array() {
				patternStr := rule.Get("pattern").String()
				model := rule.Get("model").String()
				if patternStr == "" || model == "" {
					log.Warnf("skipping invalid auto routing rule: pattern=%s, model=%s", patternStr, model)
					continue
				}
				compiled, err := regexp.Compile(patternStr)
				if err != nil {
					log.Warnf("failed to compile regex pattern '%s': %v", patternStr, err)
					continue
				}
				config.autoRoutingRules = append(config.autoRoutingRules, AutoRoutingRule{
					Pattern: compiled,
					Model:   model,
				})
				log.Debugf("loaded auto routing rule: pattern=%s, model=%s", patternStr, model)
			}
		}
	}

	// Parse providers configuration
	providers := json.Get("providers")
	if providers.Exists() && providers.IsArray() {
		for _, item := range providers.Array() {
			config.providers = append(config.providers, item.String())
		}
	}

	return nil
}

func onHttpRequestHeaders(ctx wrapper.HttpContext, config ModelRouterConfig) types.Action {
	path, err := proxywasm.GetHttpRequestHeader(":path")
	if err != nil {
		return types.ActionContinue
	}

	// Extract provider from URI if configured
	if len(config.providers) > 0 {
		// Split path to extract first segment
		parts := strings.SplitN(path, "/", 3)
		// parts[0] is empty, parts[1] is potential provider, parts[2] is remaining path
		if len(parts) >= 3 && parts[1] != "" {
			candidate := parts[1]
			// Check if candidate is in providers list
			for _, provider := range config.providers {
				if candidate == provider {
					// Rewrite path: remove provider segment
					newPath := "/" + parts[2]
					_ = proxywasm.ReplaceHttpRequestHeader(":path", newPath)
					// Store provider in context
					ctx.SetContext("uriProvider", provider)
					log.Debugf("URI provider extracted: %s, path rewritten: %s -> %s", provider, path, newPath)
					path = newPath
					break
				}
			}
		}
	}

	// Remove query parameters for suffix check
	pathWithoutQuery := path
	if idx := strings.Index(path, "?"); idx != -1 {
		pathWithoutQuery = path[:idx]
	}

	enable := false
	for _, suffix := range config.enableOnPathSuffix {
		if suffix == "*" || strings.HasSuffix(pathWithoutQuery, suffix) {
			enable = true
			break
		}
	}

	if !enable || !ctx.HasRequestBody() {
		ctx.DontReadRequestBody()
		return types.ActionContinue
	}

	// Prepare for body processing
	proxywasm.RemoveHttpRequestHeader("content-length")
	// 100MB buffer limit
	ctx.SetRequestBodyBufferLimit(DefaultMaxBodyBytes)

	return types.HeaderStopIteration
}

func onHttpRequestBody(ctx wrapper.HttpContext, config ModelRouterConfig, body []byte) types.Action {
	contentType, err := proxywasm.GetHttpRequestHeader("content-type")
	if err != nil {
		return types.ActionContinue
	}

	if strings.Contains(contentType, "application/json") {
		return handleJsonBody(ctx, config, body)
	} else if strings.Contains(contentType, "multipart/form-data") {
		return handleMultipartBody(ctx, config, body, contentType)
	}

	return types.ActionContinue
}

// extractLastUserMessage extracts the content of the last message with role "user" from the messages array
func extractLastUserMessage(body []byte) string {
	messages := gjson.GetBytes(body, "messages")
	if !messages.Exists() || !messages.IsArray() {
		return ""
	}

	var lastUserContent string
	for _, msg := range messages.Array() {
		if msg.Get("role").String() == "user" {
			content := msg.Get("content")
			if content.IsArray() {
				// Handle array content (e.g., multimodal messages with text and images)
				for _, item := range content.Array() {
					if item.Get("type").String() == "text" {
						lastUserContent = item.Get("text").String()
					}
				}
			} else {
				lastUserContent = content.String()
			}
		}
	}
	return lastUserContent
}

// matchAutoRoutingRule matches the user message against auto routing rules and returns the matched model
func matchAutoRoutingRule(config ModelRouterConfig, userMessage string) (string, bool) {
	for _, rule := range config.autoRoutingRules {
		if rule.Pattern.MatchString(userMessage) {
			log.Debugf("auto routing rule matched: pattern=%s, model=%s", rule.Pattern.String(), rule.Model)
			return rule.Model, true
		}
	}
	return "", false
}

func handleJsonBody(ctx wrapper.HttpContext, config ModelRouterConfig, body []byte) types.Action {
	if !json.Valid(body) {
		log.Error("invalid json body")
		return types.ActionContinue
	}

	// Handle URI provider extraction
	if uriProvider, ok := ctx.GetContext("uriProvider").(string); ok && uriProvider != "" {
		modelValue := gjson.GetBytes(body, config.modelKey).String()
		if modelValue == "" {
			// TODO: Handle special protocols (Gemini, etc.) without model field
			log.Debugf("URI provider extracted but no model field in body, skipping model modification")
		} else {
			// Check if model already has the provider prefix
			if !strings.HasPrefix(modelValue, uriProvider+"/") {
				modelValue = uriProvider + "/" + modelValue
				log.Debugf("Applied URI provider prefix: %s", modelValue)

				// Update model in body
				newBody, err := sjson.SetBytes(body, config.modelKey, modelValue)
				if err != nil {
					log.Errorf("failed to update model with URI provider: %v", err)
				} else {
					_ = proxywasm.ReplaceHttpRequestBody(newBody)
					body = newBody
				}
			}

			// Set header for routing
			if config.modelToHeader != "" {
				_ = proxywasm.ReplaceHttpRequestHeader(config.modelToHeader, modelValue)
			}
		}
	}

	modelValue := gjson.GetBytes(body, config.modelKey).String()
	if modelValue == "" {
		return types.ActionContinue
	}

	// Check if auto routing should be triggered
	if config.enableAutoRouting && modelValue == AutoModelPrefix {
		userMessage := extractLastUserMessage(body)
		var targetModel string
		if userMessage != "" {
			if matchedModel, found := matchAutoRoutingRule(config, userMessage); found {
				targetModel = matchedModel
				log.Infof("auto routing: user message matched, routing to model: %s", matchedModel)
			}
		}
		// No rule matched, use default model if configured
		if targetModel == "" && config.defaultModel != "" {
			targetModel = config.defaultModel
			log.Infof("auto routing: no rule matched, using default model: %s", config.defaultModel)
		}

		if targetModel != "" {
			// Set the matched model to the header for routing
			_ = proxywasm.ReplaceHttpRequestHeader("x-higress-llm-model", targetModel)
			// Update the model field in the request body
			newBody, err := sjson.SetBytes(body, config.modelKey, targetModel)
			if err != nil {
				log.Errorf("failed to update model in auto routing json body: %v", err)
				return types.ActionContinue
			}
			_ = proxywasm.ReplaceHttpRequestBody(newBody)
			log.Debugf("auto routing: updated body model field to: %s", targetModel)
		} else {
			log.Warnf("auto routing: no rule matched and no default model configured")
		}
		return types.ActionContinue
	}

	if config.modelToHeader != "" {
		_ = proxywasm.ReplaceHttpRequestHeader(config.modelToHeader, modelValue)
	}

	if config.addProviderHeader != "" {
		parts := strings.SplitN(modelValue, "/", 2)
		if len(parts) == 2 {
			provider := parts[0]
			model := parts[1]
			_ = proxywasm.ReplaceHttpRequestHeader(config.addProviderHeader, provider)

			newBody, err := sjson.SetBytes(body, config.modelKey, model)
			if err != nil {
				log.Errorf("failed to update model in json body: %v", err)
				return types.ActionContinue
			}
			_ = proxywasm.ReplaceHttpRequestBody(newBody)
			log.Debugf("model route to provider: %s, model: %s", provider, model)
		} else {
			log.Debugf("model route to provider not work, model: %s", modelValue)
		}
	}

	return types.ActionContinue
}

func handleMultipartBody(ctx wrapper.HttpContext, config ModelRouterConfig, body []byte, contentType string) types.Action {
	_, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		log.Errorf("failed to parse content type: %v", err)
		return types.ActionContinue
	}
	boundary, ok := params["boundary"]
	if !ok {
		log.Errorf("no boundary in content type")
		return types.ActionContinue
	}

	reader := multipart.NewReader(bytes.NewReader(body), boundary)
	var newBody bytes.Buffer
	writer := multipart.NewWriter(&newBody)
	writer.SetBoundary(boundary)

	modified := false

	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Errorf("failed to read multipart part: %v", err)
			return types.ActionContinue
		}

		// Read part content
		partContent, err := io.ReadAll(part)
		if err != nil {
			log.Errorf("failed to read part content: %v", err)
			return types.ActionContinue
		}

		formName := part.FormName()
		if formName == config.modelKey {
			modelValue := string(partContent)

			// Handle URI provider extraction
			if uriProvider, ok := ctx.GetContext("uriProvider").(string); ok && uriProvider != "" {
				if modelValue == "" {
					// TODO: Handle special protocols (Gemini, etc.) without model field
					log.Debugf("URI provider extracted but model field is empty, skipping model modification")
				} else if !strings.HasPrefix(modelValue, uriProvider+"/") {
					modelValue = uriProvider + "/" + modelValue
					log.Debugf("Applied URI provider prefix: %s", modelValue)
				}
			}

			if config.modelToHeader != "" {
				_ = proxywasm.ReplaceHttpRequestHeader(config.modelToHeader, modelValue)
			}

			if config.addProviderHeader != "" {
				parts := strings.SplitN(modelValue, "/", 2)
				if len(parts) == 2 {
					provider := parts[0]
					model := parts[1]
					_ = proxywasm.ReplaceHttpRequestHeader(config.addProviderHeader, provider)

					// Write modified part
					h := make(http.Header)
					for k, v := range part.Header {
						h[k] = v
					}

					pw, err := writer.CreatePart(textproto.MIMEHeader(h))
					if err != nil {
						log.Errorf("failed to create part: %v", err)
						return types.ActionContinue
					}
					_, err = pw.Write([]byte(model))
					if err != nil {
						log.Errorf("failed to write part content: %v", err)
						return types.ActionContinue
					}
					modified = true
					log.Debugf("model route to provider: %s, model: %s", provider, model)
					continue
				} else {
					log.Debugf("model route to provider not work, model: %s", modelValue)
				}
			}
		}

		// Write original part
		h := make(http.Header)
		for k, v := range part.Header {
			h[k] = v
		}
		pw, err := writer.CreatePart(textproto.MIMEHeader(h))
		if err != nil {
			log.Errorf("failed to create part: %v", err)
			return types.ActionContinue
		}
		_, err = pw.Write(partContent)
		if err != nil {
			log.Errorf("failed to write part content: %v", err)
			return types.ActionContinue
		}
	}

	writer.Close()

	if modified {
		_ = proxywasm.ReplaceHttpRequestBody(newBody.Bytes())
	}

	return types.ActionContinue
}
