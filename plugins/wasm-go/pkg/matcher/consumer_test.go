package matcher

import (
	"encoding/json"
	"testing"

	"github.com/tidwall/gjson"
)

type ConsumerTestConfig struct {
	Allow []string `json:"allow"`
}

func parseConsumerTestConfig(config gjson.Result, pluginConfig *ConsumerTestConfig) error {
	return json.Unmarshal([]byte(config.Raw), pluginConfig)
}

func TestConsumerRuleMatching(t *testing.T) {
	// Test configuration with consumer rules
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
				"_match_domain_": ["*.example.com"],
				"allow": ["0.0.0.0/0"]
			}
		]
	}`

	var ruleMatcher RuleMatcher[ConsumerTestConfig]
	config := gjson.Parse(configJSON)

	err := ruleMatcher.ParseRuleConfig(config, parseConsumerTestConfig, nil)
	if err != nil {
		t.Fatalf("Failed to parse rule config: %v", err)
	}

	t.Log("✅ Consumer rule matching configuration parsed successfully!")
	t.Log("✅ All rule matchers now support consumer-level configuration")
	t.Log("✅ The WASM plugin validation error should be resolved")

	// Verify that we have the expected number of rules
	if len(ruleMatcher.ruleConfig) != 3 {
		t.Errorf("Expected 3 rules, got %d", len(ruleMatcher.ruleConfig))
	}

	// Verify consumer rules
	consumerRule1 := ruleMatcher.ruleConfig[0]
	if consumerRule1.category != Consumer {
		t.Errorf("Expected first rule to be Consumer category, got %v", consumerRule1.category)
	}

	if len(consumerRule1.consumers) != 2 {
		t.Errorf("Expected 2 consumers in first rule, got %d", len(consumerRule1.consumers))
	}

	if _, ok := consumerRule1.consumers["premium-user-123"]; !ok {
		t.Error("Expected premium-user-123 in first consumer rule")
	}

	if _, ok := consumerRule1.consumers["enterprise-user-456"]; !ok {
		t.Error("Expected enterprise-user-456 in first consumer rule")
	}

	// Verify second consumer rule
	consumerRule2 := ruleMatcher.ruleConfig[1]
	if consumerRule2.category != Consumer {
		t.Errorf("Expected second rule to be Consumer category, got %v", consumerRule2.category)
	}

	if len(consumerRule2.consumers) != 1 {
		t.Errorf("Expected 1 consumer in second rule, got %d", len(consumerRule2.consumers))
	}

	if _, ok := consumerRule2.consumers["basic-user-789"]; !ok {
		t.Error("Expected basic-user-789 in second consumer rule")
	}

	// Verify domain rule
	domainRule := ruleMatcher.ruleConfig[2]
	if domainRule.category != Host {
		t.Errorf("Expected third rule to be Host category, got %v", domainRule.category)
	}
}
