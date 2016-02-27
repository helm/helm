/*
Copyright 2015 The Kubernetes Authors All rights reserved.

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

package chart

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/kubernetes/deployment-manager/pkg/log"
)

// Save creates an archived chart to the given directory.
//
// This takes an existing chart and a destination directory.
//
// If the directory is /foo, and the chart is named bar, with version 1.0.0, this
// will generate /foo/bar-1.0.0.tgz.
//
// This returns the absolute path to the chart archive file.
func Save(c *Chart, outDir string) (string, error) {
	// Create archive
	if fi, err := os.Stat(outDir); err != nil {
		return "", err
	} else if !fi.IsDir() {
		return "", fmt.Errorf("location %s is not a directory", outDir)
	}

	cfile := c.Chartfile()
	dir := c.Dir()
	basename := filepath.Base(dir)
	pdir := filepath.Dir(dir)
	if basename == "." {
		basename = fname(cfile.Name)
	}
	filename := fmt.Sprintf("%s-%s.tgz", fname(cfile.Name), cfile.Version)
	filename = filepath.Join(outDir, filename)

	// Fail early if the YAML is borked.
	if err := cfile.Save(filepath.Join(dir, ChartfileName)); err != nil {
		return "", err
	}

	// Create file.
	f, err := os.Create(filename)
	if err != nil {
		return "", err
	}

	// Wrap in gzip writer
	zipper := gzip.NewWriter(f)
	zipper.Header.Extra = headerBytes
	zipper.Header.Comment = "Helm"

	// Wrap in tar writer
	twriter := tar.NewWriter(zipper)
	rollback := false
	defer func() {
		twriter.Close()
		zipper.Close()
		f.Close()
		if rollback {
			log.Warn("Removing incomplete archive %s", filename)
			os.Remove(filename)
		}
	}()

	err = filepath.Walk(dir, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		hdr, err := tar.FileInfoHeader(fi, ".")
		if err != nil {
			return err
		}

		relpath, err := filepath.Rel(pdir, path)
		if err != nil {
			return err
		}
		hdr.Name = relpath

		twriter.WriteHeader(hdr)

		// Skip directories.
		if fi.IsDir() {
			return nil
		}

		in, err := os.Open(path)
		if err != nil {
			return err
		}
		_, err = io.Copy(twriter, in)
		in.Close()
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		rollback = true
		return filename, err
	}
	return filename, nil
}
