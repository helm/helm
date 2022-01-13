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

package uploader

import (
	"fmt"
	"io"
	"net/url"

	"github.com/pkg/errors"

	"helm.sh/helm/v3/pkg/pusher"
	"helm.sh/helm/v3/pkg/registry"
)

// ChartUploader handles uploading a chart.
type ChartUploader struct {
	// Out is the location to write warning and info messages.
	Out io.Writer
	// Pusher collection for the operation
	Pushers pusher.Providers
	// Options provide parameters to be passed along to the Pusher being initialized.
	Options []pusher.Option
	// RegistryClient is a client for interacting with registries.
	RegistryClient *registry.Client
}

// UploadTo uploads a chart. Depending on the settings, it may also upload a provenance file.
func (c *ChartUploader) UploadTo(ref, remote string) error {
	u, err := url.Parse(remote)
	if err != nil {
		return errors.Errorf("invalid chart URL format: %s", remote)
	}

	if u.Scheme == "" {
		return errors.New(fmt.Sprintf("scheme prefix missing from remote (e.g. \"%s://\")", registry.OCIScheme))
	}

	p, err := c.Pushers.ByScheme(u.Scheme)
	if err != nil {
		return err
	}

	return p.Push(ref, u.String(), c.Options...)
}
