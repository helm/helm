package ignore

import (
	"log/slog"
	"path/filepath"
	"strings"
)

const pathlessRulePrefix = "error_lint_ignore="

type MatchesErrors interface {
	Match(error) bool
	LogAttrs() slog.Attr
}

type BadTemplateRule struct {
	RuleText        string
	MessageText     string
	BadTemplatePath string
}

type PathlessRule struct {
	RuleText    string
	MessageText string
}

// Match errors that have no file path in their body with ignorer rules.
// An examples of errors with no file path in their body is chart metadata errors `chart metadata is missing these dependencies`
func (pr PathlessRule) Match(err error) bool {
	errText := err.Error()
	matchableParts := strings.SplitN(pr.MessageText, ":", 2)
	matchablePrefix := strings.TrimSpace(matchableParts[0])

	if match, _ := filepath.Match(pr.MessageText, errText); match {
		return true
	}
	if matched, _ := filepath.Match(matchablePrefix, errText); matched {
		return true
	}

	return false
}

// LogAttrs Used for troubleshooting and gathering data
func (pr PathlessRule) LogAttrs() slog.Attr {
	return slog.Group("BadTemplateRule",
		slog.String("rule_text", pr.RuleText),
		slog.String("value", pr.MessageText),
	)
}

// LogAttrs Used for troubleshooting and gathering data
func (btr BadTemplateRule) LogAttrs() slog.Attr {
	return slog.Group("BadTemplateRule",
		slog.String("rule_text", btr.RuleText),
		slog.String("key", btr.BadTemplatePath),
		slog.String("value", btr.MessageText),
	)
}

// Match errors that have a file path in their body with ignorer rules.
func (btr BadTemplateRule) Match(err error) bool {
	errText := err.Error()
	pathToOffendingFile, err := pathToOffendingFile(errText)
	if err != nil {
		return false
	}

	cleanRulePath := filepath.Clean(btr.BadTemplatePath)

	if strings.Contains(pathToOffendingFile, cleanRulePath) {
		if strings.Contains(errText, btr.MessageText) {
			return true
		}
	}

	return false
}
