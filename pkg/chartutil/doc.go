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

/*
Package chartutil contains tools for working with charts.

Charts are described in the chart package (pkg/chart).
This package provides utilities for serializing and deserializing charts.

A chart can be represented on the file system in one of two ways:

  - As a directory that contains a Chart.yaml file and other chart things.
  - As a tarred gzipped file containing a directory that then contains a
    Chart.yaml file.

This package provides utilities for working with those file formats.

The preferred way of loading a chart is using 'loader.Load`:

	chart, err := loader.Load(filename)

This will attempt to discover whether the file at 'filename' is a directory or
a chart archive. It will then load accordingly.

For accepting raw compressed tar file data from an io.Reader, the
'loader.LoadArchive()' will read in the data, uncompress it, and unpack it
into a Chart.

When creating charts in memory, use the 'helm.sh/helm/pkg/chart'
package directly.
*/
package chartutil // import "helm.sh/helm/v3/pkg/chartutil"
