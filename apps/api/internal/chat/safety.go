package chat

import (
	"context"
	"regexp"
	"strings"
)

type SafetyStage string

const (
	SafetyInput  SafetyStage = "input"
	SafetyOutput SafetyStage = "output"
)

type SafetyDecision struct {
	Blocked  bool
	Category string
}

type SafetyPolicy interface {
	Check(context.Context, SafetyStage, string, []string) (SafetyDecision, error)
}

type SafetyReport struct {
	Stage     SafetyStage
	Category  string
	RequestID string
}

type SafetyReporter interface {
	Report(context.Context, SafetyReport) error
}

type SafetyReporterFunc func(context.Context, SafetyReport) error

func (reporter SafetyReporterFunc) Report(ctx context.Context, report SafetyReport) error {
	return reporter(ctx, report)
}

type SafetyCategorySource func(context.Context) (input, output []string, err error)

type RuleSafetyPolicy struct {
	rules map[string]*regexp.Regexp
}

func NewRuleSafetyPolicy() *RuleSafetyPolicy {
	return &RuleSafetyPolicy{rules: map[string]*regexp.Regexp{
		"credentials": regexp.MustCompile(
			`(?i)(-----BEGIN [A-Z ]*PRIVATE KEY-----|(?:api[_ -]?key|secret|token)\s*[:=]\s*[A-Za-z0-9_\-]{12,})`,
		),
		"self_harm": regexp.MustCompile(
			`(?i)\b(?:kill myself|suicide method|cómo suicidarme|como suicidarme)\b`,
		),
		"sexual_minors": regexp.MustCompile(
			`(?i)\b(?:child sexual abuse|sexual content involving minors|contenido sexual de menores)\b`,
		),
		"violent_threat": regexp.MustCompile(
			`(?i)\b(?:how to murder|plan to kill|cómo asesinar|como asesinar)\b`,
		),
	}}
}

func (policy *RuleSafetyPolicy) Check(
	ctx context.Context,
	_ SafetyStage,
	content string,
	categories []string,
) (SafetyDecision, error) {
	if err := ctx.Err(); err != nil {
		return SafetyDecision{}, err
	}
	for _, category := range categories {
		category = strings.TrimSpace(strings.ToLower(category))
		rule := policy.rules[category]
		if rule != nil && rule.MatchString(content) {
			return SafetyDecision{Blocked: true, Category: category}, nil
		}
	}
	return SafetyDecision{}, nil
}
