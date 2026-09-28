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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsRemoteHTTPArchive(t *testing.T) {
	srv := mockArchiveServer()
	defer srv.Close()
	source := srv.URL + "/plugins/fake-plugin-0.0.1.tar.gz"

	assert.False(t, isRemoteHTTPArchive("/not/a/URL"), "Expected non-URL to return false")

	// URLs with valid archive extensions are considered valid archives
	// even if the server is unreachable (optimization to avoid unnecessary HTTP requests)
	assert.True(t, isRemoteHTTPArchive("https://127.0.0.1:123/fake/plugin-1.2.3.tgz"), "URL with .tgz extension should be considered a valid archive")

	// Test with invalid extension and unreachable server
	assert.False(t, isRemoteHTTPArchive("https://127.0.0.1:123/fake/plugin-1.2.3.notanarchive"), "Bad URL without valid extension should not succeed")
	assert.True(t, isRemoteHTTPArchive(source), "Expected %q to be a valid archive URL", source)
	assert.False(t, isRemoteHTTPArchive(source+"-not-an-extension"), "Expected media type match to fail")
}
