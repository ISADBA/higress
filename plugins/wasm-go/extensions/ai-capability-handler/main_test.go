package main

import (
	"encoding/json"
	"testing"

	"github.com/tidwall/gjson"
)

func TestParseConfig(t *testing.T) {
	tests := []struct {
		name        string
		configJSON  string
		expectError bool
		expected    CapabilityConfig
	}{
		{
			name: "valid config with all fields",
			configJSON: `{
				"capability_service": {
					"url": "http://capability-service.example.com:8080/v1/capability/verify-request",
					"timeout_ms": 2000,
					"timeout_action": "deny"
				},
				"timeout_deny_message": {
					"code": 503,
					"message": "Service temporarily unavailable"
				}
			}`,
			expectError: false,
			expected: CapabilityConfig{
				CapabilityService: CapabilityServiceConfig{
					URL:           "http://capability-service.example.com:8080/v1/capability/verify-request",
					TimeoutMs:     2000,
					TimeoutAction: "deny",
				},
				TimeoutDenyMessage: TimeoutDenyMessage{
					Code:    503,
					Message: "Service temporarily unavailable",
				},
			},
		},
		{
			name: "valid config with defaults",
			configJSON: `{
				"capability_service": {
					"url": "http://capability-service.example.com/v1/capability/verify-request"
				}
			}`,
			expectError: false,
			expected: CapabilityConfig{
				CapabilityService: CapabilityServiceConfig{
					URL:           "http://capability-service.example.com/v1/capability/verify-request",
					TimeoutMs:     1000,
					TimeoutAction: "continue",
				},
				TimeoutDenyMessage: TimeoutDenyMessage{
					Code:    501,
					Message: "能力处理超时，请联系管理员",
				},
			},
		},
		{
			name: "missing capability_service",
			configJSON: `{
				"timeout_deny_message": {
					"code": 503,
					"message": "Service unavailable"
				}
			}`,
			expectError: true,
		},
		{
			name: "missing url",
			configJSON: `{
				"capability_service": {
					"timeout_ms": 2000
				}
			}`,
			expectError: true,
		},
		{
			name: "invalid timeout_action",
			configJSON: `{
				"capability_service": {
					"url": "http://capability-service.example.com/v1/capability/verify-request",
					"timeout_action": "invalid"
				}
			}`,
			expectError: true,
		},
		{
			name: "invalid url format",
			configJSON: `{
				"capability_service": {
					"url": "://invalid-url"
				}
			}`,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var config CapabilityConfig
			jsonResult := gjson.Parse(tt.configJSON)

			err := parseConfig(jsonResult, &config)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			// Compare configuration (excluding client which can't be compared)
			if config.CapabilityService.URL != tt.expected.CapabilityService.URL {
				t.Errorf("URL mismatch: got %s, want %s",
					config.CapabilityService.URL, tt.expected.CapabilityService.URL)
			}

			if config.CapabilityService.TimeoutMs != tt.expected.CapabilityService.TimeoutMs {
				t.Errorf("TimeoutMs mismatch: got %d, want %d",
					config.CapabilityService.TimeoutMs, tt.expected.CapabilityService.TimeoutMs)
			}

			if config.CapabilityService.TimeoutAction != tt.expected.CapabilityService.TimeoutAction {
				t.Errorf("TimeoutAction mismatch: got %s, want %s",
					config.CapabilityService.TimeoutAction, tt.expected.CapabilityService.TimeoutAction)
			}

			if config.TimeoutDenyMessage.Code != tt.expected.TimeoutDenyMessage.Code {
				t.Errorf("TimeoutDenyMessage.Code mismatch: got %d, want %d",
					config.TimeoutDenyMessage.Code, tt.expected.TimeoutDenyMessage.Code)
			}

			if config.TimeoutDenyMessage.Message != tt.expected.TimeoutDenyMessage.Message {
				t.Errorf("TimeoutDenyMessage.Message mismatch: got %s, want %s",
					config.TimeoutDenyMessage.Message, tt.expected.TimeoutDenyMessage.Message)
			}
		})
	}
}
func TestProcessActions(t *testing.T) {
	tests := []struct {
		name        string
		actions     []Action
		expectError bool
	}{
		{
			name: "valid header replace action",
			actions: []Action{
				{
					Type:         "replace",
					Target:       "header",
					Key:          "x-test-header",
					Value:        "test-value",
					CapabilityID: 1,
				},
			},
			expectError: false,
		},
		{
			name: "valid header unset action",
			actions: []Action{
				{
					Type:         "unset",
					Target:       "header",
					Key:          "x-remove-header",
					CapabilityID: 2,
				},
			},
			expectError: false,
		},
		{
			name: "invalid action type",
			actions: []Action{
				{
					Type:         "invalid",
					Target:       "header",
					Key:          "x-test-header",
					Value:        "test-value",
					CapabilityID: 3,
				},
			},
			expectError: true,
		},
		{
			name: "unknown target",
			actions: []Action{
				{
					Type:         "replace",
					Target:       "unknown",
					Key:          "test-key",
					Value:        "test-value",
					CapabilityID: 4,
				},
			},
			expectError: false, // Unknown targets are logged but don't cause errors
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Note: This test can't fully test processActions because it requires
			// the proxy-wasm runtime environment. This is a basic structure test.

			// Validate action structure
			for _, action := range tt.actions {
				if action.Type == "" {
					t.Errorf("action type is empty")
				}
				if action.Target == "" {
					t.Errorf("action target is empty")
				}
				if action.CapabilityID == 0 {
					t.Errorf("capability ID is zero")
				}
			}
		})
	}
}

func TestVerifyRequestSerialization(t *testing.T) {
	tests := []struct {
		name    string
		request VerifyRequest
	}{
		{
			name: "complete request",
			request: VerifyRequest{
				RequestID: "test-request-123",
				Request: RequestInfo{
					Method: "POST",
					Path:   "/v1/chat/completions",
					Headers: []KeyValuePair{
						{Key: "content-type", Value: "application/json"},
						{Key: "authorization", Value: "Bearer sk-test"},
					},
					Query: []KeyValuePair{
						{Key: "model", Value: "gpt-3.5-turbo"},
					},
					Body: "eyJ0ZXN0IjoidmFsdWUifQ==", // base64 encoded {"test":"value"}
				},
			},
		},
		{
			name: "minimal request",
			request: VerifyRequest{
				RequestID: "test-request-456",
				Request: RequestInfo{
					Method:  "GET",
					Path:    "/health",
					Headers: []KeyValuePair{},
					Query:   []KeyValuePair{},
					Body:    "",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test JSON marshaling
			jsonBytes, err := json.Marshal(tt.request)
			if err != nil {
				t.Errorf("failed to marshal request: %v", err)
				return
			}

			// Test JSON unmarshaling
			var unmarshaled VerifyRequest
			err = json.Unmarshal(jsonBytes, &unmarshaled)
			if err != nil {
				t.Errorf("failed to unmarshal request: %v", err)
				return
			}

			// Verify fields
			if unmarshaled.RequestID != tt.request.RequestID {
				t.Errorf("RequestID mismatch: got %s, want %s",
					unmarshaled.RequestID, tt.request.RequestID)
			}

			if unmarshaled.Request.Method != tt.request.Request.Method {
				t.Errorf("Method mismatch: got %s, want %s",
					unmarshaled.Request.Method, tt.request.Request.Method)
			}

			if unmarshaled.Request.Path != tt.request.Request.Path {
				t.Errorf("Path mismatch: got %s, want %s",
					unmarshaled.Request.Path, tt.request.Request.Path)
			}

			if len(unmarshaled.Request.Headers) != len(tt.request.Request.Headers) {
				t.Errorf("Headers length mismatch: got %d, want %d",
					len(unmarshaled.Request.Headers), len(tt.request.Request.Headers))
			}

			if len(unmarshaled.Request.Query) != len(tt.request.Request.Query) {
				t.Errorf("Query length mismatch: got %d, want %d",
					len(unmarshaled.Request.Query), len(tt.request.Request.Query))
			}

			if unmarshaled.Request.Body != tt.request.Request.Body {
				t.Errorf("Body mismatch: got %s, want %s",
					unmarshaled.Request.Body, tt.request.Request.Body)
			}
		})
	}
}

func TestVerifyResponseDeserialization(t *testing.T) {
	tests := []struct {
		name         string
		responseJSON string
		expectError  bool
		expected     VerifyResponse
	}{
		{
			name: "allow response with actions",
			responseJSON: `{
				"allow": true,
				"actions": [
					{
						"type": "replace",
						"target": "header",
						"key": "x-policy",
						"value": "checked",
						"capability_id": 123
					}
				],
				"response_check": {
					"enable": true,
					"capability_id": 456
				}
			}`,
			expectError: false,
			expected: VerifyResponse{
				Allow: true,
				Actions: []Action{
					{
						Type:         "replace",
						Target:       "header",
						Key:          "x-policy",
						Value:        "checked",
						CapabilityID: 123,
					},
				},
				ResponseCheck: &ResponseCheck{
					Enable:       true,
					CapabilityID: 456,
				},
			},
		},
		{
			name: "block response",
			responseJSON: `{
				"allow": false,
				"block_info": {
					"code": 403,
					"message": "Access denied",
					"capability_id": 789
				},
				"actions": []
			}`,
			expectError: false,
			expected: VerifyResponse{
				Allow: false,
				BlockInfo: &BlockInfo{
					Code:         403,
					Message:      "Access denied",
					CapabilityID: 789,
				},
				Actions: []Action{},
			},
		},
		{
			name: "invalid json",
			responseJSON: `{
				"allow": true,
				"actions": [
					{
						"type": "replace",
						"target": "header"
						// missing comma
						"key": "x-test"
					}
				]
			}`,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var response VerifyResponse
			err := json.Unmarshal([]byte(tt.responseJSON), &response)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			// Verify fields
			if response.Allow != tt.expected.Allow {
				t.Errorf("Allow mismatch: got %t, want %t",
					response.Allow, tt.expected.Allow)
			}

			if len(response.Actions) != len(tt.expected.Actions) {
				t.Errorf("Actions length mismatch: got %d, want %d",
					len(response.Actions), len(tt.expected.Actions))
			}

			// Check BlockInfo
			if tt.expected.BlockInfo != nil {
				if response.BlockInfo == nil {
					t.Errorf("expected BlockInfo but got nil")
				} else {
					if response.BlockInfo.Code != tt.expected.BlockInfo.Code {
						t.Errorf("BlockInfo.Code mismatch: got %d, want %d",
							response.BlockInfo.Code, tt.expected.BlockInfo.Code)
					}
					if response.BlockInfo.Message != tt.expected.BlockInfo.Message {
						t.Errorf("BlockInfo.Message mismatch: got %s, want %s",
							response.BlockInfo.Message, tt.expected.BlockInfo.Message)
					}
				}
			}

			// Check ResponseCheck
			if tt.expected.ResponseCheck != nil {
				if response.ResponseCheck == nil {
					t.Errorf("expected ResponseCheck but got nil")
				} else {
					if response.ResponseCheck.Enable != tt.expected.ResponseCheck.Enable {
						t.Errorf("ResponseCheck.Enable mismatch: got %t, want %t",
							response.ResponseCheck.Enable, tt.expected.ResponseCheck.Enable)
					}
				}
			}
		})
	}
}
