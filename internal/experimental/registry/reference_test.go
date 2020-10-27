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

func TestParseReference(t *testing.T) {
	is := assert.New(t)

	// bad refs
	s := ""
	_, err := ParseReference(s)
	is.Error(err, "empty ref")

	s = "my:bad:ref"
	_, err = ParseReference(s)
	is.Error(err, "ref contains too many colons (2)")

	s = "my:really:bad:ref"
	_, err = ParseReference(s)
	is.Error(err, "ref contains too many colons (3)")

	// good refs
	s = "mychart"
	ref, err := ParseReference(s)
	is.NoError(err)
	is.Equal("mychart", ref.Repo)
	is.Equal("", ref.Tag)
	is.Equal("mychart", ref.FullName())

	s = "mychart:1.5.0"
	ref, err = ParseReference(s)
	is.NoError(err)
	is.Equal("mychart", ref.Repo)
	is.Equal("1.5.0", ref.Tag)
	is.Equal("mychart:1.5.0", ref.FullName())

	s = "myrepo/mychart"
	ref, err = ParseReference(s)
	is.NoError(err)
	is.Equal("myrepo/mychart", ref.Repo)
	is.Equal("", ref.Tag)
	is.Equal("myrepo/mychart", ref.FullName())

	s = "myrepo/mychart:1.5.0"
	ref, err = ParseReference(s)
	is.NoError(err)
	is.Equal("myrepo/mychart", ref.Repo)
	is.Equal("1.5.0", ref.Tag)
	is.Equal("myrepo/mychart:1.5.0", ref.FullName())

	s = "mychart:5001:1.5.0"
	ref, err = ParseReference(s)
	is.NoError(err)
	is.Equal("mychart:5001", ref.Repo)
	is.Equal("1.5.0", ref.Tag)
	is.Equal("mychart:5001:1.5.0", ref.FullName())

	s = "myrepo:5001/mychart:1.5.0"
	ref, err = ParseReference(s)
	is.NoError(err)
	is.Equal("myrepo:5001/mychart", ref.Repo)
	is.Equal("1.5.0", ref.Tag)
	is.Equal("myrepo:5001/mychart:1.5.0", ref.FullName())

	s = "127.0.0.1:5001/mychart:1.5.0"
	ref, err = ParseReference(s)
	is.NoError(err)
	is.Equal("127.0.0.1:5001/mychart", ref.Repo)
	is.Equal("1.5.0", ref.Tag)
	is.Equal("127.0.0.1:5001/mychart:1.5.0", ref.FullName())

	s = "localhost:5000/mychart:latest"
	ref, err = ParseReference(s)
	is.NoError(err)
	is.Equal("localhost:5000/mychart", ref.Repo)
	is.Equal("latest", ref.Tag)
	is.Equal("localhost:5000/mychart:latest", ref.FullName())

	s = "my.host.com/my/nested/repo:1.2.3"
	ref, err = ParseReference(s)
	is.NoError(err)
	is.Equal("my.host.com/my/nested/repo", ref.Repo)
	is.Equal("1.2.3", ref.Tag)
	is.Equal("my.host.com/my/nested/repo:1.2.3", ref.FullName())

	s = "localhost:5000/x/y/z"
	ref, err = ParseReference(s)
	is.NoError(err)
	is.Equal("localhost:5000/x/y/z", ref.Repo)
	is.Equal("", ref.Tag)
	is.Equal("localhost:5000/x/y/z", ref.FullName())

	s = "localhost:5000/x/y/z:123"
	ref, err = ParseReference(s)
	is.NoError(err)
	is.Equal("localhost:5000/x/y/z", ref.Repo)
	is.Equal("123", ref.Tag)
	is.Equal("localhost:5000/x/y/z:123", ref.FullName())

	s = "localhost:5000/x/y/z:123:x"
	_, err = ParseReference(s)
	is.Error(err, "ref contains too many colons (3)")

	s = "localhost:5000/x/y/z:123:x:y"
	_, err = ParseReference(s)
	is.Error(err, "ref contains too many colons (4)")
}
