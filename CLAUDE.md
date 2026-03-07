# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

Higress is a cloud-native AI Gateway built on Istio and Envoy. It serves as a Kubernetes ingress controller, microservice gateway, and AI gateway with extensive plugin capabilities. The project supports multiple plugin frameworks (Wasm-Go, Wasm-Rust, Wasm-Cpp, Golang-Filter) and provides 56+ ready-to-use plugins for AI, security, and traffic management.

## Build Commands

### Core Build Targets

```bash
# Build Higress controller binary
make build

# Build for Linux (cross-compile)
make build-linux

# Build with Docker container (no local Go toolchain needed)
BUILD_WITH_CONTAINER=1 make build

# Build hgctl CLI tool
make build-hgctl

# Run tests with coverage
make go.test.coverage
```

### Gateway and Component Builds

```bash
# Build gateway Docker image (local architecture)
make build-gateway-local

# Build gateway for multiple architectures
make build-gateway

# Build Istio pilot component
make build-pilot

# Build Envoy proxy
make build-envoy

# Build all Wasm plugins
make build-wasmplugins
```

### Plugin Development

**Wasm-Go Plugins:**
```bash
cd plugins/wasm-go
PLUGIN_NAME=<plugin-name> make build              # Build plugin to extensions/<plugin-name>/plugin.wasm
PLUGIN_NAME=<plugin-name> make build-image        # Build plugin Docker image
PLUGIN_NAME=<plugin-name> make build-push         # Build and push plugin image
```

**Golang-Filter Plugins:**
```bash
make build-golang-filter-amd64    # Build for amd64
make build-golang-filter-arm64    # Build for arm64
make build-golang-filter          # Build for both architectures
```

### Testing

```bash
# Run unit tests with coverage
make go.test.coverage

# Run Ingress API conformance tests
make higress-conformance-test

# Run WasmPlugin tests (all Go plugins)
make higress-wasmplugin-test

# Test specific plugin
PLUGIN_NAME=request-block make higress-wasmplugin-test

# Test specific C++ plugin
PLUGIN_TYPE=CPP PLUGIN_NAME=key_auth make higress-wasmplugin-test

# Test specific Rust plugin
PLUGIN_TYPE=RUST PLUGIN_NAME=request-block make higress-wasmplugin-test

# Run specific test cases only
TEST_SHORTNAME=WasmPluginsIPRestrictionAllow,WasmPluginsIPRestrictionDeny make higress-wasmplugin-test

# Skip Docker build and run tests
PLUGIN_NAME=ip-restriction make higress-wasmplugin-test-skip-docker-build
```

### Deployment

```bash
# Install Higress with Helm (local development)
make install

# Install development version with custom images
make install-dev

# Install with WasmPlugin volume support
make install-dev-wasmplugin

# Uninstall Higress
make uninstall

# Upgrade existing installation
make upgrade
```

### Cleanup

```bash
make clean-higress    # Clean build artifacts
make clean-istio      # Clean Istio dependencies
make clean-gateway    # Clean gateway and all dependencies
make clean-env        # Clean environment
```

## Architecture

### Component Structure

Higress consists of three main components:

1. **Higress Controller** (Control Plane)
   - Entry: `cmd/higress/main.go` → `pkg/cmd/server.go` → `pkg/bootstrap/server.go`
   - Manages 6 sub-controllers via `pkg/ingress/config/IngressConfig`:
     - **Ingress Controller**: Converts K8s Ingress → Istio Gateway/VirtualService/DestinationRule
     - **Gateway Controller**: Manages Gateway API resources (GatewayClass, Gateway, HttpRoute, etc.)
     - **McpBridge Controller**: Bridges external registries (Nacos, Eureka, Consul, Zookeeper) → ServiceEntry
     - **Http2Rpc Controller**: Converts HTTP protocols to RPC protocols
     - **WasmPlugin Controller**: Manages Wasm plugin lifecycle and configuration
     - **ConfigmapMgr**: Handles global configuration via EnvoyFilter

2. **Higress Gateway** (Data Plane)
   - Based on Envoy proxy with custom extensions
   - Executes Wasm plugins in sandbox
   - Handles actual traffic proxying

3. **Higress Console** (Management UI)
   - Separate repository: https://github.com/higress-group/higress-console

### Relationship with Istio and Envoy

```
Higress Controller (Control Plane)
├── Higress Core (IngressConfig) - Custom controllers
└── Istio Discovery (Pilot) - Config aggregation & xDS protocol
    │
    └── xDS gRPC (port 15051)
        │
        ▼
Higress Gateway (Data Plane)
├── Pilot Agent - Envoy lifecycle management
└── Envoy - Traffic proxying & Wasm execution
```

### Key Directories

- `cmd/higress/` - Main entry point and CLI
- `pkg/bootstrap/` - Server initialization
- `pkg/ingress/config/` - Core ingress configuration controller
- `pkg/ingress/kube/` - Kubernetes resource handlers (83 files)
- `pkg/cert/` - Certificate management (Let's Encrypt support)
- `api/extensions/v1alpha1/` - WasmPlugin CRD definitions
- `api/networking/v1/` - McpBridge and Http2Rpc CRD definitions
- `plugins/wasm-go/extensions/` - 56+ Go-based Wasm plugins
- `plugins/golang-filter/` - Native Go HTTP filters + MCP server hosting
- `plugins/wasm-cpp/extensions/` - C++ Wasm plugins
- `plugins/wasm-rust/extensions/` - Rust Wasm plugins
- `external/` - Vendored dependencies (Istio, Envoy, go-control-plane)

## Plugin Development Standards

### New Plugin Requirements

When creating new independent plugins, you **MUST**:

1. **Create a `design/` directory** within the plugin directory
2. **Include design documentation** with:
   - Plugin purpose and use cases
   - Core functionality design
   - Configuration parameters
   - Technology selection and dependencies
   - Boundary conditions and limitations
   - Testing strategy

3. **Save AI prompts** if using AI coding tools:
   - `design/ai-prompts.md` - AI prompts record
   - `design/design-doc.md` - Complete design document
   - `design/requirements.md` - Feature requirements list

4. **Directory structure example:**
```
plugins/wasm-go/extensions/my-new-plugin/
  ├── design/
  │   ├── design-doc.md
  │   ├── ai-prompts.md
  │   └── architecture.md (optional)
  ├── main.go
  ├── go.mod
  └── README.md
```

### Plugin Configuration Levels

WasmPlugin supports 4 configuration levels:
1. **Global**: `defaultConfig` in WasmPlugin
2. **Route-level**: Via `matchRules` with ingress/domain/service matching
3. **Consumer-level**: Via consumer field in match rules
4. **Route type**: HTTP or GRPC specific configuration

## Configuration Flow

1. User creates K8s resources (Ingress, Gateway, WasmPlugin, McpBridge, Http2Rpc)
2. Controllers watch resources via K8s API
3. Controllers translate to Istio resources (Gateway, VirtualService, DestinationRule, ServiceEntry, EnvoyFilter)
4. Istio Discovery aggregates configs and converts to xDS format
5. xDS pushed to Envoy via gRPC (port 15051)
6. Envoy applies configs to listeners, routes, clusters, endpoints

## AI Gateway Capabilities

Higress provides extensive AI-focused features:

- **Model Provider Support**: 20+ LLM providers (OpenAI, Claude, Gemini, etc.) via `ai-proxy` plugin
- **MCP Server Hosting**: Host Model Context Protocol servers for AI agents via golang-filter
- **AI Plugins**:
  - `ai-billing` - Usage tracking and billing
  - `ai-cache` - Response caching
  - `ai-quota` - Quota management
  - `ai-load-balancer` - Multi-model load balancing
  - `ai-rag` - Retrieval-augmented generation
  - `ai-search` - Search integration
  - `ai-security-guard` - Security protection
  - `ai-token-ratelimit` - Token-based rate limiting
- **Streaming Support**: Full streaming request/response handling for SSE (Server-Sent Events)

## Development Environment

### Prerequisites

- Go 1.24.4 (or use `BUILD_WITH_CONTAINER=1` to build in Docker)
- Docker (for building plugins and images)
- Kubernetes cluster (for testing, can use kind)
- Helm 3.x (for deployment)

### Environment Variables

- `HUB` - Docker registry for images (default: `crpi-0vih3w2g8se3zbl8.cn-hangzhou.personal.cr.aliyuncs.com/isadba`)
- `TAG` - Image tag (default: git commit SHA)
- `GOPROXY` - Go module proxy (default: `https://proxy.golang.org,direct`)
- `TARGET_ARCH` - Target architecture (default: `amd64`)
- `BUILD_WITH_CONTAINER` - Build in Docker container (default: `0`)

### Running Tests

Unit tests are located alongside source files as `*_test.go`. Key test locations:
- `plugins/wasm-go/extensions/*/main_test.go` - Plugin unit tests
- `test/e2e/` - End-to-end conformance tests
- `test/gateway/` - Gateway API conformance tests

## Important Notes

- All dependencies are vendored in `/external` directory with local replacements in `go.mod`
- Configuration changes take effect in milliseconds without traffic jitter (no Nginx reload)
- Wasm plugins run in sandbox isolation for memory safety
- Plugin versions can be upgraded independently with hot updates
- Supports both Ingress API and Gateway API standards
- Compatible with many nginx ingress controller annotations
