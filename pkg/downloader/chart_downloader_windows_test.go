//go:build windows

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

package downloader

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"helm.sh/helm/v4/pkg/cli"
	"helm.sh/helm/v4/pkg/getter"
	"helm.sh/helm/v4/pkg/repo/v1/repotest"
)

// TestParallelDownloadTo tests that parallel downloads to the same file
// don't cause "Access Denied" errors on Windows. This test is Windows-specific
// because the file locking behavior is only needed on Windows.
func TestParallelDownloadTo(t *testing.T) {
	// Set up a simple test server with a chart
	srv := repotest.NewTempServer(t, repotest.WithChartSourceGlob("testdata/*.tgz"))
	defer srv.Stop()

	if err := srv.CreateIndex(); err != nil {
		t.Fatal(err)
	}

	dest := t.TempDir()
	cacheDir := t.TempDir()

	c := ChartDownloader{
		Out:              os.Stderr,
		RepositoryConfig: repoConfig,
		RepositoryCache:  repoCache,
		ContentCache:     cacheDir,
		Cache:            &DiskCache{Root: cacheDir},
		Getters: getter.All(&cli.EnvSettings{
			RepositoryConfig: repoConfig,
			RepositoryCache:  repoCache,
			ContentCache:     cacheDir,
		}),
	}

	// Use a direct URL to bypass repository lookup
	chartURL := srv.URL() + "/local-subchart-0.1.0.tgz"

	// Number of parallel downloads to attempt
	numDownloads := 10
	var wg sync.WaitGroup
	errors := make([]error, numDownloads)

	// Launch multiple goroutines to download the same chart simultaneously
	for i := 0; i < numDownloads; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			_, _, err := c.DownloadTo(chartURL, "", dest)
			errors[index] = err
		}(i)
	}

	wg.Wait()

	// Check if any download failed
	failedCount := 0
	for i, err := range errors {
		if err != nil {
			t.Logf("Download %d failed: %v", i, err)
			failedCount++
		}
	}

	// With the file locking fix, all parallel downloads should succeed
	if failedCount > 0 {
		t.Errorf("Parallel downloads failed: %d out of %d downloads failed due to concurrent file access", failedCount, numDownloads)
	}

	// Verify the file exists and is valid
	expectedFile := filepath.Join(dest, "local-subchart-0.1.0.tgz")
	info, err := os.Stat(expectedFile)
	if err != nil {
		t.Errorf("Expected file %s does not exist: %v", expectedFile, err)
	} else {
		// Verify the file is not empty
		if info.Size() == 0 {
			t.Errorf("Downloaded file %s is empty (0 bytes)", expectedFile)
		}

		// Verify the file has the expected size (should match the source file)
		sourceFile := "testdata/local-subchart-0.1.0.tgz"
		sourceInfo, err := os.Stat(sourceFile)
		if err == nil && info.Size() != sourceInfo.Size() {
			t.Errorf("Downloaded file size (%d bytes) doesn't match source file size (%d bytes)",
				info.Size(), sourceInfo.Size())
		}

		// Verify it's a valid tar.gz file by checking the magic bytes
		file, err := os.Open(expectedFile)
		if err == nil {
			defer file.Close()
			// gzip magic bytes are 0x1f 0x8b
			magic := make([]byte, 2)
			if n, err := file.Read(magic); err == nil && n == 2 {
				if magic[0] != 0x1f || magic[1] != 0x8b {
					t.Errorf("Downloaded file is not a valid gzip file (magic bytes: %x)", magic)
				}
			}
		}

		// Verify no lock file was left behind
		lockFile := expectedFile + ".lock"
		if _, err := os.Stat(lockFile); err == nil {
			t.Errorf("Lock file %s was not cleaned up", lockFile)
		}
	}
}
