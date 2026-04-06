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
	"errors"
	"io"

	"helm.sh/helm/v4/pkg/cli"
	"helm.sh/helm/v4/pkg/pusher"
	"helm.sh/helm/v4/pkg/registry"
	"helm.sh/helm/v4/pkg/uploader"
)

// Push is the action for uploading a chart.
//
// It provides the implementation of 'helm push'.
type Push struct {
	Settings              *cli.EnvSettings
	cfg                   *Configuration
	certFile              string
	keyFile               string
	caFile                string
	insecureSkipTLSVerify bool
	plainHTTP             bool
}

// PushOpt is a type of function that sets options for a push action.
type PushOpt func(*Push)

// WithPushConfig sets the cfg field on the push configuration object.
func WithPushConfig(cfg *Configuration) PushOpt {
	return func(p *Push) {
		p.cfg = cfg
	}
}

// WithTLSClientConfig sets the certFile, keyFile, and caFile fields on the push configuration object.
func WithTLSClientConfig(certFile, keyFile, caFile string) PushOpt {
	return func(p *Push) {
		p.certFile = certFile
		p.keyFile = keyFile
		p.caFile = caFile
	}
}

// WithInsecureSkipTLSVerify determines if a TLS Certificate will be checked
func WithInsecureSkipTLSVerify(insecureSkipTLSVerify bool) PushOpt {
	return func(p *Push) {
		p.insecureSkipTLSVerify = insecureSkipTLSVerify
	}
}

// WithPlainHTTP configures the use of plain HTTP connections.
func WithPlainHTTP(plainHTTP bool) PushOpt {
	return func(p *Push) {
		p.plainHTTP = plainHTTP
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

// Run executes 'helm push' against the given chart archive and returns the
// structured push result containing the ref and manifest digest.
//
// Note: the return type changed from (string, error) to (*registry.PushResult, error)
// in Helm v4 as an intentional breaking change, enabling structured access to
// push metadata without text parsing.
func (p *Push) Run(chartRef string, remote string) (*registry.PushResult, error) {
	c := uploader.ChartUploader{
		Out:     io.Discard,
		Pushers: pusher.All(p.Settings),
		Options: []pusher.Option{
			pusher.WithTLSClientConfig(p.certFile, p.keyFile, p.caFile),
			pusher.WithInsecureSkipTLSVerify(p.insecureSkipTLSVerify),
			pusher.WithPlainHTTP(p.plainHTTP),
		},
	}

	if registry.IsOCI(remote) {
		if p.cfg == nil {
			return nil, errors.New("missing action configuration: use WithPushConfig when constructing Push")
		}
		// Don't use the default registry client if tls options are set.
		c.Options = append(c.Options, pusher.WithRegistryClient(p.cfg.RegistryClient))
	}

	return c.UploadTo(chartRef, remote)
}
