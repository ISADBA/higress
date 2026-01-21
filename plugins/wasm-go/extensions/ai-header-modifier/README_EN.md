# Description

The `ai-header-modifier` plugin extracts model information from LLM request bodies and adds it to request headers for routing and traffic management. It supports extracting model names from both JSON and multipart/form-data request bodies, with optional provider extraction.

## Features

- **Model Extraction**: Extracts model name from request body and adds it to specified request header
- **Provider Extraction**: Supports extracting provider information from `provider/model` format model names
- **Body Rewriting**: Automatically rewrites request body after provider extraction, changing model name from `provider/model` to `model`
- **Multi-Format Support**: Supports both JSON and multipart/form-data request body formats
- **Path Filtering**: Only processes requests matching specified path suffixes
- **Wildcard Support**: Supports using `*` wildcard to match all paths

## Use Cases

### Use Case 1: Model-Based Routing

In an AI gateway, different models may need to be routed to different backend services. By adding model information to request headers, you can use Higress routing rules for flexible traffic distribution.

```yaml
modelKey: model
modelToHeader: x-higress-llm-model
enableOnPathSuffix:
  - /v1/chat/completions
  - /v1/embeddings
```

### Use Case 2: Multi-Provider Routing

When using a unified API format to support multiple LLM providers, you can implement provider-based routing by extracting provider information.

```yaml
modelKey: model
modelToHeader: x-higress-llm-model
addProviderHeader: x-higress-llm-provider
enableOnPathSuffix:
  - /v1/chat/completions
```

Request example:
```json
{
  "model": "openai/gpt-4",
  "messages": [{"role": "user", "content": "Hello"}]
}
```

After processing:
- Added header: `x-higress-llm-model: openai/gpt-4`
- Added header: `x-higress-llm-provider: openai`
- Model field in request body rewritten to: `"model": "gpt-4"`

### Use Case 3: All-Path Processing

Use wildcard `*` to process all requests:

```yaml
modelKey: model
modelToHeader: x-model
enableOnPathSuffix:
  - "*"
```

## Configuration Fields

| Name | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| modelKey | string | Optional | "model" | Key name of the model field in request body |
| modelToHeader | string | Optional (at least one required) | - | Add model value to this request header |
| addProviderHeader | string | Optional (at least one required) | - | Add extracted provider information to this request header |
| enableOnPathSuffix | array of string | Optional | [default path list] | List of path suffixes to enable the plugin, supports wildcard "*" |

**Note**: At least one of `modelToHeader` and `addProviderHeader` must be configured.

### Default Path List

If `enableOnPathSuffix` is not configured, the plugin will process requests with the following path suffixes:

- `/completions`
- `/embeddings`
- `/images/generations`
- `/audio/speech`
- `/fine_tuning/jobs`
- `/moderations`
- `/image-synthesis`
- `/video-synthesis`
- `/rerank`
- `/messages`

## Configuration Examples

### Example 1: Extract Model Name Only

```yaml
modelKey: model
modelToHeader: x-higress-llm-model
enableOnPathSuffix:
  - /v1/chat/completions
  - /v1/embeddings
```

### Example 2: Extract Model and Provider

```yaml
modelKey: model
modelToHeader: x-higress-llm-model
addProviderHeader: x-higress-llm-provider
enableOnPathSuffix:
  - /v1/chat/completions
  - /v1/embeddings
```

### Example 3: Custom Model Field Name

```yaml
modelKey: llm_model
modelToHeader: x-model
enableOnPathSuffix:
  - "*"
```

### Example 4: Extract Provider Only

```yaml
modelKey: model
addProviderHeader: x-provider
enableOnPathSuffix:
  - /v1/chat/completions
```

## Processing Flow

1. **Request Headers Phase**:
   - Check if request has a body
   - Verify request path matches configured suffixes
   - Detect Content-Type (JSON or multipart/form-data)
   - Buffer request body for subsequent processing

2. **Request Body Phase**:
   - Select appropriate processing method based on Content-Type
   - Extract model value from request body
   - Add configured request headers
   - If `addProviderHeader` is configured and model value contains `/`, extract provider and rewrite request body

## Important Notes

1. **Performance Considerations**: The plugin needs to buffer the complete request body, which may increase memory usage for large requests
2. **Error Handling**: If errors occur during processing (e.g., invalid JSON), the plugin will log warnings and continue without blocking the request
3. **Provider Format**: Provider extraction only works when the model value contains `/`, in the format `provider/model`
4. **Multiple Slashes**: If the model value contains multiple `/` (e.g., `provider/namespace/model`), only the part before the first `/` is treated as the provider
5. **Query Parameters**: Path matching automatically ignores query parameters

## Compatibility

- Supports JSON format request bodies (Content-Type: application/json)
- Supports multipart/form-data format request bodies
- Compatible with all mainstream LLM API formats (OpenAI, Anthropic, Google, etc.)
