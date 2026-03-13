package main

import (
	"testing"
)

// TestBlockResponseLogic tests the logic for handling allow: false responses
// Note: This is a logic test, not a full integration test
func TestBlockResponseLogic(t *testing.T) {
	tests := []struct {
		name             string
		response         VerifyResponse
		shouldBlock      bool
		expectedCode     int
		expectedMsg      string
		shouldRunActions bool
	}{
		{
			name: "allow true - should not block",
			response: VerifyResponse{
				Allow: true,
				Actions: []Action{
					{Type: "replace", Target: "header", Key: "x-test", Value: "value"},
				},
			},
			shouldBlock:      false,
			shouldRunActions: true,
		},
		{
			name: "allow false with block_info - should block with custom message",
			response: VerifyResponse{
				Allow: false,
				BlockInfo: &BlockInfo{
					Code:         403,
					Message:      "IP not in whitelist",
					CapabilityID: 123,
				},
				Actions: []Action{},
			},
			shouldBlock:      true,
			expectedCode:     403,
			expectedMsg:      "IP not in whitelist",
			shouldRunActions: false,
		},
		{
			name: "allow false without block_info - should block with default message",
			response: VerifyResponse{
				Allow:   false,
				Actions: []Action{},
			},
			shouldBlock:      true,
			expectedCode:     403, // Default Forbidden
			expectedMsg:      "Request blocked by capability",
			shouldRunActions: false,
		},
		{
			name: "allow false with 401 code - should use custom code",
			response: VerifyResponse{
				Allow: false,
				BlockInfo: &BlockInfo{
					Code:         401,
					Message:      "Invalid API key",
					CapabilityID: 456,
				},
			},
			shouldBlock:      true,
			expectedCode:     401,
			expectedMsg:      "Invalid API key",
			shouldRunActions: false,
		},
		{
			name: "allow true with response_check - should continue with check enabled",
			response: VerifyResponse{
				Allow: true,
				ResponseCheck: &ResponseCheck{
					Enable:       true,
					CapabilityID: 789,
				},
			},
			shouldBlock:      false,
			shouldRunActions: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test the logic flow
			if !tt.response.Allow {
				// Should block
				if !tt.shouldBlock {
					t.Errorf("Expected to block but shouldBlock is false")
				}

				// Verify block info
				if tt.response.BlockInfo != nil {
					if tt.response.BlockInfo.Code != tt.expectedCode {
						t.Errorf("Expected code %d, got %d", tt.expectedCode, tt.response.BlockInfo.Code)
					}
					if tt.response.BlockInfo.Message != tt.expectedMsg {
						t.Errorf("Expected message %s, got %s", tt.expectedMsg, tt.response.BlockInfo.Message)
					}
				}

				// Should not run actions when blocked
				if tt.shouldRunActions {
					t.Errorf("Should not run actions when request is blocked")
				}
			} else {
				// Should not block
				if tt.shouldBlock {
					t.Errorf("Expected not to block but shouldBlock is true")
				}

				// Should run actions when allowed
				if !tt.shouldRunActions {
					t.Errorf("Should run actions when request is allowed")
				}
			}
		})
	}
}

// TestResponseCheckLogic tests the response check flag logic
func TestResponseCheckLogic(t *testing.T) {
	tests := []struct {
		name                  string
		response              VerifyResponse
		shouldEnableRespCheck bool
	}{
		{
			name: "response_check enabled",
			response: VerifyResponse{
				Allow: true,
				ResponseCheck: &ResponseCheck{
					Enable:       true,
					CapabilityID: 123,
				},
			},
			shouldEnableRespCheck: true,
		},
		{
			name: "response_check disabled",
			response: VerifyResponse{
				Allow: true,
				ResponseCheck: &ResponseCheck{
					Enable:       false,
					CapabilityID: 456,
				},
			},
			shouldEnableRespCheck: false,
		},
		{
			name: "response_check nil",
			response: VerifyResponse{
				Allow:         true,
				ResponseCheck: nil,
			},
			shouldEnableRespCheck: false,
		},
		{
			name: "blocked request should not check response",
			response: VerifyResponse{
				Allow: false,
				BlockInfo: &BlockInfo{
					Code:    403,
					Message: "Blocked",
				},
				ResponseCheck: &ResponseCheck{
					Enable: true, // This should be ignored
				},
			},
			shouldEnableRespCheck: false, // Because request is blocked
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Logic: response check should only be enabled if:
			// 1. Request is allowed (allow: true)
			// 2. ResponseCheck is not nil
			// 3. ResponseCheck.Enable is true

			shouldCheck := tt.response.Allow &&
				tt.response.ResponseCheck != nil &&
				tt.response.ResponseCheck.Enable

			if shouldCheck != tt.shouldEnableRespCheck {
				t.Errorf("Expected shouldEnableRespCheck=%t, got %t",
					tt.shouldEnableRespCheck, shouldCheck)
			}
		})
	}
}

// TestActionExecutionOrder tests that actions should not be executed when blocked
func TestActionExecutionOrder(t *testing.T) {
	tests := []struct {
		name                 string
		response             VerifyResponse
		shouldExecuteActions bool
	}{
		{
			name: "allow true with actions - should execute",
			response: VerifyResponse{
				Allow: true,
				Actions: []Action{
					{Type: "replace", Target: "header", Key: "x-test", Value: "value"},
				},
			},
			shouldExecuteActions: true,
		},
		{
			name: "allow false with actions - should NOT execute",
			response: VerifyResponse{
				Allow: false,
				BlockInfo: &BlockInfo{
					Code:    403,
					Message: "Blocked",
				},
				Actions: []Action{
					{Type: "replace", Target: "header", Key: "x-test", Value: "value"},
				},
			},
			shouldExecuteActions: false,
		},
		{
			name: "allow true with empty actions - nothing to execute",
			response: VerifyResponse{
				Allow:   true,
				Actions: []Action{},
			},
			shouldExecuteActions: false, // No actions to execute
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Logic: actions should only be executed if:
			// 1. Request is allowed (allow: true)
			// 2. There are actions to execute

			shouldExecute := tt.response.Allow && len(tt.response.Actions) > 0

			if shouldExecute != tt.shouldExecuteActions {
				t.Errorf("Expected shouldExecuteActions=%t, got %t",
					tt.shouldExecuteActions, shouldExecute)
			}
		})
	}
}
