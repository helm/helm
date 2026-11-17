/*
Copyright The Helm Authors.

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

package cmd

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

// sourceDateEpochFromEnv returns SOURCE_DATE_EPOCH when set, or nil when unset.
func sourceDateEpochFromEnv() (*time.Time, error) {
	epochStr, ok := os.LookupEnv("SOURCE_DATE_EPOCH")
	if !ok || epochStr == "" {
		return nil, nil
	}
	epoch, err := strconv.ParseInt(epochStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid SOURCE_DATE_EPOCH: %w", err)
	}
	if epoch < 0 {
		return nil, errors.New("invalid SOURCE_DATE_EPOCH: must not be negative")
	}
	t := time.Unix(epoch, 0).UTC()
	return &t, nil
}
