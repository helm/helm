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

import "net/http"

// isGzipArchive checks if data represents a gzip archive by checking the magic bytes
func isGzipArchive(data []byte) bool {
	return len(data) >= 2 && data[0] == 0x1f && data[1] == 0x8b
}

// isGzipArchiveFromURL checks if a URL points to a gzip archive by reading the magic bytes
func isGzipArchiveFromURL(url string) bool {
	// Make a GET request to read the first few bytes
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return false
	}

	// Request only the first 512 bytes to check magic bytes
	req.Header.Set("Range", "bytes=0-511")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	// Read the first few bytes
	buf := make([]byte, 2)
	n, err := resp.Body.Read(buf)
	if err != nil || n < 2 {
		return false
	}

	return isGzipArchive(buf)
}
