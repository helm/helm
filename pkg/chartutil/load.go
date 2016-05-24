package chartutil

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/golang/protobuf/ptypes/any"
	"github.com/kubernetes/helm/pkg/proto/hapi/chart"
)

// Load takes a string name, tries to resolve it to a file or directory, and then loads it.
//
// This is the preferred way to load a chart. It will discover the chart encoding
// and hand off to the appropriate chart reader.
func Load(name string) (*chart.Chart, error) {
	fi, err := os.Stat(name)
	if err != nil {
		return nil, err
	}
	if fi.IsDir() {
		return LoadDir(name)
	}
	return LoadFile(name)
}

// subchart is an intermediate representation of a dependency.
//
// It is used to temporarily store a dependency while we process the outer
// file.
type subchart []*afile

func newSubchart() subchart {
	return []*afile{}
}

func (s subchart) add(name string, data []byte, arch bool) subchart {
	s = append(s, &afile{name, data, arch})
	return s
}

// afile represents an archive file buffered for later processing.
type afile struct {
	name    string
	data    []byte
	archive bool
}

// LoadArchive loads from a reader containing a compressed tar archive.
func LoadArchive(in io.Reader) (*chart.Chart, error) {
	sc := map[string]subchart{}
	unzipped, err := gzip.NewReader(in)
	if err != nil {
		return nil, err
	}
	defer unzipped.Close()

	c := &chart.Chart{}
	b := bytes.NewBuffer(nil)

	tr := tar.NewReader(unzipped)
	for {
		hd, err := tr.Next()
		if err == io.EOF {
			// We're done with the reader. Now add subcharts and exit.
			e := addSubcharts(c, sc)
			return c, e
		}
		if err != nil {
			return c, err
		}

		if hd.FileInfo().IsDir() {
			// Use this instead of hd.Typeflag because we don't have to do any
			// inference chasing.
			continue
		}

		parts := strings.Split(hd.Name, "/")
		n := strings.Join(parts[1:], "/")

		if _, err := io.Copy(b, tr); err != nil {
			return c, err
		}

		if strings.HasPrefix(n, "charts/") {
			// If there are subcharts, we put those into a temporary holding
			// array for later processing.
			fmt.Printf("Appending %s to chart %s:\n%s\n", n, c.Metadata.Name, b.String())
			appendSubchart(sc, n, b.Bytes())
			b.Reset()
			continue
		}

		addToChart(c, n, b.Bytes())
		b.Reset()
	}

}

// LoadFile loads from an archive file.
func LoadFile(name string) (*chart.Chart, error) {
	if fi, err := os.Stat(name); err != nil {
		return nil, err
	} else if fi.IsDir() {
		return nil, errors.New("cannot load a directory")
	}

	raw, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer raw.Close()

	return LoadArchive(raw)
}

// LoadDir loads from a directory.
//
// This loads charts only from directories.
func LoadDir(dir string) (*chart.Chart, error) {
	topdir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}

	topdir += string(filepath.Separator)
	sc := map[string]subchart{}
	c := &chart.Chart{}
	err = filepath.Walk(topdir, func(name string, fi os.FileInfo, err error) error {
		n := strings.TrimPrefix(name, topdir)
		if err != nil {
			return err
		}
		if fi.IsDir() {
			return nil
		}

		data, err := ioutil.ReadFile(name)
		if err != nil {
			return fmt.Errorf("error reading %s: %s", n, err)
		}

		if strings.HasPrefix(n, "charts/") {
			appendSubchart(sc, n, data)
			return nil
		}

		return addToChart(c, n, data)
	})
	if err != nil {
		return c, err
	}

	// Ensure that we had a Chart.yaml file
	if c.Metadata == nil || c.Metadata.Name == "" {
		return c, errors.New("chart metadata (Chart.yaml) missing")
	}

	err = addSubcharts(c, sc)
	return c, err
}

func addToChart(c *chart.Chart, n string, data []byte) error {
	fmt.Printf("--> Scanning %s\n", n)
	if n == "Chart.yaml" {
		md, err := UnmarshalChartfile(data)
		if err != nil {
			return err
		}
		if md.Name == "" {
			fmt.Printf("Chart:\n%s\n", string(data))
		}
		fmt.Printf("--> Adding %s as Chart.yaml\n", md.Name)
		c.Metadata = md
	} else if n == "values.toml" {
		c.Values = &chart.Config{Raw: string(data)}
		fmt.Printf("--> Adding to values:\n%s\n", string(data))
	} else if strings.HasPrefix(n, "charts/") {
		// SKIP THESE. These are handled elsewhere, because they need context
		// to process.
		return nil
	} else if strings.HasPrefix(n, "templates/") {
		c.Templates = append(c.Templates, &chart.Template{Name: n, Data: data})
	} else {
		c.Files = append(c.Files, &any.Any{TypeUrl: n, Value: data})
	}
	return nil
}

func addSubcharts(c *chart.Chart, s map[string]subchart) error {
	for n, sc := range s {
		fmt.Printf("===> Unpacking %s\n", n)
		if err := addSubchart(c, sc); err != nil {
			return fmt.Errorf("error adding %q: %s", n, err)
		}
	}
	return nil
}

// addSubchart transforms a subchart to a new chart, and then embeds it into the given chart.
func addSubchart(c *chart.Chart, sc subchart) error {
	nc := &chart.Chart{}
	deps := map[string]subchart{}

	// The sc paths are all relative to the sc itself.
	for _, sub := range sc {
		if sub.archive {
			b := bytes.NewBuffer(sub.data)
			var err error
			nc, err = LoadArchive(b)
			if err != nil {
				fmt.Printf("Bad data in %s: %q", sub.name, string(sub.data))
				return err
			}
			break
		} else if strings.HasPrefix(sub.name, "charts/") {
			appendSubchart(deps, sub.name, sub.data)
		} else {
			fmt.Printf("Adding %s to subchart in %s\n", sub.name, c.Metadata.Name)
			addToChart(nc, sub.name, sub.data)
		}
	}

	if nc.Metadata == nil || nc.Metadata.Name == "" {
		return errors.New("embedded chart is not well-formed")
	}

	fmt.Printf("Added dependency: %q\n", nc.Metadata.Name)
	c.Dependencies = append(c.Dependencies, nc)
	return nil
}

func appendSubchart(sc map[string]subchart, n string, b []byte) {
	fmt.Printf("Append subchart %s\n", n)
	// TODO: Do we need to filter out 0 byte files?
	// TODO: If this finds a dependency that is a tarball, we need to untar it,
	// and express it as a subchart.
	parts := strings.SplitN(n, "/", 3)
	lp := len(parts)
	switch lp {
	case 2:
		if filepath.Ext(parts[1]) == ".tgz" {
			fmt.Printf("--> Adding archive %s\n", n)
			// Basically, we delay expanding tar files until the last minute,
			// which helps (a little) keep memory usage down.
			bn := strings.TrimSuffix(parts[1], ".tgz")
			cc := newSubchart()
			sc[bn] = cc.add(parts[1], b, true)
			return
		} else {
			// Skip directory entries and non-charts.
			return
		}
	case 3:
		if _, ok := sc[parts[1]]; !ok {
			sc[parts[1]] = newSubchart()
		}
		//fmt.Printf("Adding file %q to %s\n", parts[2], parts[1])
		sc[parts[1]] = sc[parts[1]].add(parts[2], b, false)
		return
	default:
		// Skip 1 or 0.
		return
	}

}
