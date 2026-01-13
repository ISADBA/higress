package test

import (
	"testing"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
)

// TestHost represents a test host for WASM plugin testing
type TestHost interface {
	Reset()
	GetRequestHeaders() map[string]string
	GetLocalResponse() []byte
}

// mockTestHost is a simple mock implementation
type mockTestHost struct {
	requestHeaders map[string]string
	localResponse  []byte
}

func (m *mockTestHost) Reset() {
	m.requestHeaders = make(map[string]string)
	m.localResponse = nil
}

func (m *mockTestHost) GetRequestHeaders() map[string]string {
	return m.requestHeaders
}

func (m *mockTestHost) GetLocalResponse() []byte {
	return m.localResponse
}

// RunGoTest runs a Go test with the provided function
func RunGoTest(t *testing.T, testFunc func(*testing.T)) {
	testFunc(t)
}

// RunTest runs a test with the provided function
func RunTest(t *testing.T, testFunc func(*testing.T)) {
	testFunc(t)
}

// NewTestHost creates a new test host with the given configuration
func NewTestHost(config interface{}) (TestHost, types.OnPluginStartStatus) {
	host := &mockTestHost{
		requestHeaders: make(map[string]string),
	}
	// For now, always return OK status
	// In a real implementation, this would parse the config and set up the plugin
	return host, types.OnPluginStartStatusOK
}

// HasHeaderWithValue checks if the headers contain a specific key-value pair
func HasHeaderWithValue(headers map[string]string, key, value string) bool {
	if headers == nil {
		return false
	}
	actualValue, exists := headers[key]
	return exists && actualValue == value
}
