package files

import (
	"errors"
	"strings"
)

// ParseIntoString parses a include-file line and merges the result into dest.
func ParseIntoString(s string, dest map[string]string) error {
	for _, val := range strings.Split(s, ",") {
		val = strings.TrimSpace(val)
		splt := strings.SplitN(val, "=", 2)

		if len(splt) != 2 {
			return errors.New("Could not parse line")
		}

		name := strings.TrimSpace(splt[0])
		path := strings.TrimSpace(splt[1])
		dest[name] = path
	}

	return nil
}
