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

package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"

	extism "github.com/extism/go-sdk"
	"github.com/tetratelabs/wazero"
)

const ExtismV1WasmBinaryFilename = "plugin.wasm"

// RuntimeConfigExtismV1Memory exposes the Wasm/Extism memory options for the plugin
type RuntimeConfigExtismV1Memory struct {
	// The max amount of pages the plugin can allocate
	// One page is 64Kib. e.g. 16 pages would require 1MiB.
	// Default is 4 pages (256KiB)
	MaxPages uint32 `yaml:"maxPages,omitempty"`

	// The max size of an Extism HTTP response in bytes
	// Default is 4096 bytes (4KiB)
	MaxHTTPResponseBytes int64 `yaml:"maxHttpResponseBytes,omitempty"`

	// The max size of all Extism vars in bytes
	// Default is 4096 bytes (4KiB)
	MaxVarBytes int64 `yaml:"maxVarBytes,omitempty"`
}

// RuntimeConfigExtismV1FileSystem exposes filesystem options for the configuration
// TODO: should Helm expose AllowedPaths?
type RuntimeConfigExtismV1FileSystem struct {
	// If specified, a temporary directory will be created and mapped to /tmp in the plugin's filesystem.
	// Data written to the directory will be visible on the host filesystem.
	// The directory will be removed when the plugin invocation completes.
	CreateTempDir bool `yaml:"createTempDir,omitempty"`
}

// RuntimeConfigExtismV1 defines the user-configurable options the plugin's Extism runtime
// The format loosely follows the Extism Manifest format: https://extism.org/docs/concepts/manifest/
type RuntimeConfigExtismV1 struct {
	// Describes the limits on the memory the plugin may be allocated.
	Memory RuntimeConfigExtismV1Memory `yaml:"memory"`

	// The "config" key is a free-form map that can be passed to the plugin.
	// The plugin must interpret arbitrary data this map may contain
	Config map[string]string `yaml:"config,omitempty"`

	// An optional set of hosts this plugin can communicate with.
	// This only has an effect if the plugin makes HTTP requests.
	// If not specified, then no hosts are allowed.
	AllowedHosts []string `yaml:"allowedHosts,omitempty"`

	FileSystem RuntimeConfigExtismV1FileSystem `yaml:"fileSystem,omitempty"`

	// The timeout in milliseconds for the plugin to execute
	Timeout uint64 `yaml:"timeout,omitempty"`

	// HostFunction names exposed in Helm the plugin may access
	// see: https://extism.org/docs/concepts/host-functions/
	HostFunctions []string `yaml:"hostFunctions,omitempty"`

	// The name of entry function name to call in the plugin
	// Defaults to "helm_plugin_main".
	EntryFuncName string `yaml:"entryFuncName,omitempty"`
}

var _ RuntimeConfig = (*RuntimeConfigExtismV1)(nil)

func (r *RuntimeConfigExtismV1) Validate() error {
	// TODO
	return nil
}

type RuntimeExtismV1 struct {
	HostFunctions    map[string]extism.HostFunction
	CompilationCache wazero.CompilationCache
}

var _ Runtime = (*RuntimeExtismV1)(nil)

func (r *RuntimeExtismV1) CreatePlugin(pluginDir string, metadata *Metadata) (Plugin, error) {

	rc, ok := metadata.RuntimeConfig.(*RuntimeConfigExtismV1)
	if !ok {
		return nil, fmt.Errorf("invalid extism/v1 plugin runtime config type: %T", metadata.RuntimeConfig)
	}

	wasmFile := filepath.Join(pluginDir, ExtismV1WasmBinaryFilename)
	if _, err := os.Stat(wasmFile); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("wasm binary missing for extism/v1 plugin: %q", wasmFile)
		}
		return nil, fmt.Errorf("failed to stat extism/v1 plugin wasm binary %q: %w", wasmFile, err)
	}

	return &ExtismV1PluginRuntime{
		metadata: *metadata,
		dir:      pluginDir,
		rc:       rc,
		r:        r,
	}, nil
}

type ExtismV1PluginRuntime struct {
	metadata Metadata
	dir      string
	rc       *RuntimeConfigExtismV1
	r        *RuntimeExtismV1
}

var _ Plugin = (*ExtismV1PluginRuntime)(nil)

func (p *ExtismV1PluginRuntime) Metadata() Metadata {
	return p.metadata
}

func (p *ExtismV1PluginRuntime) Dir() string {
	return p.dir
}

func (p *ExtismV1PluginRuntime) Invoke(ctx context.Context, input *Input) (*Output, error) {

	var tmpDir string
	if p.rc.FileSystem.CreateTempDir {
		tmpDirInner, err := os.MkdirTemp(os.TempDir(), "helm-plugin-*")
		slog.Debug("created plugin temp dir", slog.String("dir", tmpDirInner), slog.String("plugin", p.metadata.Name))
		if err != nil {
			return nil, fmt.Errorf("failed to create temp dir for extism compilation cache: %w", err)
		}
		defer func() {
			if err := os.RemoveAll(tmpDir); err != nil {
				slog.Warn("failed to remove plugin temp dir", slog.String("dir", tmpDir), slog.String("plugin", p.metadata.Name), slog.String("error", err.Error()))
			}
		}()

		tmpDir = tmpDirInner
	}

	manifest, err := buildManifest(p.dir, tmpDir, p.rc)
	if err != nil {
		return nil, err
	}

	config := buildPluginConfig(input, p.r)

	hostFunctions, err := buildHostFunctions(p.r.HostFunctions, p.rc)
	if err != nil {
		return nil, err
	}

	pe, err := extism.NewPlugin(ctx, manifest, config, hostFunctions)
	if err != nil {
		return nil, fmt.Errorf("failed to create existing plugin: %w", err)
	}

	pe.SetLogger(func(logLevel extism.LogLevel, s string) {
		slog.Debug(s, slog.String("level", logLevel.String()), slog.String("plugin", p.metadata.Name))
	})

	inputData, err := json.Marshal(input.Message)
	if err != nil {
		return nil, fmt.Errorf("failed to json marshal plugin input message: %T: %w", input.Message, err)
	}

	slog.Debug("plugin input", slog.String("plugin", p.metadata.Name), slog.String("inputData", string(inputData)))

	entryFuncName := p.rc.EntryFuncName
	if entryFuncName == "" {
		entryFuncName = "helm_plugin_main"
	}

	exitCode, outputData, err := pe.Call(entryFuncName, inputData)
	if err != nil {
		return nil, fmt.Errorf("plugin error: %w", err)
	}

	if exitCode != 0 {
		return nil, &InvokeExecError{
			ExitCode: int(exitCode),
		}
	}

	slog.Debug("plugin output", slog.String("plugin", p.metadata.Name), slog.Int("exitCode", int(exitCode)), slog.String("outputData", string(outputData)))

	outputMessage := reflect.New(pluginTypesIndex[p.metadata.Type].outputType)
	if err := json.Unmarshal(outputData, outputMessage.Interface()); err != nil {
		return nil, fmt.Errorf("failed to json marshal plugin output message: %T: %w", outputMessage, err)
	}

	output := &Output{
		Message: outputMessage.Elem().Interface(),
	}

	return output, nil
}

func buildManifest(pluginDir string, tmpDir string, rc *RuntimeConfigExtismV1) (extism.Manifest, error) {
	wasmFile := filepath.Join(pluginDir, ExtismV1WasmBinaryFilename)

	allowedHosts := rc.AllowedHosts
	if allowedHosts == nil {
		allowedHosts = []string{}
	}

	allowedPaths := map[string]string{}
	if tmpDir != "" {
		allowedPaths[tmpDir] = "/tmp"
	}

	return extism.Manifest{
		Wasm: []extism.Wasm{
			extism.WasmFile{
				Path: wasmFile,
				Name: wasmFile,
			},
		},
		Memory: &extism.ManifestMemory{
			MaxPages:             rc.Memory.MaxPages,
			MaxHttpResponseBytes: rc.Memory.MaxHTTPResponseBytes,
			MaxVarBytes:          rc.Memory.MaxVarBytes,
		},
		Config:       rc.Config,
		AllowedHosts: allowedHosts,
		AllowedPaths: allowedPaths,
		Timeout:      rc.Timeout,
	}, nil
}

func buildPluginConfig(input *Input, r *RuntimeExtismV1) extism.PluginConfig {
	mc := wazero.NewModuleConfig().
		WithSysWalltime()
	if input.Stdin != nil {
		mc = mc.WithStdin(input.Stdin)
	}
	if input.Stdout != nil {
		mc = mc.WithStdout(input.Stdout)
	}
	if input.Stderr != nil {
		mc = mc.WithStderr(input.Stderr)
	}
	if len(input.Env) > 0 {
		env := ParseEnv(input.Env)
		for k, v := range env {
			mc = mc.WithEnv(k, v)
		}
	}

	config := extism.PluginConfig{
		ModuleConfig: mc,
		RuntimeConfig: wazero.NewRuntimeConfigCompiler().
			WithCloseOnContextDone(true).
			WithCompilationCache(r.CompilationCache),
		EnableWasi:                true,
		EnableHttpResponseHeaders: true,
	}

	return config
}

func buildHostFunctions(hostFunctions map[string]extism.HostFunction, rc *RuntimeConfigExtismV1) ([]extism.HostFunction, error) {
	result := make([]extism.HostFunction, len(rc.HostFunctions))
	for _, fnName := range rc.HostFunctions {
		fn, ok := hostFunctions[fnName]
		if !ok {
			return nil, fmt.Errorf("plugin requested host function %q not found", fnName)
		}

		result = append(result, fn)
	}

	return result, nil
}
