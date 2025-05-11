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

// Package cache provides a key generator for vcs urls.
package cache // import "helm.sh/helm/v4/pkg/plugin/cache"

import (
	"net/url"
	"regexp"
	"strings"
)

// Thanks glide!

// scpSyntaxRe matches the SCP-like addresses used to access repos over SSH.
var scpSyntaxRe = regexp.MustCompile(`^([a-zA-Z0-9_]+)@([a-zA-Z0-9._-]+):(.*)$`)

// Key generates a cache key based on a url or scp string. The key is file
// system safe.
func Key(repo string) (string, error) {
	var (
		u   *url.URL
		err error
	)
	if m := scpSyntaxRe.FindStringSubmatch(repo); m != nil {
		// Match SCP-like syntax and convert it to a URL.
		// Eg, "git@github.com:user/repo" becomes
		// "ssh://git@github.com/user/repo".
		u = &url.URL{
			User: url.User(m[1]),
			Host: m[2],
			Path: "/" + m[3],
		}
	} else {
		u, err = url.Parse(repo)
		if err != nil {
			return "", err
		}
	}

	var key strings.Builder
	if u.Scheme != "" {
		key.WriteString(u.Scheme)
		key.WriteString("-")
	}
	if u.User != nil && u.User.Username() != "" {
		key.WriteString(u.User.Username())
		key.WriteString("-")
	}
	key.WriteString(u.Host)
	if u.Path != "" {
		key.WriteString(strings.ReplaceAll(u.Path, "/", "-"))
	}
	return strings.ReplaceAll(key.String(), ":", "-"), nil
}
