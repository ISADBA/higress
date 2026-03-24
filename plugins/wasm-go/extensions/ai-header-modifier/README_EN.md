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
defaultProvider: default  # Default provider when model doesn't contain "/"
enableOnPathSuffix:
  - /v1/chat/completions
```

Request example 1 (with provider):
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

Request example 2 (without provider):
```json
{
  "model": "gpt-4o-mini",
  "messages": [{"role": "user", "content": "Hello"}]
}
```

After processing:
- Added header: `x-higress-llm-model: gpt-4o-mini`
- Added header: `x-higress-llm-provider: default`
- Request body remains unchanged

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
| defaultProvider | string | Optional | "default" | Default provider to use when model name doesn't contain "/" |
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

## Custom Header Management

In addition to AI model extraction, the plugin supports flexible custom header management, suitable for scenarios like MSE (Microservices Engine) metadata injection.

### Feature Types

#### 1. Static Headers

Add headers with fixed configured values, suitable for gateway instance identification, environment labels, etc.

```yaml
staticHeaders:
  - key: "x-mse-gateway-instance-id"
    value: "gateway-001"
  - key: "x-environment"
    value: "production"
```

#### 2. Fixed Source Headers

Read values from fixed sources (Envoy properties or pseudo-headers) and write to target headers.

```yaml
fixedSourceHeaders:
  - source: "authority"          # Source: :authority pseudo-header
    target: "x-mse-domain-name"  # Target header
  - source: "route_name"         # Source: Envoy route name
    target: "x-mse-router-name"
  - source: "cluster_name"       # Source: Envoy cluster name
    target: "x-mse-service-name"
  - source: "consumer_name"      # Source: authenticated consumer name
    target: "x-mse-consumer-name"
```

**Supported Sources:**
- `authority` - `:authority` pseudo-header (domain name, port is automatically stripped)
- `route_name` - Envoy route name
- `cluster_name` - Envoy cluster/service name
- `consumer_name` - Authenticated consumer name

**Note:** When using `authority` as the source, the plugin automatically strips the port. For example:
- `isadba.com:8080` → `isadba.com`
- `192.168.1.1:8080` → `192.168.1.1`
- `example.com` → `example.com` (unchanged when no port)

#### 3. Priority Source Headers

Extract values from multiple candidate headers (in priority order), suitable for API key extraction scenarios.

```yaml
prioritySourceHeaders:
  - target: "x-mse-consumer-apikey"
    sources:
      - "authorization"      # Priority 1
      - "x-api-key"         # Priority 2
      - "api-key"           # Priority 3
    stripPrefix: "Bearer "  # Optional: strip prefix
```

**Features:**
- Try each source header in priority order
- Use the first non-empty value found
- Support prefix stripping (e.g., strip "Bearer " from Authorization header)

### Use Cases

#### Use Case 3: MSE Metadata Injection

Add MSE-related metadata headers to all requests for observability, billing, routing, etc.

```yaml
staticHeaders:
  - key: "x-mse-gateway-instance-id"
    value: "gateway-001"

fixedSourceHeaders:
  - source: "authority"
    target: "x-mse-domain-name"
  - source: "route_name"
    target: "x-mse-router-name"
  - source: "cluster_name"
    target: "x-mse-service-name"

prioritySourceHeaders:
  - target: "x-mse-consumer-apikey"
    sources: ["authorization", "x-api-key"]
    stripPrefix: "Bearer "
```

#### Use Case 4: AI + MSE Combined

Use both AI model extraction and MSE metadata injection features.

```yaml
# AI configuration
modelKey: "model"
modelToHeader: "x-higress-llm-model"
addProviderHeader: "x-higress-llm-provider"
enableOnPathSuffix:
  - "/v1/chat/completions"

# MSE configuration
staticHeaders:
  - key: "x-mse-gateway-instance-id"
    value: "gateway-001"

fixedSourceHeaders:
  - source: "authority"
    target: "x-mse-domain-name"
  - source: "route_name"
    target: "x-mse-router-name"

prioritySourceHeaders:
  - target: "x-mse-consumer-apikey"
    sources: ["authorization", "x-api-key"]
    stripPrefix: "Bearer "
```

**Behavior Notes:**
- MSE headers are added to ALL requests (not affected by `enableOnPathSuffix`)
- AI model extraction only happens for paths matching `enableOnPathSuffix`
- For LLM requests: both AI headers and MSE headers are added
- For non-LLM requests: only MSE headers are added
