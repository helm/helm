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
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

// isGzipArchive checks if data represents a gzip archive by checking the magic bytes
func isGzipArchive(data []byte) bool {
	return len(data) >= 2 && data[0] == 0x1f && data[1] == 0x8b
}

// isGzipArchiveFromURL checks if a URL points to a gzip archive by reading the magic bytes
func isGzipArchiveFromURL(url string) (bool, error) {
	// Use a short timeout context to avoid hanging on slow servers
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Make a GET request to read the first few bytes
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false, err
	}

	// Request only the first 512 bytes to check magic bytes
	req.Header.Set("Range", "bytes=0-511")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	// Check for valid status codes early
	// 206 = Partial Content (range supported)
	// 200 = OK (range not supported, full content returned)
	if resp.StatusCode != http.StatusPartialContent && resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("unexpected status code %d when checking gzip archive at %s", resp.StatusCode, url)
	}

	// Read exactly 2 bytes for gzip magic number check
	buf := make([]byte, 2)
	if _, err := io.ReadAtLeast(resp.Body, buf, len(buf)); err != nil {
		return false, fmt.Errorf("failed to read magic bytes from %s: %w", url, err)
	}

	return isGzipArchive(buf), nil
}
