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

// Package artifact provides unified downloading and verification for Helm artifacts.
//
// This package implements a generic artifact downloader that works with charts,
// plugins, and future artifact types. It uses the transport package for
// protocol abstraction and provides unified verification via provenance files.
//
// The core Downloader type handles:
// - Repository resolution (reponame/artifact → URL)
// - Protocol abstraction via transports
// - Content-addressable caching
// - Provenance file verification
// - Artifact naming conventions
//
// Type-specific convenience wrappers (ChartDownloader, PluginDownloader) provide
// backward-compatible APIs while using the unified implementation.
package downloader
