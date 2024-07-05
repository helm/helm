package rules

import (
    "regexp"
    "strconv"
)

func parseErrorDetails(err string) (string, int, int) {
    re := regexp.MustCompile(`([^:]+):(\d+):(\d+): executing`)
    matches := re.FindStringSubmatch(err)
    if len(matches) < 4 {
        return "", 0, 0 // Return default values if the format does not match
    }

    line, _ := strconv.Atoi(matches[2])
    col, _ := strconv.Atoi(matches[3])
    return matches[1], line, col
}