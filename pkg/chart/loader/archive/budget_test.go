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
	"path/filepath"
	"testing"
)

func TestReadFileWithBudget(t *testing.T) {
	dir := t.TempDir()

	writeFile := func(t *testing.T, name string, size int) string {
		t.Helper()
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, make([]byte, size), 0644); err != nil {
			t.Fatal(err)
		}
		return p
	}

	tcs := []struct {
		name  string
		check func(t *testing.T)
	}{
		{
			name: "reads file and decrements budget",
			check: func(t *testing.T) {
				t.Helper()
				p := writeFile(t, "small.txt", 100)
				fi, err := os.Stat(p)
				if err != nil {
					t.Fatalf("failed to stat %s: %v", p, err)
				}
				max := int64(1000)

				br := NewBudgetedReader(max)
				data, err := br.ReadFileWithBudget(p, fi.Size())
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if len(data) != 100 {
					t.Fatalf("expected 100 bytes, got %d", len(data))
				}
				if br.remaining != 900 {
					t.Fatalf("expected remaining=900, got %d", br.remaining)
				}
			},
		},
		{
			name: "rejects file exceeding budget",
			check: func(t *testing.T) {
				t.Helper()
				p := writeFile(t, "big.txt", 500)
				fi, err := os.Stat(p)
				if err != nil {
					t.Fatalf("failed to stat %s: %v", p, err)
				}
				max := int64(100)

				br := NewBudgetedReader(max)
				_, err = br.ReadFileWithBudget(p, fi.Size())
				if err == nil {
					t.Fatal("expected error for file exceeding budget")
				}
				expectedErr := fmt.Sprintf("chart exceeds maximum decompressed size of %d bytes", max)
				if err.Error() != expectedErr {
					t.Fatalf("expected %q, got %q", expectedErr, err.Error())
				}
				if br.remaining != 100 {
					t.Fatalf("budget should not change on rejection, got %d", br.remaining)
				}
			},
		},
		{
			name: "tracks budget across multiple reads",
			check: func(t *testing.T) {
				t.Helper()
				remaining := int64(250)

				br := NewBudgetedReader(remaining)
				for i := range 3 {
					p := writeFile(t, fmt.Sprintf("f%d.txt", i), 80)
					fi, err := os.Stat(p)
					if err != nil {
						t.Fatalf("failed to stat %s: %v", p, err)
					}
					if _, err := br.ReadFileWithBudget(p, fi.Size()); err != nil {
						t.Fatalf("read %d: unexpected error: %v", i, err)
					}
				}
				if br.remaining != 10 {
					t.Fatalf("expected remaining=10, got %d", br.remaining)
				}

				p := writeFile(t, "over.txt", 20)
				fi, err := os.Stat(p)
				if err != nil {
					t.Fatalf("failed to stat %s: %v", p, err)
				}
				_, err = br.ReadFileWithBudget(p, fi.Size())
				if err == nil {
					t.Fatal("expected error when cumulative reads exceed budget")
				}
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, tc.check)
	}
}
