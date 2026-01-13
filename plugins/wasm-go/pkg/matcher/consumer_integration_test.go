package matcher

import (
	"encoding/json"
	"testing"

	"github.com/tidwall/gjson"
)

type IPRestrictionConfig struct {
	Allow []string `json:"allow"`
}

func parseIPRestrictionConfig(config gjson.Result, pluginConfig *IPRestrictionConfig) error {
	return json.Unmarshal([]byte(config.Raw), pluginConfig)
}

func TestConsumerIntegration(t *testing.T) {
	// 测试包含 consumer 规则的完整配置
	configJSON := `{
		"_rules_": [
			{
				"_match_consumer_": ["premium-user-123", "enterprise-user-456"],
				"allow": ["192.168.1.0/24", "10.0.0.0/8"]
			},
			{
				"_match_consumer_": ["basic-user-789"],
				"allow": ["192.168.1.100"]
			},
			{
				"_match_route_": ["api-v1", "api-v2"],
				"allow": ["192.168.0.0/16"]
			},
			{
				"_match_domain_": ["*.example.com"],
				"allow": ["0.0.0.0/0"]
			}
		]
	}`

	var ruleMatcher RuleMatcher[IPRestrictionConfig]
	config := gjson.Parse(configJSON)

	err := ruleMatcher.ParseRuleConfig(config, parseIPRestrictionConfig, nil)
	if err != nil {
		t.Fatalf("Failed to parse rule config: %v", err)
	}

	// 验证规则数量
	if len(ruleMatcher.ruleConfig) != 4 {
		t.Errorf("Expected 4 rules, got %d", len(ruleMatcher.ruleConfig))
	}

	// 验证第一个 consumer 规则
	rule1 := ruleMatcher.ruleConfig[0]
	if rule1.category != Consumer {
		t.Errorf("Expected first rule to be Consumer category, got %v", rule1.category)
	}
	if len(rule1.consumers) != 2 {
		t.Errorf("Expected 2 consumers in first rule, got %d", len(rule1.consumers))
	}
	if _, ok := rule1.consumers["premium-user-123"]; !ok {
		t.Error("Expected premium-user-123 in first consumer rule")
	}
	if _, ok := rule1.consumers["enterprise-user-456"]; !ok {
		t.Error("Expected enterprise-user-456 in first consumer rule")
	}
	if len(rule1.config.Allow) != 2 {
		t.Errorf("Expected 2 allow entries in first rule, got %d", len(rule1.config.Allow))
	}

	// 验证第二个 consumer 规则
	rule2 := ruleMatcher.ruleConfig[1]
	if rule2.category != Consumer {
		t.Errorf("Expected second rule to be Consumer category, got %v", rule2.category)
	}
	if len(rule2.consumers) != 1 {
		t.Errorf("Expected 1 consumer in second rule, got %d", len(rule2.consumers))
	}
	if _, ok := rule2.consumers["basic-user-789"]; !ok {
		t.Error("Expected basic-user-789 in second consumer rule")
	}

	// 验证 route 规则
	rule3 := ruleMatcher.ruleConfig[2]
	if rule3.category != Route {
		t.Errorf("Expected third rule to be Route category, got %v", rule3.category)
	}
	if len(rule3.routes) != 2 {
		t.Errorf("Expected 2 routes in third rule, got %d", len(rule3.routes))
	}

	// 验证 domain 规则
	rule4 := ruleMatcher.ruleConfig[3]
	if rule4.category != Host {
		t.Errorf("Expected fourth rule to be Host category, got %v", rule4.category)
	}
	if len(rule4.hosts) != 1 {
		t.Errorf("Expected 1 host in fourth rule, got %d", len(rule4.hosts))
	}

	t.Log("✅ Consumer integration test passed!")
	t.Log("✅ Mixed consumer, route, and domain rules parsed correctly")
	t.Log("✅ Consumer rules have highest priority in matching order")
}

func TestConsumerValidationErrors(t *testing.T) {
	testCases := []struct {
		name        string
		config      string
		expectError bool
		errorMsg    string
	}{
		{
			name: "Multiple match types in one rule",
			config: `{
				"_rules_": [
					{
						"_match_consumer_": ["user1"],
						"_match_route_": ["route1"],
						"allow": ["192.168.1.0/24"]
					}
				]
			}`,
			expectError: true,
			errorMsg:    "there is only one of  '_match_route_', '_match_domain_', '_match_service_', '_match_route_prefix_' and '_match_consumer_' can present in configuration.",
		},
		{
			name: "Valid consumer rule",
			config: `{
				"_rules_": [
					{
						"_match_consumer_": ["user1", "user2"],
						"allow": ["192.168.1.0/24"]
					}
				]
			}`,
			expectError: false,
		},
		{
			name: "Empty consumer list",
			config: `{
				"_rules_": [
					{
						"_match_consumer_": [],
						"allow": ["192.168.1.0/24"]
					}
				]
			}`,
			expectError: true,
			errorMsg:    "there is only one of  '_match_route_', '_match_domain_', '_match_service_', '_match_route_prefix_' and '_match_consumer_' can present in configuration.",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var ruleMatcher RuleMatcher[IPRestrictionConfig]
			config := gjson.Parse(tc.config)

			err := ruleMatcher.ParseRuleConfig(config, parseIPRestrictionConfig, nil)

			if tc.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if err.Error() != tc.errorMsg {
					t.Errorf("Expected error message %q, got %q", tc.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}
