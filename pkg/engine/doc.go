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

/*Package engine implements the Go text template engine as needed for Helm.

When Helm renders templates it does so with additional functions and different
modes (e.g., strict, lint mode). This package handles the helm specific
implementation.
*/
package engine // import "helm.sh/helm/v3/pkg/engine"
