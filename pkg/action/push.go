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
	"strings"

	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/pusher"
	"helm.sh/helm/v3/pkg/registry"
	"helm.sh/helm/v3/pkg/uploader"
)

// Push is the action for uploading a chart.
//
// It provides the implementation of 'helm push'.
type Push struct {
	Settings *cli.EnvSettings
	cfg      *Configuration
}

// PushOpt is a type of function that sets options for a push action.
type PushOpt func(*Push)

// WithPushConfig sets the cfg field on the push configuration object.
func WithPushConfig(cfg *Configuration) PushOpt {
	return func(p *Push) {
		p.cfg = cfg
	}
}

// NewPushWithOpts creates a new push, with configuration options.
func NewPushWithOpts(opts ...PushOpt) *Push {
	p := &Push{}
	for _, fn := range opts {
		fn(p)
	}
	return p
}

// Run executes 'helm push' against the given chart archive.
func (p *Push) Run(chartRef string, remote string) (string, error) {
	var out strings.Builder

	c := uploader.ChartUploader{
		Out:     &out,
		Pushers: pusher.All(p.Settings),
		Options: []pusher.Option{},
	}

	if registry.IsOCI(remote) {
		c.Options = append(c.Options, pusher.WithRegistryClient(p.cfg.RegistryClient))
	}

	return out.String(), c.UploadTo(chartRef, remote)
}
