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

package archive

import (
	"errors"
	"fmt"
	"io"
	"math"
	"os"
)

// ReadFileWithBudget reads a file and decrements remaining by the bytes read.
// It returns an error if the total would exceed MaxDecompressedChartSize.
// The read is capped via io.LimitReader so a file that grows between stat
// and read cannot cause unbounded memory allocation.
func ReadFileWithBudget(path string, size int64, remaining *int64) ([]byte, error) {
	if remaining == nil {
		return nil, errors.New("remaining budget must not be nil")
	}
	if size > *remaining {
		return nil, fmt.Errorf("chart exceeds maximum decompressed size of %d bytes", MaxDecompressedChartSize)
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// Read at most *remaining+1 bytes so we can detect over-budget without
	// allocating unbounded memory if the file grew since stat.
	// Clamp to avoid int64 overflow when *remaining is near math.MaxInt64.
	limit := *remaining
	if limit < math.MaxInt64 {
		limit++
	}
	data, err := io.ReadAll(io.LimitReader(f, limit))
	if err != nil {
		return nil, err
	}

	if int64(len(data)) > *remaining {
		return nil, fmt.Errorf("chart exceeds maximum decompressed size of %d bytes", MaxDecompressedChartSize)
	}

	*remaining -= int64(len(data))
	return data, nil
}
