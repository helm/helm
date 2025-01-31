package ignore

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func LoadFromFilePath(chartPath, ignoreFilePath string) ([]MatchesErrors, error) {
	if ignoreFilePath == "" {
		ignoreFilePath = filepath.Join(chartPath, DefaultIgnoreFileName)
		debug("\nNo helm lint ignore filepath specified, will try and use the following default: %s\n", ignoreFilePath)
	}

	// attempt to load ignore patterns from ignoreFilePath.
	// if none are found, return an empty ignorer so the program can keep running.
	debug("\nTrying to load helm lint ignore file at %s\n", ignoreFilePath)
	file, err := os.Open(ignoreFilePath)
	if err != nil {
		debug("failed to open helm lint ignore file: %s", ignoreFilePath)
		return []MatchesErrors{}, nil
	}
	defer func() {
		err := file.Close()
		if err != nil {
			debug("failed to close helm lint ignore file: %s", ignoreFilePath)
		}
	}()

	matchers := LoadFromReader(file)
	return matchers, nil
}

// debug provides [pkg/lint/ignore] with a runtime-overridable logging function
// intended to match the behavior of the top level debug() method from package main.
//
// When no debugFn is set for the package at runtime then debug will fall back to
// defaultDebugFn.
func debug(format string, args ...interface{}) {
	if debugFn == nil {
		defaultDebugFn(format, args...)
	} else {
		debugFn(format, args...)
	}
	return
}

// TODO: figure out & fix or remove
func pathToOffendingFile(errText string) (string, error) {
	delimiter := ":"
	// splits into N parts delimited by colons
	parts := strings.Split(errText, delimiter)
	// if 3 or more parts, return the second part, after trimming its spaces
	if len(parts) > 2 {
		return strings.TrimSpace(parts[1]), nil
	}
	// if fewer than 3 parts, return empty string
	return "", fmt.Errorf("fewer than three [%s]-delimited parts found, no path here: %s", delimiter, errText)
}

func LoadFromReader(rdr io.Reader) []MatchesErrors {
	matchers := make([]MatchesErrors, 0)

	scanner := bufio.NewScanner(rdr)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.HasPrefix(line, pathlessRulePrefix) {
			matchers = append(matchers, buildPathlessRule(line, pathlessRulePrefix))
		} else {
			matchers = append(matchers, buildBadTemplateRule(line))
		}
	}

	return matchers
}

func buildPathlessRule(line string, pathlessRulePrefix string) PathlessRule {
	// handle chart-level errors
	// Drop 'error_lint_ignore=' prefix from rule before saving it
	const numSplits = 2
	tokens := strings.SplitN(line[len(pathlessRulePrefix):], pathlessRulePrefix, numSplits)
	if len(tokens) == numSplits {
		// TODO: find an example for this one - not sure we still use it
		messageText, _ := tokens[0], tokens[1]
		return PathlessRule{RuleText: line, MessageText: messageText}
	} else {
		messageText := tokens[0]
		return PathlessRule{RuleText: line, MessageText: messageText}
	}
}

func buildBadTemplateRule(line string) BadTemplateRule {
	const noMessageText = ""
	const separator = " "
	const numSplits = 2

	// handle chart yaml file errors in specific template files
	parts := strings.SplitN(line, separator, numSplits)
	if len(parts) == numSplits {
		messagePath, messageText := parts[0], parts[1]
		return BadTemplateRule{RuleText: line, BadTemplatePath: messagePath, MessageText: messageText}
	} else {
		messagePath := parts[0]
		return BadTemplateRule{RuleText: line, BadTemplatePath: messagePath, MessageText: noMessageText}
	}
}
