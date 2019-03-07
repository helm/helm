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

package tiller

import (
	"k8s.io/helm/pkg/manifest"
)

// SortOrder has been aliased to avoid breaking API
type SortOrder = manifest.SortOrder

var (
	// InstallOrder has been aliased to avoid breaking API
	InstallOrder SortOrder = manifest.InstallOrder

	// UninstallOrder has been aliased to avoid breaking API
	UninstallOrder SortOrder = manifest.UninstallOrder

	// SortByKind has been aliased to avoid breaking API
	SortByKind func([]manifest.Manifest) []manifest.Manifest = manifest.SortByKind
)
