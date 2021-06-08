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

package registry // import "helm.sh/helm/v3/internal/experimental/registry"

import (
	"fmt"

	"github.com/oras-project/oras-go/pkg/auth"
)

// Login logs into a registry
func (c *Client) Login(host string, options ...LoginOption) (*LoginResult, error) {
	operation := &loginOperation{}
	for _, option := range options {
		option(operation)
	}
	authorizerLoginOpts := []auth.LoginOption{
		auth.WithLoginContext(ctx(c.out, c.debug)),
		auth.WithLoginHostname(host),
		auth.WithLoginUsername(operation.username),
		auth.WithLoginSecret(operation.password),
		auth.WithLoginUserAgent(userAgent),
	}
	if operation.insecure {
		authorizerLoginOpts = append(authorizerLoginOpts, auth.WithLoginInsecure())
	}
	err := c.authorizer.LoginWithOpts(authorizerLoginOpts...)
	if err != nil {
		return nil, err
	}
	result := &LoginResult{
		Host: host,
	}
	fmt.Fprintln(c.out, "Login Succeeded")
	return result, nil
}
