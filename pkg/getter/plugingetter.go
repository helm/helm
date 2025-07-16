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
	"os"

	"helm.sh/helm/v4/internal/plugins"
	pluginloader "helm.sh/helm/v4/internal/plugins/loader"
	"helm.sh/helm/v4/internal/plugins/schema"
	"helm.sh/helm/v4/pkg/cli"
)

// collectDownloaderPlugins scans for getter plugins.
// This will load plugins according to the cli.
func collectDownloaderPlugins(settings *cli.EnvSettings) (Providers, error) {

	d := plugins.PluginDescriptor{
		TypeVersion: "getter/v1",
	}

	plgs, err := pluginloader.FindPlugins([]string{settings.PluginsDirectory}, d)
	if err != nil {
		return nil, err
	}

	pluginConstructorBuilder := func(plg plugins.Plugin) Constructor {
		return func(option ...Option) (Getter, error) {

			return &getterPlugin{
				options: append([]Option{}, option...),
				plg:     plg,
			}, nil
		}
	}

	results := make([]Provider, 0, len(plgs))

	for _, plg := range plgs {

		downloaderSchemes, ok := (plg.Manifest().Config["downloader_schemes"]).([]string)
		if !ok {
			return nil, fmt.Errorf("plugin %q does not have downloader_schemes defined", plg.Manifest().Name)
		}

		results = append(results, Provider{
			Schemes: downloaderSchemes,
			New:     pluginConstructorBuilder(plg),
		})
	}
	return results, nil
}

func convertOptions(globalOptions, options []Option) (schema.GetterOptionsV1, error) {
	opts := getterOptions{}
	for _, opt := range globalOptions {
		opt(&opts)
	}
	for _, opt := range options {
		opt(&opts)
	}

	result := schema.GetterOptionsV1{
		URL: opts.url,
		// CertFile              string
		// KeyFile               string
		// CAFile                string
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

	if opts.caFile != "" {
		caData, err := os.ReadFile(opts.caFile)
		if err != nil {
			return schema.GetterOptionsV1{}, fmt.Errorf("unable to read CA file: %q: %w", opts.caFile, err)
		}
		result.CA = caData
	}

	if opts.certFile != "" || opts.keyFile != "" {

		certData, err := os.ReadFile(opts.certFile)
		if err != nil {
			return schema.GetterOptionsV1{}, fmt.Errorf("unable to read cert file: %q: %w", opts.certFile, err)
		}

		keyData, err := os.ReadFile(opts.keyFile)
		if err != nil {
			return schema.GetterOptionsV1{}, fmt.Errorf("unable to read key file: %q: %w", opts.keyFile, err)
		}

		result.Cert = certData
		result.Key = keyData
	}

	return result, nil
}

type getterPlugin struct {
	options []Option
	plg     plugins.Plugin
}

func (g *getterPlugin) Get(url string, options ...Option) (*bytes.Buffer, error) {

	opts, err := convertOptions(g.options, options)
	if err != nil {
		return nil, err
	}

	input := &plugins.Input{
		Message: schema.GetterInputV1{
			URL:     url,
			Options: opts,
		},
	}
	output, err := g.plg.Invoke(context.Background(), input)
	if err != nil {
		return nil, fmt.Errorf("plugin %q failed to invoke: %w", g.plg, err)
	}

	outputMessage, ok := output.Message.(schema.GetterOutputV1)
	if !ok {
		return nil, fmt.Errorf("invalid output message type from plugin %q", g.plg.Manifest().Name)
	}

	return outputMessage.Data, nil
}
