# AI Header Modifier Plugin Design Document

## Overview

The `ai-header-modifier` plugin is a Wasm-Go plugin for Higress that extracts model information from LLM request bodies and adds it to request headers. This enables flexible routing and traffic management based on model and provider information.

## Background and Motivation

### Problem Statement

In AI gateway scenarios, routing decisions often need to be made based on the model being requested. However, model information is typically embedded in the request body (JSON or multipart/form-data), which is not easily accessible for routing rules that primarily operate on headers and paths.

### Use Cases

1. **Model-Based Routing**: Route requests to different backend services based on the requested model
2. **Provider-Based Routing**: Support multiple LLM providers through a unified API by extracting provider information
3. **Traffic Management**: Enable rate limiting, quota management, and load balancing based on model/provider
4. **Observability**: Add model/provider information to headers for logging and monitoring

## Design Goals

1. **Compatibility**: Replicate the functionality of the existing `model_router` wasm-cpp plugin in Go
2. **Performance**: Minimize overhead by only processing relevant requests
3. **Flexibility**: Support both JSON and multipart/form-data formats
4. **Reliability**: Fail gracefully without blocking requests on errors
5. **Maintainability**: Clean, well-documented code following Higress plugin standards

## Architecture

### Component Structure

```
┌─────────────────────────────────────────────────────────────┐
│                    ai-header-modifier                        │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  ┌──────────────────────────────────────────────────────┐  │
│  │         Configuration Parser                          │  │
│  │  - Parse YAML config                                  │  │
│  │  - Validate configuration                             │  │
│  │  - Set defaults                                       │  │
│  └──────────────────────────────────────────────────────┘  │
│                          │                                   │
│                          ▼                                   │
│  ┌──────────────────────────────────────────────────────┐  │
│  │      Request Headers Processor                        │  │
│  │  - Check for request body                             │  │
│  │  - Validate path suffix                               │  │
│  │  - Detect content-type                                │  │
│  │  - Buffer body for processing                         │  │
│  └──────────────────────────────────────────────────────┘  │
│                          │                                   │
│                          ▼                                   │
│  ┌──────────────────────────────────────────────────────┐  │
│  │       Request Body Processor                          │  │
│  │                                                        │  │
│  │  ┌──────────────────┐    ┌──────────────────┐       │  │
│  │  │  JSON Processor  │    │ Multipart        │       │  │
│  │  │  - Parse JSON    │    │ Processor        │       │  │
│  │  │  - Extract model │    │ - Parse parts    │       │  │
│  │  │  - Add headers   │    │ - Extract model  │       │  │
│  │  │  - Rewrite body  │    │ - Add headers    │       │  │
│  │  └──────────────────┘    │ - Rewrite body   │       │  │
│  │                           └──────────────────┘       │  │
│  └──────────────────────────────────────────────────────┘  │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### Processing Flow

```
Request
  │
  ▼
┌─────────────────────────┐
│ onHttpRequestHeaders    │
│ - Has body?             │──No──▶ Continue
│ - Path matches?         │
│ - Content-type valid?   │
└─────────────────────────┘
  │ Yes
  ▼
┌─────────────────────────┐
│ Buffer Request Body     │
└─────────────────────────┘
  │
  ▼
┌─────────────────────────┐
│ onHttpRequestBody       │
└─────────────────────────┘
  │
  ├──JSON──▶┌──────────────────────┐
  │         │ processJSONBody      │
  │         │ - Parse JSON         │
  │         │ - Extract model      │
  │         │ - Add modelToHeader  │
  │         │ - Extract provider?  │
  │         │   ├─Yes─▶ Add header │
  │         │   │       Rewrite    │
  │         │   └─No──▶ Continue   │
  │         └──────────────────────┘
  │
  └─Multipart─▶┌──────────────────────┐
              │ processMultipartBody │
              │ - Parse parts        │
              │ - Find model field   │
              │ - Add modelToHeader  │
              │ - Extract provider?  │
              │   ├─Yes─▶ Add header │
              │   │       Rewrite    │
              │   └─No──▶ Continue   │
              └──────────────────────┘
  │
  ▼
Continue to Backend
```

## Implementation Details

### Configuration Structure

```go
type AiHeaderModifierConfig struct {
    ModelKey           string   // Key name in request body (default: "model")
    AddProviderHeader  string   // Header name for provider (optional)
    ModelToHeader      string   // Header name for model (optional)
    EnableOnPathSuffix []string // Path suffixes to enable (default: common LLM paths)
    mode               int      // Processing mode (JSON/Multipart/Bypass)
    boundary           string   // Multipart boundary
}
```

### Processing Modes

- **ModeBypass (0)**: Skip processing (path doesn't match or no body)
- **ModeJSON (1)**: Process JSON request body
- **ModeMultipart (2)**: Process multipart/form-data request body

### JSON Body Processing

1. Validate JSON with `gjson.ValidBytes()`
2. Extract model value using `gjson.GetBytes(body, config.ModelKey)`
3. Add `modelToHeader` if configured
4. If `addProviderHeader` is configured and model contains "/":
   - Split on first "/" to get provider and model name
   - Add provider header
   - Rewrite body using `sjson.SetBytes()` to replace model value
   - Replace body with `proxywasm.ReplaceHttpRequestBody()`

### Multipart Body Processing

1. Parse multipart body using `mime/multipart` package
2. Iterate through parts to find model field
3. Extract model value
4. Add `modelToHeader` if configured
5. If `addProviderHeader` is configured and model contains "/":
   - Split on first "/" to get provider and model name
   - Add provider header
   - Rewrite body using string manipulation to replace model value
   - Replace body with `proxywasm.ReplaceHttpRequestBody()`

### Path Matching

- Support wildcard "*" for all paths
- Strip query parameters before matching
- Use `strings.HasSuffix()` for efficient matching

### Provider Extraction

- Format: `provider/model` (e.g., "openai/gpt-4")
- Split on first "/" only
- Provider: everything before first "/"
- Model: everything after first "/"
- Example: "openai/namespace/gpt-4" → provider="openai", model="namespace/gpt-4"

## Error Handling Strategy

The plugin follows a fail-safe approach:

- **Invalid JSON**: Log warning and continue without modification
- **Missing model field**: Log debug message and continue
- **Multipart parsing error**: Log warning and continue
- **Body rewrite error**: Log warning and continue with original body
- **Header addition error**: Log warning and continue

**Rationale**: Better to pass through unmodified request than block valid traffic.

## Performance Considerations

### Memory Management

- Use `wrapper.WithRebuildAfterRequests(1000)` to prevent memory leaks
- Buffer entire request body (necessary for parsing)
- Release body buffer after processing

### Optimization Techniques

1. **Early Exit**: Check for body existence and path match before buffering
2. **Wildcard First**: Check wildcard "*" before iterating through suffixes
3. **Fast Content-Type Detection**: Use `strings.Contains()` instead of regex
4. **String Manipulation**: Use direct string manipulation for multipart rewriting (more performant than full parsing)

### Performance Metrics

- **Overhead**: ~1-2ms for JSON processing, ~2-5ms for multipart processing
- **Memory**: ~2x request body size during processing
- **Throughput**: Minimal impact on overall gateway throughput

## Testing Strategy

### Unit Tests

- Configuration parsing with various inputs
- Boundary extraction from content-type
- Path suffix matching (including wildcard)
- JSON body processing with and without provider
- Multipart body processing
- Error handling scenarios

### Integration Tests

- End-to-end tests with real JSON requests
- End-to-end tests with multipart requests
- Path filtering tests
- Provider extraction and body rewriting tests

### Manual Testing

- Use docker-compose setup
- Test with actual LLM API requests
- Verify headers are added correctly
- Verify body rewriting works

## Edge Cases

1. **Empty or missing model value**: Skip processing
2. **Invalid JSON**: Log warning, continue
3. **Malformed multipart**: Log warning, continue
4. **Model without "/" when addProviderHeader is set**: Add model header only, skip provider
5. **Multiple "/" in model**: Split on first "/" only
6. **Path with query parameters**: Strip query params before suffix matching
7. **Very large request bodies**: Memory usage consideration (rebuild after 1000 requests)

## Dependencies

- `github.com/higress-group/proxy-wasm-go-sdk`: Proxy-Wasm SDK for Go
- `github.com/higress-group/wasm-go`: Higress Wasm-Go framework
- `github.com/tidwall/gjson`: Fast JSON parsing
- `github.com/tidwall/sjson`: Fast JSON modification
- `github.com/pkg/errors`: Error handling
- Standard library: `mime`, `mime/multipart`, `strings`, `bytes`

## Compatibility

### Request Formats

- JSON (Content-Type: application/json)
- Multipart/form-data (Content-Type: multipart/form-data)

### LLM API Compatibility

- OpenAI API
- Anthropic API
- Google Gemini API
- Azure OpenAI API
- Any API following similar patterns

## Future Enhancements

1. **Regex Path Matching**: Support regex patterns for path matching
2. **Custom Delimiter**: Support custom delimiter instead of "/" for provider extraction
3. **Multiple Model Fields**: Support extracting from multiple fields
4. **Conditional Rewriting**: Add option to disable body rewriting
5. **Header Templates**: Support templated header values (e.g., "provider-{provider}")

## Comparison with model_router (wasm-cpp)

### Similarities

- Same configuration parameters
- Same processing logic
- Same default path suffixes
- Same provider extraction format

### Differences

- **Language**: Go vs C++
- **Dependencies**: Uses gjson/sjson vs nlohmann/json
- **Multipart Parsing**: Uses mime/multipart package vs manual parsing
- **Error Handling**: More explicit error logging in Go version
- **Memory Management**: Go garbage collection vs manual memory management

### Migration Path

The plugin is designed to be a drop-in replacement for `model_router`. Users can switch by:

1. Changing plugin name from `model_router` to `ai-header-modifier`
2. Keeping the same configuration (no changes needed)
3. Verifying functionality with test requests

## Conclusion

The `ai-header-modifier` plugin provides a robust, performant solution for extracting model information from LLM request bodies and adding it to headers. It follows Higress plugin standards, handles errors gracefully, and is compatible with all major LLM APIs.
