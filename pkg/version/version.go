/*
Copyright 2016 The Kubernetes Authors All rights reserved.

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

// Package version represents the current version of the project.
package version // import "k8s.io/helm/pkg/version"

// Version is the current version of the Helm.
// Update this whenever making a new release.
// The version is of the format Major.Minor.Patch
// Increment major number for new feature additions and behavioral changes.
// Increment minor number for bug fixes and performance enhancements.
// Increment patch number for critical fixes to existing releases.
var Version = "v2.0.0-alpha.3"
