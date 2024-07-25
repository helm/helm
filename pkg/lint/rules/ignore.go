package rules

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func ParseIgnoreFile(filePath string) (map[string][]string, error) {
	patterns := make(map[string][]string)
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && !strings.HasPrefix(line, "#") {
			parts := strings.SplitN(line, " ", 2)
			if len(parts) > 1 {
				// Check if the key already exists and append to its slice
				patterns[parts[0]] = append(patterns[parts[0]], parts[1])
			} else if len(parts) == 1 {
				// Add an empty pattern if only the path is present
				patterns[parts[0]] = append(patterns[parts[0]], "")
			}
		}
	}

	return patterns, scanner.Err()
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

