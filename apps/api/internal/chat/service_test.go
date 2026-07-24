package chat

import (
	"context"
	"strings"
	"testing"
)

func TestValidContent(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		value string
		valid bool
	}{
		{name: "plain", value: "hello\nworld", valid: true},
		{name: "blank", value: " \t\n", valid: false},
		{name: "control", value: "hello\x00world", valid: false},
		{name: "too long", value: strings.Repeat("a", 32001), valid: false},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := validContent(test.value); got != test.valid {
				t.Fatalf("validContent() = %v, want %v", got, test.valid)
			}
		})
	}
}

func TestRuleSafetyPolicyUsesOnlyEnabledCategories(t *testing.T) {
	t.Parallel()
	policy := NewRuleSafetyPolicy()
	content := "api_key = abcdefghijklmnop"
	decision, err := policy.Check(context.Background(), SafetyInput, content, nil)
	if err != nil || decision.Blocked {
		t.Fatalf("disabled decision = %#v, error %v", decision, err)
	}
	decision, err = policy.Check(
		context.Background(), SafetyInput, content, []string{"credentials"},
	)
	if err != nil || !decision.Blocked || decision.Category != "credentials" {
		t.Fatalf("enabled decision = %#v, error %v", decision, err)
	}
}

func TestStrictJSONRejectsUnknownFields(t *testing.T) {
	t.Parallel()
	var target struct {
		Content string `json:"content"`
	}
	if err := strictJSON([]byte(`{"content":"ok","secret":"no"}`), &target); err == nil {
		t.Fatal("strictJSON accepted an unknown field")
	}
}
