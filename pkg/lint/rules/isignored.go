package rules

import (
    "path/filepath"
    "strings"
)

func IsIgnored(path string, patterns []string) bool {
    for _, pattern := range patterns {
        cleanedPath := filepath.Clean(path)
        cleanedPattern := filepath.Clean(pattern)
        if match, err := filepath.Match(cleanedPattern, cleanedPath); err == nil && match {
            return true
        }
        if strings.HasSuffix(cleanedPattern, "/") || strings.HasSuffix(cleanedPattern, "\\") {
            patternDir := strings.TrimRight(cleanedPattern, "/\\")
            if strings.HasPrefix(cleanedPath, patternDir) {
                return true
            }
        }
    }
    return false
}
