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
	"fmt"
	"os"
)

// ReadFileWithBudget reads a file and decrements remaining by the bytes read.
// It returns an error if the total would exceed MaxDecompressedChartSize.
func ReadFileWithBudget(path string, size int64, remaining *int64) ([]byte, error) {
	if size > *remaining {
		return nil, fmt.Errorf("chart exceeds maximum decompressed size of %d bytes", MaxDecompressedChartSize)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Re-check with actual length: the file may have grown between stat and read.
	if int64(len(data)) > *remaining {
		return nil, fmt.Errorf("chart exceeds maximum decompressed size of %d bytes", MaxDecompressedChartSize)
	}

	*remaining -= int64(len(data))
	return data, nil
}
