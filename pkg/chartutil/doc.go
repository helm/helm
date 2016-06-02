/*Package chartutil contains tools for working with charts.

Charts are described in the protocol buffer definition (pkg/proto/hapi/charts).
This packe provides utilities for serializing and deserializing charts.

A chart can be represented on the file system in one of two ways:

	- As a directory that contains a Chart.yaml file and other chart things.
	- As a tarred gzipped file containing a directory that then contains a
	Chart.yaml file.

This package provides utilitites for working with those file formats.

The preferred way of loading a chart is using 'chartutil.Load`:

	chart, err := chartutil.Load(filename)

This will attempt to discover whether the file at 'filename' is a directory or
a chart archive. It will then load accordingly.

For accepting raw compressed tar file data from an io.Reader, the
'chartutil.LoadArchive()' will read in the data, uncompress it, and unpack it
into a Chart.

When creating charts in memory, use the 'github.com/kubernetes/helm/pkg/proto/happy/chart'
package directly.
*/
package chartutil
