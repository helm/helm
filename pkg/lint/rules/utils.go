package rules

import (
    "regexp"
    "strconv"
)

// parseErrorDetails extracts the file path and the line of the error from a given error message.
func parseErrorDetails(err string) (filePath string, line int, snippet string) {
    // Regular expression to find the file path and line:column numbers
    // This pattern assumes the error format is stable and always similar to the provided example
    re := regexp.MustCompile(`(?m)([^:]+):(\d+):(\d+): executing "([^"]+)"`)
    matches := re.FindStringSubmatch(err)
    if len(matches) < 5 {
        return "", 0, "" // Return default empty values if the format does not match
    }
    line, errConvert := strconv.Atoi(matches[2])
    if errConvert != nil {
        return matches[1], 0, matches[4]
    }

    return matches[1], line, matches[4]
}
