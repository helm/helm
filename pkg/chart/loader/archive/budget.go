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
	"io"
	"math"
	"os"
)

// budgetedReader tracks cumulative file reads against a size limit.
type budgetedReader struct {
	max       int64
	remaining int64
}

// newBudgetedReader creates a new budgetedReader with the given maximum decompressed chart size.
// The remaining budget is initialized to the maximum size.
func NewBudgetedReader(max int64) *budgetedReader {
	return &budgetedReader{
		max:       max,
		remaining: max,
	}
}

// readFileWithBudget reads a file and decrements remaining by the bytes read.
// It returns an error if the total would exceed the maximum decompressed chart size.
// The read is capped via io.LimitReader so a file that grows between stat
// and read cannot cause unbounded memory allocation.
func (r *budgetedReader) ReadFileWithBudget(path string, size int64) ([]byte, error) {
	if size > r.remaining {
		return nil, fmt.Errorf("chart exceeds maximum decompressed size of %d bytes", r.max)
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// Read at most r.remaining+1 bytes so we can detect over-budget without
	// allocating unbounded memory if the file grew since stat.
	// Clamp to avoid int64 overflow when r.remaining is near math.MaxInt64.
	limit := r.remaining
	if limit < math.MaxInt64 {
		limit++
	}
	data, err := io.ReadAll(io.LimitReader(f, limit))
	if err != nil {
		return nil, err
	}

	if int64(len(data)) > r.remaining {
		return nil, fmt.Errorf("chart exceeds maximum decompressed size of %d bytes", r.max)
	}

	r.remaining -= int64(len(data))
	return data, nil
}
