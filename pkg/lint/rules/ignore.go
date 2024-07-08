package rules

import (
    "bufio"
    "os"
    "strings"
)

func ParseIgnoreFile(filePath string) (map[string]string, error) {
    patterns := make(map[string]string)
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
                patterns[parts[0]] = parts[1]
            } else if len(parts) == 1 {
                patterns[parts[0]] = ""
            }
        }
    }

    return patterns, scanner.Err()
}