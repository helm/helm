/*
Copyright 2017 The Kubernetes Authors All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package logutil

import (
	"fmt"
	"strings"

	log "github.com/Sirupsen/logrus"
)

// LogLevels is the mapping for
var LogLevels = map[string]log.Level{
	"ERROR": log.ErrorLevel,
	"INFO":  log.InfoLevel,
	"DEBUG": log.DebugLevel,
}

// GetLevel returns the proper logrus level for the given string level.
// It returns an error if the level does not exist
func GetLevel(level string) (log.Level, error) {
	var logLevel log.Level
	var ok bool
	if logLevel, ok = LogLevels[level]; !ok {
		return 0, fmt.Errorf("Invalid log level %s given. Must be one of: %s", level, strings.Join(GetLogLevels(), ", "))
	}
	return logLevel, nil

}

// GetLogLevels returns a list of allowed log levels. Helpful for error messages.
func GetLogLevels() []string {
	var levels []string
	for k := range LogLevels {
		levels = append(levels, k)
	}
	return levels
}
