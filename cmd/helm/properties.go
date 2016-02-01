package main

import (
	"errors"
	"strconv"
	"strings"
)

// TODO: The concept of property here is really simple. We could definitely get
// better about the values we allow. Also, we need some validation on the names.

var errInvalidProperty = errors.New("property is not in name=value format")

// parseProperties is a utility for parsing a comma-separated key=value string.
func parseProperties(kvstr string) (map[string]interface{}, error) {
	properties := map[string]interface{}{}

	if len(kvstr) == 0 {
		return properties, nil
	}

	pairs := strings.Split(kvstr, ",")
	for _, p := range pairs {
		// Allow for "k=v, k=v"
		p = strings.TrimSpace(p)
		pair := strings.Split(p, "=")
		if len(pair) == 1 {
			return properties, errInvalidProperty
		}

		// If the value looks int-like, convert it.
		if i, err := strconv.Atoi(pair[1]); err == nil {
			properties[pair[0]] = pair[1]
		} else {
			properties[pair[0]] = i
		}
	}

	return properties, nil
}
