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

package fake

import (
	"io"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/cli-runtime/pkg/resource"

	"helm.sh/helm/pkg/kube"
)

// PrintingKubeClient implements KubeClient, but simply prints the reader to
// the given output.
type PrintingKubeClient struct {
	Out io.Writer
}

// Create prints the values of what would be created with a real KubeClient.
func (p *PrintingKubeClient) Create(r io.Reader) error {
	_, err := io.Copy(p.Out, r)
	return err
}

func (p *PrintingKubeClient) Wait(r io.Reader, _ time.Duration) error {
	_, err := io.Copy(p.Out, r)
	return err
}

// Get prints the values of what would be created with a real KubeClient.
func (p *PrintingKubeClient) Get(r io.Reader) (string, error) {
	_, err := io.Copy(p.Out, r)
	return "", err
}

// Delete implements KubeClient delete.
//
// It only prints out the content to be deleted.
func (p *PrintingKubeClient) Delete(r io.Reader) error {
	_, err := io.Copy(p.Out, r)
	return err
}

// WatchUntilReady implements KubeClient WatchUntilReady.
func (p *PrintingKubeClient) WatchUntilReady(r io.Reader, _ time.Duration) error {
	_, err := io.Copy(p.Out, r)
	return err
}

// Update implements KubeClient Update.
func (p *PrintingKubeClient) Update(_, modifiedReader io.Reader, _, _ bool) error {
	_, err := io.Copy(p.Out, modifiedReader)
	return err
}

// Build implements KubeClient Build.
func (p *PrintingKubeClient) Build(_ io.Reader) (kube.Result, error) {
	return []*resource.Info{}, nil
}

func (p *PrintingKubeClient) BuildUnstructured(_ io.Reader) (kube.Result, error) {
	return p.Build(nil)
}

// WaitAndGetCompletedPodPhase implements KubeClient WaitAndGetCompletedPodPhase.
func (p *PrintingKubeClient) WaitAndGetCompletedPodPhase(_ string, _ time.Duration) (v1.PodPhase, error) {
	return v1.PodSucceeded, nil
}
