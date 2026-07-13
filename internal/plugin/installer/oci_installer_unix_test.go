//go:build !windows

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

package installer

import (
	"archive/tar"
	"bytes"
	"fmt"
	"syscall"
	"testing"
)

func TestExtractTarFileDescriptorLeak(t *testing.T) {
	var rLimit syscall.Rlimit
	err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit)
	if err != nil {
		t.Skipf("Skipping test because Getrlimit failed: %v", err)
	}

	// Lower the limit to 50
	oldLimit := rLimit
	rLimit.Cur = 50
	if rLimit.Max < 50 {
		rLimit.Max = 50
	}

	err = syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rLimit)
	if err != nil {
		t.Skipf("Skipping test because Setrlimit failed: %v", err)
	}
	defer func() {
		// Restore the original limit
		if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &oldLimit); err != nil {
			t.Logf("Failed to restore RLIMIT_NOFILE: %v", err)
		}
	}()

	// Create a dummy tar archive with 100 files (more than the limit of 50)
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for i := 0; i < 100; i++ {
		hdr := &tar.Header{
			Name:     fmt.Sprintf("file_%d.txt", i),
			Mode:     0600,
			Size:     0,
			Typeflag: tar.TypeReg,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("Failed to write header: %v", err)
		}
		if _, err := tw.Write([]byte{}); err != nil {
			t.Fatalf("Failed to write content: %v", err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("Failed to close tar writer: %v", err)
	}

	// Extract the tar archive
	tempDir := t.TempDir()
	if err := extractTar(&buf, tempDir); err != nil {
		t.Fatalf("extractTar failed, likely due to file descriptor exhaustion: %v", err)
	}
}
