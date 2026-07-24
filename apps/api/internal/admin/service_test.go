package admin

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/aikssen/glazz-chat/apps/api/internal/platform/store"
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

func TestSummaryModelSettingRequiresUUID(t *testing.T) {
	t.Parallel()
	if !validSettingValue(
		"chat.summary_model_id",
		json.RawMessage(`"00000000-0000-7000-8000-000000000101"`),
	) {
		t.Fatal("valid summary model UUID was rejected")
	}
	if validSettingValue(
		"chat.summary_model_id",
		json.RawMessage(`"not-a-uuid-but-exactly-36-characters-x"`),
	) {
		t.Fatal("invalid summary model UUID was accepted")
	}
}

func TestRuntimeSettingValidationRejectsAmbiguousValues(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		key   string
		value string
		valid bool
	}{
		{name: "maintenance boolean", key: "maintenance.enabled", value: "true", valid: true},
		{name: "maintenance null", key: "maintenance.enabled", value: "null", valid: false},
		{name: "message integer", key: "quota.user.messages", value: "50", valid: true},
		{name: "message exponent", key: "quota.user.messages", value: "5e1", valid: false},
		{name: "system prompt", key: "chat.system_prompt", value: `"System policy"`, valid: true},
		{name: "blank prompt", key: "chat.system_prompt", value: `"  "`, valid: false},
		{name: "categories", key: "safety.input_categories", value: `["violence","abuse"]`, valid: true},
		{name: "duplicate category", key: "safety.input_categories", value: `["abuse","abuse"]`, valid: false},
		{name: "padded category", key: "safety.input_categories", value: `[" abuse"]`, valid: false},
		{name: "unknown key", key: "provider.api_key", value: `"secret"`, valid: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := validSettingValue(test.key, json.RawMessage(test.value)); got != test.valid {
				t.Fatalf("validSettingValue(%q, %s) = %t, want %t", test.key, test.value, got, test.valid)
			}
		})
	}
}

func TestAuditMappingRedactsPromptContentAndSecrets(t *testing.T) {
	t.Parallel()
	event := mapAudit(store.AdminAuditLog{
		ID:         uuid.New(),
		Action:     "setting.updated",
		TargetType: "runtime_setting",
		TargetID:   "chat.system_prompt",
		BeforeValue: []byte(`{
			"value":"private system prompt",
			"nested":{"apiKey":"credential","safe":"visible"}
		}`),
		AfterValue: []byte(`{
			"value":"new private system prompt",
			"authorization":"Bearer credential"
		}`),
		OccurredAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	})
	if event.Before["value"] != "[REDACTED]" || event.After["value"] != "[REDACTED]" {
		t.Fatalf("prompt values were not redacted: before=%#v after=%#v", event.Before, event.After)
	}
	nested, ok := event.Before["nested"].(map[string]any)
	if !ok || nested["apiKey"] != "[REDACTED]" || nested["safe"] != "visible" {
		t.Fatalf("nested audit redaction = %#v", event.Before["nested"])
	}
	if event.After["authorization"] != "[REDACTED]" {
		t.Fatalf("authorization redaction = %#v", event.After)
	}
}

func TestSystemPromptAuditValueNeverStoresPrompt(t *testing.T) {
	t.Parallel()
	value := settingAuditValue("chat.system_prompt", json.RawMessage(`"private"`), 2)
	if value["value"] != "[REDACTED]" {
		t.Fatalf("stored audit value = %#v", value)
	}
}
