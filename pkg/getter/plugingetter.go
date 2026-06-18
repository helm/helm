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

package getter

import (
	"bytes"
	"context"
	"fmt"

	"net/url"

	"helm.sh/helm/v4/internal/plugin"

	"helm.sh/helm/v4/internal/plugin/schema"
	"helm.sh/helm/v4/pkg/cli"
)

// collectGetterPlugins scans for getter plugins.
// This will load plugins according to the cli.
func collectGetterPlugins(settings *cli.EnvSettings) (Providers, error) {
	d := plugin.Descriptor{
		Type: "getter/v1",
	}
	plgs, err := plugin.FindPlugins([]string{settings.PluginsDirectory}, d)
	if err != nil {
		return nil, err
	}
	env := plugin.FormatEnv(settings.EnvVars())
	pluginConstructorBuilder := func(plg plugin.Plugin) Constructor {
		return func(option ...Option) (Getter, error) {

			return &getterPlugin{
				options: append([]Option{}, option...),
				plg:     plg,
				env:     env,
			}, nil
		}
	}
	results := make([]Provider, 0, len(plgs))
	for _, plg := range plgs {
		if c, ok := plg.Metadata().Config.(*schema.ConfigGetterV1); ok {
			results = append(results, Provider{
				Schemes: c.Protocols,
				New:     pluginConstructorBuilder(plg),
			})
		}
	}
	return results, nil
}

func convertOptions(globalOptions, options []Option) schema.GetterOptionsV1 {
	opts := getterOptions{}
	for _, opt := range globalOptions {
		opt(&opts)
	}
	for _, opt := range options {
		opt(&opts)
	}

	result := schema.GetterOptionsV1{
		URL:                   opts.url,
		CertFile:              opts.certFile,
		KeyFile:               opts.keyFile,
		CAFile:                opts.caFile,
		UNTar:                 opts.unTar,
		InsecureSkipVerifyTLS: opts.insecureSkipVerifyTLS,
		PlainHTTP:             opts.plainHTTP,
		AcceptHeader:          opts.acceptHeader,
		Username:              opts.username,
		Password:              opts.password,
		PassCredentialsAll:    opts.passCredentialsAll,
		UserAgent:             opts.userAgent,
		Version:               opts.version,
		Timeout:               opts.timeout,
	}

	return result
}

type getterPlugin struct {
	options []Option
	plg     plugin.Plugin
	env     []string
}

func (g *getterPlugin) Get(href string, options ...Option) (*bytes.Buffer, error) {
	opts := convertOptions(g.options, options)

	// TODO optimization: pass this along to Get() instead of re-parsing here
	u, err := url.Parse(href)
	if err != nil {
		return nil, err
	}

	input := &plugin.Input{
		Message: schema.InputMessageGetterV1{
			Href:     href,
			Options:  opts,
			Protocol: u.Scheme,
		},
		Env: g.env,
		// TODO should we pass Stdin, Stdout, and Stderr through Input here to getter plugins?
		// Stdout: os.Stdout,
	}
	output, err := g.plg.Invoke(context.Background(), input)
	if err != nil {
		return nil, fmt.Errorf("plugin %q failed to invoke: %w", g.plg, err)
	}

	outputMessage, ok := output.Message.(schema.OutputMessageGetterV1)
	if !ok {
		return nil, fmt.Errorf("invalid output message type from plugin %q", g.plg.Metadata().Name)
	}

	return bytes.NewBuffer(outputMessage.Data), nil
}
