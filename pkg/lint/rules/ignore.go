package rules

import (
    "bufio"
    "os"
    "strings"
)

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
        if line != "" && !strings.HasPrefix(line, "#") { 
            patterns = append(patterns, line)
        }
    }

    return patterns, scanner.Err()
}