//go:build !windows

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

package cmd

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"

	"helm.sh/helm/v4/pkg/chart"
	"helm.sh/helm/v4/pkg/release"
)

// helm install --verify must install the chart it verified. The chart path
// here is a FIFO that serves the signed chart on its first read and a
// different chart on every later read, so a second read of the path would
// install the wrong chart.
func TestInstallVerifiedChartIsReadOnce(t *testing.T) {
	signed, err := os.ReadFile("testdata/testcharts/signtest-0.1.0.tgz")
	require.NoError(t, err)
	other, err := os.ReadFile("testdata/testcharts/compressedchart-0.1.0.tgz")
	require.NoError(t, err)
	prov, err := os.ReadFile("testdata/testcharts/signtest-0.1.0.tgz.prov")
	require.NoError(t, err)

	dir := t.TempDir()
	fifo := filepath.Join(dir, "signtest-0.1.0.tgz")
	require.NoError(t, syscall.Mkfifo(fifo, 0o600))
	require.NoError(t, os.WriteFile(fifo+".prov", prov, 0o644))

	done := make(chan struct{})
	go func() {
		for n := 0; ; n++ {
			// Blocks until the next reader opens the FIFO.
			w, err := os.OpenFile(fifo, os.O_WRONLY, 0)
			if err != nil {
				return
			}
			body := other
			if n == 0 {
				body = signed
			}
			_, _ = w.Write(body)
			w.Close()
			select {
			case <-done:
				return
			default:
			}
		}
	}()
	t.Cleanup(func() {
		close(done)
		// Release the writer if it is waiting for another reader.
		if r, err := os.OpenFile(fifo, os.O_RDONLY|syscall.O_NONBLOCK, 0); err == nil {
			r.Close()
		}
	})

	store := storageFixture()
	_, _, err = executeActionCommandC(store, "install signtest "+fifo+" --verify --keyring testdata/helm-test-key.pub")
	require.NoError(t, err)

	rel, err := store.Last("signtest")
	require.NoError(t, err)
	rac, err := release.NewAccessor(rel)
	require.NoError(t, err)
	cac, err := chart.NewAccessor(rac.Chart())
	require.NoError(t, err)
	require.Equal(t, "signtest", cac.Name())
}
