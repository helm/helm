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

package action

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/helm/pkg/version"
)

func TestVersionFormatter(t *testing.T) {
	var _ VersionFormatter = DefaultVersion()
	var _ VersionFormatter = ShortVersion()
	var _ VersionFormatter = TemplateVersion("")
}

func TestDefaultVersion(t *testing.T) {
	golden := version.GetBuildInfo()
	is := assert.New(t)
	expect := fmt.Sprintf("%#v", golden)
	result, err := DefaultVersion()(golden)
	is.NoError(err)
	is.Equal(expect, result)
}

func TestShortVersion(t *testing.T) {
	v := version.GetBuildInfo()
	v.GitCommit = "aaaaaaaaaaaaaaaa"
	expect := fmt.Sprintf("%s+g%s", v.Version, v.GitCommit[:7])
	is := assert.New(t)

	got, err := ShortVersion()(v)
	is.NoError(err)
	is.Equal(expect, got)

	// Make sure it handles the case where it can't get commit info
	v.GitCommit = "test"
	expect = fmt.Sprintf("%s+g%s", v.Version, v.GitCommit)
	got, err = ShortVersion()(v)
	is.NoError(err)
	is.Equal(expect, got)
}

func TestTemplateVersion(t *testing.T) {
	v := version.GetBuildInfo()
	is := assert.New(t)
	expect := fmt.Sprintf("foo-%s", v.GitCommit)

	got, err := TemplateVersion("foo-{{.GitCommit}}")(v)
	is.NoError(err)
	is.Equal(expect, got)

	// Test a broken template, too
	_, err = TemplateVersion("foo-{{.GitCommit")(v)
	is.Error(err)
}

func TestVersion(t *testing.T) {
	// This test ensures that the default formatter works when none is specified
	golden := version.GetBuildInfo()
	is := assert.New(t)
	expect := fmt.Sprintf("%#v", golden)

	ver := new(Version)
	got, err := ver.Run()
	is.NoError(err)
	is.Equal(expect, got)
}

func TestVersion_TemplateVersion(t *testing.T) {
	// The purpose of this test is to ensure that we can override the formatter
	v := version.GetBuildInfo()
	is := assert.New(t)
	expect := fmt.Sprintf("foo-%s", v.GitCommit)

	ver := &Version{
		Formatter: TemplateVersion("foo-{{.GitCommit}}"),
	}
	got, err := ver.Run()
	is.NoError(err)
	is.Equal(expect, got)
}
