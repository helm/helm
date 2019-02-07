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

package registry

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReference(t *testing.T) {
	is := assert.New(t)

	// bad ref
	s := ""
	_, err := ParseReference(s)
	is.Error(err)

	// good refs
	s = "localhost:5000/mychart:latest"
	ref, err := ParseReference(s)
	is.NoError(err)
	is.Equal("localhost:5000", ref.Hostname())
	is.Equal("mychart", ref.Repo())
	is.Equal("localhost:5000/mychart", ref.Locator)
	is.Equal("latest", ref.Object)

	s = "my.host.com/my/nested/repo:1.2.3"
	ref, err = ParseReference(s)
	is.NoError(err)
	is.Equal("my.host.com", ref.Hostname())
	is.Equal("my/nested/repo", ref.Repo())
	is.Equal("my.host.com/my/nested/repo", ref.Locator)
	is.Equal("1.2.3", ref.Object)
}
