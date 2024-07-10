package rules

import (
    "path/filepath"
    "strings"
    "fmt"
)

type LintResult struct {
    Messages []string
}

func IsIgnored(errorMessage string, patterns map[string][]string) bool {
    for path, pathPatterns := range patterns {
        cleanedPath := filepath.Clean(path)
        if strings.Contains(errorMessage, cleanedPath) {
            for _, pattern := range pathPatterns {
                if strings.Contains(errorMessage, pattern) {
                    fmt.Printf("Ignoring error related to path: %s with pattern: %s\n", path, pattern)
                    return true
                }
            }
        }
    }
    return false
}

