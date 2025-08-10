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

import "testing"

func TestIsRemoteHTTPArchive(t *testing.T) {
	srv := mockArchiveServer()
	defer srv.Close()
	source := srv.URL + "/plugins/fake-plugin-0.0.1.tar.gz"

	if isRemoteHTTPArchive("/not/a/URL") {
		t.Errorf("Expected non-URL to return false")
	}

	if isRemoteHTTPArchive("https://127.0.0.1:123/fake/plugin-1.2.3.tgz") {
		t.Errorf("Bad URL should not have succeeded.")
	}

	if !isRemoteHTTPArchive(source) {
		t.Errorf("Expected %q to be a valid archive URL", source)
	}

	if isRemoteHTTPArchive(source + "-not-an-extension") {
		t.Error("Expected media type match to fail")
	}
}
