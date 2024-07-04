package rules

import (
    "bufio"
    "os"
    "strings"
)

// ParseIgnoreFile reads and parses the .helmlintignore file, returning a list of patterns
func ParseIgnoreFile(filePath string) ([]string, error) {
    var patterns []string
    file, err := os.Open(filePath)
    if err != nil {
        return nil, err
    }
    defer file.Close()

    scanner := bufio.NewScanner(file)
    for scanner.Scan() {
        line := strings.TrimSpace(scanner.Text())
        if line != "" && !strings.HasPrefix(line, "#") { // Ignore comments and empty lines
            patterns = append(patterns, line)
        }
    }

    return patterns, scanner.Err()
}
