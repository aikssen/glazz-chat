package admin

import (
	"encoding/json"
	"testing"
)

func TestGlobalOutputBudgetSettingValidation(t *testing.T) {
	tests := []struct {
		name  string
		value string
		valid bool
	}{
		{name: "minimum", value: "128", valid: true},
		{name: "configured budget", value: "10000000", valid: true},
		{name: "below minimum", value: "127", valid: false},
		{name: "fractional", value: "128.5", valid: false},
		{name: "wrong type", value: `"10000000"`, valid: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if actual := validSettingValue(
				"quota.global.output_tokens", json.RawMessage(test.value),
			); actual != test.valid {
				t.Fatalf("validSettingValue() = %t, want %t", actual, test.valid)
			}
		})
	}
}
